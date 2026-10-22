package handler

import (
	"bytes"
	"strings"

	"golang.org/x/net/html"
)

var articleTags = map[string]bool{
	"p": true, "br": true, "strong": true, "b": true, "em": true,
	"i": true, "u": true, "h2": true, "h3": true, "ul": true,
	"ol": true, "li": true, "blockquote": true,
}

var articleDropTags = map[string]bool{
	"script": true, "style": true, "iframe": true, "object": true,
	"svg": true, "math": true, "template": true,
}

// sanitizeArticleHTML keeps only formatting emitted by RichTextEditor.
// All attributes are dropped; articles deliberately do not permit links/media.
func sanitizeArticleHTML(raw string) string {
	doc, err := html.Parse(strings.NewReader(raw))
	if err != nil {
		return ""
	}
	var out bytes.Buffer
	renderArticleNode(&out, doc)
	return strings.TrimSpace(out.String())
}

func renderArticleNode(out *bytes.Buffer, node *html.Node) {
	switch node.Type {
	case html.DocumentNode:
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			renderArticleNode(out, child)
		}
	case html.TextNode:
		out.WriteString(html.EscapeString(node.Data))
	case html.ElementNode:
		if articleDropTags[node.Data] {
			return
		}
		if !articleTags[node.Data] {
			for child := node.FirstChild; child != nil; child = child.NextSibling {
				renderArticleNode(out, child)
			}
			return
		}
		out.WriteByte('<')
		out.WriteString(node.Data)
		out.WriteByte('>')
		if node.Data != "br" {
			for child := node.FirstChild; child != nil; child = child.NextSibling {
				renderArticleNode(out, child)
			}
			out.WriteString("</")
			out.WriteString(node.Data)
			out.WriteByte('>')
		}
	}
}
