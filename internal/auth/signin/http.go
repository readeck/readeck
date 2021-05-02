package signin

import (
	"errors"
	"net/http"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/go-chi/chi/v5"

	"github.com/readeck/readeck/internal/auth/users"
	"github.com/readeck/readeck/internal/server"
	"github.com/readeck/readeck/pkg/form"
)

// SetupRoutes mounts the routes for the auth domain.
func SetupRoutes(s *server.Server) {
	newAuthHandler(s)

	api := newAuthAPI(s)
	s.AddRoute("/api/auth", api)
}

type authHandler struct {
	chi.Router
	srv *server.Server
}

func newAuthHandler(s *server.Server) *authHandler {
	// Non authenticated routes
	r := chi.NewRouter()
	r.Use(
		s.WithSession(),
		s.Csrf(),
	)

	h := &authHandler{r, s}
	s.AddRoute("/login", r)
	r.Get("/", h.loginView)
	r.Post("/", h.login)

	// Authenticated routes
	ar := s.AuthenticatedRouter()
	s.AddRoute("/logout", ar)

	ar.Post("/", h.logout)

	return h
}

func (h *authHandler) loginView(w http.ResponseWriter, r *http.Request) {
	u := &loginForm{}
	f := form.NewForm(u)

	h.renderLoginForm(w, r, 200, f)
}

func (h *authHandler) login(w http.ResponseWriter, r *http.Request) {
	u := new(loginForm)
	f := form.NewForm(u)

	form.Bind(f, r)

	if !f.IsValid() {
		h.renderLoginForm(w, r, http.StatusBadRequest, f)
		return
	}

	user, err := users.Users.GetOne(goqu.C("username").Eq(u.Username))
	if err != nil || !user.CheckPassword(u.Password) {
		f.Errors.Add(errors.New("Invalid user and/or password"))
		h.renderLoginForm(w, r, http.StatusUnauthorized, f)
		return
	}

	// User is authenticated, let's carry on
	sess := h.srv.GetSession(r)
	sess.Values["u"] = user.ID
	sess.Values["s"] = user.Seed
	sess.Values["t"] = time.Now().Unix()
	sess.Save(r, w)

	h.srv.Redirect(w, r, "/")
}

func (h *authHandler) renderLoginForm(w http.ResponseWriter, r *http.Request, status int, f *form.Form) {
	h.srv.RenderTemplate(w, r, status, "/auth/login", map[string]interface{}{
		"Form": f,
	})
}

func (h *authHandler) logout(w http.ResponseWriter, r *http.Request) {
	sess := h.srv.GetSession(r)
	sess.Options.MaxAge = -1
	if err := sess.Save(r, w); err != nil {
		h.srv.Error(w, r, err)
		return
	}

	h.srv.Redirect(w, r, "/")
}

type loginForm struct {
	Username string `json:"username" conform:"trim"`
	Password string `json:"password"`
}

func (lf *loginForm) Validate(f *form.Form) {
	f.Fields["username"].Validate(form.IsRequired)
	f.Fields["password"].Validate(form.IsRequired)
}
