package server

import (
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/Masterminds/sprig"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	log "github.com/sirupsen/logrus"

	"codeberg.org/readeck/readeck/assets"
	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/internal/auth"
	"codeberg.org/readeck/readeck/internal/auth/users"
	"codeberg.org/readeck/readeck/pkg/glob"
)

// Server is a wrapper around chi router.
type Server struct {
	Router   *chi.Mux
	BasePath string
}

// New create a new server. Routes must be added manually before
// calling ListenAndServe.
func New(basePath string) *Server {
	basePath = path.Clean("/" + basePath)
	if !strings.HasSuffix(basePath, "/") {
		basePath += "/"
	}

	s := &Server{
		Router:   chi.NewRouter(),
		BasePath: basePath,
	}

	s.Router.Use(
		middleware.Recoverer,
		middleware.RealIP,
		middleware.RequestID,
		Logger(),
		s.InitRequest,
		auth.Init(
			&auth.BasicAuthProvider{},
			&auth.TokenAuthProvider{},
			&auth.SessionAuthProvider{
				GetSession: s.GetSession,
				Redirect: func(w http.ResponseWriter, r *http.Request) {
					s.Redirect(w, r, "/login")
				},
			},
		),
		s.ErrorPages,
		s.SetSecurityHeaders(),
	)

	// Init templates
	s.TemplateFuncs(sprig.FuncMap())
	s.TemplateFuncs(template.FuncMap{
		"assetURL": func(ctx TC, name string) string {
			r := ctx["request"].(*http.Request)
			return s.AssetURL(r, name)
		},
		"cp": func(out io.Writer, in io.Reader) (string, error) {
			_, err := io.Copy(out, in)
			return "", err
		},
		"hasPermission": func(ctx TC, obj, act string) bool {
			user, ok := ctx["user"].(*users.User)
			if !ok || user == nil {
				return false
			}
			return user.HasPermission(obj, act)
		},
		"icon": func(ctx TC, name string, args ...interface{}) template.HTML {
			r := ctx["request"].(*http.Request)
			attrs := ""
			if len(args)%2 == 0 {
				seenClass := false
				for i := 0; i < len(args); i += 2 {
					seenClass = seenClass || args[i] == "class"
					attrs = fmt.Sprintf(`%s %s="%s"`, attrs, args[i], args[i+1])
				}
				if !seenClass {
					attrs = fmt.Sprintf(`%s %s="%s"`, attrs, "class", "svgicon")
				}
			}

			return template.HTML(
				fmt.Sprintf(
					svgTemplate,
					attrs, s.AssetURL(r, "img/icons.svg"), name),
			)
		},
		"pathIs": func(ctx TC, patterns ...string) bool {
			r := ctx["request"].(*http.Request)
			cp := "/" + strings.TrimPrefix(r.URL.Path, s.BasePath)
			for _, p := range patterns {
				if glob.Glob(p, cp) {
					return true
				}
			}
			return false
		},
		"safeAttr": func(val string) template.HTMLAttr {
			return template.HTMLAttr(val)
		},
		"urlFor": func(ctx TC, name ...string) string {
			r := ctx["request"].(*http.Request)
			return s.AbsoluteURL(r, name...).Path
		},
	})

	return s
}

// AuthenticatedRouter returns a chi.Router instance
// with middlewares to force authentication.
func (s *Server) AuthenticatedRouter() chi.Router {
	r := chi.NewRouter()
	r.Use(
		s.WithSession(),
		auth.Required,
		s.Csrf(),
	)

	return r
}

// AddRoute adds a new route to the server, prefixed with
// the BasePath.
func (s *Server) AddRoute(pattern string, handler http.Handler) {
	s.Router.Mount(path.Join(s.BasePath, pattern), handler)
}

// ListenAndServe starts the HTTP server
func (s *Server) ListenAndServe() error {
	srv := &http.Server{
		Addr:           fmt.Sprintf("%s:%d", configs.Config.Server.Host, configs.Config.Server.Port),
		Handler:        s.Router,
		MaxHeaderBytes: 1 << 20,
	}

	// System routes
	s.AddRoute("/api/sys", s.sysRoutes())

	// Add the profiler in dev mode
	if configs.Config.Main.DevMode {
		s.AddRoute("/debug", s.debugRoutes())
	}

	// Init templates
	s.initTemplates()

	return srv.ListenAndServe()
}

