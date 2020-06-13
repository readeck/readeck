package fftr

import (
	"fmt"
	"net/url"
	"strings"

	"golang.org/x/net/html"

	"github.com/antchfx/htmlquery"
	"github.com/araddon/dateparse"
	"github.com/go-shiori/dom"

	"github.com/readeck/readeck/pkg/extract"
)

// LoadConfiguration will try to find a matching fftr configuration
// for the first Drop (the extraction starting point).
//
// If a configuration is found, it will be added to the context.
//
// If the configuration indicates custom HTTP headers, they'll be added to
// the client.
func LoadConfiguration(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
	if m.Step() == extract.StepDom {
		// remove configuration if document type is a media
		if m.Extractor.Drop().IsMedia() {
			m.Log.Debug("fftr removing configuration (document is a media)")
			m.SetValue("config", nil)
		}
	}

	if m.Position > 0 || m.Step() != extract.StepStart {
		return next
	}

	// Find fftr configuration for this site
	cfg, err := NewConfigForURL(m.Extractor.Drop().URL, DefaultConfigurationFolders)
	if err != nil {
		m.Log.WithError(err).Error("load fftr")
		return next
	}

	if cfg != nil {
		m.Log.WithField("files", cfg.Files).Debug("fftr configuration loaded")
	} else {
		m.Log.Debug("no fftr configuration found")
		cfg = &Config{}
	}

	m.SetValue("config", cfg)

	// Set custom headers from configuration file
	prepareHeaders(m, cfg)

	return next
}

func prepareHeaders(m *extract.ProcessMessage, cfg *Config) {
	if len(cfg.HTTPHeaders) == 0 {
		return
	}

	tr, ok := m.Extractor.Client().Transport.(*extract.Transport)
	if !ok {
		return
	}

	for k, v := range cfg.HTTPHeaders {
		m.Log.WithField("header", []string{k, v}).Debug("fftr custom headers")
		tr.SetHeader(k, v)
	}
}

// ReplaceStrings applies all the replace_string directive in fftr
// configuration file on the received body.
func ReplaceStrings(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
	if m.Step() != extract.StepBody {
		return next
	}

	cfg, ok := m.Value("config").(*Config)
	if !ok {
		return next
	}

	d := m.Extractor.Drop()
	for _, r := range cfg.ReplaceStrings {
		d.Body = []byte(strings.ReplaceAll(string(d.Body), r[0], r[1]))
		m.Log.WithField("replace", r[:]).Debug("fftr replace_string")
	}

	return next
}

// ExtractBody tries to find a body as defined by the "body" directives
// in the configuration file.
func ExtractBody(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
	if m.Step() != extract.StepDom {
		return next
	}

	cfg, ok := m.Value("config").(*Config)
	if !ok {
		return next
	}

	bodyNodes := dom.GetElementsByTagName(m.Dom, "body")
	if len(bodyNodes) == 0 {
		return next
	}
	body := bodyNodes[0]

	for _, selector := range cfg.BodySelectors {
		node, _ := htmlquery.Query(m.Dom, selector)
		if node == nil {
			continue
		}

		// First match, replace the root node and stop
		m.Log.WithField("nodes", len(dom.Children(node))).Debug("fftr body found")

		newBody := dom.CreateElement("body")
		section := dom.CreateElement("section")
		dom.SetAttribute(section, "class", "article")
		dom.SetAttribute(section, "id", "article")
		dom.AppendChild(newBody, section)

		dom.AppendChild(section, node)
		dom.ReplaceChild(body.Parent, newBody, body)

		break
	}

	return next
}

// ExtractAuthor applies the "author" directives to find an author.
func ExtractAuthor(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
	if m.Position > 0 || m.Step() != extract.StepDom {
		return next
	}

	cfg, ok := m.Value("config").(*Config)
	if !ok {
		return next
	}

	for _, selector := range cfg.AuthorSelectors {
		// nodes, _ := m.Dom.Root().Search(selector)
		nodes, _ := htmlquery.QueryAll(m.Dom, selector)
		for _, n := range nodes {
			value := dom.TextContent(n)
			if value == "" {
				continue
			}
			m.Log.WithField("author", value).Debug("fftr author")
			m.Extractor.Drop().AddAuthors(value)
		}
	}

	return next
}

// ExtractDate applies the "date" directives to find a date. If a date is found
// we try to parse it.
func ExtractDate(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
	if m.Position > 0 || m.Step() != extract.StepDom {
		return next
	}

	if !m.Extractor.Drop().Date.IsZero() {
		return next
	}

	cfg, ok := m.Value("config").(*Config)
	if !ok {
		return next
	}

	for _, selector := range cfg.DateSelectors {
		nodes, _ := htmlquery.QueryAll(m.Dom, selector)
		for _, n := range nodes {
			date, err := dateparse.ParseLocal(dom.TextContent(n))
			if err == nil && !date.IsZero() {
				m.Log.WithField("date", date).Debug("fftr date")
				m.Extractor.Drop().Date = date
				return next
			}
		}
	}

	return next
}

