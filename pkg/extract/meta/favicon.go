package meta

import (
	// "fmt"

	"net/url"
	"path"
	"regexp"
	"sort"
	"strconv"

	"golang.org/x/net/html"

	"github.com/antchfx/htmlquery"
	"github.com/go-shiori/dom"

	"github.com/readeck/readeck/pkg/extract"
)

var (
	rxIconSize *regexp.Regexp = regexp.MustCompile("(\\d+)x\\d+")
)

var iconExt map[string]string = map[string]string{
	".png":  "image/png",
	".ico":  "image/ico",
	".jpeg": "image/jpeg",
	".jpg":  "image/jpeg",
}

// ExtractFavicon is a processor that extracts the favicon
// for the first extracted document.
func ExtractFavicon(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
	if m.Step() != extract.StepDom || m.Position > 0 {
		return next
	}

	m.Log.Debug("loading icon")
	list := newFaviconList(m.Dom, m.Extractor.Drop().URL)

	// Load icons until we find a suitable one
	for _, icon := range list {
		if err := icon.Load(m.Extractor.Client(), 48, "png"); err != nil {
			continue
		}
		m.Extractor.Drop().Pictures["icon"] = icon
		m.Log.WithField("href", icon.Href).WithField("size", icon.Size[:]).Debug("icon loaded")
		break
	}

	return next
}

func newFavicon(node *html.Node, base *url.URL) (res *extract.Picture, err error) {
	var href *url.URL
	href, err = base.Parse(dom.GetAttribute(node, "href"))
	if err != nil {
		return
	}

	res = &extract.Picture{
		Href: href.String(),
		Type: dom.GetAttribute(node, "type"),
	}

	// Get size
	sizes := make([]int, 0)
	for _, m := range rxIconSize.FindAllStringSubmatch(dom.GetAttribute(node, "sizes"), -1) {
		s, _ := strconv.Atoi(m[1])
		sizes = append(sizes, s)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(sizes)))
	if len(sizes) > 0 {
		res.Size = [2]int{sizes[0], sizes[0]}
	} else {
		res.Size = [2]int{32, 32} // Default size
	}

	// Get type
	if res.Type == "" {
		t, ok := iconExt[path.Ext(href.Path)]
		if ok {
			res.Type = t
		}
	}

	return
}

type faviconList []*extract.Picture

func (l faviconList) Len() int      { return len(l) }
func (l faviconList) Swap(i, j int) { l[i], l[j] = l[j], l[i] }
func (l faviconList) Less(i, j int) bool {
	return l[i].Size[0] > l[j].Size[0]
}

func newFaviconList(doc *html.Node, base *url.URL) faviconList {
	res := make(faviconList, 0)
	selector := `//link[@href][
		@rel='icon' or
		@rel='shortcut-icon' or
		@rel='apple-touch-icon'
	]`
	nodes, _ := htmlquery.QueryAll(doc, selector)

	for _, node := range nodes {
		f, err := newFavicon(node, base)
		if err != nil {
			continue
		}
		res = append(res, f)
	}

	sort.Sort(res)

	// Add the default /favicon at the end of the list
	href, _ := base.Parse("/favicon.ico")
	res = append(res, &extract.Picture{
		Href: href.String(),
		Type: "image/ico",
		Size: [2]int{32, 32},
	})

	return res
}
