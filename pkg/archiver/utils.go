package archiver

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var (
	rxStyleURL = regexp.MustCompile(`(?i)^url\((.+)\)$`)
)

// isValidURL checks if URL is valid.
func isValidURL(s string) bool {
	_, err := url.ParseRequestURI(s)
	return err == nil
}

// createAbsoluteURL convert url to absolute path based on base.
func createAbsoluteURL(uri string, base *url.URL) string {
	uri = strings.TrimSpace(uri)
	if uri == "" || base == nil {
		return ""
	}

	// If it is data url, return as it is
	if strings.HasPrefix(uri, "data:") {
		return uri
	}

	// If it is fragment path, return as it is
	if strings.HasPrefix(uri, "#") {
		return uri
	}

	// If it is already an absolute URL, clean the URL then return it
	tmp, err := url.ParseRequestURI(uri)
	if err == nil && tmp.Scheme != "" && tmp.Hostname() != "" {
		cleanURL(tmp)
		return tmp.String()
	}

	// Otherwise, resolve against base URL.
	tmp, err = url.Parse(uri)
	if err != nil {
		return uri
	}

	cleanURL(tmp)
	return base.ResolveReference(tmp).String()
}

// cleanURL removes fragment (#fragment) and UTM queries from URL
func cleanURL(uri *url.URL) {
	queries := uri.Query()

	for key := range queries {
		if strings.HasPrefix(key, "utm_") {
			queries.Del(key)
		}
	}

	uri.Fragment = ""
	uri.RawQuery = queries.Encode()
}

// sanitizeStyleURL sanitizes the URL in CSS by removing `url()`,
// quotation mark and trailing slash
func sanitizeStyleURL(uri string) string {
	cssURL := rxStyleURL.ReplaceAllString(uri, "$1")
	cssURL = strings.TrimSpace(cssURL)

	if strings.HasPrefix(cssURL, `"`) {
		return strings.Trim(cssURL, `"`)
	}

	if strings.HasPrefix(cssURL, `'`) {
		return strings.Trim(cssURL, `'`)
	}

	return cssURL
}

// createDataURL returns base64 encoded data URL
func createDataURL(content []byte, contentType string) string {
	b64encoded := base64.StdEncoding.EncodeToString(content)
	return fmt.Sprintf("data:%s;base64,%s", contentType, b64encoded)
}
