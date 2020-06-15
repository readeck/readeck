package server

import (
	"context"
	"crypto/sha256"
	"net/http"

	"github.com/gorilla/sessions"

	"github.com/readeck/readeck/configs"
)

type ctxKeySession struct{}

var (
	store         *sessions.CookieStore
	ctxSessionKey = &ctxKeySession{}
)

func initStore() {
	sk := sha256.Sum256([]byte(configs.Config.Main.SignKey))
	ek := sha256.Sum256([]byte(configs.Config.Main.SecretKey))

	store = sessions.NewCookieStore(sk[:], ek[:])
	store.Options.HttpOnly = true
	store.Options.MaxAge = 86400 * 7
	store.Options.SameSite = http.SameSiteStrictMode
}

// WithSession initialize a session store that will be available
// on the included routes.
func (s *Server) WithSession() func(next http.Handler) http.Handler {
	if store == nil {
		initStore()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session, _ := store.Get(r, "sxid")
			ctx := r.Context()
			ctx = context.WithValue(ctx, ctxSessionKey, session)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetSession returns the session currently stored in context.
// It will panic (on purpose) if the route is not using the
// WithSession() middleware.
func (s *Server) GetSession(r *http.Request) *sessions.Session {
	return r.Context().Value(ctxSessionKey).(*sessions.Session)
}
