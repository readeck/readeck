package auth

import (
	"errors"
	"net/http"
	"strings"

	"github.com/readeck/readeck/internal/auth/tokens"
	"github.com/readeck/readeck/internal/auth/users"
)

// TokenAuthProvider handles authentication using a bearer token
// passed in the request "Authorization" header with the scheme
// "Bearer".
type TokenAuthProvider struct{}

// Info return information about the provider.
func (p *TokenAuthProvider) Info() *ProviderInfo {
	return &ProviderInfo{
		Name: "bearer token",
	}
}

// IsActive returns true when the client submits a bearer token.
func (p *TokenAuthProvider) IsActive(r *http.Request) bool {
	_, ok := p.getToken(r)
	return ok
}

// Authenticate performs the authentication using the "Authorization: Bearer"
// header provided.
func (p *TokenAuthProvider) Authenticate(w http.ResponseWriter, r *http.Request) (*users.User, error) {
	token, ok := p.getToken(r)
	if !ok {
		p.denyAccess(w)
		return nil, errors.New("Invalid authentication header")
	}

	claims, err := tokens.GetJwtClaims(token)
	if err != nil {
		p.denyAccess(w)
		return nil, err
	}

	res, err := tokens.Tokens.GetUser(claims.ID)
	if err != nil {
		p.denyAccess(w)
		return nil, err
	}

	if res.Token.IsExpired() {
		p.denyAccess(w)
		return nil, nil
	}

	return res.User, nil
}

// CsrfExempt is always true for this provider.
func (p *TokenAuthProvider) CsrfExempt(r *http.Request) bool {
	return true
}

// getToken reads the token from the "Authorization" header.
func (p *TokenAuthProvider) getToken(r *http.Request) (token string, ok bool) {
	const prefix = "Bearer "
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return
	}

	if len(auth) < len(prefix) || !strings.EqualFold(auth[:len(prefix)], prefix) {
		return
	}
	token = auth[len(prefix):]
	ok = true
	return
}

func (p *TokenAuthProvider) denyAccess(w http.ResponseWriter) {
	w.WriteHeader(http.StatusForbidden)
}
