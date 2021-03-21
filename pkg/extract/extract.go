package extract

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/html"

	"github.com/go-shiori/dom"
	log "github.com/sirupsen/logrus"
)

type (
	// ProcessStep defines a type of process applied during extraction
	ProcessStep int

	// Processor is the process function
	Processor func(*ProcessMessage, Processor) Processor

	// ProcessList holds the processes that will be applied
	ProcessList []Processor

	// ProcessMessage holds the process message that is passed (and changed)
	// by the subsequent processes.
	ProcessMessage struct {
		Context   context.Context
		Extractor *Extractor
		Position  int
		Log       *log.Entry
		Dom       *html.Node

		step     ProcessStep
		canceled bool
		values   map[string]interface{}
	}
)

const (
	// StepStart happens before the connection is made.
	StepStart ProcessStep = iota + 1

	// StepBody happens after receiving the resource body.
	StepBody

	// StepDom happens after parsing the resource DOM tree.
	StepDom

	// StepFinish happens at the very end of the extraction.
	StepFinish

	// StepPostProcess happens after looping over each Drop.
	StepPostProcess
)

// Step returns the current process step
func (m *ProcessMessage) Step() ProcessStep {
	return m.step
}

// Value returns a stored message value.
func (m *ProcessMessage) Value(name string) interface{} {
	return m.values[name]
}

// SetValue sets a new message value.
func (m *ProcessMessage) SetValue(name string, value interface{}) {
	m.values[name] = value
}

// ResetContent empty the message Dom and all the drops body
func (m *ProcessMessage) ResetContent() {
	m.Dom = nil
	m.Extractor.Drops()[m.Position].Body = []byte{}
}

func (m *ProcessMessage) Cancel(reason string, args ...interface{}) {
	m.Log.WithError(fmt.Errorf(reason, args...)).Error("operation canceled")
	m.canceled = true
}

// Error holds all the non-fatal errors that were
// caught during extraction.
type Error []error

func (e Error) Error() string {
	s := make([]string, len(e))
	for i, err := range e {
		s[i] = err.Error()
	}
	return strings.Join(s, ", ")
}

// URLList hold a list of URLs
type URLList map[string]bool

// Add adds a new URL to the list
func (l URLList) Add(v *url.URL) {
	c := *v
	c.Fragment = ""
	l[c.String()] = true
}

// IsPresent returns
func (l URLList) IsPresent(v *url.URL) bool {
	c := *v
	c.Fragment = ""
	return l[c.String()]
}

// Extractor is a page extractor.
type Extractor struct {
	URL     *url.URL
	HTML    []byte
	Text    string
	Visited URLList
	Logs    []string

	client     *http.Client
	processors ProcessList
	errors     Error
	drops      []*Drop
	Context    context.Context
	LogFields  *log.Fields
}

// New returns an Extractor instance for a given URL,
// with a default HTTP client.
func New(src string, html []byte) (*Extractor, error) {
	URL, err := url.Parse(src)
	if err != nil {
		return nil, err
	}
	URL.Fragment = ""

	res := &Extractor{
		URL:        URL,
		Visited:    URLList{},
		Context:    context.TODO(),
		client:     NewClient(),
		processors: ProcessList{},
		drops:      []*Drop{NewDrop(URL)},
	}

	if len(html) > 0 {
		res.drops[0].Body = html
	}

	return res, nil
}

// Client returns the extractor's HTTP client.
func (e *Extractor) Client() *http.Client {
	return e.client
}

// Errors returns the extractor's error list.
func (e *Extractor) Errors() Error {
	return e.errors
}

// AddError add a new error to the extractor's error list.
func (e *Extractor) AddError(err error) {
	e.errors = append(e.errors, err)
}

// Drops returns the extractor's drop list.
func (e *Extractor) Drops() []*Drop {
	return e.drops
}

// Drop return the extractor's first drop, when there is one.
func (e *Extractor) Drop() *Drop {
	if len(e.drops) == 0 {
		return nil
	}
	return e.drops[0]
}

// AddDrop adds a new Drop to the drop list.
func (e *Extractor) AddDrop(src *url.URL) {
	e.drops = append(e.drops, NewDrop(src))
}

