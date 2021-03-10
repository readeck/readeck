package archiver

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"sync"

	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/css"
	"golang.org/x/sync/errgroup"
)

func (arc *Archiver) processCSS(ctx context.Context, input io.Reader, baseURL *url.URL) (string, error) {
	// Prepare buffer to store content from input
	buffer := bytes.NewBuffer(nil)

	// Scan CSS and find all URLs
	urls := make(map[string]struct{})
	lexer := css.NewLexer(parse.NewInput(input))

	for {
		token, bt := lexer.Next()

		// Check for error or EOF
		if token == css.ErrorToken {
			break
		}

		// If it's URL save it
		if token == css.URLToken {
			urls[string(bt)] = struct{}{}
		}

		buffer.Write(bt)
	}

	// Process each url concurrently
	mutex := sync.RWMutex{}
	processedURLs := make(map[string]string)

	g, ctx := errgroup.WithContext(ctx)
	for uri := range urls {
		uri := uri
		g.Go(func() error {
			cssURL := sanitizeStyleURL(uri)
			cssURL = createAbsoluteURL(cssURL, baseURL)
			content, contentType, err := arc.processURL(ctx, cssURL, baseURL.String())
			if err != nil && err != errSkippedURL {
				arc.SendEvent(ctx, &EventError{err, uri})
				return err
			}

			var result string
			if err == errSkippedURL {
				arc.SendEvent(ctx, &EventError{err, uri})
				result = `url("` + cssURL + `")`
			} else {
				result = fmt.Sprintf(`url("%s")`, arc.URLProcessor(uri, content, contentType))
			}

			mutex.Lock()
			processedURLs[uri] = result
			mutex.Unlock()

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return buffer.String(), err
	}

	// Convert all url into the processed URL
	cssRules := buffer.String()
	for url, processedURL := range processedURLs {
		cssRules = strings.ReplaceAll(cssRules, url, processedURL)
	}

	return cssRules, nil
}
