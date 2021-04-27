package meta

import (
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/net/html"

	"github.com/antchfx/htmlquery"
	"github.com/araddon/dateparse"
	"github.com/go-shiori/dom"

	"codeberg.org/readeck/readeck/pkg/extract"
)

var (
	rxOpenGraphType = regexp.MustCompile(`^([^:]*:)?(.+?)(\..*|$)`)
)

// ExtractMeta is a processor that extracts metadata from the
// document and set the Drop values accordingly.
func ExtractMeta(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
	if m.Step() != extract.StepDom || m.Dom == nil || m.Position() > 0 {
		return next
	}

	m.Log.Debug("loading metadata")
	res := parseMeta(m.Dom)

	// Set raw meta
	d := m.Extractor.Drop()
	d.Meta = res

	// Set some values
	d.Title = d.Meta.LookupGet(
		"graph.title",
		"twitter.title",
		"html.title",
	)

	d.Description = d.Meta.LookupGet(
		"graph.description",
		"twitter.description",
		"html.description",
	)
	// Keep a short description (60 words)
	parts := strings.Split(d.Description, " ")
	if len(parts) > 60 {
		d.Description = strings.Join(parts[:60], " ") + "..."
	}

	d.AddAuthors(d.Meta.Lookup(
		"schema.author",
		"dc.creator",
		"html.author",
		"html.byl",
	)...)

	site := d.Meta.LookupGet(
		"graph.site_name",
		"schema.name",
	)
	if site != "" {
		d.Site = site
	}

	d.Lang = d.Meta.LookupGet(
		"html.lang",
		"html.language",
	)
	if len(d.Lang) < 2 {
		d.Lang = ""
	} else {
		d.Lang = d.Lang[0:2]
	}

	m.Log.WithField("count", len(d.Meta)).Debug("metadata loaded")
	return next
}

// SetDropProperties will set some Drop properties bases on the retrieved
// metadata. It must be run after ExtractMeta and ExtractOembed.
func SetDropProperties(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
	if m.Step() != extract.StepDom || m.Position() > 0 {
		return next
	}

	d := m.Extractor.Drop()

	// Set publication date
	d.Date, _ = dateparse.ParseLocal(d.Meta.LookupGet("html.date"))

	// Set document type from opengraph value
	if d.DocumentType == "" {
		ogt := d.Meta.LookupGet("graph.type")
		if ogt != "" {
			d.DocumentType = rxOpenGraphType.ReplaceAllString(ogt, "$2")
		}
	}

	// If no authors, try to get them from oembed
	if len(d.Authors) == 0 {
		d.AddAuthors(d.Meta.Lookup("oembed.author_name")...)
	}

	// Same for website name
	if d.Site == "" || d.Site == d.URL.Hostname() {
		if site := d.Meta.LookupGet("oembed.provider_name"); site != "" {
			d.Site = site
		}
	}

	// If we have a picture type, we force the type and set a new meta
	// for the picture url
	otype := d.Meta.LookupGet("oembed.type")
	if d.DocumentType == "photo" || otype == "photo" {
		d.DocumentType = "photo"

		if otype == "photo" {
			d.Meta.Add("x.picture_url", d.Meta.LookupGet("oembed.url"))
		}
	}

	if otype == "video" {
		d.DocumentType = otype
	}

	// Document type is only a predefined set and nothing more
	switch d.DocumentType {
	case "article", "photo", "video":
		// Valid values
	default:
		d.DocumentType = "article"
	}

	m.Log.WithField("type", d.DocumentType).Info("document type")
	return next
}

type rawSpec struct {
	name     string
	selector string
	fn       func(*html.Node) (string, string)
}

func extMeta(k, v string, trim int) func(*html.Node) (string, string) {
	return func(n *html.Node) (string, string) {
		k := strings.TrimSpace(dom.GetAttribute(n, k)[trim:])
		v := strings.TrimSpace(dom.GetAttribute(n, v))

		// Some attributes may contain HTML, we don't want that
		a, _ := html.Parse(strings.NewReader(v))
		return k, dom.TextContent(a)
	}
}

var specList = []rawSpec{
	{"html", "//title", func(n *html.Node) (string, string) {
		return "title", dom.TextContent(n)
	}},
	{"html", "/html[@lang]/@lang", func(n *html.Node) (string, string) {
		return "lang", dom.TextContent(n)
	}},

	// Common HTML meta tags
	{"html", `//meta[@content][
		@name='author' or
		@name='byl' or
		@name='copyright' or
		@name='date' or
		@name='description' or
		@name='keywords' or
		@name='language' or
		@name='subtitle'
	]`, extMeta("name", "content", 0)},

	// Dublin Core
	{"dc", `//meta[@content][
		starts-with(@name, 'DC.') or
		starts-with(@name, 'dc.')
	]`, extMeta("name", "content", 3)},

	// Facebook opengraph
	{"graph", "//meta[@content][starts-with(@property, 'og:')]",
		extMeta("property", "content", 3)},

	// Twitter cards
	{"twitter", "//meta[@content][starts-with(@name, 'twitter:')]",
		extMeta("name", "content", 8)},

	// Schema.org meta tags
	{"schema", "//meta[@content][@itemprop]",
		extMeta("itemprop", "content", 0)},

	// Schema.org author in content
	{"schema", "//*[contains(concat(' ',normalize-space(@itemprop),' '),' author ')]//*[contains(concat(' ',normalize-space(@itemprop),' '),' name ')]",
		func(n *html.Node) (string, string) {
			return "author", dom.TextContent(n)
		}},

	// Header links (excluding icons and stylesheets)
	{"link", `//link[@href][@rel][
		not(contains(@rel, 'icon')) and
		not(contains(@rel, 'stylesheet'))
	]`, extMeta("rel", "href", 0)},
}

func parseMeta(doc *html.Node) extract.DropMeta {
	res := extract.DropMeta{}

	for _, x := range specList {
		nodes, _ := htmlquery.QueryAll(doc, x.selector)

		for _, node := range nodes {
			name, value := x.fn(node)
			if name == "" || value == "" {
				continue
			}

			name = fmt.Sprintf("%s.%s", x.name, strings.TrimSpace(name))
			res.Add(name, strings.TrimSpace(value))
		}
	}

	return res
}
