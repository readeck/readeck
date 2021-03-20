package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"

	"codeberg.org/readeck/readeck/configs"
)

// Message is used by the server's Message() method.
type Message struct {
	Status  int     `json:"status"`
	Message string  `json:"message"`
	Errors  []Error `json:"errors,omitempty"`
}

// Error is mainly used to return payload/querystring errors.
type Error struct {
	Location string `json:"location"`
	Error    string `json:"error"`
}

// Render converts any value to JSON and sends the response.
func (s *Server) Render(w http.ResponseWriter, r *http.Request, status int, value interface{}) {
	b := &bytes.Buffer{}
	enc := json.NewEncoder(b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(value); err != nil {
		s.Log(r).WithError(err).Error()
		http.Error(w, http.StatusText(500), 500)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if status >= 100 {
		w.WriteHeader(status)
	}
	w.Write(b.Bytes())
}

// Message sends a JSON formatted message response.
func (s *Server) Message(w http.ResponseWriter, r *http.Request, message *Message) {
	s.Render(w, r, message.Status, message)

	// Log errors only in dev mode
	if message.Status >= 400 && configs.Config.Main.DevMode {
		s.Log(r).WithField("message", message).Warn(message.Message)
	}
}

// TextMessage sends a JSON formatted message response with a status and a message.
func (s *Server) TextMessage(w http.ResponseWriter, r *http.Request, status int, msg string) {
	s.Message(w, r, &Message{
		Status:  status,
		Message: msg,
	})
}

// Status sends a text plain response with the given status code.
func (s *Server) Status(w http.ResponseWriter, r *http.Request, status int) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	fmt.Fprintln(w, http.StatusText(status))
}

// Error sends an HTTP 500 and log the given error.
func (s *Server) Error(w http.ResponseWriter, r *http.Request, err error) {
	s.Log(r).WithError(err).Error("server error")
	s.Status(w, r, 500)
}

// CheckIfModifiedSince checks a if-modified-since header
// against the most recent of "mtimes" and returns true when it's before
// or equal the received time in the header.
func (s *Server) CheckIfModifiedSince(r *http.Request, mtimes ...time.Time) bool {
	if len(mtimes) == 0 {
		return false
	}

	ius := r.Header.Get("If-Modified-Since")
	if ius == "" {
		return false
	}
	t, err := http.ParseTime(ius)
	if err != nil {
		return false
	}

	sort.Slice(mtimes, func(i, j int) bool {
		return mtimes[i].After(mtimes[j])
	})

	mtime := mtimes[0].Truncate(time.Second)
	if mtime.Before(t) || mtime.Equal(t) {
		return true
	}
	return false
}

// SetLastModified sets the Last-Modified response header to the
// most recent dates of "mtimes".
func (s *Server) SetLastModified(w http.ResponseWriter, mtimes ...time.Time) {
	if len(mtimes) == 0 {
		return
	}

	sort.Slice(mtimes, func(i, j int) bool {
		return mtimes[i].After(mtimes[j])
	})

	w.Header().Set("Last-Modified", mtimes[0].Format(http.TimeFormat))
}
