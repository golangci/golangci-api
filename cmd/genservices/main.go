package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/types"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/pkg/errors"
	"golang.org/x/tools/go/loader"
)

const endpointTmpl = `// Code generated by genservices. DO NOT EDIT.
package {{.PkgName}}
{{range .ServiceMethods}}
type {{.Name}}Request struct {
{{.RequestDef}}
}

type {{.Name}}Response struct {
{{.ResponseDef}}
}

func make{{.Name}}Endpoint(svc Service, log logutil.Log) endpoint.Endpoint {
	return func(ctx context.Context, reqObj interface{}) (resp interface{}, err error) {
		{{if .HasRequestParams}}
		req := reqObj.({{.Name}}Request)
		{{end}}

		reqLogger := log
		defer func() {
			if rerr := recover(); rerr != nil {
				reqLogger.Errorf("Panic occured")
				reqLogger.Infof("%s", debug.Stack())
				resp = {{.Name}}Response{
					err: errors.New("panic occured"),
				}
				err = nil
			}
		}()

		if err := endpointutil.Error(ctx); err != nil {
			log.Warnf("Error occurred during request context creation: %s", err)
			resp = {{.Name}}Response{
				err: err,
			}
			return resp, nil
		}

		rc := endpointutil.RequestContext(ctx).(*request.{{if .Authorized}}Authorized{{else}}Anonymous{{end}}Context)
		reqLogger = rc.Log

		{{range .ArgsToFillLctx}}{{.}}.FillLogContext(rc.Lctx)
		{{end}}

		{{if .HasRetVal}}
			v, err := svc.{{.Name}}({{.CallArgs}})
			if err != nil {
				rc.Log.Errorf("{{.FullName}} failed: %s", err)
				return {{.Name}}Response{err, v}, nil
			}

			return {{.Name}}Response{nil, v}, nil
		{{else}}
			err = svc.{{.Name}}({{.CallArgs}})
			if err != nil {
				if !apierrors.IsErrorLikeResult(err) {
					rc.Log.Errorf("{{.FullName}} failed: %s", err)
				}
				return {{.Name}}Response{err}, nil
			}

			return {{.Name}}Response{nil}, nil
		{{end}}
	}
}
{{end}}
`

const transportTmpl = `// Code generated by genservices. DO NOT EDIT.
package {{.PkgName}}

import httptransport "github.com/go-kit/kit/transport/http"

func RegisterHandlers(svc Service, regCtx *transportutil.HandlerRegContext) {
	{{range .ServiceMethods}}
		h{{.Name}} := httptransport.NewServer(
			make{{.Name}}Endpoint(svc, regCtx.Log),
			decode{{.Name}}Request,
			encode{{.Name}}Response,
			httptransport.ServerBefore(transportutil.StoreHTTPRequestToContext),
			httptransport.ServerAfter(transportutil.FinalizeSession),
			{{if .Authorized}}
			httptransport.ServerBefore(transportutil.MakeStoreAuthorizedRequestContext(regCtx.Log,
				regCtx.ErrTracker, regCtx.DB, regCtx.AuthSessFactory)),
			{{else}}
			httptransport.ServerBefore(transportutil.MakeStoreAnonymousRequestContext(
				regCtx.Log, regCtx.ErrTracker, regCtx.DB)),
			{{end}}
			httptransport.ServerFinalizer(transportutil.FinalizeRequest),
			httptransport.ServerErrorEncoder(transportutil.EncodeError),
			httptransport.ServerErrorLogger(transportutil.AdaptErrorLogger(regCtx.Log)),
		)
		regCtx.Router.Methods("{{.HTTPMethod}}").Path("{{.URL}}").Handler(h{{.Name}})

	{{end}}
}

{{range .ServiceMethods}}
func decode{{.Name}}Request(_ context.Context, r *http.Request) (interface{}, error) {
	var request {{.Name}}Request;
	if err := transportutil.DecodeRequest(&request, r); err != nil {
		return nil, errors.Wrap(err, "can't decode request")
	}

	return request, nil
}

func encode{{.Name}}Response(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	w.Header().Add("Content-Type", "application/json; charset=UTF-8")
	if err := transportutil.GetContextError(ctx); err != nil {
		wrappedResp := struct {
			Error *transportutil.Error
		}{
			Error: transportutil.MakeError(err),
		}
		w.WriteHeader(wrappedResp.Error.HTTPCode)
		return json.NewEncoder(w).Encode(wrappedResp)
	}

	resp := response.({{.Name}}Response)
	wrappedResp := struct {
		transportutil.ErrorResponse
		{{.Name}}Response
	}{
		{{.Name}}Response: resp,
	}

	if resp.err != nil {
		if apierrors.IsErrorLikeResult(resp.err) {
			return transportutil.HandleErrorLikeResult(ctx, w, resp.err)
		}

		terr := transportutil.MakeError(resp.err)
		wrappedResp.Error = terr
		w.WriteHeader(terr.HTTPCode)
	}

	return json.NewEncoder(w).Encode(wrappedResp)
}
{{end}}
`

