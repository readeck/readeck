package assets

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/internal/server"
)

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

// randomSvg sends an SVG image with a gradient. The gradient's color
// is based on the name.
func randomSvg(s *server.Server) http.Handler {
	r := chi.NewRouter()

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		if s.CheckIfModifiedSince(r, configs.BuildTime()) {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		name := chi.URLParam(r, "name")
		c1 := hashCode(name)
		c2 := (c1 + 120) % 360
		c3 := (c2 + 25) % 360
		w.Header().Set("Content-Type", "image/svg+xml")
		s.SetLastModified(w, configs.BuildTime())
		w.Header().Set("Cache-Control", `public, max-age=31536000`)
		fmt.Fprintf(w, svgGradient, c1, c2, c3)
	})
	return r
}

func hashCode(input string) int {
	data := uint64(0)
	for _, b := range []byte(input) {
		data = (data << 8) | uint64(b)
	}
	return int(data) % 360
}
