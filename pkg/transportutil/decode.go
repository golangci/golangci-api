package transportutil

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
)

func DecodeRequest(request interface{}, r *http.Request) error {
	val := reflect.ValueOf(request)
	if val.Type().Kind() != reflect.Ptr {
		return fmt.Errorf("invalid request type %s, pointer expected", val.Type().Kind())
	}
	val = val.Elem()

	for i := 0; i < val.NumField(); i++ {
		f := val.Field(i)
		if !f.CanSet() {
			continue // service field, e.g. httpRequest
		}

		if err := decodeRequestField(f, r); err != nil {
			return errors.Wrapf(err, "can't decode request field %s", val.Type().Field(i).Name)
		}
	}

	return nil
}

type structField struct {
	rf  reflect.StructField
	val reflect.Value
}

func extractStructFields(sv reflect.Value) []structField {
	var fields []structField
	for i := 0; i < sv.NumField(); i++ {
		vf := sv.Field(i)
		rf := sv.Type().Field(i)
		if rf.Anonymous {
			fields = append(fields, extractStructFields(vf)...)
			continue
		}

		fields = append(fields, structField{
			rf:  rf,
			val: vf,
		})
	}

	return fields
}

func getURLParamName(rf reflect.StructField) string {
	request := rf.Tag.Get("request")
	if request == "" {
		return ""
	}

	parts := strings.Split(request, ",")
	if len(parts) != 3 {
		panic("bad tag " + rf.Tag)
	}

	if parts[1] != "url" {
		panic("bad tag parts[1] " + rf.Tag)
	}

	if parts[0] == "" {
		return rf.Name
	}

	return parts[0]
}

func isRequiredURLParam(rf reflect.StructField) bool {
	request := rf.Tag.Get("request")
	if request == "" {
		return false
	}

	parts := strings.Split(request, ",")
	if len(parts) != 3 {
		panic("bad tag " + rf.Tag)
	}

	if parts[2] == "" || parts[2] == "required" {
		return true
	}

	if parts[2] == "optional" {
		return false
	}

	panic("bad tag required field " + rf.Tag)
}

func isURLParamField(rf reflect.StructField) bool {
	return getURLParamName(rf) != ""
}

func decodeRequestField(f reflect.Value, r *http.Request) error {
	if f.Kind() != reflect.Ptr {
		return fmt.Errorf("invalid field type %s, pointer to struct expected", f.Kind())
	}

	pointedType := f.Type().Elem()
	if pointedType.Kind() != reflect.Struct {
		return fmt.Errorf("invalid field type %s, struct expected", pointedType.Kind())
	}

	ptrVal := reflect.New(pointedType)
	f.Set(ptrVal)

	pointedVal := ptrVal.Elem()
	structFields := extractStructFields(pointedVal)

	isURLParam := isURLParamField(structFields[0].rf)
	for _, sf := range structFields {
		if isURLParam != isURLParamField(sf.rf) {
			return errors.New("all struct fields must be URL or JSON params, not combined")
		}
	}

	if isURLParam {
		if err := decodeRequestFieldFromURL(structFields, r); err != nil {
			return errors.Wrap(err, "can't decode from url")
		}

		return nil
	}

	if err := decodeRequestFieldFromBody(ptrVal, r); err != nil {
		return errors.Wrap(err, "can't decode from body")
	}

	return nil
}

func decodeRequestFieldFromURL(structFields []structField, r *http.Request) error {
	for _, sf := range structFields {
		urlParamName := getURLParamName(sf.rf)
		urlParamName = strings.ToLower(urlParamName)
		vars := mux.Vars(r)
		urlVar := vars[urlParamName]
		if urlVar == "" {
			if isRequiredURLParam(sf.rf) {
				return fmt.Errorf("no url param %s, all params are %#v", urlParamName, vars)
			}
			return nil
		}

		if err := decodeRequestParamFromString(sf.val, urlVar); err != nil {
			return fmt.Errorf("failed to decode url param %s with value %q: %s", urlParamName, urlVar, err)
		}
	}

	return nil
}

func decodeRequestParamFromString(param reflect.Value, s string) error {
	switch param.Kind() {
	case reflect.String:
		param.SetString(s)
	case reflect.Uint:
		v, err := strconv.ParseUint(s, 10, 32)
		if err != nil {
			return fmt.Errorf("can't parse number from %q: %s", s, err)
		}
		param.SetUint(v)
	default:
		return fmt.Errorf("unsupported type %s", param.Kind())
	}

	return nil
}

func decodeRequestFieldFromBody(ptr reflect.Value, r *http.Request) error {
	if err := json.NewDecoder(r.Body).Decode(ptr.Interface()); err != nil {
		return errors.Wrap(err, "invalid payload json")
	}

	return nil
}
