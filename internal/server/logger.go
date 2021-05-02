package server

import (
	"bytes"
	"net/http"
	"time"

	"github.com/fatih/color"
	"github.com/go-chi/chi/v5/middleware"
	log "github.com/sirupsen/logrus"

	"github.com/readeck/readeck/configs"
)

type httpLogFormatter struct{}

func (f *httpLogFormatter) Format(entry *log.Entry) ([]byte, error) {
	var b bytes.Buffer

	w := color.New(color.FgWhite)
	c := color.New(color.FgCyan)
	bl := color.New(color.FgBlue)

	w.Fprint(&b, "[HTTP")
	if reqID, ok := entry.Data["@id"]; ok {
		bl.Fprintf(&b, " %s", reqID)
	}
	w.Fprint(&b, "] ")

	met := entry.Data["http_method"].(string)
	switch met {
	case "GET", "HEAD":
		color.New(color.Bold, color.FgHiBlue).Fprint(&b, met)
	case "POST":
		color.New(color.Bold, color.FgHiGreen).Fprint(&b, met)
	case "PATCH", "PUT":
		color.New(color.Bold, color.FgYellow).Fprint(&b, met)
	case "DELETE":
		color.New(color.Bold, color.FgRed).Fprint(&b, met)
	default:
		color.New(color.Bold, color.FgHiWhite).Fprint(&b, met)
	}

	w.Fprintf(&b, " %s ", entry.Data["path"])

	status := entry.Data["status"].(int)
	switch {
	case status < 200:
		color.New(color.FgBlue).Fprint(&b, status)
	case status < 300:
		color.New(color.FgGreen).Fprint(&b, status)
	case status < 400:
		color.New(color.FgCyan).Fprint(&b, status)
	case status < 500:
		color.New(color.FgYellow).Fprint(&b, status)
	default:
		color.New(color.FgRed).Fprint(&b, status)
	}

	c.Fprintf(&b, " %d", entry.Data["length"])
	w.Fprint(&b, " in ")

	elapsed := time.Duration(entry.Data["elapsed_ms"].(float64) * 1000000.0)
	switch {
	case elapsed < 500*time.Millisecond:
		color.New(color.FgGreen).Fprint(&b, elapsed)
	case elapsed < 1*time.Second:
		color.New(color.FgYellow).Fprint(&b, elapsed)
	default:
		color.New(color.FgRed).Fprint(&b, elapsed)
	}

	b.WriteString("\n")
	return b.Bytes(), nil
}

// Logger is a middleware that logs requests.
func Logger() func(next http.Handler) http.Handler {
	return middleware.RequestLogger(newLogger())
}

func newLogger() *structuredLogger {
	l := &structuredLogger{}
	if configs.Config.Main.DevMode {
		color.NoColor = false
		l.logger = log.New()
		l.logger.Formatter = &httpLogFormatter{}
		l.logger.Level = log.StandardLogger().Level
	} else {
		l.logger = log.StandardLogger()
	}

	return l
}

type structuredLogger struct {
	logger *log.Logger
}

func (sl *structuredLogger) NewLogEntry(r *http.Request) middleware.LogEntry {
	le := sl.logger.WithField("@id", middleware.GetReqID(r.Context())).
		WithFields(log.Fields{
			"http_method": r.Method,
			"http_proto":  r.Proto,
			"remote_addr": r.RemoteAddr,
			"path":        r.RequestURI,
			"ua":          r.UserAgent(),
		})
	e := &structuredLoggerEntry{r, le}

	return e
}

type structuredLoggerEntry struct {
	r *http.Request
	e *log.Entry
}

func (l *structuredLoggerEntry) Write(status, bytes int, _ http.Header, elapsed time.Duration, _ interface{}) {
	l.e.WithFields(log.Fields{
		"status":     status,
		"length":     bytes,
		"elapsed_ms": float64(elapsed.Nanoseconds()) / 1000000.0,
	}).Info("http")
}

func (l *structuredLoggerEntry) Panic(_ interface{}, _ []byte) {
}
