package profile

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"codeberg.org/readeck/readeck/internal/auth"
	"codeberg.org/readeck/readeck/internal/auth/tokens"
	"codeberg.org/readeck/readeck/internal/server"
	"codeberg.org/readeck/readeck/pkg/form"
	"codeberg.org/readeck/readeck/pkg/timers"
)

// profileViews is an HTTP handler for the user profile web views
type profileViews struct {
	chi.Router
	*profileAPI
}

// Token deletion timers
var tokenTimers = timers.NewTimerStore("_tokenTimers")

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
	tl := r.Context().Value(ctxTokenListKey{}).(tokenList)

	tokenTimers.Clean(w, r, v.srv.GetSession(r))

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
				v.srv.AddFlash(w, r, "info", "Token was updated.")
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

	sess := v.srv.GetSession(r)
	timerID := tokenTimers.Get(sess, ti.UID)
	if !tokenTimers.Exists(timerID) {
		tokenTimers.Save(w, r, sess, ti.UID, timerID)
	} else {
		ctx["Deleted"] = true
	}

	v.srv.RenderTemplate(w, r, 200, "profile/token.gohtml", ctx)
}

func (v *profileViews) tokenDelete(w http.ResponseWriter, r *http.Request) {
	df := &deleteForm{}
	f := form.NewForm(df)
	form.Bind(f, r)

	ti := r.Context().Value(ctxtTokenKey{}).(tokenItem)
	defer func() {
		v.srv.Redirect(w, r, "..", ti.UID)
	}()

	sess := v.srv.GetSession(r)
	if df.Cancel {
		timerID := tokenTimers.Get(sess, ti.UID)
		tokenTimers.Stop(timerID)
		return
	}

	timerID := tokenTimers.Start(15*time.Second, func() {
		log := v.srv.Log(r).WithField("token", ti.UID)
		if err := ti.Delete(); err != nil {
			log.WithError(err).Error("removing token")
			return
		}
		log.Debug("token removed")
	})

	tokenTimers.Save(w, r, sess, ti.UID, timerID)
}

type deleteForm struct {
	Cancel bool `json:"cancel"`
}
