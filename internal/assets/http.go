package assets

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/readeck/readeck/assets"
	"github.com/readeck/readeck/configs"
	"github.com/readeck/readeck/internal/server"
	"github.com/readeck/readeck/pkg/accept"
)

var (
	reAssetHashed = regexp.MustCompile(`\.[a-z0-9]{8}\.[a-z]+$`)
)

// SetupRoutes setup the static asset routes on /assets
func SetupRoutes(s *server.Server) {
	s.AddRoute("/assets", serveAssets())
	s.AddRoute("/assets/rnd/{name}.svg", randomSvg(s))
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

var canditateEncodings = [][2]string{
	{"br", ".br"},
	{"gzip", ".gz"},
	{"", ""},
}

// directFileServer implements http.FileServer without the magic.
// no redirect */index.html to */ and no directory listing.
type directFileServer struct {
	root http.FileSystem
}

func (f *directFileServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	mtime := configs.BuildTime().Truncate(time.Second)

	if r.URL.Path == "/manifest.json" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Super shortchut for If-Modified-Since
	ims := r.Header.Get("If-Modified-Since")
	t, err := http.ParseTime(ims)
	if err == nil {
		t = t.Truncate(time.Second)
		if mtime.Before(t) || mtime.Equal(t) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}

	name := filepath.Base(r.URL.Path)
	accepts := map[string]bool{}
	for _, x := range accept.ParseAccept(r.Header, "Accept-Encoding") {
		accepts[x.Value] = true
	}

	// Loop over all encoding candidates and return the first matching file
	// with the corresponding Content-Encoding when applicable.
	for _, x := range canditateEncodings {
		encoding := x[0]
		ext := x[1]
		last := encoding == ""

		if _, ok := accepts[encoding]; !ok && !last {
			continue
		}

		fd, err := f.root.Open(fmt.Sprintf("%s%s", r.URL.Path, ext))
		if err != nil && os.IsNotExist(err) {
			if !last {
				continue
			}
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		} else if err != nil {
			if !last {
				continue
			}
			http.Error(w, http.StatusText(500), 500)
			return
		}

		defer fd.Close()
		st, err := fd.Stat()
		if st.IsDir() {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		} else if err != nil {
			http.Error(w, http.StatusText(500), 500)
			return
		}

		// Go 1.16 did not implement any form of caching control for embed.FS.
		// Since all the files in assets/www have a hash fragment, we're just going to
		// use it for caching.
		if reAssetHashed.MatchString(path.Base(name)) {
			w.Header().Set("Cache-Control", `public, max-age=31536000`)
		}

		if encoding != "" {
			w.Header().Set("Content-Encoding", encoding)
		}

		// And in any case, we use the build time as Last-Modified
		http.ServeContent(w, r, name, mtime, fd)
		return
	}
}