func main() {
	root := flag.String("root", "pkg/app/services", "root of services")
	flag.Parse()

	if err := generate(*root); err != nil {
		log.Fatalf("can't generate on root %s: %s", *root, err)
	}
}

func generate(root string) error {
	rootDir, err := ioutil.ReadDir(root)
	if err != nil {
		return errors.Wrap(err, "can't read root dir")
	}

	var serviceDirs []string
	for _, fi := range rootDir {
		if fi.IsDir() {
			serviceDirs = append(serviceDirs, filepath.Join(root, fi.Name()))
		}
	}

	for _, sd := range serviceDirs {
		sg := serviceGenerator{}
		if err = sg.Generate(sd); err != nil {
			return errors.Wrapf(err, "can't generate for service %s", sd)
		}
	}

	return nil
}

type serviceGenerator struct {
	f                *ast.File
	serviceInterface *ast.InterfaceType
	pkg              *loader.PackageInfo
}

func (sg serviceGenerator) Generate(serviceRoot string) error {
	if err := sg.Load(serviceRoot); err != nil {
		return errors.Wrap(err, "can't load")
	}

	if err := sg.findServiceInterface(); err != nil {
		return err
	}

	if sg.serviceInterface.Methods.NumFields() == 0 {
		return errors.New("empty Service interface")
	}

	var methodsCtxs []map[string]interface{}
	for _, method := range sg.serviceInterface.Methods.List {
		fn, ok := method.Type.(*ast.FuncType)
		if !ok {
			return fmt.Errorf("unknown service method type %#v", method.Type)
		}

		ctx, err := sg.generateForMethod(fn, method)
		if err != nil {
			return errors.Wrapf(err, "can't generate for method %s", method.Names[0])
		}
		methodsCtxs = append(methodsCtxs, ctx)
	}

	tmplCtx := map[string]interface{}{
		"ServiceMethods": methodsCtxs,
		"PkgName":        sg.pkg.Pkg.Name(),
	}

	if err := buildGoFile(endpointTmpl, tmplCtx, filepath.Join(serviceRoot, "endpoint.go")); err != nil {
		return errors.Wrap(err, "can't build endpoint.go")
	}

	if err := buildGoFile(transportTmpl, tmplCtx, filepath.Join(serviceRoot, "transport.go")); err != nil {
		return errors.Wrap(err, "can't build transport.go")
	}

	return nil
}

func buildGoFile(tmpl string, tmplCtx map[string]interface{}, outFile string) error {
	f, err := os.Create(outFile)
	if err != nil {
		return errors.Wrapf(err, "can't create %s", outFile)
	}
	defer f.Close()

	et := template.Must(template.New(outFile).Parse(tmpl))
	if err := et.Execute(f, tmplCtx); err != nil {
		return errors.Wrap(err, "can't execute template")
	}

	if err := exec.Command("goimports", "-w", outFile).Run(); err != nil {
		return errors.Wrap(err, "can't run goimports")
	}

	return nil
}

func (sg *serviceGenerator) Load(serviceRoot string) error {
	defFile := filepath.Join(serviceRoot, "service.go")
	conf := loader.Config{
		ParserMode:          parser.ParseComments,
		TypeCheckFuncBodies: func(path string) bool { return false },
	}
	conf.CreateFromFilenames(serviceRoot, defFile)

	prog, err := conf.Load()
	if err != nil {
		return errors.Wrapf(err, "can't load program from package %q",
			serviceRoot)
	}

	sg.pkg = prog.Created[0]
	f := sg.pkg.Files[0]
	sg.f = f
	return nil
}

func (sg *serviceGenerator) findServiceInterface() error {
	v := serviceInterfaceFinder{}
	ast.Walk(&v, sg.f)
	if v.serviceType == nil {
		return errors.New("can't find Service interface")
	}

	sg.serviceInterface = v.serviceType
	return nil
}

type serviceInterfaceFinder struct {
	curGenDecl  *ast.GenDecl
	serviceType *ast.InterfaceType
}

func (v *serviceInterfaceFinder) Visit(n ast.Node) (w ast.Visitor) {
	switch n := n.(type) {
	case *ast.GenDecl:
		v.curGenDecl = n
	case *ast.TypeSpec:
		if ti, ok := n.Type.(*ast.InterfaceType); ok && n.Name.Name == "Service" {
			v.serviceType = ti
			return nil
		}
	}

	return v
}

func checkIsIdentWithName(e ast.Expr, name string) error {
	ei, ok := e.(*ast.Ident)
	if !ok {
		return fmt.Errorf("expected identifier with name %s, found %#v", name, e)
	}

	if ei.Name != name {
		return fmt.Errorf("expected identfier with name %s, found %s", name, ei.Name)
	}

	return nil
}

func uppercaseFirstLetter(s string) string {
	if len(s) == 1 {
		return strings.ToUpper(s)
	}

	return strings.ToUpper(string(s[0])) + s[1:]
}

