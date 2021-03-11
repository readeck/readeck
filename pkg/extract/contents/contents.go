package contents

import (
	"bytes"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/net/html"

	"github.com/antchfx/htmlquery"
	"github.com/go-shiori/dom"
	"github.com/go-shiori/go-readability"

	"github.com/readeck/readeck/pkg/extract"
)

var (
	rxSpace     = regexp.MustCompile(`[ ]+`)
	rxNewLine   = regexp.MustCompile(`\r?\n\s*(\r?\n)+`)
	rxSrcsetURL = regexp.MustCompile(`(?i)(\S+)(?:\s+([\d.]+)[xw])?(\s*(?:,|$))`)
)

// Readability is a processor that executes readability on the drop content.
func Readability(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
	if m.Step() != extract.StepDom {
		return next
	}

	if m.Extractor.Drop().IsMedia() {
		m.ResetContent()
		return next
	}

	fixNoscriptImages(m.Dom)

	// It's a shame we have to render the document instead of passing
	// directly the node. But that's how readability works for now.
	buf := &bytes.Buffer{}
	html.Render(buf, m.Dom)

	article, err := readability.FromReader(buf, m.Extractor.Drop().URL.String())
	if err != nil {
		m.Log.WithError(err).Error("readability error")
		m.ResetContent()
		return next
	}

	if article.Node == nil {
		m.Log.Error("could not extract content")
		m.ResetContent()
		return next
	}

	m.Log.Debug("readability on contents")

	doc := &html.Node{Type: html.DocumentNode}
	body := dom.CreateElement("body")
	doc.AppendChild(body)
	dom.AppendChild(body, article.Node)

	// final cleanup
	removeEmbeds(body)
	fixImages(body, m)

	// Simplify the top hierarchy
	node := findFirstContentNode(body)
	if node != body.FirstChild {
		dom.ReplaceChild(body, node, body.FirstChild)
	}

	// Ensure we always start with a <section>
	encloseArticle(body)

	m.Dom = doc

	return next
}

// Text is a processor that sets the pure text content of the final HTML.
func Text(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
	if m.Step() != extract.StepPostProcess {
		return next
	}

	if len(m.Extractor.HTML) == 0 {
		return next
	}
	if !m.Extractor.Drop().IsHTML() {
		return next
	}

	m.Log.Debug("get text content")

	doc, _ := html.Parse(bytes.NewReader(m.Extractor.HTML))
	text := dom.TextContent(doc)

	text = rxSpace.ReplaceAllString(text, " ")
	text = rxNewLine.ReplaceAllString(text, "\n\n")
	text = strings.TrimSpace(text)

	m.Extractor.Text = text
	return next
}

func findFirstContentNode(node *html.Node) *html.Node {
	children := dom.ChildNodes(node)
	count := 0
	for _, x := range children {
		if x.Type == html.TextNode && strings.TrimSpace(x.Data) != "" {
			count++
		} else if x.Type == html.ElementNode {
			count++
		}

	}

	if count > 1 || dom.FirstElementChild(node) == nil {
		return node
	}

	return findFirstContentNode(dom.FirstElementChild(node))
}

func encloseArticle(top *html.Node) {
	children := dom.ChildNodes(top)

	if len(children) == 1 {
		node := children[0]
		switch node.Type {
		case html.TextNode:
			section := dom.CreateElement("section")
			dom.AppendChild(node.Parent, section)
			dom.AppendChild(section, node)
		case html.ElementNode:
			if node.Data == "div" {
				node.Data = "section"
			} else {
				section := dom.CreateElement("section")
				dom.AppendChild(node.Parent, section)
				dom.AppendChild(section, node)
			}
		}
	} else {
		section := dom.CreateElement("section")
		dom.AppendChild(top, section)
		for _, x := range children {
			dom.AppendChild(section, x)
		}
	}
}

func removeEmbeds(top *html.Node) {
	dom.RemoveNodes(dom.GetAllNodesWithTag(top, "object", "embed", "iframe", "video", "audio"), nil)
}

func fixNoscriptImages(top *html.Node) {
	// A bug in readability prevents us to extract images.
	// It does move the noscript content when it's a single image
	// but only when the noscript previous sibling is an image.
	// This will replace the noscript content with the image
	// in the other case.

	noscripts := dom.GetElementsByTagName(top, "noscript")
	dom.ForEachNode(noscripts, func(noscript *html.Node, _ int) {
		noscriptContent := dom.TextContent(noscript)
		tmpDoc, err := html.Parse(strings.NewReader(noscriptContent))
		if err != nil {
			return
		}

		tmpBody := dom.GetElementsByTagName(tmpDoc, "body")[0]
		if !isSingleImage(tmpBody) {
			return
		}

		// Sometimes, the image is *after* the noscript tag.
		// Let's move it before so the next step can detect it.
		nextElement := dom.NextElementSibling(noscript)
		if nextElement != nil && isSingleImage(nextElement) {
			if noscript.Parent != nil {
				noscript.Parent.InsertBefore(dom.Clone(nextElement, true), noscript)
				noscript.Parent.RemoveChild(nextElement)
			}
		}

		prevElement := dom.PreviousElementSibling(noscript)
		if prevElement == nil || !isSingleImage(prevElement) {
			dom.ReplaceChild(noscript.Parent, dom.FirstElementChild(tmpBody), noscript)
		}
	})
}

func isSingleImage(node *html.Node) bool {
	if dom.TagName(node) == "img" {
		return true
	}
	children := dom.Children(node)
	textContent := dom.TextContent(node)
	if len(children) != 1 || strings.TrimSpace(textContent) != "" {
		return false
	}

	return isSingleImage(children[0])
}

func fixImages(top *html.Node, m *extract.ProcessMessage) {
	// Fix images with an srcset attribute and only keep the
	// best one.

	m.Log.Debug("fixing images")
	nodes, err := htmlquery.QueryAll(top, "//*[@srcset]")
	if err != nil {
		m.Log.WithError(err).Error()
	}

	dom.ForEachNode(nodes, func(node *html.Node, _ int) {
		srcset := dom.GetAttribute(node, "srcset")
		set := []srcSetItem{}
		for _, x := range rxSrcsetURL.FindAllStringSubmatch(srcset, -1) {
			src := x[1]
			w := x[2]
			if w == "" {
				w = "1"
			}
			z, err := strconv.Atoi(w)
			if err != nil {
				continue
			}
			set = append(set, srcSetItem{src, z})
		}
		sort.SliceStable(set, func(i int, j int) bool {
			return set[i].dsc > set[j].dsc
		})

		if len(set) > 0 {
			dom.SetAttribute(node, "src", set[0].src)
			dom.RemoveAttribute(node, "srcset")

			dom.RemoveAttribute(node, "width")
			dom.RemoveAttribute(node, "height")
		}
	})
}

type srcSetItem struct {
	src string
	dsc int
}
