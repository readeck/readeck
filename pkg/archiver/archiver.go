package archiver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"golang.org/x/sync/semaphore"
)

// ArchiveFlag is an archiver feature to enable.
type ArchiveFlag uint8

const (
	// EnableCSS enables extraction of CSS files and tags.
	EnableCSS ArchiveFlag = 1 << iota

	// EnableEmbeds enables extraction of Embedes contents.
	EnableEmbeds

	// EnableJS enables extraction of JavaScript contents.
	EnableJS

	// EnableMedia enables extraction of media contents
	// other than image.
	EnableMedia

	// EnableImages enables extraction of images.
	EnableImages
)

// Logger is the interface that must implement the archiver
// logger.
type Logger interface {
	Info(...interface{})
	Debug(...interface{})
	Warn(...interface{})
}

type defaultLogger struct {
	*log.Logger
}

func (l *defaultLogger) Info(args ...interface{}) {
	l.Print(args...)
}
func (l *defaultLogger) Debug(args ...interface{}) {
	l.Print(args...)
}
func (l *defaultLogger) Warn(args ...interface{}) {
	l.Print(args...)
}

// Request is data of archival request.
type Request struct {
	Input  io.Reader
	URL    *url.URL
	Client *http.Client
	Logger Logger
}

// Asset is asset that used in a web page.
type Asset struct {
	Data        []byte
	ContentType string
}

// Archiver is the core of obelisk, which used to download a
// web page then embeds its assets.
type Archiver struct {
	sync.RWMutex

	Cache   map[string]Asset
	Request *Request
	Result  []byte

	Flags ArchiveFlag

	ImageProcessor imageProcessor
	URLProcessor   urlProcessor

	EnableLog bool
	DebugLog  bool

	RequestTimeout        time.Duration
	SkipTLSVerification   bool
	MaxConcurrentDownload int64

	cookies     []*http.Cookie
	dlSemaphore *semaphore.Weighted
}

// New creates a new Archiver using a Request instance.
func New(req *Request) (*Archiver, error) {
	if req.URL == nil {
		return nil, errors.New("no URL in request")
	}
	if req.Input == nil {
		return nil, errors.New("no input in request")
	}
	if req.Client == nil {
		req.Client = http.DefaultClient
	}
	if req.Logger == nil {
		req.Logger = &defaultLogger{log.New(os.Stdout, "[archiver] ", log.LstdFlags)}
	}

	return &Archiver{
		Cache:   make(map[string]Asset),
		Request: req,
		Result:  []byte{},

		Flags: EnableImages | EnableEmbeds,

		ImageProcessor: DefaultImageProcessor,
		URLProcessor:   DefaultURLProcessor,

		EnableLog: true,
		DebugLog:  false,

		RequestTimeout:        20 * time.Second,
		SkipTLSVerification:   false,
		MaxConcurrentDownload: 10,

		cookies: make([]*http.Cookie, 0),
	}, nil
}

// Archive starts archival process for the specified request.
// Returns the archival result, content type and error if there are any.
func (arc *Archiver) Archive(ctx context.Context) error {
	arc.dlSemaphore = semaphore.NewWeighted(arc.MaxConcurrentDownload)

	res, err := arc.processHTML(ctx, arc.Request.Input, arc.Request.URL)
	if err != nil {
		return err
	}

	arc.Result = []byte(res)
	return nil
}

func (arc *Archiver) log(format string, v ...interface{}) {
	if !arc.EnableLog {
		return
	}
	arc.Request.Logger.Info(fmt.Sprintf(format, v...))
}

func (arc *Archiver) debug(format string, v ...interface{}) {
	if !arc.EnableLog || !arc.DebugLog {
		return
	}
	arc.Request.Logger.Debug(fmt.Sprintf(format, v...))
}

func (arc *Archiver) error(err error, args ...interface{}) {
	if !arc.EnableLog {
		return
	}
	args = append(args, err.Error())
	arc.Request.Logger.Warn(args...)
}

func (arc *Archiver) logURL(url, parentURL string, isCached bool) {
	cached := ""
	if isCached {
		cached = " (cached)"
	}
	arc.debug("%s%s (from %s)", url, cached, parentURL)
}

func (arc *Archiver) downloadFile(url string, parentURL string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	if parentURL != "" {
		req.Header.Set("Referer", parentURL)
	}

	for _, cookie := range arc.cookies {
		req.AddCookie(cookie)
	}

	resp, err := arc.Request.Client.Do(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}
