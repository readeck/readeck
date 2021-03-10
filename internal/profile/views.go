package profile

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/readeck/readeck/internal/auth"
	"github.com/readeck/readeck/internal/server"
	"github.com/readeck/readeck/pkg/form"
)

// profileViews is an HTTP handler for the user profile web views
type profileViews struct {
	chi.Router
	*profileAPI
}

// newProfileViews returns an new instance of ProfileViews
func newProfileViews(api *profileAPI) *profileViews {
	r := api.srv.AuthenticatedRouter()
	v := &profileViews{r, api}

	r.Get("/", v.userProfile)
	r.Post("/", v.userProfile)
	r.Get("/password", v.userPassword)
	r.Post("/password", v.userPassword)

	return v
}

// userProfile handles GET and POST requests on /profile.
func (v *profileViews) userProfile(w http.ResponseWriter, r *http.Request) {
	user := auth.GetRequestUser(r)
	pf := &profileForm{}
	f := form.NewForm(pf)

	if r.Method == http.MethodGet {
		pf.Username = &user.Username
		pf.Email = &user.Email
	}

	if r.Method == http.MethodPost {
		form.Bind(f, r)
		if f.IsValid() {
			if _, err := v.UpdateProfile(user, pf); err != nil {
				v.srv.AddFlash(w, r, "error", "Error while updating profile")
			} else {
				v.srv.AddFlash(w, r, "info", "Profile updated")
			}

			v.srv.Redirect(w, r, "profile")
			return
		}
	}

	ctx := server.TC{
		"Form": f,
	}
	v.srv.RenderTemplate(w, r, 200, "profile/index.gohtml", ctx)
}

// userPassword handles GET and POST requests on /profile/password
func (v *profileViews) userPassword(w http.ResponseWriter, r *http.Request) {
	pf := &passwordForm{}
	f := form.NewForm(pf)

	if r.Method == http.MethodPost {
		form.Bind(f, r)
		user := auth.GetRequestUser(r)

		if pf.validateForView(f, user) {
			if err := v.UpdatePassword(user, pf); err != nil {
				v.srv.AddFlash(w, r, "error", "Error while updating your password")
			} else {
				v.srv.AddFlash(w, r, "info", "Your password was changed.")
			}

			v.srv.Redirect(w, r, "password")
			return
		}
	}

	ctx := server.TC{
		"Form": f,
	}
	v.srv.RenderTemplate(w, r, 200, "profile/password.gohtml", ctx)
}