// ReplaceDrop replaces the main Drop with a new one.
func (e *Extractor) ReplaceDrop(src *url.URL) error {
	if len(e.drops) != 1 {
		return errors.New("cannot replace a drop when there are more that one")
	}

	e.drops[0] = NewDrop(src)
	return nil
}

// AddProcessors adds extract processor(s) to the list
func (e *Extractor) AddProcessors(p ...Processor) {
	e.processors = append(e.processors, p...)
}

// NewProcessMessage returns a new ProcessMessage for a given step.
func (e *Extractor) NewProcessMessage(step ProcessStep) *ProcessMessage {
	logEntry := log.NewEntry(e.GetLogger())
	if e.LogFields != nil {
		logEntry = logEntry.WithFields(*e.LogFields)
	}

	return &ProcessMessage{
		Extractor: e,
		Log:       logEntry,
		step:      step,
		values:    make(map[string]interface{}),
	}
}

// GetLogger returns a logger for the extractor.
// This standard logger will copy everything to the
// extractor Log slice.
func (e *Extractor) GetLogger() *log.Logger {
	logger := log.New()
	logger.Formatter = log.StandardLogger().Formatter
	logger.Level = log.StandardLogger().Level
	logger.AddHook(&messageLogHook{e})

	return logger
}

// Run start the extraction process.
func (e *Extractor) Run() {
	i := 0
	m := e.NewProcessMessage(0)

	for i < len(e.drops) {
		d := e.drops[i]

		// Don't visit the same URL twice
		if e.Visited.IsPresent(d.URL) {
			i++
			continue
		}

		e.Visited.Add(d.URL)

		m.Position = i

		// Start extraction
		m.Log.WithField("idx", i).WithField("url", d.URL.String()).Info("start")
		m.step = StepStart
		e.runProcessors(m)
		if m.canceled {
			return
		}

		err := d.Load(e.client)
		if err != nil {
			m.Log.WithError(err).Error("cannot load resource")
			return
		}

		// First process pass
		m.Log.Debug("step body")
		m.step = StepBody
		e.runProcessors(m)
		if m.canceled {
			return
		}

		// Load the dom
		if d.IsHTML() {
			func() {
				doc, err := html.Parse(bytes.NewReader(d.Body))
				defer func() {
					m.Dom = nil
				}()

				if err != nil {
					m.Log.WithError(err).Error("cannot parse resource")
					return
				}

				m.Log.Debug("step DOM")
				m.Dom = doc
				m.step = StepDom
				e.runProcessors(m)
				if m.canceled {
					return
				}

				// Render the final document body
				if m.Dom != nil {
					buf := bytes.NewBuffer(nil)
					html.Render(buf, convertBodyNodes(m.Dom))
					d.Body = buf.Bytes()
				}
			}()
		}

		// Final processes
		m.Log.Debug("step finish")
		m.step = StepFinish
		e.runProcessors(m)
		if m.canceled {
			return
		}

		// A processor can change the position in the loop
		i = m.Position + 1
	}

	// Postprocess
	m.Log.Debug("postprocess")
	m.step = StepPostProcess
	e.setFinalHTML()
	e.runProcessors(m)
}

func (e *Extractor) runProcessors(m *ProcessMessage) {
	if e.processors == nil || len(e.processors) == 0 {
		return
	}

	p := e.processors[0]
	i := 0
	for {
		var next Processor
		i++
		if i < len(e.processors) {
			next = e.processors[i]
		}
		p = p(m, next)
		if p == nil {
			return
		}
	}
}

// convertBodyNodes extracts all the element from a
// document body and then returns a new HTML Document
// containing only the body's children.
func convertBodyNodes(top *html.Node) *html.Node {
	doc := &html.Node{
		Type: html.DocumentNode,
	}
	for _, node := range dom.GetElementsByTagName(top, "body") {
		for _, c := range dom.ChildNodes(node) {
			dom.AppendChild(doc, c)
		}
	}

	return doc
}

func (e *Extractor) setFinalHTML() {
	buf := &bytes.Buffer{}
	for i, d := range e.drops {
		if len(d.Body) == 0 {
			continue
		}
		fmt.Fprintf(buf, "<!-- page %d -->\n", i+1)
		buf.Write(d.Body)
		buf.WriteString("\n")
	}
	e.HTML = buf.Bytes()
}
