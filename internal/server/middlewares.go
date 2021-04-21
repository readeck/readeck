package server

import (
	"encoding/json"
	"net/http"
	"path"
	"strings"

	"github.com/gorilla/csrf"
	log "github.com/sirupsen/logrus"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/internal/auth"
	"codeberg.org/readeck/readeck/pkg/accept"
)

const (
	csrfCookieName = "__csrf_key"
	csrfFieldName  = "__csrf__"
	csrfHeaderName = "X-CSRF-Token"
)

var acceptOffers = []string{
	"text/plain",
	"text/html",
	"application/json",
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
				w.Header().Set("content-type", "text/plain; charset=utf-8")
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte("access denied"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ErrorPages is a middleware that overrides the response writer so
// that, under some conditions, it can send a response matching the
// "accept" request header.
//
// Conditions are: response status must be >= 400, its content-type
// is text/plain and it has some content.
func (s *Server) ErrorPages(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wi := &responseWriterInterceptor{
			ResponseWriter: w,
			r:              r,
			srv:            s,
			accept:         accept.NegotiateContentType(r.Header, acceptOffers, "text/html"),
		}

		next.ServeHTTP(wi, r)
	})
}

type responseWriterInterceptor struct {
	http.ResponseWriter
	r           *http.Request
	srv         *Server
	accept      string
	contentType string
	statusCode  int
}

// needsOverride returns true when a content-type is text/plain and status >= 400
func (w *responseWriterInterceptor) needsOverride() bool {
	return w.contentType == "text/plain" && w.statusCode >= 400
}

// WriteHeader intercepts the status code sent to the writter and saves some
// information if needed.
func (w *responseWriterInterceptor) WriteHeader(statusCode int) {
	defer func() {
		w.ResponseWriter.WriteHeader(statusCode)
	}()

	if statusCode < 400 { // immediate shortcut
		return
	}
	w.statusCode = statusCode

	if w.contentType == "" {
		w.contentType = "text/plain"
		ct := strings.SplitN(w.Header().Get("content-type"), ";", 2)
		if ct[0] != "" {
			w.contentType = ct[0]
		}
	}

	if w.needsOverride() {
		w.ResponseWriter.Header().Set("Content-Type", w.accept+"; charset=utf-8")
	}
}

// Write overrides the wrapped Write method to discard all contents and
// send its own response when it needs to.
func (w *responseWriterInterceptor) Write(c []byte) (int, error) {
	if !w.needsOverride() {
		return w.ResponseWriter.Write(c)
	}

	switch w.accept {
	case "application/json":
		b, _ := json.Marshal(Message{
			Status:  w.statusCode,
			Message: http.StatusText(w.statusCode),
		})
		return w.ResponseWriter.Write(b)
	case "text/html":
		ctx := TC{"Status": w.statusCode}
		w.srv.RenderTemplate(w.ResponseWriter, w.r, 0, "error", ctx)
	default:
		return w.ResponseWriter.Write([]byte(http.StatusText(w.statusCode)))
	}

	return 0, nil
}
