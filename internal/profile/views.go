package profile

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/readeck/readeck/internal/auth"
	"github.com/readeck/readeck/internal/auth/tokens"
	"github.com/readeck/readeck/internal/auth/users"
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

	r.With(api.srv.WithPermission("read")).Group(func(r chi.Router) {
		r.Get("/", v.userProfile)
		r.Get("/password", v.userPassword)
		r.With(api.withTokenList).Get("/tokens", v.tokenList)
		r.With(api.withToken).Get("/tokens/{uid}", v.tokenInfo)
	})

	r.With(api.srv.WithPermission("write")).Group(func(r chi.Router) {
		r.Post("/", v.userProfile)
		r.Post("/password", v.userPassword)
		r.Post("/tokens", v.tokenCreate)
		r.With(api.withToken).Post("/tokens/{uid}", v.tokenInfo)
		r.With(api.withToken).Post("/tokens/{uid}/delete", v.tokenDelete)
	})

	return v
}

// userProfile handles GET and POST requests on /profile.
func (v *profileViews) userProfile(w http.ResponseWriter, r *http.Request) {
	user := auth.GetRequestUser(r)
	pf := &users.ProfileForm{}
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
				// Set the new seed in the session.
				// We needn't save the session since AddFlash does it already.
				sess := v.srv.GetSession(r)
				sess.Values["s"] = user.Seed
				v.srv.AddFlash(w, r, "success", "Profile updated")
			}

			v.srv.Redirect(w, r, "profile")
			return
		}
	}

	ctx := server.TC{
		"Form": f,
	}
	v.srv.RenderTemplate(w, r, 200, "profile/index", ctx)
}

// userPassword handles GET and POST requests on /profile/password
func (v *profileViews) userPassword(w http.ResponseWriter, r *http.Request) {
	pf := &users.PasswordForm{}
	f := form.NewForm(pf)

	if r.Method == http.MethodPost {
		user := auth.GetRequestUser(r)
		pf.SetUser(f, user)

		form.Bind(f, r)
		if f.IsValid() {
			if err := v.UpdatePassword(user, pf); err != nil {
				v.srv.AddFlash(w, r, "error", "Error while updating your password")
			} else {
				// Set the new seed in the session.
				// We needn't save the session since AddFlash does it already.
				sess := v.srv.GetSession(r)
				sess.Values["s"] = user.Seed
				v.srv.AddFlash(w, r, "success", "Your password was changed.")
			}

			v.srv.Redirect(w, r, "password")
			return
		}
	}

	ctx := server.TC{
		"Form": f,
	}
	v.srv.RenderTemplate(w, r, 200, "profile/password", ctx)
}

func (v *profileViews) tokenList(w http.ResponseWriter, r *http.Request) {
	tl := r.Context().Value(ctxTokenListKey{}).(tokenList)

	ctx := server.TC{
		"Pagination": tl.Pagination,
		"Tokens":     tl.Items,
	}
	v.srv.RenderTemplate(w, r, 200, "profile/token_list", ctx)
}

func (v *profileViews) tokenCreate(w http.ResponseWriter, r *http.Request) {
	user := auth.GetRequestUser(r)
	t := &tokens.Token{
		UserID:      &user.ID,
		IsEnabled:   true,
		Application: "internal",
	}
	if err := tokens.Tokens.Create(t); err != nil {
		v.srv.Log(r).WithError(err).Error("server error")
		v.srv.AddFlash(w, r, "error", "An error append while creating your token.")
		v.srv.Redirect(w, r, "tokens")
		return
	}

	v.srv.AddFlash(w, r, "success", "New token created.")
	v.srv.Redirect(w, r, ".", t.UID)
}

func (v *profileViews) tokenInfo(w http.ResponseWriter, r *http.Request) {
	ti := r.Context().Value(ctxtTokenKey{}).(tokenItem)

	tf := &tokenForm{}
	f := form.NewForm(tf)

	if r.Method == http.MethodGet {
		tf.Expires = ti.Token.Expires
		tf.IsEnabled = ti.Token.IsEnabled
	}

	if r.Method == http.MethodPost {
		form.Bind(f, r)
		if f.IsValid() {
			if tf.Expires != nil && tf.Expires.IsZero() {
				tf.Expires = nil
			}
			ti.Token.IsEnabled = tf.IsEnabled
			ti.Token.Expires = tf.Expires
			if err := ti.Token.Save(); err != nil {
				v.srv.Log(r).WithError(err).Error("server error")
				v.srv.AddFlash(w, r, "error", "Error while updating token")
			} else {
				v.srv.AddFlash(w, r, "success", "Token was updated.")
			}
			v.srv.Redirect(w, r, ti.UID)
			return
		}
	}

	jwt, err := tokens.NewJwtToken(ti.UID)
	if err != nil {
		v.srv.Status(w, r, http.StatusInternalServerError)
		return
	}

	ctx := server.TC{
		"Token": ti,
		"JWT":   jwt,
		"Form":  f,
	}

	v.srv.RenderTemplate(w, r, 200, "profile/token", ctx)
}

func (v *profileViews) tokenDelete(w http.ResponseWriter, r *http.Request) {
	df := &deleteForm{}
	f := form.NewForm(df)
	form.Bind(f, r)

	ti := r.Context().Value(ctxtTokenKey{}).(tokenItem)
	defer func() {
		v.srv.Redirect(w, r, "..", ti.UID)
	}()

	if df.Cancel {
		tokenTimers.Stop(ti.Token.ID)
		return
	}

	tokenTimers.Start(ti.Token.ID, 20*time.Second, func() {
		log := v.srv.Log(r).WithField("token", ti.UID)
		if err := ti.Token.Delete(); err != nil {
			log.WithError(err).Error("removing token")
			return
		}
		log.Debug("token removed")
	})
}
