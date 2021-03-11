package profile

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/readeck/readeck/internal/auth"
	"github.com/readeck/readeck/internal/server"
	"github.com/readeck/readeck/internal/users"
	"github.com/readeck/readeck/pkg/form"
)

// profileAPI is the base settings API router.
type profileAPI struct {
	chi.Router
	srv *server.Server
}

// newProfileAPI returns a SettingAPI with its routes set up.
func newProfileAPI(s *server.Server) *profileAPI {
	r := s.AuthenticatedRouter()
	api := &profileAPI{r, s}

	r.Get("/", api.profileInfo)
	r.Patch("/", api.profileUpdate)
	r.Put("/password", api.passwordUpdate)

	return api
}

// UpdateProfile updates the user profile information.
func (api *profileAPI) UpdateProfile(u *users.User, sf *profileForm) (map[string]interface{}, error) {
	updated := map[string]interface{}{}
	if sf.Email != nil {
		u.Email = *sf.Email
		updated["email"] = u.Email
	}
	if sf.Username != nil {
		u.Username = *sf.Username
		updated["username"] = u.Username
	}

	if len(updated) > 0 {
		updated["updated"] = time.Now()
		if err := u.Update(updated); err != nil {
			return updated, err
		}
	}

	updated["id"] = u.ID
	return updated, nil
}

// UpdatePassword updates the user password.
func (api *profileAPI) UpdatePassword(u *users.User, pf *passwordForm) error {
	return u.SetPassword(pf.Password)
}

// userProfile is the mapping returned by the profileInfo route.
type userProfile struct {
	Username string    `json:"username"`
	Email    string    `json:"email"`
	Created  time.Time `json:"created"`
	Updated  time.Time `json:"updated"`
}

// newUserProfile creates a new userProfile from a user instance.
func newUserProfile(user *users.User) userProfile {
	return userProfile{
		Username: user.Username,
		Email:    user.Email,
		Created:  user.Created,
		Updated:  user.Updated,
	}
}

// profileInfo returns the current user information.
func (api *profileAPI) profileInfo(w http.ResponseWriter, r *http.Request) {
	user := auth.GetRequestUser(r)

	api.srv.Render(w, r, 200, newUserProfile(user))
}

// profileUpdate updates the current user profile information.
func (api *profileAPI) profileUpdate(w http.ResponseWriter, r *http.Request) {
	uf := &profileForm{}
	f := form.NewForm(uf)
	form.Bind(f, r)

	if !f.IsValid() {
		api.srv.Render(w, r, http.StatusUnprocessableEntity, f)
		return
	}

	user := auth.GetRequestUser(r)
	updated, err := api.UpdateProfile(user, uf)
	if err != nil {
		api.srv.Error(w, r, err)
		return
	}

	api.srv.Render(w, r, 200, updated)
}

// passwordUpdate updates the current user's password.
func (api *profileAPI) passwordUpdate(w http.ResponseWriter, r *http.Request) {
	pf := &passwordForm{}
	f := form.NewForm(pf)
	form.Bind(f, r)

	if !f.IsValid() {
		api.srv.Render(w, r, http.StatusUnprocessableEntity, f)
		return
	}

	user := auth.GetRequestUser(r)
	if err := api.UpdatePassword(user, pf); err != nil {
		api.srv.Error(w, r, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// profileForm is the form used by the profile update routes.
type profileForm struct {
	Username *string `json:"username" conform:"trim"`
	Email    *string `json:"email" conform:"trim"`
}

func (sf *profileForm) Validate(f *form.Form) {
	form.RequiredOrNull(f.Fields["username"])
	form.RequiredOrNull(f.Fields["email"])
	form.IsEmail(f.Fields["email"])
}

// passwordForm is the form used by the password update routes.
type passwordForm struct {
	Current  string `json:"current"`
	Password string `json:"password"`
}

func (pf *passwordForm) Validate(f *form.Form) {
	form.Required(f.Fields["password"])
}

// validateForView is the form validator used by the password update
// web view. It makes the "current" value mandatory and checks it
// against the current user password.
func (pf *passwordForm) validateForView(f *form.Form, u *users.User) bool {
	form.Required(f.Fields["current"])
	if !f.IsValid() {
		return false
	}
	if !u.CheckPassword(pf.Current) {
		f.Fields["current"].Errors.Add(errors.New("Invalid password"))
		return false
	}

	return true
}
