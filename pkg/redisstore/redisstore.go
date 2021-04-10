package redisstore

import (
	"context"
	"encoding/base32"
	"net/http"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
)

// RedisStore is a session store backed by a redis server.
type RedisStore struct {
	Codecs    []securecookie.Codec
	Options   *sessions.Options // default configuration
	client    redis.UniversalClient
	keyPrefix string
}

// NewRedisStore creates a new RedisStore.
func NewRedisStore(ctx context.Context, client redis.UniversalClient,
	keyPairs ...[]byte) (*RedisStore, error) {
	s := &RedisStore{
		Codecs: securecookie.CodecsFromPairs(keyPairs...),
		Options: &sessions.Options{
			Path:   "/",
			MaxAge: 86400 * 30,
		},
		client:    client,
		keyPrefix: "session",
	}
	return s, client.Ping(ctx).Err()
}

// Get returns a session for the given name after adding it to the registry.
func (s *RedisStore) Get(r *http.Request, name string) (*sessions.Session, error) {
	return sessions.GetRegistry(r).Get(s, name)
}

// New creates a new session but does not save data yet.
func (s *RedisStore) New(r *http.Request, name string) (*sessions.Session, error) {
	session := sessions.NewSession(s, name)
	opts := *s.Options
	session.Options = &opts
	session.IsNew = true
	var err error
	if c, errCookie := r.Cookie(name); errCookie == nil {
		err = securecookie.DecodeMulti(name, c.Value, &session.ID, s.Codecs...)
		if err == nil {
			err = s.load(r.Context(), session)
			if err == nil {
				session.IsNew = false
			}
		}
	}
	return session, err
}

// Save persists session data to redis.
func (s *RedisStore) Save(r *http.Request, w http.ResponseWriter,
	session *sessions.Session) error {
	// Delete if max-age is <= 0
	if session.Options.MaxAge <= 0 {
		if err := s.delete(r.Context(), session); err != nil {
			return err
		}
		http.SetCookie(w, sessions.NewCookie(session.Name(), "", session.Options))
		return nil
	}

	if session.ID == "" {
		session.ID = strings.TrimRight(
			base32.StdEncoding.EncodeToString(
				securecookie.GenerateRandomKey(32)), "=")
	}
	if err := s.save(r.Context(), session); err != nil {
		return err
	}
	encoded, err := securecookie.EncodeMulti(session.Name(), session.ID,
		s.Codecs...)
	if err != nil {
		return err
	}
	http.SetCookie(w, sessions.NewCookie(session.Name(), encoded, session.Options))
	return nil
}

func (s *RedisStore) save(ctx context.Context, session *sessions.Session) error {
	encoded, err := securecookie.EncodeMulti(session.Name(), session.Values,
		s.Codecs...)
	if err != nil {
		return err
	}

	return s.client.Set(ctx, s.keyPrefix+session.ID,
		[]byte(encoded), time.Duration(session.Options.MaxAge)*time.Second,
	).Err()
}

func (s *RedisStore) load(ctx context.Context, session *sessions.Session) error {

	cmd := s.client.Get(ctx, s.keyPrefix+session.ID)
	if cmd.Err() != nil {
		return cmd.Err()
	}

	b, err := cmd.Bytes()
	if err != nil {
		return err
	}

	return securecookie.DecodeMulti(
		session.Name(), string(b),
		&session.Values, s.Codecs...)
}

func (s *RedisStore) delete(ctx context.Context, session *sessions.Session) error {
	return s.client.Del(ctx, s.keyPrefix+session.ID).Err()
}
