package bookmarks

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/lithammer/shortuuid"
	log "github.com/sirupsen/logrus"

	"github.com/readeck/readeck/pkg/archiver"
	"github.com/readeck/readeck/pkg/extract"
)

const (
	resourceDirName = "_resources"
)

var (
	rxTplMark = regexp.MustCompile(`({{|}})`)
)

// NewArchive runs the archiver and returns a BookmarkArchive instance.
func NewArchive(ex *extract.Extractor, logger *log.Entry) (*archiver.Archiver, error) {
	req := &archiver.Request{
		Client: ex.Client(),
		Logger: logger,
		Input:  bytes.NewReader(ex.HTML),
		URL:    ex.Drop().URL,
	}

	arc, err := archiver.New(req)
	if err != nil {
		return nil, err
	}

	arc.EnableLog = true
	arc.DebugLog = true
	arc.MaxConcurrentDownload = 5
	arc.Flags = archiver.EnableImages
	arc.RequestTimeout = 20 * time.Second

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
	r, err := ioutil.ReadAll(input)
	if err != nil {
		return []byte{}, "", err
	}
	return r, contentType, nil
}
