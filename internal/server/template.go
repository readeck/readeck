package server

import (
	"fmt"
	"html"
	"io"
	"io/fs"
	"net/http"
	"os"
	"reflect"
	"strings"

	"github.com/CloudyKit/jet/v6"
	"github.com/gorilla/csrf"

	"codeberg.org/readeck/readeck/assets"
	"codeberg.org/readeck/readeck/internal/auth"
	"codeberg.org/readeck/readeck/internal/auth/users"
	"codeberg.org/readeck/readeck/pkg/glob"
	"codeberg.org/readeck/readeck/pkg/libjet"
)

// TC is a simple type to carry template context.
type TC map[string]interface{}

// tplLoader implements a jet.Loader using fs.FS so we can use it
// with embed fs.
type tplLoader struct {
	fs.FS
}

// Exists returns true if the template exists in the filesystem.
func (l *tplLoader) Exists(templatePath string) bool {
	_, err := l.Open(templatePath)
	return err == nil && !os.IsNotExist(err)
}

// Open opens the template at the give path.
func (l *tplLoader) Open(templatePath string) (io.ReadCloser, error) {
	templatePath = strings.TrimPrefix(templatePath, "/")
	return l.FS.Open(templatePath)
}

// views holds all the views (templates)
var views *jet.Set

func init() {
	loader := &tplLoader{assets.TemplatesFS()}
	views = jet.NewSet(
		loader,
		jet.WithTemplateNameExtensions([]string{"", ".jet.html"}),
	)
}

// RenderTemplate yields an HTML response using the given template and context.
func (s *Server) RenderTemplate(w http.ResponseWriter, r *http.Request, status int,
	name string, ctx TC) {
	t, err := views.GetTemplate(name)
	if err != nil {
		s.Error(w, r, err)
		return
	}

	w.Header().Set("content-type", "text/html; charset=utf-8")
	w.WriteHeader(status)

	if err = t.Execute(w, s.templateVars(r), ctx); err != nil {
		panic(err)
	}
}

// RenderTurboStream yields an HTML response with turbo-stream content-type using the
// given template and context. The template result is enclosed in a turbo-stream
// tag with action and target as specified.
// You can call this method as many times as needed to output several turbo-stream tags
// in the same HTTP response.
func (s *Server) RenderTurboStream(
	w http.ResponseWriter, r *http.Request,
	name, action, target string, ctx interface{},
) {
	t, err := views.GetTemplate(name)
	if err != nil {
		s.Error(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "text/vnd.turbo-stream.html; charset=utf-8")

	fmt.Fprintf(w, `<turbo-stream action="%s" target="%s"><template>%s`, action, target, "\n")
	if err = t.Execute(w, s.templateVars(r), ctx); err != nil {
		panic(err)
	}
	fmt.Fprint(w, "</template></turbo-stream>\n\n")
}

// initTemplates add global functions to the views.
func (s *Server) initTemplates() {
	strType := reflect.TypeOf("")

	for k, v := range libjet.FuncMap() {
		views.AddGlobalFunc(k, v)
	}

	views.AddGlobal("rawCopy", func(out io.Writer, in io.Reader) {
		if _, err := io.Copy(out, in); err != nil {
			panic(err)
		}
	})

	views.AddGlobalFunc("assetURL", func(args jet.Arguments) reflect.Value {
		args.RequireNumOfArguments("assetURL", 1, 1)
		name := args.Get(0).String()
		r := args.Runtime().Resolve("request").Interface().(*http.Request)

		return reflect.ValueOf(s.AssetURL(r, name))
	})
	views.AddGlobalFunc("urlFor", func(args jet.Arguments) reflect.Value {
		parts := make([]string, args.NumOfArguments())
		for i := 0; i < args.NumOfArguments(); i++ {
			parts[i] = libjet.ToString(args.Get(i))
		}

		r := args.Runtime().Resolve("request").Interface().(*http.Request)
		return reflect.ValueOf(s.AbsoluteURL(r, parts...).Path)
	})
	views.AddGlobalFunc("pathIs", func(args jet.Arguments) reflect.Value {
		r := args.Runtime().Resolve("request").Interface().(*http.Request)
		cp := "/" + strings.TrimPrefix(r.URL.Path, s.BasePath)
		for i := 0; i < args.NumOfArguments(); i++ {
			if glob.Glob(fmt.Sprintf("%v", args.Get(i)), cp) {
				return reflect.ValueOf(true)
			}
		}
		return reflect.ValueOf(false)
	})
	views.AddGlobalFunc("hasPermission", func(args jet.Arguments) reflect.Value {
		args.RequireNumOfArguments("urlFor", 2, 2)
		obj := libjet.ToString(args.Get(0))
		act := libjet.ToString(args.Get(1))
		user, ok := args.Runtime().Resolve("user").Interface().(*users.User)
		if !ok || user == nil {
			return reflect.ValueOf(false)
		}

		return reflect.ValueOf(user.HasPermission(obj, act))
	})
	views.AddGlobalFunc("attrList", func(args jet.Arguments) reflect.Value {
		if args.NumOfArguments()%2 > 0 {
			panic("attrList(): incomplete key-value pair")
		}

		m := make([]string, args.NumOfArguments()/2)

		for i := 0; i < args.NumOfArguments(); i += 2 {
			k := args.Get(i)
			v := args.Get(i + 1)
			if !k.IsValid() {
				args.Panicf("attrList(): key argument at position %d is not a valid value!", i)
			}
			if !v.IsValid() {
				args.Panicf("attrList(): key argument at position %d is not a valid value!", i+1)
			}
			if !k.Type().ConvertibleTo(strType) {
				args.Panicf("attrList(): can't use %+v as string key: %s is not convertible to string", k, k.Type())
			}
			if !v.Type().ConvertibleTo(strType) {
				args.Panicf("attrList(): can't use %+v as string key: %s is not convertible to string", v, v.Type())
			}
			m[i/2] = fmt.Sprintf(`%s="%s"`, html.EscapeString(k.String()), html.EscapeString(v.String()))
		}

		return reflect.ValueOf(strings.Join(m, " "))
	})
}

// templateVars returns the default variables set for a template
// in the request's context.
func (s *Server) templateVars(r *http.Request) jet.VarMap {
	return make(jet.VarMap).
		Set("basePath", s.BasePath).
		Set("csrfName", csrfFieldName).
		Set("csrfToken", csrf.Token(r)).
		Set("currentPath", s.CurrentPath(r)).
		Set("request", r).
		Set("user", auth.GetRequestUser(r)).
		Set("flashes", s.Flashes(r))
}
