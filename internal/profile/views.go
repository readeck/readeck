package profile

import (
	"errors"
	"net/http"

	"github.com/doug-martin/goqu/v9"
	"github.com/go-chi/chi/v5"

	"codeberg.org/readeck/readeck/internal/auth"
	"codeberg.org/readeck/readeck/internal/auth/tokens"
	"codeberg.org/readeck/readeck/internal/server"
	"codeberg.org/readeck/readeck/pkg/form"
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
	r.Get("/tokens", v.tokenList)
	r.Post("/tokens", v.tokenCreate)
	r.Get("/tokens/{uid}", v.tokenInfo)
	r.Post("/tokens/{uid}", v.tokenInfo)

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
				// Renew the check code in this user's session.
				// We needn't save the session since AddFlash does it already.
				sess := v.srv.GetSession(r)
				sess.Values["check_code"] = user.CheckCode()
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
				// Renew the check code in this user's session.
				// We needn't save the session since AddFlash does it already.
				sess := v.srv.GetSession(r)
				sess.Values["check_code"] = user.CheckCode()
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

func (v *profileViews) tokenList(w http.ResponseWriter, r *http.Request) {
	tl, err := v.getTokens(r, ".")
	if err != nil {
		if errors.Is(err, tokens.ErrNotFound) {
			v.srv.Status(w, r, http.StatusNotFound)
			return
		}
		v.srv.Error(w, r, err)
		return
	}

	ctx := server.TC{
		"Pagination": tl.Pagination,
		"Tokens":     tl.Items,
	}
	v.srv.RenderTemplate(w, r, 200, "profile/token_list.gohtml", ctx)
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

	v.srv.AddFlash(w, r, "info", "New token created.")
	v.srv.Redirect(w, r, ".", t.UID)
}

func (v *profileViews) tokenInfo(w http.ResponseWriter, r *http.Request) {
	uid := chi.URLParam(r, "uid")
	t, err := tokens.Tokens.GetOne(
		goqu.C("uid").Eq(uid),
		goqu.C("user_id").Eq(auth.GetRequestUser(r).ID),
	)
	if err != nil {
		v.srv.Status(w, r, http.StatusNotFound)
		return
	}

	tf := &tokenForm{}
	f := form.NewForm(tf)

	if r.Method == http.MethodGet {
		tf.Expires = t.Expires
		tf.IsEnabled = t.IsEnabled
	}

	if r.Method == http.MethodPost {
		form.Bind(f, r)
		if f.IsValid() {
			if tf.Expires != nil && tf.Expires.IsZero() {
				tf.Expires = nil
			}
			t.IsEnabled = tf.IsEnabled
			t.Expires = tf.Expires
			if err := t.Save(); err != nil {
				v.srv.Log(r).WithError(err).Error("server error")
				v.srv.AddFlash(w, r, "error", "Error while updating token")
			} else {
				v.srv.AddFlash(w, r, "info", "Token was updated.")
			}
			v.srv.Redirect(w, r, t.UID)
			return

		}
	}

	jwt, err := tokens.NewJwtToken(t.UID)
	if err != nil {
		v.srv.Status(w, r, http.StatusInternalServerError)
		return
	}

	ctx := server.TC{
		"Token": newTokenItem(v.srv, r, t, "."),
		"JWT":   jwt,
		"Form":  f,
	}
	v.srv.RenderTemplate(w, r, 200, "profile/token.gohtml", ctx)
}
