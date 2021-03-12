package assets

import (
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/readeck/readeck/assets"
	"github.com/readeck/readeck/configs"
	"github.com/readeck/readeck/internal/server"
)

var (
	reAssetHashed = regexp.MustCompile(`\.[a-z0-9]{8}\.[a-z]+$`)
)

// SetupRoutes setup the static asset routes on /assets
func SetupRoutes(s *server.Server) {
	s.AddRoute("/assets", serveAssets())
}

func serveAssets() http.HandlerFunc {
	fs := directFileServer{assets.StaticFilesFS()}

	return func(w http.ResponseWriter, r *http.Request) {
		rctx := chi.RouteContext(r.Context())
		pathPrefix := strings.TrimSuffix(rctx.RoutePattern(), "/*")
		p := strings.TrimPrefix(r.URL.Path, pathPrefix)

		r2 := new(http.Request)
		*r2 = *r
		r2.URL = new(url.URL)
		*r2.URL = *r.URL
		r2.URL.Path = p

		fs.ServeHTTP(w, r2)
	}
}

// directFileServer implements http.FileServer without the magic.
// no redirect */index.html to */ and no directory listing.
type directFileServer struct {
	root http.FileSystem
}

func (f *directFileServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fd, err := f.root.Open(r.URL.Path)
	if err != nil && os.IsNotExist(err) {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, http.StatusText(500), 500)
		return
	}
	defer fd.Close()

	st, err := fd.Stat()
	if st.IsDir() {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}

	// Go 1.16 did not implement any form of caching control for embed.FS.
	// Since all the files in assets/www have a hash fragment, we're just going to
	// use it for caching.
	if reAssetHashed.MatchString(path.Base(st.Name())) {
		w.Header().Set("Cache-Control", `public, max-age=31536000`)
	}

	// And in any case, we use the build time as Last-Modified
	http.ServeContent(w, r, st.Name(), configs.BuildTime(), fd)
}
