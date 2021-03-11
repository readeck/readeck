package archiver

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
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

// Request is data of archival request.
type Request struct {
	Input  io.Reader
	URL    *url.URL
	Client *http.Client
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
	EventHandler   eventHandler

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

	return &Archiver{
		Cache:   make(map[string]Asset),
		Request: req,
		Result:  []byte{},

		Flags: EnableImages | EnableEmbeds,

		ImageProcessor: DefaultImageProcessor,
		URLProcessor:   DefaultURLProcessor,

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

	timeout := arc.Request.Client.Timeout
	arc.Request.Client.Timeout = arc.RequestTimeout
	defer func() {
		arc.Request.Client.Timeout = timeout
	}()

	resp, err := arc.Request.Client.Do(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}
