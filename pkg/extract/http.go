package extract

import (
	"net/http"
	"time"
)

// Transport is a wrapper around http.RoundTripper that
// lets you set default headers sent with every request.
type Transport struct {
	tr     http.RoundTripper
	header http.Header
}

// RoundTrip is the transport interceptor.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header = t.header
	return t.tr.RoundTrip(req)
}

// SetHeader lets you set a default header for any subsequent request.
func (t *Transport) SetHeader(name, value string) {
	t.header.Set(name, value)
}

// GetHeader returns a header value from transport
func (t *Transport) GetHeader(name string) string {
	return t.header.Get(name)
}

// NewClient returns a new http.Client with our custom transport.
func NewClient() *http.Client {
	client := http.DefaultClient
	client.Timeout = 10 * time.Second

	tr := http.DefaultTransport
	htr, ok := tr.(*http.Transport)
	if ok {
		htr.DisableKeepAlives = true
		htr.DisableCompression = true
		htr.MaxIdleConns = 1
		htr.ForceAttemptHTTP2 = false
	}

	t := &Transport{tr: http.DefaultTransport, header: http.Header{}}

	t.SetHeader("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:75.0) Gecko/20100101 Firefox/75.0")
	t.SetHeader("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	t.SetHeader("Accept-Language", "en-US,en;q=0.8")
	t.SetHeader("Cache-Control", "max-age=0")
	t.SetHeader("Upgrade-Insecure-Requests", "1")

	client.Transport = t

	return client
}

// SetHeader set a header on a given client
func SetHeader(client *http.Client, name, value string) {
	client.Transport.(*Transport).header.Set(name, value)
}