// AbsoluteURL resolve the absolute URL for the given ref path parts.
// If the ref starts with "./", it will resolve relative to the current
// URL.
func (s *Server) AbsoluteURL(r *http.Request, parts ...string) *url.URL {
	// First deal with parts
	for i, p := range parts {
		if i == 0 && strings.HasPrefix(p, "./") {
			p = "."
		}
		if i > 0 {
			parts[i] = strings.TrimLeft(p, "/")
		}
	}

	pathName := strings.Join(parts, "/")

	cur, _ := r.URL.Parse("")

	p, _ := url.Parse(pathName) // Never let a full URL pass in the parts
	pathName = p.Path

	// If the url is relative, we need a final slash on the original path
	if strings.HasPrefix(pathName, "./") && !strings.HasSuffix(cur.Path, "/") {
		cur.Path += "/"
	}

	// If the url is absolute, we must prepend the basePath
	if strings.HasPrefix(pathName, "/") {
		pathName = s.BasePath + pathName[1:]
	}

	// Append query string if any
	if p.RawQuery != "" {
		pathName += "?" + p.RawQuery
	}

	var u *url.URL
	var err error
	if u, err = url.Parse(pathName); err != nil {
		return r.URL
	}

	return cur.ResolveReference(u)
}

// CurrentPath returns the path of the current request
// after striping the server's base path. This value
// can later be used in the AbsoluteURL
// or Redirect functions.
func (s *Server) CurrentPath(r *http.Request) string {
	p := strings.TrimPrefix(r.URL.Path, s.BasePath)
	p = "/" + p
	if r.URL.RawQuery != "" {
		p += "?" + r.URL.RawQuery
	}

	return p
}

// AssetURL returns the real URL for a given asset.
func (s *Server) AssetURL(r *http.Request, name string) string {
	return s.AbsoluteURL(r, "/assets", assets.AssetMap()[name]).String()
}

// IsTurboRequest returns true when the request was made with
// an x-turbo header.
func (s *Server) IsTurboRequest(r *http.Request) bool {
	return r.Header.Get("x-turbo") == "1"
}

// Redirect yields a 303 redirection with a location header.
// The given "ref" values are joined togegher with the server's base path
// to provide a full absolute URL.
func (s *Server) Redirect(w http.ResponseWriter, r *http.Request, ref ...string) {
	w.Header().Set("Location", s.AbsoluteURL(r, ref...).String())
	w.WriteHeader(http.StatusSeeOther)
}

// Log returns a log entry including the request ID
func (s *Server) Log(r *http.Request) *log.Entry {
	return log.WithField("@id", s.GetReqID(r))
}

// sysRoutes returns the route returning some system
// information.
func (s *Server) sysRoutes() http.Handler {
	r := s.AuthenticatedRouter()
	r.Use(s.WithPermission("read"))

	type memInfo struct {
		Alloc      uint64 `json:"alloc"`
		TotalAlloc uint64 `json:"totalalloc"`
		Sys        uint64 `json:"sys"`
		NumGC      uint32 `json:"numgc"`
	}

	type sysInfo struct {
		Version   string    `json:"version"`
		BuildDate time.Time `json:"build_date"`
		OS        string    `json:"os"`
		Platform  string    `json:"platform"`
		Hostname  string    `json:"hostname"`
		CPUs      int       `json:"cpus"`
		GoVersion string    `json:"go_version"`
		Mem       memInfo   `json:"mem"`
	}

	bToMb := func(b uint64) uint64 {
		return b / 1024 / 1024
	}

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		host, _ := os.Hostname()

		res := sysInfo{
			Version:   configs.Version(),
			BuildDate: configs.BuildTime(),
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

func (s *Server) debugRoutes() http.Handler {
	r := s.AuthenticatedRouter()
	r.Use(s.WithPermission("read"))
	r.Mount("/", middleware.Profiler())
	return r
}

// GetReqID returns the request ID.
func (s *Server) GetReqID(r *http.Request) string {
	return middleware.GetReqID(r.Context())
}
