package server

import (
	"encoding/json"
	"net/http"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/gorilla/schema"
)

var schemaDecoder *schema.Decoder

func init() {
	schemaDecoder = schema.NewDecoder()
	schemaDecoder.IgnoreUnknownKeys(true)
}

// BindQueryString binds the request's query string with the
// given pointer to a struct of data.
// If the destination implements Validate(), it runs the validation
// as well.
func (s *Server) BindQueryString(r *http.Request, dst interface{}) *Message {
	if err := schemaDecoder.Decode(dst, r.URL.Query()); err != nil {
		return &Message{
			Status:  400,
			Message: err.Error(),
		}
	}

	v, ok := dst.(validation.Validatable)
	if !ok {
		return nil
	}

	return s.Validate(v)
}

// LoadJSON loads the JSON payload from the request body to the
// destination variable.
// If the destination implements Validate(), it runs the validation
// as well.
func (s *Server) LoadJSON(r *http.Request, dst interface{}) *Message {
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		return &Message{
			Status:  400,
			Message: err.Error(),
		}
	}

	v, ok := dst.(validation.Validatable)
	if !ok {
		return nil
	}

	return s.Validate(v)
}

// Validate runs the validation on a given destination data and returns
// a formatted message with the encountered errors, if any.
func (s *Server) Validate(data interface{}) *Message {
	err := validation.Validate(data)
	if err == nil {
		return nil
	}

	verr := err.(validation.Errors)
	elist := []Error{}
	for k, v := range verr {
		elist = append(elist, Error{
			Location: k,
			Error:    v.Error(),
		})
	}

	return &Message{
		Status:  400,
		Message: "Invalid input data",
		Errors:  elist,
	}
}
