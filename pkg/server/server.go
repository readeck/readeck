package server

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"runtime"
	"strings"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	log "github.com/sirupsen/logrus"

	"github.com/readeck/readeck/pkg/assets"
	"github.com/readeck/readeck/pkg/config"
)

// Server is a wrapper around chi router.
type Server struct {
	Router *chi.Mux
}

// New create a new server. Routes must be added manually before
// calling ListenAndServe.
func New() *Server {
	router := chi.NewRouter()
	router.Use(middleware.Recoverer)
	router.Use(middleware.RealIP)
	router.Use(middleware.RequestID)
	router.Use(Logger())
	router.Use(SetRequestInfo)

	router.Use(middleware.SetHeader("Referrer-Policy", "same-origin"))

	s := &Server{
		Router: router,
	}

	return s
}

// ListenAndServe starts the HTTP server
func (s *Server) ListenAndServe() {
	srv := &http.Server{
		Addr:           fmt.Sprintf("%s:%d", config.Config.Server.Host, config.Config.Server.Port),
		Handler:        s.Router,
		MaxHeaderBytes: 1 << 20,
	}

	// Add the profiler in dev mode
	if config.Config.Main.DevMode {
		s.Router.Mount("/debug", middleware.Profiler())
	}

	srv.ListenAndServe()
}

// AbsoluteURL resolve the absolute URL for the given ref path parts.
// If the ref starts with "./", it will resolve relative to the current
// URL.
func (s *Server) AbsoluteURL(r *http.Request, ref ...string) *url.URL {
	pathName := strings.Join(ref, "/")
	cur, _ := r.URL.Parse("")
	if strings.HasPrefix(pathName, "./") && !strings.HasSuffix(cur.Path, "/") {
		cur.Path = cur.Path + "/"
	}

	var u *url.URL
	var err error
	if u, err = url.Parse(pathName); err != nil {
		return r.URL
	}

	return cur.ResolveReference(u)
}

// Log returns a log entry including the request ID
func (s *Server) Log(r *http.Request) *log.Entry {
	return middleware.GetLogEntry(r).(*structuredLoggerEntry).l
}

// BaseRoutes returns the common routes for SPA assets
// and user media resources.
func (s *Server) BaseRoutes() http.Handler {
	r := chi.NewRouter()
	r.Handle("/*", s.serveAssets())
	r.Handle("/media/*", s.serveMedia())

	return r
}

func (s *Server) serveAssets() http.HandlerFunc {
	fs := directFileServer{assets.Assets}

	return func(w http.ResponseWriter, r *http.Request) {
		p := chi.URLParam(r, "*")
		p = strings.TrimLeft(p, "/")
		p = path.Clean(p)

		// Redirect /index.html to /
		if p == "index.html" {
			w.Header().Set("Location", "./")
			w.WriteHeader(http.StatusMovedPermanently)
			return
		}

		if p == "." || p == "/" {
			p = "index.html"
		}

		r2 := new(http.Request)
		*r2 = *r
		r2.URL = new(url.URL)
		*r2.URL = *r.URL
		r2.URL.Path = p

		// redirect non existent resources to "/"
		// so we can serve our SPA urls
		if f, err := fs.root.Open(r2.URL.Path); err != nil {
			if os.IsNotExist(err) {
				r2.URL.Path = "index.html"
			}
		} else {
			f.Close()
		}

		fs.ServeHTTP(w, r2)
	}
}

func (s *Server) serveMedia() http.HandlerFunc {
	fs := directFileServer{
		http.Dir(path.Join(config.Config.Main.DataDirectory, "files")),
	}

	return func(w http.ResponseWriter, r *http.Request) {
		p := chi.URLParam(r, "*")
		p = strings.TrimLeft(p, "/")
		p = path.Clean(p)

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
	if err != nil {
		http.Error(w, http.StatusText(500), 500)
		return
	}
	defer fd.Close()

	st, err := fd.Stat()
	if st.IsDir() {
		http.Error(w, http.StatusText(404), 404)
		return
	}

	http.ServeContent(w, r, st.Name(), st.ModTime(), fd)
}

// SysRoutes returns the route returning some system
// information.
func (s *Server) SysRoutes() http.Handler {
	r := chi.NewRouter()
	r.Use(s.WithSession(), s.WithAuth)

	type memInfo struct {
		Alloc      uint64 `json:"alloc"`
		TotalAlloc uint64 `json:"totalalloc"`
		Sys        uint64 `json:"sys"`
		NumGC      uint32 `json:"numgc"`
	}

	type sysInfo struct {
		OS        string  `json:"os"`
		Platform  string  `json:"platform"`
		Hostname  string  `json:"hostname"`
		CPUs      int     `json:"cpus"`
		GoVersion string  `json:"go_version"`
		Mem       memInfo `json:"mem"`
	}

	bToMb := func(b uint64) uint64 {
		return b / 1024 / 1024
	}

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		host, _ := os.Hostname()

		res := sysInfo{
			OS:        runtime.GOOS,
			Platform:  runtime.GOARCH,
			Hostname:  host,
			CPUs:      runtime.NumCPU(),
			GoVersion: runtime.Version(),
			Mem: memInfo{
				Alloc:      bToMb(m.Alloc),
				TotalAlloc: bToMb(m.TotalAlloc),
				Sys:        bToMb(m.Sys),
				NumGC:      m.NumGC,
			},
		}

		s.Render(w, r, 200, res)
	})

	return r
}
