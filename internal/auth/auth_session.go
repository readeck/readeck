package auth

import (
	"net/http"

	"github.com/doug-martin/goqu/v9"
	"github.com/gorilla/sessions"

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
	id, ok := sess.Values["u"].(int)
	if !ok {
		p.clearSession(sess, w, r)
		return r, nil
	}
	seed, ok := sess.Values["s"].(int)
	if !ok {
		p.clearSession(sess, w, r)
		return r, nil
	}

	u, err := users.Users.GetOne(goqu.C("id").Eq(id))
	if err != nil {
		p.clearSession(sess, w, r)
		return r, err
	}

	if u.Seed != seed {
		p.clearSession(sess, w, r)
		return r, err
	}

	// At this point, the user is granted access.
	// We renew its session for another max age duration.
	sess.Save(r, w)
	return SetRequestAuthInfo(r, &Info{
		Provider: &ProviderInfo{
			Name: "http session",
		},
		User: u,
	}), nil
}

func (p *SessionAuthProvider) clearSession(sess *sessions.Session, w http.ResponseWriter, r *http.Request) {
	sess.Options.MaxAge = -1
	sess.Save(r, w)
	p.Redirect(w, r)
}
