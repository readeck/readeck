package bookmarks

import (
	"net/http"
	"net/url"
	"path"

	"github.com/go-chi/chi/v5"

	"codeberg.org/readeck/readeck/internal/server"
	"codeberg.org/readeck/readeck/pkg/zipfs"
)

type ctxKey struct{}

var (
	ctxBookmarkKey = &ctxKey{}
)

// SetupRoutes mounts the routes for the bookmarks domain.
// "/bm" is a public route outside the api scope in order to avoid
// sending the session cookie.
func SetupRoutes(s *server.Server) {
	// Routes
	// Saved bookmark resources (images & all)
	s.AddRoute("/bm", mediaRoutes(s))

	// API routes
	api := newBookmarkAPI(s)
	s.AddRoute("/api/bookmarks", api)

	// Website routes
	s.AddRoute("/bookmarks", newBookmarkViews(api))
}

// mediaRoutes serves files from a bookmark's saved archive. It reads
// directly from the zip file and returns the requested file's content.
func mediaRoutes(_ *server.Server) http.Handler {
	r := chi.NewRouter()
	r.Get("/{dom}/{d}/{uid}/{p:^(img|_resources)$}/{name}", func(w http.ResponseWriter, r *http.Request) {
		p := path.Join(
			chi.URLParam(r, "p"),
			chi.URLParam(r, "name"),
		)
		p = path.Clean(p)

		r2 := new(http.Request)
		*r2 = *r
		r2.URL = new(url.URL)
		*r2.URL = *r.URL
		r2.URL.Path = p

		zipfile := path.Join(
			StoragePath(),
			chi.URLParam(r, "dom"),
			chi.URLParam(r, "d"),
			chi.URLParam(r, "uid")+".zip",
		)

		fs := zipfs.HTTPZipFile(zipfile)
		fs.ServeHTTP(w, r2)
	})

	return r
}
