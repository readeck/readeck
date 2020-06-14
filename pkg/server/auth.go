package server

import (
	"context"
	"net/http"

	"github.com/doug-martin/goqu/v9"
	"github.com/go-chi/chi"

	"github.com/readeck/readeck/pkg/auth"
)

type ctxKeyUser struct{}

var (
	ctxUserKey = &ctxKeyUser{}
)

// WithAuth checks if the user is authenticated.
func (s *Server) WithAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var userID int
		sess := s.GetSession(r)

		userID, ok := sess.Values["user_id"].(int)
		if !ok {
			sess.Options.MaxAge = -1
			sess.Save(r, w)
			s.TextMessage(w, r, 401, http.StatusText(401))
			return
		}

		user, err := auth.Users.GetOne(goqu.C("id").Eq(userID))
		if err != nil {
			sess.Options.MaxAge = -1
			sess.Save(r, w)
			s.TextMessage(w, r, 401, http.StatusText(401))
			return
		}

		// Add user to context
		ctx := context.WithValue(r.Context(), ctxUserKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetUser returns the user currently logged-in.
// It will panic (on purpose) if the route is not using the
// WithAuth() middleware.
func (s *Server) GetUser(r *http.Request) *auth.User {
	return r.Context().Value(ctxUserKey).(*auth.User)
}

//
// Unlike other modules, authentication routes are directly in
// the server module, making it easier to provide the server's
// GetUser() method and avoid circular imports.
//

// AuthRoutes returns the authentication and user profile routes.
func (s *Server) AuthRoutes() http.Handler {
	r := chi.NewRouter()
	r.Use(s.WithSession())

	type authPayload struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	// POST login
	r.Post("/login", func(w http.ResponseWriter, r *http.Request) {
		data := &authPayload{}
		if msg := s.LoadJSON(r, data); msg != nil {
			s.Message(w, r, msg)
			return
		}

		user, err := auth.Users.GetOne(goqu.C("username").Eq(data.Username))
		if err != nil {
			s.TextMessage(w, r, 401, http.StatusText(401))
			return
		}

		if !user.CheckPassword(data.Password) {
			s.TextMessage(w, r, 401, http.StatusText(401))
			return
		}

		sess := s.GetSession(r)
		sess.Values["user_id"] = user.ID
		if err := sess.Save(r, w); err != nil {
			panic(err)
		}

		s.TextMessage(w, r, 200, http.StatusText(200))
	})

	// POST logout
	r.With(s.WithAuth).Post("/logout", func(w http.ResponseWriter, r *http.Request) {
		sess := s.GetSession(r)
		sess.Options.MaxAge = -1
		sess.Save(r, w)

		s.Status(w, r, 204)
	})

	type profileResponse struct {
		Username string `json:"username"`
		Email    string `json:"email"`
	}

	// GET profile
	r.With(s.WithAuth).Get("/profile", func(w http.ResponseWriter, r *http.Request) {
		user := s.GetUser(r)

		// Renew session
		sess := s.GetSession(r)
		sess.Save(r, w)

		s.Render(w, r, 200, &profileResponse{
			user.Username,
			user.Email,
		})
	})

	return r
}
