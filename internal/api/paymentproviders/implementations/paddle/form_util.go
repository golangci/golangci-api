package paddle

import (
	"fmt"
	"net/url"

	"github.com/gorilla/schema"
	"github.com/pkg/errors"
)

func structToGrequestsData(s interface{}) (map[string]string, error) {
	form := url.Values{}
	enc := schema.NewEncoder()
	if err := enc.Encode(s, form); err != nil {
		return nil, errors.Wrap(err, "failed to encode struct to form")
	}

	ret := map[string]string{}
	for key, values := range form {
		if len(values) != 1 {
			return nil, fmt.Errorf("invalid count (%d) of key %s values: %v", len(values), key, values)
		}

		ret[key] = values[0]
	}

	return ret, nil
}
