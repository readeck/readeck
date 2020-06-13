package extract

import (
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"

	"github.com/jarcoal/httpmock"
)

func mustParse(src string) *url.URL {
	u, err := url.Parse(src)
	if err != nil {
		panic(err)
	}
	return u
}

func newFileResponder(name string) httpmock.Responder {
	fd, err := os.Open(path.Join("test-fixtures", name))
	if err != nil {
		panic(err)
	}
	defer fd.Close()

	data, err := ioutil.ReadAll(fd)
	if err != nil {
		panic(err)
	}

	return httpmock.NewBytesResponder(200, data)
}

func newContentResponder(status int, headers map[string]string, name string) httpmock.Responder {
	return func(req *http.Request) (*http.Response, error) {
		fd, err := os.Open(path.Join("test-fixtures", name))
		if err != nil {
			panic(err)
		}
		defer fd.Close()

		data, err := ioutil.ReadAll(fd)
		if err != nil {
			panic(err)
		}

		rsp := httpmock.NewBytesResponse(status, data)
		for k, v := range headers {
			rsp.Header.Set(k, v)
		}
		rsp.Request = req
		return rsp, nil
	}
}

func newHTMLResponder(status int, name string) httpmock.Responder {
	return newContentResponder(
		status,
		map[string]string{"content-type": "text/html"},
		name)
}

type errReader int

func (errReader) Read([]byte) (n int, err error) {
	return 0, errors.New("read error")
}
func (errReader) Close() error {
	return nil
}

func newIOErrorResponder(status int, headers map[string]string) httpmock.Responder {
	return func(req *http.Request) (*http.Response, error) {
		rsp := httpmock.NewBytesResponse(status, []byte{})
		for k, v := range headers {
			rsp.Header.Set(k, v)
		}
		rsp.Request = req
		rsp.Body = errReader(0)
		return rsp, nil
	}
}
