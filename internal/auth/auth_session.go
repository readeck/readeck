package auth

import (
	"net/http"

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
func (p *SessionAuthProvider) IsActive(r *http.Request) bool {
	return true
}

// Authenticate checks if the request's session cookie is valid and
// the user exists.
func (p *SessionAuthProvider) Authenticate(w http.ResponseWriter, r *http.Request) (*users.User, error) {
	sess := p.GetSession(r)
	if sess.IsNew {
		p.clearSession(sess, w, r)
		return nil, nil
	}

	userID, ok := sess.Values["user_id"].(int)
	if !ok {
		p.clearSession(sess, w, r)
		return nil, nil
	}

	u, err := users.Users.GetOne(goqu.C("id").Eq(userID))
	if err != nil {
		p.clearSession(sess, w, r)
		return nil, err
	}

	return u, nil
}

func (p *SessionAuthProvider) clearSession(sess *sessions.Session, w http.ResponseWriter, r *http.Request) {
	sess.Options.MaxAge = -1
	delete(sess.Values, "user_id")
	sess.Save(r, w)
	p.Redirect(w, r)
}
