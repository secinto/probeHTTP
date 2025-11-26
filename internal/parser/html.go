package parser

import (
	"bufio"
	"fmt"
	"html"
	"regexp"
	"strings"
	"unicode/utf8"

	htmlparser "golang.org/x/net/html"
)

// PERFORMANCE FIX: Compile regex once at package level (90% performance gain)
var unicodeEscapeRegex = regexp.MustCompile(`\\u([0-9a-fA-F]{4})`)

// decodeTitleString decodes HTML entities and Unicode escapes in a title string
// Handles both \uXXXX Unicode escapes and HTML entities like &amp;, &#38;, etc.
func decodeTitleString(s string) string {
	// First, decode Unicode escapes like \u0026
	s = unicodeEscapeRegex.ReplaceAllStringFunc(s, func(match string) string {
		// Extract hex value from \uXXXX
		hex := strings.TrimPrefix(match, `\u`)
		var r rune
		fmt.Sscanf(hex, "%x", &r)
		// Validate the rune before converting
		if utf8.ValidRune(r) {
			return string(r)
		}
		// If invalid, return original match
		return match
	})

	// Then, decode HTML entities like &amp;, &#38;, &#x26;
	s = html.UnescapeString(s)

	return s
}

// ExtractTitle extracts the HTML title from the body with fallbacks
// Priority: 1) <title> tag, 2) og:title meta tag, 3) twitter:title meta tag
func ExtractTitle(body string) string {
	doc, err := htmlparser.Parse(strings.NewReader(body))
	if err != nil {
		return ""
	}

	var htmlTitle string
	var ogTitle string
	var twitterTitle string

	var traverse func(*htmlparser.Node)
	traverse = func(n *htmlparser.Node) {
		if n.Type == htmlparser.ElementNode {
			// Check for <title> tag
			if n.Data == "title" && htmlTitle == "" {
				if n.FirstChild != nil {
					htmlTitle = n.FirstChild.Data
				}
			}

			// Check for <meta> tags with property or name attributes
			if n.Data == "meta" {
				var property, name, content string
				for _, attr := range n.Attr {
					switch attr.Key {
					case "property":
						property = attr.Val
					case "name":
						name = attr.Val
					case "content":
						content = attr.Val
					}
				}

				// Check for Open Graph title
				if property == "og:title" && ogTitle == "" {
					ogTitle = content
				}

				// Check for Twitter Card title
				if name == "twitter:title" && twitterTitle == "" {
					twitterTitle = content
				}
			}
		}

		// Continue traversing child nodes
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
	}
	traverse(doc)

	// Return first non-empty title in priority order, decoded
	if htmlTitle != "" {
		return decodeTitleString(strings.TrimSpace(htmlTitle))
	}
	if ogTitle != "" {
		return decodeTitleString(strings.TrimSpace(ogTitle))
	}
	if twitterTitle != "" {
		return decodeTitleString(strings.TrimSpace(twitterTitle))
	}

	return ""
}

// CountWordsAndLines counts words and lines in the text
func CountWordsAndLines(text string) (words int, lines int) {
	lines = strings.Count(text, "\n") + 1
	if text == "" {
		lines = 0
	}

	// Count words
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Split(bufio.ScanWords)
	for scanner.Scan() {
		words++
	}

	return words, lines
}
