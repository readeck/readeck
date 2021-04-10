package server

import (
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/sessions"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/pkg/redisstore"
)

type (
	ctxSessionKey struct{}
	ctxFlashKey   struct{}
)

var (
	store sessions.Store
)

// FlashMessage contains a message type and content.
type FlashMessage struct {
	Type    string
	Message string
}

// InitSession creates the session store based on the value of
// server.session.store_url.
func (s *Server) InitSession() error {
	// Register flash message type
	gob.Register(FlashMessage{})

	// Load session store
	storeURL, err := url.Parse(configs.Config.Server.Session.StoreURL)
	if err != nil {
		return err
	}

	switch storeURL.Scheme {
	case "file":
		// File session store
		// If not path is specified, we use "sessions" in the data folder
		p := storeURL.Path
		if storeURL.Path == "" {
			p = path.Join(configs.Config.Main.DataDirectory, "sessions")
		}
		// Create the session path if needed
		stat, err := os.Stat(p)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				if err := os.MkdirAll(p, 0750); err != nil {
					return err
				}
			} else {
				return err
			}
		} else if !stat.IsDir() {
			return fmt.Errorf("'%s' is not a directory", p)
		}

		// Start the store
		store = sessions.NewFilesystemStore(
			path.Join(p),
			configs.CookieHashKey(),
			configs.CookieBlockKey(),
		)
	case "redis":
		// Redis store.
		// URL is: redis://host:port?db=n (port and db are optional)
		host := storeURL.Host
		p := storeURL.Port()
		if p == "" {
			p = "6379"
		}
		port, err := strconv.Atoi(p)
		if err != nil {
			return fmt.Errorf("invalid redis port %s", p)
		}
		d := storeURL.Query().Get("db")
		if d == "" {
			d = "0"
		}
		db, err := strconv.Atoi(d)
		if err != nil {
			return fmt.Errorf("invalid redis db number %s", d)
		}

		// Start the store
		client := redis.NewClient(&redis.Options{
			Addr: fmt.Sprintf("%s:%d", host, port),
			DB:   db,
		})
		store, err = redisstore.NewRedisStore(
			context.Background(),
			client,
			configs.CookieHashKey(),
			configs.CookieBlockKey(),
		)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown session backend: %s", storeURL.Scheme)
	}

	return nil
}

// WithSession initialize a session store that will be available
// on the included routes.
func (s *Server) WithSession() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Store session
			session, _ := store.Get(r, configs.Config.Server.Session.CookieName)
			session.Options.HttpOnly = true
			session.Options.Secure = r.URL.Scheme == "https"
			session.Options.MaxAge = configs.Config.Server.Session.MaxAge
			session.Options.SameSite = http.SameSiteStrictMode
			session.Options.Path = path.Join(s.BasePath)

			ctx := r.Context()
			ctx = context.WithValue(ctx, ctxSessionKey{}, session)

			// Pop messages and store then. We must do it before
			// anything is sent to the client.
			flashes := session.Flashes()
			ctx = context.WithValue(ctx, ctxFlashKey{}, flashes)
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
	return r.Context().Value(ctxSessionKey{}).(*sessions.Session)
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
	if msgs := r.Context().Value(ctxFlashKey{}); msgs != nil {
		res := make([]FlashMessage, len(msgs.([]interface{})))
		for i, item := range msgs.([]interface{}) {
			res[i] = item.(FlashMessage)
		}
		return res
	}
	return make([]FlashMessage, 0)
}
