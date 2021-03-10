package bookmarks

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/lithammer/shortuuid"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/semaphore"

	"github.com/readeck/readeck/pkg/archiver"
	"github.com/readeck/readeck/pkg/extract"
	"github.com/readeck/readeck/pkg/img"
)

const (
	resourceDirName = "_resources"
)

var ctxLogger = struct{}{}

var (
	// We can't process too many images at the same time if we don't want to overload
	// the system and freeze everything just because the image processing has way
	// too much work to do.
	imgSem = semaphore.NewWeighted(2)
	imgCtx = context.TODO()
)

// newArchive runs the archiver and returns a BookmarkArchive instance.
func newArchive(ctx context.Context, ex *extract.Extractor) (*archiver.Archiver, error) {
	req := &archiver.Request{
		// Client: ex.Client(),
		Input: bytes.NewReader(ex.HTML),
		URL:   ex.Drop().URL,
	}

	arc, err := archiver.New(req)
	if err != nil {
		return nil, err
	}

	arc.MaxConcurrentDownload = 4
	arc.Flags = archiver.EnableImages
	arc.RequestTimeout = 45 * time.Second

	arc.EventHandler = eventHandler(ex)

	arc.ImageProcessor = imageProcessor
	arc.URLProcessor = urlProcessor

	if err := arc.Archive(context.Background()); err != nil {
		return nil, err
	}

	return arc, nil
}

var (
	mimeTypes = map[string]string{
		"application/javascript":        ".js",
		"application/json":              ".json",
		"application/ogg":               ".ogx",
		"application/pdf":               ".pdf",
		"application/rtf":               ".rtf",
		"application/vnd.ms-fontobject": ".eot",
		"application/xhtml+xml":         ".xhtml",
		"application/xml":               ".xml",
		"audio/aac":                     ".aac",
		"audio/midi":                    ".midi",
		"audio/x-midi":                  ".midi",
		"audio/mpeg":                    ".mp3",
		"audio/ogg":                     ".oga",
		"audio/opus":                    ".opus",
		"audio/wav":                     ".wav",
		"audio/webm":                    ".weba",
		"font/otf":                      ".otf",
		"font/ttf":                      ".ttf",
		"font/woff":                     ".woff",
		"font/woff2":                    ".woff2",
		"image/bmp":                     ".bmp",
		"image/gif":                     ".gif",
		"image/jpeg":                    ".jpg",
		"image/png":                     ".png",
		"image/svg+xml":                 ".svg",
		"image/tiff":                    ".tiff",
		"image/vnd.microsoft.icon":      ".ico",
		"image/webp":                    ".webp",
		"text/calendar":                 ".ics",
		"text/css":                      ".css",
		"text/csv":                      ".csv",
		"text/html":                     ".html",
		"text/javascript":               ".js",
		"text/plain":                    ".txt",
		"video/mp2t":                    ".ts",
		"video/mp4":                     ".mp4",
		"video/mpeg":                    ".mpeg",
		"video/ogg":                     ".ogv",
		"video/webm":                    ".webm",
		"video/x-msvideo":               ".avi",
	}
)

func eventHandler(ex *extract.Extractor) func(ctx context.Context, arc *archiver.Archiver, evt archiver.Event) {
	entry := log.NewEntry(ex.GetLogger()).WithFields(*ex.LogFields)

	return func(ctx context.Context, arc *archiver.Archiver, evt archiver.Event) {
		switch evt.(type) {
		case *archiver.EventError:
			entry.WithFields(evt.Fields()).Warn("archive error")
		case archiver.EventStartHTML:
			entry.WithFields(evt.Fields()).Info("start archive")
		case *archiver.EventFetchURL:
			entry.WithFields(evt.Fields()).Debug("load archive resource")
		default:
			entry.WithFields(evt.Fields()).Debug("archiver")
		}
	}
}

func getURLfilename(uri string, contentType string) string {
	ext, ok := mimeTypes[strings.Split(contentType, ";")[0]]
	if !ok {
		ext = ".bin"
	}

	return shortuuid.NewWithNamespace(uri) + ext
}

func urlProcessor(uri string, content []byte, contentType string) string {
	return "./" + path.Join(resourceDirName, getURLfilename(uri, contentType))
}

func imageProcessor(ctx context.Context, arc *archiver.Archiver, input io.Reader, contentType string, uri *url.URL) ([]byte, string, error) {
	err := imgSem.Acquire(imgCtx, 1)
	if err != nil {
		return nil, "", err
	}
	defer imgSem.Release(1)

	if _, ok := imageTypes[contentType]; !ok {
		r, err := ioutil.ReadAll(input)
		if err != nil {
			return []byte{}, "", err
		}
		return r, contentType, nil
	}

	im, err := img.New(input)
	// If for any reason, we can't read the image, just return it
	if err != nil {
		arc.SendEvent(ctx, &archiver.EventError{Err: err, URI: uri.String()})
		return nil, "", err
	}
	defer im.Close()

	err = im.Pipeline(
		func(im img.Image) error { return im.SetQuality(75) },
		func(im img.Image) error { return im.SetCompression(img.CompressionBest) },
		func(im img.Image) error { return im.Fit(1280, 1920) },
	)
	if err != nil {
		arc.SendEvent(ctx, &archiver.EventError{Err: err, URI: uri.String()})
		return nil, "", err
	}

	var buf bytes.Buffer
	err = im.Encode(&buf)
	if err != nil {
		arc.SendEvent(ctx, &archiver.EventError{Err: err, URI: uri.String()})
		return nil, "", err
	}

	arc.SendEvent(ctx, archiver.EventInfo{"uri": uri.String(), "format": im.Format()})
	return buf.Bytes(), "image/" + im.Format(), nil
}

// Note: we skip gif files since they're usually optimized already
// and could be animated, which isn't supported by all backends.
var imageTypes = map[string]struct{}{
	"image/bmp":  {},
	"image/jpg":  {},
	"image/jpeg": {},
	"image/png":  {},
	"image/tiff": {},
	"image/webp": {},
}
