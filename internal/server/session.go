package server

import (
	"context"
	"encoding/gob"
	"net/http"
	"path"

	"github.com/gorilla/sessions"

	"github.com/readeck/readeck/configs"
)

type ctxKeySession struct{}
type ctxKeyFlash struct{}

var (
	store         sessions.Store
	ctxSessionKey = &ctxKeySession{}
	ctxFlashKey   = &ctxKeyFlash{}
)

// FlashMessage contains a message type and content.
type FlashMessage struct {
	Type    string
	Message string
}

func initStore() {
	store = sessions.NewFilesystemStore(
		path.Join(configs.Config.Main.DataDirectory, "sessions"),
		configs.CookieHashKey(),
		configs.CookieBlockKey(),
	)

	// Register flash message type
	gob.Register(FlashMessage{})
}

// WithSession initialize a session store that will be available
// on the included routes.
func (s *Server) WithSession() func(next http.Handler) http.Handler {
	if store == nil {
		initStore()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Store session
			session, _ := store.Get(r, configs.Config.Server.Session.CookieName)
			session.Options.HttpOnly = true
			session.Options.MaxAge = configs.Config.Server.Session.MaxAge
			session.Options.SameSite = http.SameSiteStrictMode
			session.Options.Path = path.Join(s.BasePath)

			ctx := r.Context()
			ctx = context.WithValue(ctx, ctxSessionKey, session)

			// Pop messages and store then. We must do it before
			// anything is sent to the client.
			flashes := session.Flashes()
			ctx = context.WithValue(ctx, ctxFlashKey, flashes)
			if len(flashes) > 0 {
				session.Save(r, w)
			}

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

// AddFlash saves a flash message in the session.
func (s *Server) AddFlash(w http.ResponseWriter, r *http.Request, typ, msg string) error {
	session := s.GetSession(r)
	session.AddFlash(FlashMessage{typ, msg})
	return session.Save(r, w)
}

// Flashes returns the flash messages retrieved from the session
// store in the session middleware.
func (s *Server) Flashes(r *http.Request) []FlashMessage {
	if msgs := r.Context().Value(ctxFlashKey); msgs != nil {
		res := make([]FlashMessage, len(msgs.([]interface{})))
		for i, item := range msgs.([]interface{}) {
			res[i] = item.(FlashMessage)
		}
		return res
	}
	return make([]FlashMessage, 0)
}
