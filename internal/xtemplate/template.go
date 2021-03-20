package xtemplate

// Inspired by https://github.com/dannyvankooten/extemplate
// released under MIT license.
// This fork adds support for fs.FS so it plays nicely with embed files.

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	extendsRegex = regexp.MustCompile(`\{\{ *?extends +?"(.+?)" *?\}\}`)
)

// Xtemplate holds a reference to all templates
// and shared configuration like Delims or FuncMap
type Xtemplate struct {
	shared    *template.Template
	templates map[string]*template.Template
}

type templatefile struct {
	contents []byte
	layout   string
}

// New allocates a new, empty, template map
func New() *Xtemplate {
	shared := template.New("")
	return &Xtemplate{
		shared:    shared,
		templates: make(map[string]*template.Template),
	}
}

// Delims sets the action delimiters to the specified strings,
// to be used in subsequent calls to ParseDir.
// Nested template  definitions will inherit the settings.
// An empty delimiter stands for the corresponding default: {{ or }}.
// The return value is the template, so calls can be chained.
func (x *Xtemplate) Delims(left, right string) *Xtemplate {
	x.shared.Delims(left, right)
	return x
}

// Funcs adds the elements of the argument map to the template's function map.
// It must be called before templates are parsed
// It panics if a value in the map is not a function with appropriate return
// type or if the name cannot be used syntactically as a function in a template.
// It is legal to overwrite elements of the map. The return value is the Xtemplate instance,
// so calls can be chained.
func (x *Xtemplate) Funcs(funcMap template.FuncMap) *Xtemplate {
	x.shared.Funcs(funcMap)
	return x
}

// Lookup returns the template with the given name
// It returns nil if there is no such template or the template has no definition.
func (x *Xtemplate) Lookup(name string) *template.Template {
	if t, ok := x.templates[name]; ok {
		return t
	}

	return nil
}

// ExecuteTemplate applies the template named name to the specified data object and writes the output to wr.
func (x *Xtemplate) ExecuteTemplate(wr io.Writer, name string, data interface{}) error {
	tmpl := x.Lookup(name)
	if tmpl == nil {
		return fmt.Errorf("Xtemplate: no template %q", name)
	}

	return tmpl.Execute(wr, data)
}

// ParseFs walks the given directory root and parses all files with any of the registered extensions.
// Default extensions are .html and .tmpl
// If a template file has {{/* extends "other-file.tmpl" */}} as its first line it will parse that file for base templates.
// Parsed templates are named relative to the given root directory
func (x *Xtemplate) ParseFs(root fs.FS, extensions []string) error {
	var b []byte
	// var err error

	files, err := findTemplateFiles(root, extensions)
	if err != nil {
		return err
	}

	// parse all non-child templates into the shared template namespace
	for name, tf := range files {
		if tf.layout != "" {
			continue
		}

		_, err = x.shared.New(name).Parse(string(tf.contents))
		if err != nil {
			return err
		}
	}

	// then, parse all templates again but with inheritance
	for name, tf := range files {

		// if this is a non-child template, no need to re-parse
		if tf.layout == "" {
			x.templates[name] = x.shared.Lookup(name)
			continue
		}

		tmpl := template.Must(x.shared.Clone()).New(name)

		// add to set under normalized name (path from root)
		x.templates[name] = tmpl

		// parse parent templates
		templateFiles := []string{name}
		pname := tf.layout
		parent, parentExists := files[pname]
		for parentExists {
			templateFiles = append(templateFiles, pname)
			pname = parent.layout
			parent, parentExists = files[parent.layout]
		}

		// parse template files in reverse order (because childs should override parents)
		for j := len(templateFiles) - 1; j >= 0; j-- {
			b = files[templateFiles[j]].contents
			_, err = tmpl.Parse(string(b))
			if err != nil {
				return err
			}
		}

	}

	return nil
}

func findTemplateFiles(root fs.FS, extensions []string) (map[string]*templatefile, error) {
	var files = map[string]*templatefile{}
	var exts = map[string]bool{}

	// create map of allowed extensions
	for _, e := range extensions {
		exts[e] = true
	}

	// find all template files
	err := fs.WalkDir(root, ".", func(name string, info fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Skip directories
		if info.IsDir() {
			return nil
		}

		// skip if extension not in list of allowed extensions
		e := filepath.Ext(name)
		if _, ok := exts[e]; !ok {
			return nil
		}

		// read file into memory
		f, err := root.Open(name)
		if err != nil {
			return err
		}
		defer f.Close()

		contents, err := ioutil.ReadAll(f)
		if err != nil {
			return err
		}

		name = strings.TrimPrefix(name, "/")

		tf, err := newTemplateFile(contents)
		if err != nil {
			return err
		}

		files[name] = tf
		return nil
	})

	return files, err
}

// newTemplateFile parses the file contents into something that text/template can understand
func newTemplateFile(c []byte) (*templatefile, error) {
	tf := &templatefile{
		contents: c,
	}

	r := bytes.NewReader(tf.contents)
	pos := 0
	var line []byte
	for {
		ch, l, err := r.ReadRune()
		pos += l

		// read until first line or EOF
		if ch == '\n' || err == io.EOF {
			line = c[0:pos]
			break
		}
	}

	if len(line) < 10 {
		return tf, nil
	}

	// if we have a match, strip first line of content
	if m := extendsRegex.FindSubmatch(line); m != nil {
		tf.layout = string(m[1])
		tf.contents = c[len(line):]
	}

	return tf, nil
}
