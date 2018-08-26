package transportutil

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
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

func isURLParamField(rf reflect.StructField) bool {
	return rf.Tag == `request:",url"` // TODO
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
		if sf.val.Kind() != reflect.String {
			return fmt.Errorf("invalid struct field of type %s: only string supported", sf.val.Kind())
		}

		urlParam := strings.ToLower(sf.rf.Name)
		urlVar := mux.Vars(r)[urlParam]
		if urlVar == "" {
			return fmt.Errorf("no url param %s", urlParam)
		}

		sf.val.SetString(urlVar)
	}

	return nil
}

func decodeRequestFieldFromBody(ptr reflect.Value, r *http.Request) error {
	if err := json.NewDecoder(r.Body).Decode(ptr.Interface()); err != nil {
		return errors.Wrap(err, "invalid payload json")
	}

	return nil
}
