package assets

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/internal/server"
)

type ctxNameKey struct{}

const svgGradient = `<?xml version="1.0" encoding="UTF-8"?>` +
	`<svg xmlns="http://www.w3.org/2000/svg" version="1.1" viewBox="0 0 256 160" width="256" height="160">` +
	`<defs>` +
	`<linearGradient id="gradient" x1="30%%" x2="70%%" y2="100%%">` +
	`<stop stop-color="hsl(%d, 70%%, 50%%)" offset="0"/>` +
	`<stop stop-color="hsl(%d, 70%%, 50%%)" offset="0.7"/>` +
	`<stop stop-color="hsl(%d, 70%%, 50%%)" offset="1"/>` +
	`</linearGradient>` +
	`</defs>` +
	`<rect width="100%%" height="100%%" fill="url(#gradient)"/>` +
	`</svg>`

type hashCode int

func (c hashCode) GetSumStrings() []string {
	return []string{fmt.Sprintf("%d", c)}
}

func (c hashCode) GetLastModified() []time.Time {
	return []time.Time{configs.BuildTime()}
}

// randomSvg sends an SVG image with a gradient. The gradient's color
// is based on the name.
func randomSvg(s *server.Server) http.Handler {
	r := chi.NewRouter()

	withHashCode := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			name := chi.URLParam(r, "name")
			data := uint64(0)
			for _, b := range []byte(name) {
				data = (data << 8) | uint64(b)
			}
			hc := hashCode(data) % 360
			ctx := context.WithValue(r.Context(), ctxNameKey{}, hc)

			s.WriteEtag(w, hc)
			s.WriteLastModified(w, hc)
			s.WithCaching(next).ServeHTTP(w, r.WithContext(ctx))
		})
	}

	r.With(withHashCode).Get("/", func(w http.ResponseWriter, r *http.Request) {
		c1 := r.Context().Value(ctxNameKey{}).(hashCode)
		c2 := (c1 + 120) % 360
		c3 := (c2 + 25) % 360

		w.Header().Set("Content-Type", "image/svg+xml")
		fmt.Fprintf(w, svgGradient, c1, c2, c3)
	})
	return r
}