func (sg *serviceGenerator) genDefinitionFromFields(fields []*ast.Field) []string {
	defElems := []string{}
	for _, f := range fields {
		fDefType := sg.pkg.TypeOf(f.Type)
		fPrintType := types.TypeString(fDefType, func(pkg *types.Package) string {
			if pkg == sg.pkg.Pkg {
				return ""
			}
			return pkg.Name()
		})
		var elem string
		if len(f.Names) != 0 {
			elem = fmt.Sprintf("\t%s %s", uppercaseFirstLetter(f.Names[0].String()), fPrintType)
		} else {
			elem = fmt.Sprintf("\t%s", fPrintType)
		}
		defElems = append(defElems, elem)
	}

	return defElems
}

func parseServiceMethodComment(doc string) map[string]string {
	ret := map[string]string{}

	doc = strings.TrimSpace(doc)
	parts := strings.Split(doc, " ")
	for _, p := range parts {
		kv := strings.Split(p, ":")
		if len(kv) != 2 {
			continue
		}
		ret[kv[0]] = kv[1]
	}

	return ret
}

func (sg *serviceGenerator) generateForMethod(fn *ast.FuncType, method *ast.Field) (map[string]interface{}, error) {
	isAuthorized, err := sg.checkMethod(fn)
	if err != nil {
		return nil, err
	}

	if method.Doc == nil {
		return nil, errors.New("no doc for method")
	}
	methodMetadata := parseServiceMethodComment(method.Doc.Text())
	url := methodMetadata["url"]
	if url == "" {
		return nil, fmt.Errorf("invalid method doc without url: %s", method.Doc.Text())
	}
	httpMethod := methodMetadata["method"]
	if httpMethod == "" {
		httpMethod = "GET"
	}

	reqDefElems := []string{}
	reqDefElems = append(reqDefElems, sg.genDefinitionFromFields(fn.Params.List[1:])...)

	respDefElems := []string{
		"\terr error",
	}
	if len(fn.Results.List) > 1 {
		elems := sg.genDefinitionFromFields(fn.Results.List[:len(fn.Results.List)-1])
		respDefElems = append(respDefElems, elems...)
	}

	callArgs := []string{"rc"}
	for _, arg := range fn.Params.List[1:] {
		callArgs = append(callArgs, "req."+uppercaseFirstLetter(arg.Names[0].Name))
	}

	var argsToFillLctx []string
	for _, arg := range fn.Params.List[1:] {
		argsToFillLctx = append(argsToFillLctx, "req."+uppercaseFirstLetter(arg.Names[0].Name))
	}

	ctx := map[string]interface{}{
		"Name":             method.Names[0].Name,
		"FullName":         fmt.Sprintf("%s.Service.%s", sg.pkg.Pkg.Name(), method.Names[0].Name),
		"RequestDef":       strings.Join(reqDefElems, "\n"),
		"ResponseDef":      strings.Join(respDefElems, "\n"),
		"URL":              url,
		"HTTPMethod":       httpMethod,
		"CallArgs":         strings.Join(callArgs, ", "),
		"HasRequestParams": fn.Params.NumFields() > 1,
		"ArgsToFillLctx":   argsToFillLctx,
		"HasRetVal":        fn.Results.NumFields() == 2, // value and error
		"Authorized":       isAuthorized,
	}
	return ctx, nil
}

//nolint:gocyclo
func (sg serviceGenerator) checkMethod(fn *ast.FuncType) (bool, error) {
	res := fn.Results
	if res.NumFields() == 0 || res.NumFields() > 2 {
		return false, fmt.Errorf("unsupported return values count: %d", res.NumFields())
	}

	lastRet := res.List[res.NumFields()-1]
	if err := checkIsIdentWithName(lastRet.Type, "error"); err != nil {
		return false, errors.Wrap(err, "invalid last return value type")
	}

	args := fn.Params
	if args.NumFields() == 0 {
		return false, fmt.Errorf("unsupported args count: %d", args.NumFields())
	}

	firstArg := args.List[0]
	firstArgStarExpr, ok := firstArg.Type.(*ast.StarExpr)
	if !ok {
		return false, fmt.Errorf("invalid first arg value type %#v, star expr expected", firstArg.Type)
	}

	firstArgSE, ok := firstArgStarExpr.X.(*ast.SelectorExpr)
	if !ok {
		return false, fmt.Errorf("invalid first arg value type %#v, selector expr expected", firstArgStarExpr.X)
	}

	var isAuthorized bool
	switch firstArgSE.Sel.Name {
	case "AnonymousContext":
		break
	case "AuthorizedContext":
		isAuthorized = true
	default:
		return false, fmt.Errorf("invalid first arg value type %#v", firstArgSE.Sel.Name)
	}

	if err := checkIsIdentWithName(firstArgSE.X, "request"); err != nil {
		return false, errors.Wrap(err, "invalid first arg selector type")
	}

	return isAuthorized, nil
}
