package download

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const maxPageBytes = 4 << 20

type PageResource struct {
	URL, Name, Extension string
}

func (q *Queue) SniffPage(ctx context.Context, rawURL string) ([]PageResource, error) {
	pageURL, err := parseURL(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	resp, err := q.requestOnce(ctx, pageURL.String(), "", "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("page returned HTTP %d", resp.StatusCode)
	}
	if header := resp.Header.Get("Content-Type"); header != "" {
		mediaType, _, _ := mime.ParseMediaType(header)
		if mediaType != "text/html" && mediaType != "application/xhtml+xml" {
			return nil, errors.New("URL did not return an HTML page")
		}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxPageBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxPageBytes {
		return nil, errors.New("HTML page is larger than 4 MiB")
	}
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	return pageResources(doc, resp.Request.URL), nil
}

func pageResources(doc *html.Node, base *url.URL) []PageResource {
	for n := range doc.Descendants() {
		if n.Type == html.ElementNode && n.DataAtom == atom.Base {
			if href := htmlAttr(n, "href"); href != "" {
				if resolved := resolvePageLink(base, href); resolved != nil {
					base = resolved
				}
				break
			}
		}
	}

	var resources []PageResource
	seen := make(map[string]bool)
	for n := range doc.Descendants() {
		if n.Type != html.ElementNode {
			continue
		}
		var raw string
		switch n.DataAtom {
		case atom.A, atom.Area:
			raw = htmlAttr(n, "href")
		case atom.Video, atom.Audio, atom.Source, atom.Img, atom.Embed:
			raw = htmlAttr(n, "src")
		case atom.Object:
			raw = htmlAttr(n, "data")
		}
		resolved := resolvePageLink(base, raw)
		if resolved == nil {
			continue
		}
		name := path.Base(resolved.Path)
		ext := strings.ToLower(path.Ext(name))
		if !downloadableExtension(ext) {
			continue
		}
		key := resolved.String()
		if !seen[key] {
			seen[key] = true
			resources = append(resources, PageResource{key, safeFilename(name), ext})
		}
	}
	return resources
}

func htmlAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Namespace == "" && attr.Key == key {
			return strings.TrimSpace(attr.Val)
		}
	}
	return ""
}

func resolvePageLink(base *url.URL, raw string) *url.URL {
	if raw == "" || strings.HasPrefix(raw, "#") {
		return nil
	}
	resolved, err := base.Parse(raw)
	if err != nil {
		return nil
	}
	u, _ := parseURL(resolved.String())
	return u
}

func downloadableExtension(ext string) bool {
	switch ext {
	case ".zip", ".7z", ".rar", ".tar", ".gz", ".bz2", ".xz", ".zst", ".tgz", ".tbz2", ".txz",
		".exe", ".msi", ".msix", ".apk", ".appimage", ".deb", ".rpm", ".pkg", ".dmg", ".iso", ".img",
		".mp4", ".m4v", ".mkv", ".mov", ".webm", ".avi", ".mpg", ".mpeg", ".wmv",
		".mp3", ".flac", ".wav", ".m4a", ".ogg", ".opus", ".aac", ".wma",
		".pdf", ".doc", ".docx", ".odt", ".rtf", ".txt", ".xls", ".xlsx", ".ods", ".csv", ".ppt", ".pptx", ".odp", ".epub", ".md",
		".jpg", ".jpeg", ".png", ".gif", ".webp", ".avif", ".svg", ".bmp", ".tif", ".tiff", ".ico", ".heic":
		return true
	}
	return false
}
