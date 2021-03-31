package form

import (
	"net/url"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/go-ozzo/ozzo-validation/v4/is"
)

// Validate applies the given validation rules to the field.
// When the validation fails, it adds an error to the field
// so it can be retrieved later.
func (f *Field) Validate(rules ...validation.Rule) {
	if err := validation.Validate(f.Value(), rules...); err != nil {
		f.Errors.Add(err)
	}
}

// IsValidURL checks is a url is a valid http or https url.
func IsValidURL(schemes map[string]bool) validation.StringRule {
	return validation.NewStringRule(func(v string) bool {
		u, err := url.Parse(v)
		if err != nil {
			return false
		}

		return schemes[u.Scheme]
	}, "invalid URL")
}

// IsValidEmail adds an error to the field if it's not a valid email address
var IsValidEmail = is.Email

// IsRequired is an alias to validation.Required
var IsRequired = validation.Required

// IsRequiredOrNull is an alias to validation.NilOrNotEmpty
var IsRequiredOrNull = validation.NilOrNotEmpty
