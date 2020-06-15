package server

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/middleware"
	log "github.com/sirupsen/logrus"
)

// Logger is a middleware that logs requests.
func Logger() func(next http.Handler) http.Handler {
	return middleware.RequestLogger(newLogger())
}

func newLogger() *structuredLogger {
	return &structuredLogger{}
}

type structuredLogger struct{}

func (l *structuredLogger) NewLogEntry(r *http.Request) middleware.LogEntry {
	le := log.WithField("@id", middleware.GetReqID(r.Context()))
	e := &structuredLoggerEntry{r, le}

	le.WithFields(log.Fields{
		"http_method": r.Method,
		"http_proto":  r.Proto,
		"remote_addr": r.RemoteAddr,
		"path":        r.RequestURI,
		"ua":          r.UserAgent(),
	}).Info("request started")

	return e
}

type structuredLoggerEntry struct {
	r *http.Request
	l *log.Entry
}

func (l *structuredLoggerEntry) Write(status, bytes int, header http.Header, elapsed time.Duration, extra interface{}) {
	l.l.WithFields(log.Fields{
		"status":     status,
		"length":     bytes,
		"elapsed_ms": float64(elapsed.Nanoseconds()) / 1000000.0,
	}).Info("request completed")
}

func (l *structuredLoggerEntry) Panic(v interface{}, stack []byte) {
}

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
