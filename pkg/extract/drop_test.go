package extract

import (
	"errors"
	"net/url"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
)

func TestDrop(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("GET", "/404", httpmock.NewJsonResponderOrPanic(404, ""))
	httpmock.RegisterResponder("GET", "/error", httpmock.NewErrorResponder(errors.New("HTTP")))
	httpmock.RegisterResponder("GET", "/ioerror", newIOErrorResponder(200,
		map[string]string{"content-type": "text/html; charset=UTF-8"}))
	httpmock.RegisterResponder("GET", "/ch1",
		newContentResponder(200,
			map[string]string{"content-type": "text/html; charset=UTF-8"},
			"html/ch1.html"))
	httpmock.RegisterResponder("GET", "/ch1-nocharset",
		newContentResponder(200,
			map[string]string{"content-type": "text/html"},
			"html/ch1.html"))
	httpmock.RegisterResponder("GET", "/ch1-notype",
		newContentResponder(200, nil, "html/ch1.html"))
	httpmock.RegisterResponder("GET", "/ch2",
		newContentResponder(200,
			map[string]string{"content-type": "text/html; charset=ISO-8859-15"},
			"html/ch2.html"))
	httpmock.RegisterResponder("GET", "/ch2-detect",
		newContentResponder(200,
			map[string]string{"content-type": "text/html"},
			"html/ch2.html"))
	httpmock.RegisterResponder("GET", "/ch3",
		newContentResponder(200,
			map[string]string{"content-type": "application/xhtml+xml; charset=EUC-JP"},
			"html/ch3.html"))
	httpmock.RegisterResponder("GET", "/ch3-detect",
		newContentResponder(200,
			map[string]string{"content-type": "application/xhtml+xml"},
			"html/ch3.html"))
	httpmock.RegisterResponder("GET", "/ch4-detect",
		newContentResponder(200,
			map[string]string{"content-type": "text/html"},
			"html/ch4.html"))

	t.Run("errors", func(t *testing.T) {
		tests := []struct {
			name string
			url  *url.URL
			err  string
		}{
			{"no url", nil, "No document URL"},
			{"http", mustParse("http://x/error"), `Get "http://x/error": HTTP`},
			{"404", mustParse("http://x/404"), "Invalid status code (404)"},
			{"ioerror", mustParse("http://x/ioerror"), "read error"},
		}

		for _, x := range tests {
			t.Run(x.name, func(t *testing.T) {
				d := NewDrop(x.url)
				err := d.Load(nil)
				if err == nil {
					t.Fatal("error is nil")
				}

				assert.Equal(t, x.err, err.Error())
			})
		}
	})

	t.Run("url", func(t *testing.T) {
		tests := []struct {
			src string
			res string
			dom string
		}{
			{
				"http://example.net/test/test",
				"http://example.net/test/test",
				"example.net",
			},
			{
				"http://belgië.icom.museum/€test",
				"http://belgië.icom.museum/€test",
				"icom.museum",
			},
			{
				"http://xn--wgv71a.icom.museum/%C2%A9",
				"http://日本.icom.museum/©",
				"icom.museum",
			},
			{
				"http://日本.icom.museum/",
				"http://日本.icom.museum/",
				"icom.museum",
			},
			{
				"http://example.co.jp",
				"http://example.co.jp",
				"example.co.jp",
			},
		}

		for _, x := range tests {
			t.Run(x.src, func(t *testing.T) {
				d := NewDrop(mustParse(x.src))
				assert.Equal(t, x.res, d.UnescapedURL())
				assert.Equal(t, x.dom, d.Domain)
			})
		}
	})

	t.Run("charset", func(t *testing.T) {
		tests := []struct {
			path        string
			isHTML      bool
			isMedia     bool
			contentType string
			charset     string
			contains    string
		}{
			{"ch1", true, false, "text/html", "utf-8", ""},
			{"ch1-nocharset", true, false, "text/html", "utf-8", ""},
			{"ch1-notype", false, false, "", "", ""},
			{"ch2", true, false, "text/html", "iso-8859-15", "grand mammifère"},
			{"ch2-detect", true, false, "text/html", "windows-1252", "grand mammifère"},
			{"ch3", true, false, "application/xhtml+xml", "euc-jp", "センチメートル"},
			{"ch3-detect", true, false, "application/xhtml+xml", "euc-jp", "センチメートル"},
			{"ch4-detect", true, false, "text/html", "utf-8", ""},
		}

		for _, x := range tests {
			t.Run(x.path, func(t *testing.T) {
				d := NewDrop(mustParse("http://x/" + x.path))

				err := d.Load(nil)
				assert.Nil(t, err)
				assert.Equal(t, "x", d.Site)
				assert.Equal(t, x.isHTML, d.IsHTML())
				assert.Equal(t, x.isMedia, d.IsMedia())
				assert.Equal(t, x.contentType, d.ContentType)
				assert.Equal(t, x.charset, d.Charset)

				if x.contains != "" {
					assert.Contains(t, string(d.Body), x.contains)
				}
			})
		}
	})
}

func TestDropAuthors(t *testing.T) {
	uri, _ := url.Parse("/")
	d := NewDrop(uri)

	assert.Equal(t, []string{}, d.Authors)

	d.AddAuthors("John Doe")
	assert.Equal(t, []string{"John Doe"}, d.Authors)

	d.AddAuthors("john Doe")
	assert.Equal(t, []string{"John Doe"}, d.Authors)

	d.AddAuthors("Someone Else")
	assert.Equal(t, []string{"John Doe", "Someone Else"}, d.Authors)

	d.Authors = []string{}
	d.AddAuthors("By   John   Doe")
	assert.Equal(t, []string{"John Doe"}, d.Authors)
	d.AddAuthors(" john doe   ")
	assert.Equal(t, []string{"John Doe"}, d.Authors)
	d.AddAuthors("By:   John   Doe")
	assert.Equal(t, []string{"John Doe"}, d.Authors)
	d.AddAuthors("by :  John   Doe")
	assert.Equal(t, []string{"John Doe"}, d.Authors)
}

func TestDropMeta(t *testing.T) {
	m := DropMeta{}
	m.Add("meta1", "foo")

	assert.Equal(t, []string{"foo"}, m.Lookup("meta1"))

	m.Add("meta1", "bar")
	assert.Equal(t, []string{"foo", "bar"}, m.Lookup("meta1"))
	assert.Equal(t, "foo", m.LookupGet("meta1"))

	assert.Equal(t, []string{}, m.Lookup("meta2"))
	assert.Equal(t, "", m.LookupGet("meta2"))

	m.Add("meta2", "m2a")
	m.Add("meta2", "m2b")
	m.Add("meta3", "m3")

	assert.Equal(t, []string{"m2a", "m2b"}, m.Lookup("metaZ", "meta2", "meta1"))
	assert.Equal(t, "m2a", m.LookupGet("metaZ", "meta2", "meta1"))
}
