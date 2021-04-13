package rules

import (
	"embed"
	"fmt"
	"html"
	"io/fs"
	"io/ioutil"
	"net/url"
	"path"

	"github.com/dop251/goja"
	"github.com/sirupsen/logrus"
	"github.com/spyzhov/ajson"

	"codeberg.org/readeck/readeck/pkg/extract"
)

//go:embed site-rules/*
var assets embed.FS

// rulesFS returns the site-rules subfolder
func rulesFS() fs.FS {
	sub, err := fs.Sub(assets, "site-rules")
	if err != nil {
		panic(err)
	}
	return sub
}

type script struct {
	prog *goja.Program
	name string
}

var scripts []*script

// init compiles all the JS files to have them ready during extraction.
func init() {
	files, err := fs.ReadDir(rulesFS(), ".")
	if err != nil {
		panic(err)
	}

	for _, x := range files {
		if x.IsDir() || path.Ext(x.Name()) != ".js" {
			continue
		}

		f, err := rulesFS().Open(x.Name())
		if err != nil {
			panic(err)
		}

		b, err := ioutil.ReadAll(f)
		if err != nil {
			panic(err)
		}
		p := goja.MustCompile(x.Name(), string(b), true)
		scripts = append(scripts, &script{prog: p, name: x.Name()})
	}

}

// ApplyRules is a processor that applies a matching JS script.
// It gives more latitude to do some site by site improvements.
func ApplyRules(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
	if m.Step() != extract.StepDom || m.Position > 0 {
		return next
	}

	// // Init script and globals
	vm, err := getVM(m)
	if err != nil {
		m.Log.WithError(err).Error("script vm init")
		return next
	}

	for _, s := range scripts {
		vm.GlobalObject().Set("__name__", s.name)
		logger := m.Log.WithField("script", s.name)
		logger.Info("running rule script")

		if _, err := vm.RunProgram(s.prog); err != nil {
			logger.Error("rules script")
			continue
		}
	}

	return next
}

// getVM returns a JS vm with the available API the script can use.
func getVM(m *extract.ProcessMessage) (*goja.Runtime, error) {
	vm := goja.New()

	definitions := map[string]interface{}{
		"drop":    newDropWrapper(m.Extractor.Drop()),
		"console": newConsole(vm, m.Log).functions(),
		"$":       newUtilities(vm, m).functions(),
	}

	var err error
	for k, v := range definitions {
		if err = vm.Set(k, v); err != nil {
			return nil, err
		}
	}
	return vm, nil
}

// dropWrapper gives access to a "copy" of the current extract.Drop
// with some methods to perform write operations.
type dropWrapper struct {
	drop   *extract.Drop
	URL    *url.URL
	Domain string
	Meta   map[string][]string
}

// newDropWrapper returns an instance of dropWrapper.
func newDropWrapper(d *extract.Drop) *dropWrapper {
	urlCopy := *d.URL

	res := &dropWrapper{
		drop:   d,
		URL:    &urlCopy,
		Domain: d.Domain,
		Meta:   make(map[string][]string),
	}

	for k, v := range d.Meta {
		res.Meta[k] = v
	}

	return res
}

// SetMeta overwrite a meta value in the drop.
func (d *dropWrapper) SetMeta(name string, value string) {
	d.drop.Meta[name] = []string{value}
}

// SetDocumentType sets the document type.
func (d *dropWrapper) SetDocumentType(v string) {
	d.drop.DocumentType = v
}

// SetTitle sets the document title.
func (d *dropWrapper) SetTitle(v string) {
	d.drop.Title = v
}

// SetDescription sets the document description.
func (d *dropWrapper) SetDescription(v string) {
	d.drop.Description = v
}

// SetAuthor sets the document authors.
func (d *dropWrapper) SetAuthors(args ...string) {
	d.drop.Authors = args
}

// console is a console object to the JS vm.
type console struct {
	vm    *goja.Runtime
	entry *logrus.Entry
}

// newConsole returns a new console instance.
func newConsole(vm *goja.Runtime, entry *logrus.Entry) *console {
	return &console{vm: vm, entry: entry}
}

// functions returns a function map for common console functions.
func (c *console) functions() map[string]interface{} {
	return map[string]interface{}{
		"log":   c.log("info"),
		"debug": c.log("debug"),
		"error": c.log("error"),
	}
}

func (c *console) log(level string) func(...interface{}) {
	return func(args ...interface{}) {
		logger := c.entry.WithField("script", c.vm.GlobalObject().Get("__name__"))
		msg := []interface{}{}

		for _, x := range args {
			if fields, ok := x.(map[string]interface{}); ok {
				logger = logger.WithFields(fields)
			} else {
				msg = append(msg, x)
			}
		}

		var fn func(...interface{})
		switch level {
		case "debug":
			fn = logger.Debug
		case "error":
			fn = logger.Error
		default:
			fn = logger.Info
		}

		fn(msg...)
	}
}

// utilities is a tool belt providing some functions
type utilities struct {
	vm *goja.Runtime
	m  *extract.ProcessMessage
}

// newUtilities returns a utilities instance
func newUtilities(vm *goja.Runtime, m *extract.ProcessMessage) *utilities {
	return &utilities{vm: vm, m: m}
}

// functions returns a function map for utility function
func (u *utilities) functions() map[string]interface{} {
	return map[string]interface{}{
		"fetchJSON":   u.fetchJSON,
		"parseURL":    u.parseURL,
		"unescape":    html.UnescapeString,
		"unescapeURL": u.unescapeURL,
	}
}

// unescapeURL returns an URL with unescaped path and query string.
func (u *utilities) unescapeURL(src string) string {
	v, err := url.Parse(src)
	if err != nil {
		panic(u.vm.ToValue(err))
	}

	if v.RawQuery, err = url.QueryUnescape(v.RawQuery); err != nil {
		panic(u.vm.ToValue(err))
	}
	if v.Path, err = url.PathUnescape(v.Path); err != nil {
		panic(u.vm.ToValue(err))
	}

	return v.String()
}

// parseURL returns an URL instance.
func (u *utilities) parseURL(src string) *url.URL {
	res, err := url.Parse(src)
	if err != nil {
		panic(u.vm.ToValue(err))
	}
	return res
}

// fetchJSON fetch a json resource and returns a jsonNode instance that
// can be queried with JSONPath expressions.
func (u *utilities) fetchJSON(src string) *jsonNode {
	u.m.Log.
		WithField("script", u.vm.GlobalObject().Get("__name__")).
		WithField("url", src).
		Debug("script fetch")

	r, err := u.m.Extractor.Client().Get(src)
	if err != nil {
		panic(u.vm.ToValue(err))
	}
	defer r.Body.Close()
	if r.StatusCode/100 != 2 {
		panic(u.vm.ToValue(fmt.Errorf("invalid status code %d", r.StatusCode)))
	}
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		panic(u.vm.ToValue(err))
	}

	res, err := ajson.Unmarshal(buf)
	if err != nil {
		panic(u.vm.ToValue(err))
	}
	return &jsonNode{Node: res, vm: u.vm}
}

// jsonNode is a wrapper around ajson.Node
type jsonNode struct {
	*ajson.Node
	vm *goja.Runtime
}

// Get returns the first node value for a given path, or nil
func (n *jsonNode) Get(path string) interface{} {
	nodes, err := n.JSONPath(path)
	if err != nil {
		panic(n.vm.ToValue(err))
	}
	if len(nodes) == 0 {
		return nil
	}

	r, err := nodes[0].Value()
	if err != nil {
		panic(n.vm.ToValue(err))
	}
	return r
}
