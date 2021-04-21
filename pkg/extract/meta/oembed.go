package meta

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"golang.org/x/net/html"

	"github.com/antchfx/htmlquery"
	"github.com/go-shiori/dom"

	"codeberg.org/readeck/readeck/pkg/extract"
)

// ExtractOembed is a processor that extracts the picture from the document
// metadata. It has to come after ExtractMeta.
func ExtractOembed(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
	if m.Step() != extract.StepDom || m.Dom == nil || m.Position() > 0 {
		return next
	}

	d := m.Extractor.Drop()
	if d.Meta == nil {
		return next
	}
	m.Log.Debug("looking for oembed URL")
	o, err := newOembed(m.Dom, d.URL, m.Extractor.Client())
	if err != nil {
		m.Log.WithError(err).Warn("oembed error")
		return next
	}

	if o == nil {
		return next
	}

	// m.Log.Debug().Str("url", o._url.String()).Msg("found oembed")
	m.Log.WithField("url", o._url.String()).Debug("found oembed")

	setOembedMeta(d, "type", o.Type)
	setOembedMeta(d, "version", o.Version)
	setOembedMeta(d, "title", o.Title)
	setOembedMeta(d, "author_name", o.AuthorName)
	setOembedMeta(d, "author_url", o.AuthorURL)
	setOembedMeta(d, "provider_name", o.ProviderName)
	setOembedMeta(d, "provider_url", o.ProviderURL)
	setOembedMeta(d, "cache_age", o.CacheAge)
	setOembedMeta(d, "thumbnail_url", o.ThumbnailURL)
	setOembedMeta(d, "thumbnail_width", o.ThumbnailWidth)
	setOembedMeta(d, "thumbnail_height", o.ThumbnailHeight)
	setOembedMeta(d, "url", o.URL)
	setOembedMeta(d, "width", o.Width)
	setOembedMeta(d, "height", o.Height)
	setOembedMeta(d, "html", o.HTML)

	return next
}

func setOembedMeta(d *extract.Drop, name string, v jsonString) {
	if v == "" {
		return
	}
	d.Meta.Add(fmt.Sprintf("oembed.%s", name), string(v))
}

type oembed struct {
	_url            *url.URL
	Type            jsonString `json:"type"`
	Version         jsonString `json:"version"`
	Title           jsonString `json:"title"`
	AuthorName      jsonString `json:"author_name"`
	AuthorURL       jsonString `json:"author_url"`
	ProviderName    jsonString `json:"provider_name"`
	ProviderURL     jsonString `json:"provider_url"`
	CacheAge        jsonString `json:"cache_age"`
	ThumbnailURL    jsonString `json:"thumbnail_url"`
	ThumbnailWidth  jsonString `json:"thumbnail_width"`
	ThumbnailHeight jsonString `json:"thumbnail_height"`
	URL             jsonString `json:"url"`
	Width           jsonString `json:"width"`
	Height          jsonString `json:"height"`
	HTML            jsonString `json:"html"`
}

type jsonString string

func (s *jsonString) UnmarshalJSON(d []byte) error {
	if d[0] == '"' {
		return json.Unmarshal(d, (*string)(s))
	}
	*s = jsonString(string(d))
	return nil
}

func newOembed(doc *html.Node, base *url.URL, client *http.Client) (res *oembed, err error) {
	node, _ := htmlquery.Query(
		doc,
		"//link[@href][@type='application/json+oembed']")
	if node == nil {
		return
	}

	href := dom.GetAttribute(node, "href")
	if href == "" {
		return
	}
	src, err := base.Parse(href)
	if err != nil {
		return
	}

	rsp, err := client.Get(src.String())
	if err != nil {
		return
	}
	defer rsp.Body.Close()

	if rsp.StatusCode/100 != 2 {
		err = fmt.Errorf("Oembed invalid status code (%d) for %s", rsp.StatusCode, src)
		return
	}

	dec := json.NewDecoder(rsp.Body)
	err = dec.Decode(&res)
	if err != nil {
		return
	}
	res._url = src
	return
}
