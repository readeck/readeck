package archiver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"strings"

	"github.com/go-shiori/dom"
	"github.com/h2non/filetype"
	"github.com/h2non/filetype/matchers"
	"github.com/h2non/filetype/types"
	"golang.org/x/net/html"
)

var errSkippedURL = errors.New("skip processing url")

type imageProcessor func(context.Context, *Archiver, io.Reader, string, *url.URL) ([]byte, string, error)
type urlProcessor func(uri string, content []byte, contentType string) string

// DefaultImageProcessor is the default image processor.
// It simply reads and return the content.
func DefaultImageProcessor(_ context.Context, _ *Archiver,
	input io.Reader, contentType string, _ *url.URL) ([]byte, string, error) {
	res, err := ioutil.ReadAll(input)
	return res, contentType, err
}

// DefaultURLProcessor is the default URL processor.
// It returns the base64 encoded URL.
func DefaultURLProcessor(_ string, content []byte, contentType string) string {
	return createDataURL(content, contentType)
}

func (arc *Archiver) processURL(ctx context.Context, uri string, parentURL string, embedded ...bool) ([]byte, string, error) {
	// Parse embedded value
	isEmbedded := len(embedded) != 0 && embedded[0]

	// Make sure this URL is not empty, data or hash. If yes, just skip it.
	uri = strings.TrimSpace(uri)
	if uri == "" || strings.HasPrefix(uri, "data:") || strings.HasPrefix(uri, "#") {
		arc.SendEvent(ctx, &EventError{errSkippedURL, uri})
		return nil, "", errSkippedURL
	}

	// Parse URL to make sure it's valid request URL. If not, then
	// it's a real error.
	parsedURL, err := url.ParseRequestURI(uri)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Hostname() == "" {
		arc.SendEvent(ctx, &EventError{errSkippedURL, uri})
		return nil, "", errors.New("can't parse URL")
	}

	// Check in cache to see if this URL already processed
	arc.RLock()
	cache, cacheExist := arc.Cache[uri]
	arc.RUnlock()

	if cacheExist {
		arc.SendEvent(ctx, &EventFetchURL{uri, parentURL, true})
		return cache.Data, cache.ContentType, nil
	}

	// Download the resource, use semaphore to limit concurrent downloads
	arc.SendEvent(ctx, &EventFetchURL{uri, parentURL, false})
	err = arc.dlSemaphore.Acquire(ctx, 1)
	if err != nil {
		arc.SendEvent(ctx, &EventError{err, uri})
		return nil, "", nil
	}

	resp, err := arc.downloadFile(uri, parentURL)
	arc.dlSemaphore.Release(1)
	if err != nil {
		arc.SendEvent(ctx, &EventError{err, uri})
		return nil, "", fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	// Get content type
	contentType := resp.Header.Get("Content-Type")
	contentType = strings.TrimSpace(contentType)
	if contentType == "" {
		contentType = "text/plain"
	}
	mainType := strings.Split(contentType, "/")[0]

	// Read content of response body. If the downloaded file is HTML
	// or CSS it need to be processed again
	var bodyContent []byte

	switch {
	case contentType == "text/html" && isEmbedded:
		newHTML, err := arc.processHTML(ctx, resp.Body, parsedURL)
		if err == nil {
			bodyContent = []byte(newHTML)
		} else {
			arc.SendEvent(ctx, &EventError{err, uri})
			return nil, "", err
		}

	case contentType == "text/css":
		newCSS, err := arc.processCSS(ctx, resp.Body, parsedURL)
		if err == nil {
			bodyContent = []byte(newCSS)
		} else {
			arc.SendEvent(ctx, &EventError{err, uri})
			return nil, "", err
		}
	case mainType == "image":
		bodyContent, contentType, err = arc.ImageProcessor(ctx, arc, resp.Body, contentType, parsedURL)
		if err != nil {
			arc.SendEvent(ctx, &EventError{err, uri})
			return nil, "", err
		}
	default:
		bodyContent, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			arc.SendEvent(ctx, &EventError{err, uri})
			return nil, "", err
		}
	}

	if err := arc.checkContent(ctx, bodyContent); err != nil {
		return nil, "", err
	}

	// Save data URL to cache
	arc.Lock()
	arc.Cache[uri] = Asset{
		Data:        bodyContent,
		ContentType: contentType,
	}
	arc.Unlock()

	return bodyContent, contentType, nil
}

// checkContent checks if the downloaded content is really what it is supposed to
// be. For now, only check if an image is really an image.
func (arc *Archiver) checkContent(ctx context.Context, content []byte) error {
	node, ok := ctx.Value(ctxNodeKey).(*html.Node)
	if !ok || dom.TagName(node) != "img" {
		return nil
	}

	t, _ := filetype.Match(content)
	if _, ok := imageTypes[t]; !ok {
		return errors.New("not an image")
	}

	return nil
}

var imageTypes = map[types.Type]bool{
	matchers.TypeJpeg: true,
	matchers.TypePng:  true,
	matchers.TypeGif:  true,
	matchers.TypeWebp: true,
	matchers.TypeIco:  true,
	matchers.TypeBmp:  true,
	matchers.TypeTiff: true,
}
