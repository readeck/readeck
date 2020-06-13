package extract

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
)

func TestClient(t *testing.T) {
	t.Run("Client", func(t *testing.T) {
		client := NewClient()
		assert.Equal(t, 10*time.Second, client.Timeout)

		tr := client.Transport.(*Transport)
		assert.Equal(t, "en-US,en;q=0.8", tr.header.Get("Accept-Language"))
		assert.Equal(t, "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8", tr.header.Get("Accept"))

		htr := tr.tr.(*http.Transport)
		assert.Equal(t, true, htr.DisableKeepAlives)
		assert.Equal(t, true, htr.DisableCompression)
		assert.Equal(t, 1, htr.MaxIdleConns)
		assert.Equal(t, false, htr.ForceAttemptHTTP2)
	})

	t.Run("SetHeader", func(t *testing.T) {
		client := NewClient()
		SetHeader(client, "x-test", "abc")

		tr := client.Transport.(*Transport)
		assert.Equal(t, "abc", tr.header.Get("x-test"))
	})

	t.Run("RoundTrip", func(t *testing.T) {
		type echoResponse struct {
			URL    string
			Method string
			Header http.Header
		}

		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder("GET", `=~.*`,
			func(req *http.Request) (*http.Response, error) {
				return httpmock.NewJsonResponse(200, echoResponse{
					URL:    req.URL.String(),
					Header: req.Header,
				})
			})

		client := NewClient()
		clientHeaders := client.Transport.(*Transport).header

		rsp, err := client.Get("https://example.net/")
		if err != nil {
			t.Fatal(err)
		}
		defer rsp.Body.Close()

		dec := json.NewDecoder(rsp.Body)
		var data echoResponse
		dec.Decode(&data)

		assert.Equal(t, "https://example.net/", data.URL)
		assert.Equal(t, clientHeaders, data.Header)
	})
}
