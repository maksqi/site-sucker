package crawler

import (
	"bytes"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
)

var (
	cssURLPattern    = regexp.MustCompile(`(?i)url\(\s*(['"]?)([^'"\)]+)(['"]?)\s*\)`)
	cssImportPattern = regexp.MustCompile(`(?i)(@import\s+)(['"])([^'"]+)(['"])`)
	metaURLPattern   = regexp.MustCompile(`(?i)(?:^|;)\s*url\s*=\s*([^;]+)`)
)

type rewriteTarget func(target *url.URL, kind ResourceKind) (string, bool)

func processHTML(body []byte, contentType string, base *url.URL, localize, crawlNoFollow bool, rewrite rewriteTarget) ([]byte, []link, error) {
	reader, err := charset.NewReader(bytes.NewReader(body), contentType)
	if err != nil {
		return nil, nil, err
	}
	doc, err := html.Parse(reader)
	if err != nil {
		return nil, nil, err
	}
	effectiveBase := findHTMLBase(doc, base)
	var links []link
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode {
			normalizeHTMLCharset(node)
			if node.Data == "base" && localize {
				for i := range node.Attr {
					if strings.EqualFold(node.Attr[i].Key, "href") {
						node.Attr[i].Val = "."
					}
				}
			}
			noFollow := hasNoFollow(node)
			for i := range node.Attr {
				attr := &node.Attr[i]
				kind, ok := attributeKind(node.Data, attr.Key)
				if ok && node.Data == "link" && attr.Key == "href" && linkIsDocument(node) {
					kind = KindDocument
				}
				if !ok {
					if attr.Key == "style" {
						attr.Val, links = processCSSValue(attr.Val, effectiveBase, rewrite, links)
					} else if localize && absoluteOnlyAttribute(node.Data, attr.Key) {
						if resolved := resolveURL(effectiveBase, attr.Val); resolved != nil {
							attr.Val = resolved.String()
						}
					}
					continue
				}
				if attr.Key == "srcset" {
					attr.Val, links = processSrcset(attr.Val, effectiveBase, rewrite, links)
					continue
				}
				if node.Data == "meta" && attr.Key == "content" {
					match := metaURLPattern.FindStringSubmatch(attr.Val)
					if len(match) > 1 {
						raw := strings.TrimSpace(strings.Trim(match[1], "'\""))
						resolved := resolveURL(effectiveBase, raw)
						if resolved != nil {
							links = append(links, link{URL: resolved, Kind: KindDocument, NoFollow: noFollow})
							if noFollow && !crawlNoFollow {
								if localize {
									attr.Val = strings.Replace(attr.Val, match[1], resolved.String(), 1)
								}
								continue
							}
							if local, allowed := rewrite(resolved, KindDocument); allowed {
								attr.Val = strings.Replace(attr.Val, match[1], local, 1)
							}
						}
					}
					continue
				}
				resolved := resolveURL(effectiveBase, attr.Val)
				if resolved == nil {
					continue
				}
				links = append(links, link{URL: resolved, Kind: kind, NoFollow: noFollow})
				if noFollow && kind == KindDocument && !crawlNoFollow {
					if localize {
						attr.Val = resolved.String()
					}
					continue
				}
				if local, allowed := rewrite(resolved, kind); allowed {
					attr.Val = local
				}
			}
			if node.Data == "style" && node.FirstChild != nil && node.FirstChild.Type == html.TextNode {
				node.FirstChild.Data, links = processCSSValue(node.FirstChild.Data, effectiveBase, rewrite, links)
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	var output bytes.Buffer
	if err := html.Render(&output, doc); err != nil {
		return nil, nil, err
	}
	return output.Bytes(), links, nil
}

func normalizeHTMLCharset(node *html.Node) {
	if node.Data != "meta" {
		return
	}
	var httpEquiv bool
	for i := range node.Attr {
		attr := &node.Attr[i]
		if strings.EqualFold(attr.Key, "charset") {
			attr.Val = "utf-8"
		}
		if strings.EqualFold(attr.Key, "http-equiv") && strings.EqualFold(attr.Val, "content-type") {
			httpEquiv = true
		}
	}
	if httpEquiv {
		for i := range node.Attr {
			if strings.EqualFold(node.Attr[i].Key, "content") {
				node.Attr[i].Val = "text/html; charset=utf-8"
			}
		}
	}
}

func linkIsDocument(node *html.Node) bool {
	for _, attr := range node.Attr {
		if !strings.EqualFold(attr.Key, "rel") {
			continue
		}
		for _, rel := range strings.Fields(strings.ToLower(attr.Val)) {
			switch rel {
			case "stylesheet", "icon", "manifest", "preload", "prefetch", "modulepreload", "apple-touch-icon", "mask-icon":
				return false
			case "canonical", "alternate":
				return true
			}
		}
	}
	return false
}

func absoluteOnlyAttribute(tag, attribute string) bool {
	tag = strings.ToLower(tag)
	attribute = strings.ToLower(attribute)
	switch attribute {
	case "action":
		return tag == "form"
	case "formaction":
		return tag == "button" || tag == "input"
	case "cite":
		return tag == "blockquote" || tag == "q" || tag == "del" || tag == "ins"
	}
	return false
}

func processCSS(body []byte, base *url.URL, rewrite rewriteTarget) ([]byte, []link) {
	rewritten, links := processCSSValue(string(body), base, rewrite, nil)
	return []byte(rewritten), links
}

func processCSSValue(value string, base *url.URL, rewrite rewriteTarget, links []link) (string, []link) {
	seen := make(map[string]struct{})
	rewritten := cssURLPattern.ReplaceAllStringFunc(value, func(match string) string {
		parts := cssURLPattern.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		resolved := resolveURL(base, strings.TrimSpace(parts[2]))
		if resolved == nil {
			return match
		}
		seen[resolved.String()] = struct{}{}
		links = append(links, link{URL: resolved, Kind: KindAsset})
		if local, allowed := rewrite(resolved, KindAsset); allowed {
			quote := parts[1]
			return "url(" + quote + local + quote + ")"
		}
		return match
	})
	rewritten = cssImportPattern.ReplaceAllStringFunc(rewritten, func(match string) string {
		parts := cssImportPattern.FindStringSubmatch(match)
		if len(parts) < 5 {
			return match
		}
		resolved := resolveURL(base, strings.TrimSpace(parts[3]))
		if resolved == nil {
			return match
		}
		if _, ok := seen[resolved.String()]; !ok {
			links = append(links, link{URL: resolved, Kind: KindAsset})
		}
		if local, allowed := rewrite(resolved, KindAsset); allowed {
			return parts[1] + parts[2] + local + parts[4]
		}
		return match
	})
	return rewritten, links
}

func findHTMLBase(doc *html.Node, fallback *url.URL) *url.URL {
	var found *url.URL
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if found != nil {
			return
		}
		if node.Type == html.ElementNode && strings.EqualFold(node.Data, "base") {
			for _, attr := range node.Attr {
				if strings.EqualFold(attr.Key, "href") {
					found = resolveURL(fallback, attr.Val)
					return
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	if found != nil {
		return found
	}
	return fallback
}

func processSrcset(value string, base *url.URL, rewrite rewriteTarget, links []link) (string, []link) {
	items := strings.Split(value, ",")
	for i, item := range items {
		fields := strings.Fields(strings.TrimSpace(item))
		if len(fields) == 0 {
			continue
		}
		resolved := resolveURL(base, fields[0])
		if resolved == nil {
			continue
		}
		links = append(links, link{URL: resolved, Kind: KindAsset})
		if local, allowed := rewrite(resolved, KindAsset); allowed {
			fields[0] = local
			items[i] = strings.Join(fields, " ")
		}
	}
	return strings.Join(items, ", "), links
}

func resolveURL(base *url.URL, raw string) *url.URL {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(raw, "#") {
		return nil
	}
	lower := strings.ToLower(raw)
	for _, prefix := range []string{"data:", "javascript:", "mailto:", "tel:", "blob:", "about:"} {
		if strings.HasPrefix(lower, prefix) {
			return nil
		}
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil
	}
	resolved := base.ResolveReference(parsed)
	if resolved.Scheme != "http" && resolved.Scheme != "https" {
		return nil
	}
	return resolved
}

func attributeKind(tag, attribute string) (ResourceKind, bool) {
	tag = strings.ToLower(tag)
	attribute = strings.ToLower(attribute)
	switch attribute {
	case "srcset":
		return KindAsset, tag == "img" || tag == "source"
	case "poster":
		return KindAsset, tag == "video"
	case "data":
		return KindAsset, tag == "object"
	case "content":
		return KindDocument, tag == "meta"
	case "href":
		if tag == "a" || tag == "area" {
			return KindDocument, true
		}
		if tag == "link" {
			return KindAsset, true
		}
	case "src":
		if tag == "iframe" || tag == "frame" {
			return KindDocument, true
		}
		switch tag {
		case "script", "img", "source", "video", "audio", "embed", "input", "track":
			return KindAsset, true
		}
	}
	return "", false
}

func hasNoFollow(node *html.Node) bool {
	for _, attr := range node.Attr {
		if strings.EqualFold(attr.Key, "rel") {
			for _, value := range strings.Fields(strings.ToLower(attr.Val)) {
				if value == "nofollow" {
					return true
				}
			}
		}
	}
	return false
}

func relativeLink(fromPath, toPath, fragment string) string {
	relative, err := filepath.Rel(filepath.Dir(fromPath), toPath)
	if err != nil {
		return toPath
	}
	relative = filepath.ToSlash(relative)
	if fragment != "" {
		relative += "#" + fragment
	}
	return relative
}

func debugLink(link link) string {
	return fmt.Sprintf("%s (%s)", link.URL, link.Kind)
}
