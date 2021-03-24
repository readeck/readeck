package bookmarks

import (
	"regexp"
	"strings"

	"github.com/go-shiori/dom"
	"golang.org/x/net/html"
)

// bleachPolicy holds the cleaning rules and provides methods to
// perform the DOM cleaning.
type bleachPolicy struct {
	blockAttrs []*regexp.Regexp
}

var bleach = bleachPolicy{
	blockAttrs: []*regexp.Regexp{
		regexp.MustCompile(`^class$`),
		regexp.MustCompile(`^data-`),
		regexp.MustCompile(`^on[a-z]+`),
		regexp.MustCompile(`^(rel|srcset|sizes)$`),
	},
}

// clean discards unwanted attributes from all nodes.
func (p bleachPolicy) clean(node *html.Node) {
	for i := len(node.Attr) - 1; i >= 0; i-- {
		k := node.Attr[i].Key
		for _, r := range p.blockAttrs {
			if r.MatchString(k) {
				dom.RemoveAttribute(node, k)
				break
			}
		}
	}

	for child := dom.FirstElementChild(node); child != nil; child = dom.NextElementSibling(child) {
		p.clean(child)
	}
}

// removeEmptyNodes removes the nodes that are empty.
// empty means: no child nodes, no attributes and no text content.
func (p bleachPolicy) removeEmptyNodes(top *html.Node) {
	nodes := dom.QuerySelectorAll(top, "*")
	dom.RemoveNodes(nodes, func(node *html.Node) bool {
		if len(node.Attr) > 0 {
			return false
		}
		if len(dom.Children(node)) > 0 {
			return false
		}
		if strings.TrimSpace(dom.TextContent(node)) != "" {
			return false
		}
		return true
	})
}

// setLinkRel adds a default "rel" attribute on all "a" tags.
func (p bleachPolicy) setLinkRel(top *html.Node) {
	dom.ForEachNode(dom.QuerySelectorAll(top, "a"), func(node *html.Node, _ int) {
		dom.SetAttribute(node, "rel", "nofollow noopener noreferrer")
	})
}
