package auth

import (
	"net/http"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/gorilla/sessions"

	"github.com/readeck/readeck/internal/auth/users"
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
	u, err := p.checkSession(sess)
	if u == nil || err != nil {
		p.clearSession(sess, w, r)
		return r, err
	}

	// At this point, the user is granted access.
	// We renew its session for another max age duration.
	sess.Values["t"] = time.Now().Unix()
	sess.Save(r, w)
	return SetRequestAuthInfo(r, &Info{
		Provider: &ProviderInfo{
			Name: "http session",
		},
		User: u,
	}), nil
}

func (p *SessionAuthProvider) checkSession(sess *sessions.Session) (u *users.User, err error) {
	if sess.IsNew {
		return
	}

	// Check expired first
	st, ok := sess.Values["t"].(int64)
	if !ok {
		return
	}
	if time.Now().Unix() > st+int64(sess.Options.MaxAge) {
		return
	}

	seed, ok := sess.Values["s"].(int)
	if !ok {
		return
	}

	id, ok := sess.Values["u"].(int)
	if !ok {
		return
	}

	if u, err = users.Users.GetOne(goqu.C("id").Eq(id)); err != nil {
		return nil, err
	}

	if u.Seed != seed {
		return nil, nil
	}

	return
}

func (p *SessionAuthProvider) clearSession(sess *sessions.Session, w http.ResponseWriter, r *http.Request) {
	sess.Options.MaxAge = -1
	sess.Save(r, w)
	p.Redirect(w, r)
}
