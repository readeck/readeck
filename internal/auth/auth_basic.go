package auth

import (
	"errors"
	"net/http"
	"strings"

	"github.com/doug-martin/goqu/v9"

	"github.com/readeck/readeck/internal/auth/users"
)

// BasicAuthProvider handles basic HTTP authentication method
// with "Authorization: Basic {payload}" header.
type BasicAuthProvider struct{}

// IsActive returns true when the client submits basic HTTP authorization
// header.
func (p *BasicAuthProvider) IsActive(r *http.Request) bool {
	_, _, ok := r.BasicAuth()
	return ok
}

// Authenticate performs the authentication using the HTTP basic authentication
// information provided.
func (p *BasicAuthProvider) Authenticate(w http.ResponseWriter, r *http.Request) (*users.User, error) {
	username, password, ok := r.BasicAuth()
	if !ok {
		p.denyAccess(w)
		return nil, errors.New("Invalid authentication header")
	}

	if strings.TrimSpace(username) == "" || strings.TrimSpace(password) == "" {
		p.denyAccess(w)
		return nil, errors.New("No username and/or password provided")
	}

	u, err := users.Users.GetOne(goqu.C("username").Eq(username))
	if err != nil {
		p.denyAccess(w)
		return nil, err
	}

	if u.CheckPassword(password) {
		return u, nil
	}

	p.denyAccess(w)
	return nil, nil
}

// CsrfExempt is always true for this provider.
func (p *BasicAuthProvider) CsrfExempt(r *http.Request) bool {
	return true
}

func (p *BasicAuthProvider) denyAccess(w http.ResponseWriter) {
	w.Header().Add("WWW-Authenticate", `Basic realm="Restricted"`)
	w.WriteHeader(http.StatusUnauthorized)
}
