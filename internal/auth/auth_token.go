package auth

import (
	"errors"
	"net/http"
	"strings"

	"codeberg.org/readeck/readeck/internal/auth/tokens"
)

// TokenAuthProvider handles authentication using a bearer token
// passed in the request "Authorization" header with the scheme
// "Bearer".
type TokenAuthProvider struct{}

// IsActive returns true when the client submits a bearer token.
func (p *TokenAuthProvider) IsActive(r *http.Request) bool {
	_, ok := p.getToken(r)
	return ok
}

// Authenticate performs the authentication using the "Authorization: Bearer"
// header provided.
func (p *TokenAuthProvider) Authenticate(w http.ResponseWriter, r *http.Request) (*http.Request, error) {
	token, ok := p.getToken(r)
	if !ok {
		p.denyAccess(w)
		return r, errors.New("invalid authentication header")
	}

	claims, err := tokens.GetJwtClaims(token)
	if err != nil {
		p.denyAccess(w)
		return r, err
	}

	res, err := tokens.Tokens.GetUser(claims.ID)
	if err != nil {
		p.denyAccess(w)
		return r, err
	}

	if res.Token.IsExpired() {
		p.denyAccess(w)
		return r, nil
	}

	// ctx := context.WithValue(r.Context(), ctxAuthToken, res.Token)
	return SetRequestAuthInfo(r, &Info{
		Provider: &ProviderInfo{
			Name:        "bearer token",
			Application: res.Token.Application,
		},
		User: res.User,
	}), nil
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
