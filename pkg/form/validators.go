package form

import (
	"errors"
	"net/url"
	"reflect"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/go-ozzo/ozzo-validation/v4/is"
)

// IsEmpty checks if a value is empty or not.
// A value is considered empty if
// - integer, float: zero
// - bool: false
// - string, array: len() == 0
// - slice, map: nil or len() == 0
// - interface, pointer: nil or the referenced value is empty
func IsEmpty(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String, reflect.Array, reflect.Map, reflect.Slice:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Invalid:
		return true
	case reflect.Interface, reflect.Ptr:
		if v.IsNil() {
			return true
		}
		return IsEmpty(v.Elem())
	case reflect.Struct:
		v, ok := v.Interface().(time.Time)
		if ok && v.IsZero() {
			return true
		}
	}

	return false
}

// Required adds an error to the field when its value is empty.
func Required(f *Field) {
	if IsEmpty(f.instance) {
		f.Errors.Add(errors.New("This field is required"))
	}
}

// RequiredOrNull adds an error to the fields when its value is empty
// except if it's null.
func RequiredOrNull(f *Field) {
	if !f.instance.IsNil() {
		Required(f)
	}
}

// IsValidEmail adds an error to the field if it's not a valid email address
func IsValidEmail(f *Field) {
	if err := validation.Validate(f.Value(), is.Email); err != nil {
		f.Errors.Add(errors.New("Invalid email address"))
	}
}

// IsValidURL checks is a url is a valid http or https url.
func IsValidURL(f *Field, schemes map[string]bool) {
	e := errors.New("Invalid URL")
	u, err := url.Parse(f.Value().(string))

	if err != nil {
		f.Errors.Add(e)
		return
	}

	if !schemes[u.Scheme] {
		f.Errors.Add(e)
	}
}
