package users

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"codeberg.org/readeck/readeck/pkg/form"
	"github.com/doug-martin/goqu/v9"
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

// SetUser adds a user to the wrapping form's context.
func (pf *PasswordForm) SetUser(f *form.Form, u *User) {
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

type GroupChoice string

func (c *GroupChoice) Options() [][2]string {
	return availableGroups
}

func (c *GroupChoice) String() string {
	return fmt.Sprint(*c)
}

func (c *GroupChoice) Validate(f *form.Field) error {
	value, isNil := validation.Indirect(f.Value())
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

// CreateForm describes a user creation form
type CreateForm struct {
	Username string       `json:"username" conform:"trim"`
	Email    string       `json:"email" conform:"trim"`
	Group    *GroupChoice `json:"group" conform:"trim"`
	Password string       `json:"password"`
}

// Validate validates the form.
func (uf *CreateForm) Validate(f *form.Form) {
	f.Fields["username"].Validate(form.IsRequired, isValidUsername)
	f.Fields["password"].Validate(form.IsRequired)
	f.Fields["email"].Validate(form.IsRequired, form.IsValidEmail)
	f.Fields["group"].Validate(form.IsRequiredOrNull)

	// Check that username is not already in use
	c, err := Users.Query().Where(
		goqu.C("username").Eq(uf.Username),
	).Count()
	if err != nil {
		f.Errors.Add(errors.New("validation process error"))
		return
	}
	if c > 0 {
		f.Fields["username"].Errors.Add(errors.New("username is already in use"))
	}

	// Check that email is not already in use
	c, err = Users.Query().Where(
		goqu.C("email").Eq(uf.Email),
	).Count()
	if err != nil {
		f.Errors.Add(errors.New("validation process error"))
		return
	}
	if c > 0 {
		f.Fields["email"].Errors.Add(errors.New("email is already in use"))
	}
}

// UpdateForm describes a user update form.
type UpdateForm struct {
	Username *string      `json:"username" conform:"trim"`
	Email    *string      `json:"email" conform:"trim"`
	Group    *GroupChoice `json:"group" conform:"trim"`
	Password *string      `json:"password"`
}

// SetUser adds a user to the wrapping form's context.
func (uf *UpdateForm) SetUser(f *form.Form, u *User) {
	ctx := context.WithValue(f.Context(), ctxUserFormKey{}, u)
	f.SetContext(ctx)
}

// Validate validates the form
func (uf *UpdateForm) Validate(f *form.Form) {
	f.Fields["username"].Validate(form.IsRequiredOrNull, isValidUsername)
	f.Fields["email"].Validate(form.IsRequiredOrNull, form.IsValidEmail)
	f.Fields["group"].Validate(form.IsRequiredOrNull)

	u := f.Context().Value(ctxUserFormKey{}).(*User)

	// Check that username is not already in use
	if uf.Username != nil {
		c, err := Users.Query().Where(
			goqu.C("username").Eq(uf.Username),
			goqu.C("id").Neq(u.ID),
		).Count()
		if err != nil {
			f.Errors.Add(errors.New("validation process error"))
			return
		}
		if c > 0 {
			f.Fields["username"].Errors.Add(errors.New("username is already in use"))
		}
	}

	// Check that email is not already in use
	if uf.Email != nil {
		c, err := Users.Query().Where(
			goqu.C("email").Eq(uf.Email),
			goqu.C("id").Neq(u.ID),
		).Count()
		if err != nil {
			f.Errors.Add(errors.New("validation process error"))
			return
		}
		if c > 0 {
			f.Fields["email"].Errors.Add(errors.New("email is already in use"))
		}
	}
}
