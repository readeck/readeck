package auth

import (
	"net/http"

	"github.com/doug-martin/goqu/v9"
	"github.com/gorilla/sessions"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/internal/auth/users"
)

// SessionAuthProvider is the last authentication provider.
// It's alway enabled in case of every previous provider failing.
type SessionAuthProvider struct {
	// A function that returns the request's session
	GetSession func(*http.Request) *sessions.Session

	// A function that sets a Location header when
	// authentication fails.
	Redirect func(http.ResponseWriter, *http.Request)
}

// IsActive always returns true. As it's the last provider, when authentication fail it
// will with a redirect to the login page.
func (p *SessionAuthProvider) IsActive(_ *http.Request) bool {
	return true
}

// Authenticate checks if the request's session cookie is valid and
// the user exists.
func (p *SessionAuthProvider) Authenticate(w http.ResponseWriter, r *http.Request) (*http.Request, error) {
	sess := p.GetSession(r)
	if sess.IsNew {
		p.clearSession(sess, w, r)
		return r, nil
	}

	userID, ok := sess.Values["user_id"].(int)
	if !ok {
		p.clearSession(sess, w, r)
		return r, nil
	}

	u, err := users.Users.GetOne(goqu.C("id").Eq(userID))
	if err != nil {
		p.clearSession(sess, w, r)
		return r, err
	}

	chk, _ := sess.Values["check_code"].(uint32)
	if chk != u.CheckCode() {
		p.clearSession(sess, w, r)
		return r, err
	}

	// At this point, the user is granted access.
	// We renew its cookie for another max age duration.
	p.resendCookie(sess, w, r)
	return SetRequestAuthInfo(r, &Info{
		Provider: &ProviderInfo{
			Name: "http session",
		},
		User: u,
	}), nil
}

func (p *SessionAuthProvider) clearSession(sess *sessions.Session, w http.ResponseWriter, r *http.Request) {
	sess.Options.MaxAge = -1
	delete(sess.Values, "user_id")
	sess.Save(r, w)
	p.Redirect(w, r)
}

// resendCookie sends a new session cookie so it increases its expiration time.
// We could save the session with the same effect but since the data doesn't
// change, it's not needed.
func (p *SessionAuthProvider) resendCookie(sess *sessions.Session, w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(configs.Config.Server.Session.CookieName); err == nil {
		http.SetCookie(w, sessions.NewCookie(
			configs.Config.Server.Session.CookieName,
			c.Value,
			sess.Options,
		))
	}
}
