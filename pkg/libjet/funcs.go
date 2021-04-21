package libjet

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/CloudyKit/jet/v6"
)

var funcMap = map[string]jet.Func{
	"string": func(a jet.Arguments) reflect.Value {
		a.RequireNumOfArguments("string", 1, 1)
		return reflect.ValueOf(ToString(a.Get(0)))
	},
	"empty": jet.Func(func(a jet.Arguments) reflect.Value {
		a.RequireNumOfArguments("empty", 1, 1)
		return reflect.ValueOf(IsEmpty(a.Get(0)))
	}),
	"default": func(a jet.Arguments) reflect.Value {
		a.RequireNumOfArguments("default", 2, 2)
		if ToString(a.Get(0)) == "" {
			return a.Get(1)
		}
		return a.Get(0)
	},
	"join": func(a jet.Arguments) reflect.Value {
		a.RequireNumOfArguments("join", 2, 2)
		vl, isNil := Indirect(a.Get(0))
		if isNil {
			return reflect.ValueOf("")
		}
		list, ok := vl.([]string)
		if !ok {
			panic("invalid list type in join()")
		}
		sep := ToString(a.Get(1))

		return reflect.ValueOf(strings.Join(list, sep))
	},
	"date": func(a jet.Arguments) reflect.Value {
		a.RequireNumOfArguments("date", 2, 2)
		return reflect.ValueOf(ToDateFmt(a.Get(0), a.Get(1)))
	},
}

// FuncMap returns the jet function map.
func FuncMap() map[string]jet.Func {
	return funcMap
}

// AddFuncToSet adds a given function to a jet.Set template set.
func AddFuncToSet(set *jet.Set, key string) {
	if f, ok := funcMap[key]; ok {
		set.AddGlobalFunc(key, f)
	}
}

// Indirect returns the underlying value of a reflect.Value.
// It resolves pointers and indicates if the value is nil.
func Indirect(val reflect.Value) (interface{}, bool) {
	switch val.Kind() {
	case reflect.Invalid:
		return nil, true
	case reflect.Ptr, reflect.Interface:
		if val.IsNil() {
			return nil, true
		}
		return Indirect(val.Elem())
	case reflect.Slice, reflect.Map, reflect.Func, reflect.Chan:
		if val.IsNil() {
			return nil, true
		}
	}

	return val.Interface(), false
}

// IsEmpty returns true if the value is considered empty.
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
		val, _ := Indirect(v)
		if t, ok := val.(time.Time); ok && t.IsZero() {
			return true
		}
	}
	return false
}

// ToString converts a value to a string.
func ToString(v reflect.Value) string {
	val, isNil := Indirect(v)
	if isNil || val == nil {
		return ""
	}

	switch v := val.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	}

	if val, ok := val.(fmt.Stringer); ok {
		return val.String()
	}

	return fmt.Sprintf("%v", val)
}

// ToDateFmt returns a date formatted with the given format.
func ToDateFmt(d reflect.Value, f reflect.Value) string {
	v, isNil := Indirect(d)
	if isNil {
		return ""
	}
	date, ok := v.(time.Time)
	if !ok {
		panic("first argument must be a time.Time value or pointer")
	}

	layout := ToString(f)
	return date.Format(layout)
}
