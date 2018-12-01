package transportutil

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
)

const (
	urlPartType  = "urlPart"
	urlParamType = "urlParam"
	headerType   = "header"
	bodyType     = "body"
)

var types = []string{urlPartType, urlParamType, headerType, bodyType}

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

func getFieldType(rf reflect.StructField) string {
	request := rf.Tag.Get("request")
	if request == "" {
		return bodyType
	}

	parts := strings.Split(request, ",")
	if len(parts) != 3 {
		panic("bad tag " + rf.Tag)
	}

	for _, t := range types {
		if t == parts[1] {
			return t
		}
	}

	panic("invalid field type " + parts[1])
}

func getFieldName(rf reflect.StructField) string {
	request := rf.Tag.Get("request")
	if request == "" {
		return ""
	}

	parts := strings.Split(request, ",")
	if parts[0] == "" {
		return strings.ToLower(rf.Name)
	}

	return strings.ToLower(parts[0])
}

func isRequiredField(rf reflect.StructField) bool {
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

func decodeRequestBody(f reflect.Value, r *http.Request) error {
	if r.Body == nil {
		return errors.New("no request body")
	}
	defer r.Body.Close()

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return errors.Wrap(err, "failed to read http request body")
	}
	f.SetBytes(body)
	return nil
}

func decodeRequestField(f reflect.Value, r *http.Request) error {
	if f.Type().ConvertibleTo(reflect.TypeOf([]byte(nil))) {
		return decodeRequestBody(f, r)
	}

	if f.Kind() != reflect.Ptr {
		return fmt.Errorf("invalid field type %s (%#v), pointer to struct expected", f.Kind(), f.Interface())
	}

	pointedType := f.Type().Elem()
	if pointedType.Kind() != reflect.Struct {
		return fmt.Errorf("invalid field type %s (%#v), struct expected", pointedType.Kind(), f.Interface())
	}

	ptrVal := reflect.New(pointedType)
	f.Set(ptrVal)

	pointedVal := ptrVal.Elem()
	structFields := extractStructFields(pointedVal)

	isFirstFieldFromBody := getFieldType(structFields[0].rf) == bodyType
	for _, sf := range structFields {
		isBodyField := getFieldType(sf.rf) == bodyType
		if isFirstFieldFromBody != isBodyField {
			return errors.New("body fields can't be combined with another field types")
		}
	}

	if isFirstFieldFromBody {
		if err := decodeRequestFieldFromBody(ptrVal, r); err != nil {
			return errors.Wrap(err, "can't decode from body")
		}

		return nil
	}

	if err := decodeRequestFields(structFields, r); err != nil {
		return errors.Wrap(err, "can't decode from url")
	}

	return nil
}

func decodeRequestFields(structFields []structField, r *http.Request) error {
	typeToDataGetter := map[string]func(string) string{
		urlParamType: func(k string) string {
			return r.URL.Query().Get(k)
		},
		urlPartType: func(k string) string {
			return mux.Vars(r)[k]
		},
		headerType: func(k string) string {
			return r.Header.Get(k)
		},
	}

	for _, sf := range structFields {
		fieldType := getFieldType(sf.rf)
		dataGetter := typeToDataGetter[fieldType]
		if dataGetter == nil {
			return fmt.Errorf("invalid field type %s: no data getter for it", fieldType)
		}

		fieldName := getFieldName(sf.rf)
		if fieldName == "" {
			return fmt.Errorf("no field name for %#v", sf.rf)
		}

		fieldValue := dataGetter(fieldName)
		if fieldValue == "" {
			if isRequiredField(sf.rf) {
				return fmt.Errorf("no required field %s", fieldName)
			}

			continue
		}

		if err := decodeRequestParamFromString(sf.val, fieldValue); err != nil {
			return errors.Wrapf(err, "failed to decode field %s value %q", fieldName, fieldValue)
		}
	}

	return nil
}

//nolint:gocyclo
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
	case reflect.Int:
		v, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return fmt.Errorf("can't parse number from %q: %s", s, err)
		}
		param.SetInt(v)
	case reflect.Bool:
		v, err := strconv.ParseUint(s, 10, 32)
		if err != nil {
			return fmt.Errorf("can't parse number from %q: %s", s, err)
		}
		if v != 0 && v != 1 {
			return fmt.Errorf("boolean var can be only 0 or 1, but it's %d", v)
		}
		param.SetBool(v == 1)
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
