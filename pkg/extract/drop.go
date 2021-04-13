package extract

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"
	"golang.org/x/text/encoding"
	"golang.org/x/text/transform"
)

var (
	rxAuthor = regexp.MustCompile(`^(?i)by(\s*:)?\s+`)
	rxSpaces = regexp.MustCompile(`\s+`)
)

// Drop is the result of a content extraction of one resource.
type Drop struct {
	URL          *url.URL
	Domain       string
	ContentType  string
	Charset      string
	DocumentType string

	Title       string
	Description string
	Authors     []string
	Site        string
	Lang        string
	Date        time.Time

	Header http.Header
	Meta   DropMeta
	Body   []byte `json:"-"`

	Pictures map[string]*Picture
}

// NewDrop returns a Drop instance.
func NewDrop(src *url.URL) *Drop {
	// First, copy url and ensure it's a unicode version
	var uri *url.URL
	domain := ""
	if src != nil {
		uri = new(url.URL)
		*uri = *src
		if host, err := idna.ToUnicode(uri.Host); err == nil {
			uri.Host = host
		}
		domain, _ = publicsuffix.EffectiveTLDPlusOne(uri.Hostname())
	}

	return &Drop{
		URL:      uri,
		Domain:   domain,
		Meta:     DropMeta{},
		Authors:  []string{},
		Body:     []byte{},
		Pictures: map[string]*Picture{},
	}
}

// Load loads the remote URL and retrieve data.
func (d *Drop) Load(client *http.Client) error {
	if d.URL == nil {
		return fmt.Errorf("No document URL")
	}

	if len(d.Body) > 0 {
		// If we have a body already, we don't load anything and
		// just go with it.
		d.Site = d.URL.Hostname()
		d.ContentType = "text/html"
		d.Charset = "utf-8"
		return nil
	}

	if client == nil {
		client = http.DefaultClient
	}
	defer client.CloseIdleConnections()

	var err error
	var rsp *http.Response

	if rsp, err = client.Get(d.URL.String()); err != nil {
		return err
	}
	defer rsp.Body.Close()

	// Save headers
	d.Header = rsp.Header

	if rsp.StatusCode/100 != 2 {
		return fmt.Errorf("Invalid status code (%d)", rsp.StatusCode)
	}

	// Set final URL in case it was redirected
	d.URL = rsp.Request.URL

	// Set mime type
	d.ContentType, _, _ = mime.ParseMediaType(rsp.Header.Get("content-type"))

	// Set site
	d.Site = d.URL.Hostname()

	if !d.IsHTML() {
		return nil
	}

	return d.loadHTMLBody(rsp)
}

// IsHTML returns true when the resource is of type HTML
func (d *Drop) IsHTML() bool {
	return d.ContentType == "text/html" || d.ContentType == "application/xhtml+xml"
}

// IsMedia returns true when the document type is a media type
func (d *Drop) IsMedia() bool {
	t := d.DocumentType
	return t == "photo" || t == "video" || t == "audio" || t == "music"
}

// UnescapedURL returns the Drop's URL unescaped, for storage.
func (d *Drop) UnescapedURL() string {
	var (
		u   string
		err error
	)
	if u, err = url.PathUnescape(d.URL.String()); err != nil {
		return d.URL.String()
	}

	return u
}

// AddAuthors add authors to the author list, ignoring potential
// duplicates.
func (d *Drop) AddAuthors(values ...string) {
	keys := map[string]string{}
	for _, v := range d.Authors {
		keys[strings.ToLower(v)] = v
	}
	for _, v := range values {
		v = strings.TrimSpace(v)
		v = rxSpaces.ReplaceAllLiteralString(v, " ")
		v = rxAuthor.ReplaceAllString(v, "")
		if _, ok := keys[strings.ToLower(v)]; !ok {
			keys[strings.ToLower(v)] = v
		}
	}
	res := make([]string, len(keys))
	i := 0
	for _, v := range keys {
		res[i] = v
		i++
	}

	sort.Strings(res)
	d.Authors = res
}

// loadHTMLBody will load the document body and try to convert
// it to UTF-8 when encoding is different.
func (d *Drop) loadHTMLBody(rsp *http.Response) error {
	var err error
	var body []byte

	if body, err = ioutil.ReadAll(rsp.Body); err != nil {
		return err
	}

	// Determine encoding (fast way)
	enc, encName, certain := charset.DetermineEncoding(body, rsp.Header.Get("content-type"))

	// When encoding is not 100% certain, we resort to find the charset
	// parsing part of the received HTML. More than recommended
	// by the HTMLWG, since 1024 bytes is often not enough.
	if !certain {
		lr := io.LimitReader(bytes.NewReader(body), 1024*3)
		ctHeader := scanForCharset(lr)

		if ctHeader != "" {
			enc, encName, _ = charset.DetermineEncoding(body, ctHeader)
		}
	}

	if enc != encoding.Nop {
		r := transform.NewReader(bytes.NewReader(body), enc.NewDecoder())
		body, _ = ioutil.ReadAll(r)
	}

	// Eventually set the original charset and UTF8 body
	d.Charset = encName
	d.Body = body

	return nil
}

func scanForCharset(r io.Reader) string {
	z := html.NewTokenizer(r)

	getAttrs := func(t html.Token) map[string]string {
		res := map[string]string{}
		for _, x := range t.Attr {
			res[x.Key] = x.Val
		}
		return res
	}

	for {
		switch z.Next() {
		case html.ErrorToken:
			return ""
		case html.StartTagToken, html.SelfClosingTagToken:
			t := z.Token()
			if t.DataAtom.String() == "meta" {
				attrs := getAttrs(t)
				if v, ok := attrs["charset"]; ok {
					return fmt.Sprintf("text/html; charset=%s", v)
				}
				if v, ok := attrs["name"]; ok && v == "http-equiv" {
					if v, ok := attrs["content"]; ok {
						return v
					}
				}
			}
		}
	}
}

// DropMeta is a map of list of strings that contains the
// collected metadata.
type DropMeta map[string][]string

// Add adds a value to the raw metadata list.
func (m DropMeta) Add(name, value string) {
	_, ok := m[name]
	if ok {
		m[name] = append(m[name], value)
	} else {
		m[name] = []string{value}
	}
}

// Lookup returns all the found values for the
// provided metadata names.
func (m DropMeta) Lookup(names ...string) []string {
	for _, x := range names {
		v, ok := m[x]
		if ok {
			return v
		}
	}

	return []string{}
}

// LookupGet returns the first value found for the
// provided metadata names.
func (m DropMeta) LookupGet(names ...string) string {
	r := m.Lookup(names...)
	if len(r) > 0 {
		return r[0]
	}
	return ""
}