// StripTags removes the tags from the DOM root node, according to
// "strip_tags" configuration directives.
func StripTags(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
	if m.Step() != extract.StepDom {
		return next
	}

	cfg, ok := m.Value("config").(*Config)
	if !ok {
		return next
	}

	var value string

	for _, value = range cfg.StripSelectors {
		nodes, _ := htmlquery.QueryAll(m.Dom, value)
		dom.RemoveNodes(nodes, func(_ *html.Node) bool { return true })
		m.Log.WithField("value", value).
			WithField("nodes", len(nodes)).
			Debug("fftr strip_tags")
	}

	for _, value = range cfg.StripIDOrClass {
		selector := fmt.Sprintf(
			"//*[@id='%s' or contains(concat(' ',normalize-space(@class),' '),' %s ')]",
			value, value,
		)

		nodes, _ := htmlquery.QueryAll(m.Dom, selector)
		dom.RemoveNodes(nodes, func(_ *html.Node) bool { return true })
		m.Log.WithField("value", value).
			WithField("nodes", len(nodes)).
			Debug("fftr strip_id_or_class")
	}

	for _, value = range cfg.StripImageSrc {
		selector := fmt.Sprintf("//img[contains(@src, '%s')]", value)

		nodes, _ := htmlquery.QueryAll(m.Dom, selector)
		dom.RemoveNodes(nodes, func(_ *html.Node) bool { return true })
		m.Log.WithField("value", value).
			WithField("nodes", len(nodes)).
			Debug("fftr strip_image_src")
	}

	return next
}

// FindContentPage searches for SinglePageLinkSelectors in the page and,
// if it finds one, it reset the process to its begining with the newly
// found URL.
func FindContentPage(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
	if m.Step() != extract.StepDom {
		return next
	}

	cfg, ok := m.Value("config").(*Config)
	if !ok {
		return next
	}

	for _, selector := range cfg.SinglePageLinkSelectors {
		node, _ := htmlquery.Query(m.Dom, selector)
		if node == nil {
			continue
		}

		href := dom.GetAttribute(node, "href")
		if href == "" {
			href = dom.TextContent(node)
		}
		if href == "" {
			continue
		}
		u, err := m.Extractor.Drop().URL.Parse(href)
		if err != nil {
			continue
		}
		u.Fragment = ""

		if m.Extractor.Visited.IsPresent(u) {
			m.Log.WithField("url", u.String()).Debug("single page already visited")
			continue
		}

		m.Log.WithField("url", u.String()).Info("fftr found single page link")
		m.Extractor.ReplaceDrop(u)
		m.Position = -1

		return nil
	}

	return next
}

// FindNextPage looks for NextPageLinkSelectors and if it finds a URL, it's added to
// the message and can be processed later with GoToNextPage.
func FindNextPage(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
	if m.Step() != extract.StepDom {
		return next
	}

	cfg, ok := m.Value("config").(*Config)
	if !ok {
		return next
	}

	for _, selector := range cfg.NextPageLinkSelectors {
		// nodes, _ := m.Dom.Root().Search(selector)
		node, _ := htmlquery.Query(m.Dom, selector)
		if node == nil {
			continue
		}

		// href := nodes[0].Attr("href")
		href := dom.GetAttribute(node, "href")
		if href == "" {
			// href = nodes[0].Content()
			href = dom.TextContent(node)
		}
		if href == "" {
			continue
		}
		u, err := m.Extractor.Drop().URL.Parse(href)
		if err != nil {
			continue
		}
		u.Fragment = ""

		m.Log.WithField("url", u.String()).Debug("fftr found next page")
		m.SetValue("next_page", u)
	}

	return next
}

// GoToNextPage checks if there is a "next_page" value in the process message. It then
// creates a new drop with the URL.
func GoToNextPage(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
	if m.Step() != extract.StepFinish {
		return next
	}

	u, ok := m.Value("next_page").(*url.URL)
	if !ok {
		return next
	}

	// Avoid crazy loops
	if m.Extractor.Visited.IsPresent(u) {
		m.Log.WithField("url", u.String()).Debug("next page already visited")
		return next
	}

	m.Log.WithField("url", u.String()).Info("go to next page")
	m.Extractor.AddDrop(u)
	m.SetValue("next_page", nil)

	return next
}
