package server

import (
	"net/http"
	"path"
	"strings"

	"github.com/gorilla/csrf"
	log "github.com/sirupsen/logrus"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/internal/auth"
)

const (
	csrfCookieName = "__csrf_key"
	csrfFieldName  = "__csrf__"
	csrfHeaderName = "X-CSRF-Token"
)

// Csrf setup the CSRF protection.
func (s *Server) Csrf() func(next http.Handler) http.Handler {
	CSRF := csrf.Protect(configs.CsrfKey(),
		csrf.CookieName(csrfCookieName),
		csrf.Path(path.Join(s.BasePath)),
		csrf.HttpOnly(true),
		csrf.MaxAge(0),
		csrf.SameSite(csrf.SameSiteStrictMode),
		csrf.FieldName(csrfFieldName),
		csrf.RequestHeader(csrfHeaderName),
	)

	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			// Always enable CSRF protection, unless the current auth provider
			// states otherwise.
			p, ok := auth.GetRequestProvider(r).(auth.FeatureCsrfProvider)
			if ok && p.CsrfExempt(r) {
				next.ServeHTTP(w, r)
				return
			}
			CSRF(next).ServeHTTP(w, r)
		}

		return http.HandlerFunc(fn)
	}
}

// WithPermission enforce a permission check on the request's path for
// the given action.
//
// In the RBAC configuration, the user's group is the subject, the
// request's path is the object and "act" is the action.
func (s *Server) WithPermission(act string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u := auth.GetRequestUser(r)
			p := "/" + strings.TrimPrefix(r.URL.Path, s.BasePath)
			ok := u.HasPermission(p, act)

			logger := s.Log(r).WithFields(log.Fields{
				"user":    u.Username,
				"sub":     u.Group,
				"obj":     p,
				"act":     act,
				"granted": ok,
			})

			if s.Log(r).Logger.IsLevelEnabled(log.DebugLevel) {
				logger.WithField("roles", u.Roles()).Debug("access control")
			}

			if !ok {
				logger.Warn("access denied")
				w.WriteHeader(http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
