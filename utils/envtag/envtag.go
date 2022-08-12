package envtag

import (
	"os"
	"reflect"
	"strings"
)

// s must be a pointer to a struct
func Unmarshal(tagName string, prefix string, s interface{}) {
	structVal := reflect.ValueOf(s)

	if structVal.Kind() != reflect.Ptr {
		panic("s must be an interface")
	}
	structVal = structVal.Elem()
	typ := structVal.Type()

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		tag := field.Tag.Get(tagName)
		if tag == "" || tag == "-" {
			continue
		}

		v := structVal.FieldByName(field.Name)

		if !v.IsValid() || !v.CanSet() {
			continue
		}

		if tag == ",squash" && field.Type.Kind() == reflect.Struct {
			Unmarshal(tagName, prefix, v.Addr().Interface())
		}
		if v.Kind() != reflect.String {
			continue
		}
		if envVal := os.Getenv(strings.ToUpper(prefix + tag)); envVal != "" {
			v := structVal.FieldByName(field.Name)
			v.SetString(envVal)
		}
	}
}
