package users

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"codeberg.org/readeck/readeck/pkg/form"
	validation "github.com/go-ozzo/ozzo-validation/v4"
)

type (
	ctxUserFormKey struct{}
)

var (
	isValidPassword = validation.NewStringRule(func(v string) bool {
		if strings.TrimSpace(v) == "" {
			return false
		}
		return len(v) >= 8
	}, "password must be at least 8 character long")

	rxUsername      = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	isValidUsername = validation.NewStringRule(func(v string) bool {
		return rxUsername.MatchString(v)
	}, `must contain English letters, digits, "_" and "-" only`)
)

// ProfileForm is the form used by the profile update routes.
type ProfileForm struct {
	Username *string `json:"username" conform:"trim"`
	Email    *string `json:"email" conform:"trim"`
}

// Validate validates the form
func (sf *ProfileForm) Validate(f *form.Form) {
	f.Fields["username"].Validate(
		form.IsRequiredOrNull, isValidUsername,
	)
	f.Fields["email"].Validate(
		form.IsRequiredOrNull, form.IsValidEmail,
	)
}

// PasswordForm is a form to update a user's password.
type PasswordForm struct {
	Current  string `json:"current"`
	Password string `json:"password"`
}

// AddUser adds a user to the wrapping form's context.
func (pf *PasswordForm) AddUser(f *form.Form, u *User) {
	ctx := context.WithValue(f.Context(), ctxUserFormKey{}, u)
	f.SetContext(ctx)
}

// Validate validates the form.
func (pf *PasswordForm) Validate(f *form.Form) {
	f.Fields["password"].Validate(form.IsRequired, isValidPassword)

	// If a user was passed in context, then "current"
	// is mandatory and must match the current user
	// password.
	u, ok := f.Context().Value(ctxUserFormKey{}).(*User)
	if !ok {
		return
	}

	f.Fields["current"].Validate(form.IsRequired)
	if !f.IsValid() {
		return
	}
	if !u.CheckPassword(pf.Current) {
		f.Fields["current"].Errors.Add(errors.New("invalid password"))
	}
}

type userGroupRule struct{}

func (r userGroupRule) Validate(value interface{}) error {
	value, isNil := validation.Indirect(value)
	if isNil || validation.IsEmpty(value) {
		return nil
	}

	str, err := validation.EnsureString(value)
	if err != nil {
		return err
	}

	if _, ok := AvailableGroups()[str]; ok {
		return nil
	}

	return fmt.Errorf("must be one of %s", strings.Join(ValidGroups(), ", "))
}

var isValidGroup = userGroupRule{}

type UserForm struct {
	Username *string `json:"username" conform:"trim"`
	Email    *string `json:"email" conform:"trim"`
	Group    *string `json:"group" conform:"trim"`
	Password *string `json:"password"`
}

func (uf *UserForm) Validate(f *form.Form) {
	f.Fields["username"].Validate(form.IsRequiredOrNull, isValidUsername)
	f.Fields["email"].Validate(form.IsRequiredOrNull, form.IsValidEmail)
	f.Fields["group"].Validate(form.IsRequiredOrNull, isValidGroup)
}

type CreateForm struct {
	Username string  `json:"username" conform:"trim"`
	Email    string  `json:"email" conform:"trim"`
	Group    *string `json:"group" conform:"trim"`
	Password string  `json:"password"`
}

func (uf *CreateForm) Validate(f *form.Form) {
	f.Fields["username"].Validate(form.IsRequired, isValidUsername)
	f.Fields["password"].Validate(form.IsRequired)
	f.Fields["email"].Validate(form.IsRequired, form.IsValidEmail)
	f.Fields["group"].Validate(form.IsRequiredOrNull, isValidGroup)
}
