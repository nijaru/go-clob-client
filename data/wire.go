package data

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

var unmarshalerType = reflect.TypeFor[json.Unmarshaler]()

// Required tags live alongside the model's wire names. Go's JSON decoder alone
// treats missing fields and null structs as zero values, which would silently
// turn a malformed financial row into apparently valid accounting data.
func validateWire(raw []byte, typ reflect.Type) error {
	if typ.Kind() == reflect.Pointer {
		if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return nil
		}
		return validateWire(raw, typ.Elem())
	}
	// Custom model decoders own their variant validation and normalization.
	// Their local wire aliases have no methods and are checked here once.
	if reflect.PointerTo(typ).Implements(unmarshalerType) {
		return nil
	}
	switch typ.Kind() {
	case reflect.Slice, reflect.Array:
		if typ.Elem().Kind() == reflect.Uint8 {
			return nil
		}
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			return err
		}
		if items == nil {
			return fmt.Errorf("data: expected a list")
		}
		for _, item := range items {
			if err := validateWire(item, typ.Elem()); err != nil {
				return err
			}
		}
	case reflect.Struct:
		// Timestamp/time.Time have custom scalar encodings, not JSON fields.
		fields := false
		for i := 0; i < typ.NumField(); i++ {
			if tag := typ.Field(i).Tag.Get("json"); tag != "" && tag != "-" {
				fields = true
				break
			}
		}
		if !fields {
			return nil
		}
		var object map[string]json.RawMessage
		if err := json.Unmarshal(raw, &object); err != nil {
			return err
		}
		if object == nil {
			return fmt.Errorf("data: expected a %s object", typ.Name())
		}
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			name := strings.Split(field.Tag.Get("json"), ",")[0]
			if name == "" || name == "-" {
				continue
			}
			value, present := object[name]
			isNull := bytes.Equal(bytes.TrimSpace(value), []byte("null"))
			isEmpty := bytes.Equal(bytes.TrimSpace(value), []byte(`""`))
			if field.Tag.Get("required") == "true" &&
				(!present || ((isNull || isEmpty) && field.Type.Kind() != reflect.Pointer)) {
				return fmt.Errorf("data: %s requires %s", typ.Name(), name)
			}
			if present && !isNull {
				if err := validateWire(value, field.Type); err != nil {
					return fmt.Errorf("%s.%s: %w", typ.Name(), name, err)
				}
			}
		}
	}
	return nil
}

func decodeWire[T any](raw []byte, out *T) error {
	if err := validateWire(raw, reflect.TypeFor[T]()); err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}
