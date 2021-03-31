package form

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/araddon/dateparse"
	"github.com/gorilla/schema"
	"github.com/leebenson/conform"
)

var decoder = schema.NewDecoder()

type (
	// Form contains the data instance and the derived fields that
	// can be used in a template or returned in a JSON response.
	Form struct {
		ctx      context.Context
		instance interface{}
		Fields   map[string]*Field
		Errors   Errors
	}

	// Field represents a form field.
	Field struct {
		instance reflect.Value
		Name     string
		Type     reflect.Type
		Errors   Errors
	}

	// Errors is a error list that is attached to a form or a field.
	Errors []error

	// Validator is the interface for types implementing a validation
	// routine after the data had been converted.
	Validator interface {
		Validate(*Form)
	}
)

func init() {
	decoder.IgnoreUnknownKeys(true)
	decoder.SetAliasTag("json")
	decoder.RegisterConverter(time.Time{}, func(value string) reflect.Value {
		if value == "" {
			return reflect.ValueOf(time.Time{})
		}
		t, err := dateparse.ParseLocal(value)
		if err != nil {
			return reflect.Value{}
		}
		return reflect.ValueOf(t)
	})
}

// MarshalJSON performs the field's JSON serialization.
func (f *Field) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Value  interface{} `json:"value"`
		Errors Errors      `json:"errors"`
	}{
		f.Value(),
		f.Errors,
	})
}

// Value returns the field's value.
func (f *Field) Value() interface{} {
	return f.instance.Interface()
}

// MarshalJSON performs the error list's JSON serialization.
func (e Errors) MarshalJSON() ([]byte, error) {
	if len(e) == 0 {
		return json.Marshal(nil)
	}

	res := make([]string, len(e))
	for i := range e {
		res[i] = e[i].Error()
	}

	return json.Marshal(res)
}

// Add adds an new error to the list.
func (e *Errors) Add(err error) {
	*e = append(*e, err)
}

// NewForm creates a new form instance and defines the derived fields.
func NewForm(instance interface{}) *Form {
	val := reflect.ValueOf(instance)
	if val.Kind() != reflect.Ptr {
		panic("Form instance must be a pointer to a struct")
	}
	val = reflect.Indirect(val)
	if val.Kind() != reflect.Struct {
		panic("Form instance must be a pointer to a struct")
	}
	ret := &Form{
		instance: instance,
		Fields:   map[string]*Field{},
	}

	for i := 0; i < val.NumField(); i++ {
		fd := val.Field(i)
		// Exclude private field
		if !fd.CanInterface() {
			continue
		}

		tp := val.Type().Field(i)
		name := tp.Name
		tag := strings.SplitN(tp.Tag.Get("json"), ",", 2)[0]

		// Exclude omitted fields
		if tag == "-" {
			continue
		}

		// Set field name
		if tag != "" {
			name = tag
		}

		o := &Field{
			instance: fd,
			Name:     tp.Name,
			Type:     tp.Type,
		}
		ret.Fields[name] = o
	}

	return ret
}

// MarshalJSON performs the form's JSON serialization.
func (f *Form) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		IsValid bool              `json:"is_valid"`
		Errors  Errors            `json:"errors"`
		Fields  map[string]*Field `json:"fields"`
	}{
		f.IsValid(),
		f.Errors,
		f.Fields,
	})
}

// IsValid returns true when the form and its fields have no error
func (f *Form) IsValid() bool {
	if len(f.Errors) > 0 {
		return false
	}

	for k := range f.Fields {
		if len(f.Fields[k].Errors) > 0 {
			return false
		}
	}

	return true
}

// BindValues binds the values from any URL encoded value list.
func (f *Form) BindValues(values url.Values) {
	err := decoder.Decode(f.instance, values)
	if err != nil {
		if err, ok := err.(schema.MultiError); ok {
			for k, err := range err {
				if _, ok := f.Fields[k]; ok {
					if _, ok := err.(schema.ConversionError); ok {
						f.Fields[k].Errors.Add(errors.New("Invalid input"))
					} else {
						f.Fields[k].Errors.Add(errors.New("Unknown error"))
					}
				} else {
					f.Errors.Add(err)
				}
			}
		}
	}

	conform.Strings(f.instance)
}

// BindJSON decodes a JSON payload from an io.Reader into the form instance.
func (f *Form) BindJSON(r io.Reader) {
	err := json.NewDecoder(r).Decode(f.instance)
	if err != nil {
		f.Errors.Add(err)
		return
	}

	conform.Strings(f.instance)
}

// Validate performs the data validation on the form
// instance when it exists.
func (f *Form) Validate() {
	if validator, ok := f.instance.(Validator); ok {
		validator.Validate(f)
	}
}

// Context returns the form current context
func (f *Form) Context() context.Context {
	if f.ctx != nil {
		return f.ctx
	}
	return context.Background()
}

// SetContext set the new form's context
func (f *Form) SetContext(ctx context.Context) *Form {
	if ctx == nil {
		panic("nil context")
	}
	f.ctx = ctx
	return f
}

// Bind loads and validates the data using the method tied
// to the request's content-type header.
func Bind(f *Form, r *http.Request, validators ...func(f *Form)) {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("content-type"))
	if err != nil {
		f.Errors.Add(errors.New("Invalid content-type"))
		return
	}

	switch mediaType {
	case "application/json", "text/json":
		defer r.Body.Close()
		f.BindJSON(r.Body)
	case "application/x-www-form-urlencoded":
		r.ParseForm()
		f.BindValues(r.PostForm)
	default:
		f.Errors.Add(errors.New("Unknown content-type"))
	}
	for _, fn := range validators {
		fn(f)
	}
	f.Validate()
}
