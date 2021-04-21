package extract

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"golang.org/x/net/idna"
)

// Transport is a wrapper around http.RoundTripper that
// lets you set default headers sent with every request.
type Transport struct {
	tr        http.RoundTripper
	header    http.Header
	deniedIPs []*net.IPNet
}

// RoundTrip is the transport interceptor.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := t.checkDestIP(req); err != nil {
		return nil, err
	}
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

func (t *Transport) checkDestIP(r *http.Request) error {
	if len(t.deniedIPs) == 0 {
		// An empty list disables the IP check altogether
		return nil
	}

	hostname := r.URL.Hostname()
	host, err := idna.ToASCII(hostname)
	if err != nil {
		return fmt.Errorf("invalid hostname %s", hostname)
	}

	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("cannot resolve %s", host)
	}

	for _, cidr := range t.deniedIPs {
		for _, ip := range ips {
			if cidr.Contains(ip) {
				return fmt.Errorf("ip %s is blocked by rule %s", ip, cidr)
			}
		}
	}

	return nil
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

// SetHeader sets a header on a given client
func SetHeader(client *http.Client, name, value string) {
	if t, ok := client.Transport.(*Transport); ok {
		t.header.Set(name, value)
	}
}
