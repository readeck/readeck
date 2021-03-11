package server

import (
	"html/template"
	"net/http"

	"github.com/gorilla/csrf"

	"github.com/readeck/readeck/internal/templates"
	"github.com/readeck/readeck/internal/xtemplate"
)

var xt *xtemplate.Xtemplate

const svgTemplate = `<span class="svgicon"><svg xmlns="http://www.w3.org/2000/svg" viewbox="0 0 100 100" width="16"><use xlink:href="%s#%s"></use></svg></span>`

// RenderTemplate yields an HTML response using the given template and context.
func (s *Server) RenderTemplate(w http.ResponseWriter, r *http.Request,
	status int, name string, context TC) {

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if status >= 100 {
		w.WriteHeader(status)
	}

	xt, err := s.getTemplates()
	if err != nil {
		panic(err)
	}

	err = xt.ExecuteTemplate(w, name, s.templatePayload(r, context))
	if err != nil {
		panic(err)
	}
}

// TC is a simple type to carry template context.
type TC map[string]interface{}

var templateFuncs = template.FuncMap{}

func (s *Server) initTemplates() {
	var err error
	xt, err = s.newTemplates()
	if err != nil {
		panic(err)
	}
}

func (s *Server) getTemplates() (*xtemplate.Xtemplate, error) {
	if xt != nil {
		return xt, nil
	}

	return s.newTemplates()
}

// TemplateFuncs adds a new function map to the template engine.
func (s *Server) TemplateFuncs(funcMap template.FuncMap) {
	for k, v := range funcMap {
		templateFuncs[k] = v
	}
}

func (s *Server) newTemplates() (*xtemplate.Xtemplate, error) {
	xt := xtemplate.New()
	xt.Funcs(templateFuncs)

	err := xt.ParseFs(templates.Templates, []string{".gohtml"})

	return xt, err
}

func (s *Server) templatePayload(r *http.Request, context TC) TC {
	res := TC{
		"basePath":    s.BasePath,
		"csrfName":    csrfFieldName,
		"csrfToken":   csrf.Token(r),
		"currentPath": s.CurrentPath(r),
		"request":     r,
		"assets":      map[string]string{},
		"flashes":     s.Flashes(r),
	}

	for k, v := range context {
		res[k] = v
	}

	return res
}
