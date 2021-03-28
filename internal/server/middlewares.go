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

// SetRequestInfo update the scheme and host on the incoming
// HTTP request URL (r.URL), based on provided headers and/or
// current environnement.
func SetRequestInfo(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		// Set full scheme and host value
		r.URL.Scheme = "http"
		if proto := r.Header.Get("x-forwarded-proto"); proto != "" {
			r.URL.Scheme = proto
		} else if r.TLS != nil {
			r.URL.Scheme = "https"
		}

		if host := r.Header.Get("x-forwarded-host"); host != "" {
			r.Host = host
		}
		r.URL.Host = r.Host

		next.ServeHTTP(w, r)
	}

	return http.HandlerFunc(fn)
}

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

// SetSecurity adds some headers to improve client side security.
func (s *Server) SetSecurity() func(next http.Handler) http.Handler {
	cspHeader := strings.Join([]string{
		"default-src 'self'",
		"img-src 'self' data:",
		"media-src 'self' data:",
		"style-src 'self' 'unsafe-inline'",
		"child-src *", // Allow iframes for videos
	}, "; ")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Referrer-Policy", "same-origin")
			w.Header().Add("Content-Security-Policy", cspHeader)

			next.ServeHTTP(w, r)
		})
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
