package auth

import (
	"context"
	"net/http"

	"codeberg.org/readeck/readeck/internal/auth/users"
)

type (
	ctxProviderKey struct{}
	ctxAuthKey     struct{}
)

// Info is the payload with the currently authenticated user
// and some information about the provider
type Info struct {
	Provider *ProviderInfo
	User     *users.User
}

// ProviderInfo contains information about the provider.
type ProviderInfo struct {
	Name        string
	Application string
}

// Provider is the interface that must implement any authentication
// provider.
type Provider interface {
	// Must return true to enable the provider for the current request.
	IsActive(r *http.Request) bool

	// Must return a request with the Info provided when successful.
	Authenticate(http.ResponseWriter, *http.Request) (*http.Request, error)
}

// FeatureCsrfProvider allows a provider to implement a method
// to bypass all CSRF protection.
type FeatureCsrfProvider interface {
	// Must return true to disable CSRF protection for the request.
	CsrfExempt(r *http.Request) bool
}

// NullProvider is the provider returned when no other provider
// could be activated.
type NullProvider struct{}

// Info return information about the provider.
func (p *NullProvider) Info(_ *http.Request) *ProviderInfo {
	return &ProviderInfo{
		Name: "null",
	}
}

// IsActive is always false
func (p *NullProvider) IsActive(_ *http.Request) bool {
	return false
}

// Authenticate doesn't do anything
func (p *NullProvider) Authenticate(_ http.ResponseWriter, r *http.Request) (*http.Request, error) {
	return r, nil
}

// Init returns an http.Handler that will try to find a suitable
// authentication provider on each request. The first to return
// true with its IsActive() method becomes the request authentication
// provider.
//
// If no provider could be found, the NullProvider will then be used.
//
// The provider is then stored in the request's context and can be
// retrieved using GetRequestProvider().
func Init(providers ...Provider) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var provider Provider
			for _, p := range providers {
				if p.IsActive(r) {
					provider = p
					break
				}
			}

			if provider == nil {
				provider = &NullProvider{}
			}

			r = setRequestProvider(r, provider)
			next.ServeHTTP(w, r)
		})
	}
}

// Required returns an http.Handler that will enforce authentication
// on the request. It uses the request authentication provider to perform
// the authentication.
//
// A provider performing a successful authentication must store
// its authentication information using SetRequestAuthInfo.
//
// When the request has this attribute it will carry on.
// Otherwise it stops the response with a 403 error.
//
// The logged in user can be retrieved with GetRequestUser().
func Required(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		provider := GetRequestProvider(r)
		r, err := provider.Authenticate(w, r)
		if err != nil {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		if !HasRequestAuthInfo(r) {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	}

	return http.HandlerFunc(fn)
}

// setRequestProvider stores the current provider for the request.
func setRequestProvider(r *http.Request, provider Provider) *http.Request {
	ctx := context.WithValue(r.Context(), ctxProviderKey{}, provider)
	return r.WithContext(ctx)
}

// GetRequestProvider returns the current request's authentication
// provider.
func GetRequestProvider(r *http.Request) Provider {
	return r.Context().Value(ctxProviderKey{}).(Provider)
}

// SetRequestAuthInfo stores the request's user.
func SetRequestAuthInfo(r *http.Request, info *Info) *http.Request {
	ctx := context.WithValue(r.Context(), ctxAuthKey{}, info)
	return r.WithContext(ctx)
}

// HasRequestAuthInfo returns true if the request context contains
// an auth.Info instance.
func HasRequestAuthInfo(r *http.Request) bool {
	if _, ok := r.Context().Value(ctxAuthKey{}).(*Info); ok {
		return true
	}
	return false
}

// GetRequestAuthInfo returns the current request's auth info
func GetRequestAuthInfo(r *http.Request) *Info {
	return r.Context().Value(ctxAuthKey{}).(*Info)
}

// GetRequestUser returns the current request's user.
func GetRequestUser(r *http.Request) *users.User {
	return GetRequestAuthInfo(r).User
}
