package fftr

import (
	"net/url"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFftr(t *testing.T) {
	folders := ConfigFolderList{
		{os.DirFS("./test-fixtures"), "fixtures"},
		{os.DirFS("./site-config/custom"), "custom"},
		{os.DirFS("./site-config/standard"), "standard"},
	}

	t.Run("merge", func(t *testing.T) {
		cf := &Config{
			Files:         []string{"cf1"},
			BodySelectors: []string{"//div[@id='content']"},
			Prune:         true,
			HTTPHeaders: map[string]string{
				"x-test": "abc",
			},
		}
		cf.Merge(&Config{
			Files:         []string{"cf2"},
			BodySelectors: []string{"//div[@id='page']"},
			Prune:         false,
			HTTPHeaders: map[string]string{
				"x-test": "123",
				"x-v":    "abc",
			},
		})

		assert.Equal(t, &Config{
			Files:         []string{"cf1", "cf2"},
			BodySelectors: []string{"//div[@id='content']", "//div[@id='page']"},
			Prune:         false,
			HTTPHeaders: map[string]string{
				"x-test": "123",
				"x-v":    "abc",
			},
		}, cf)
	})

	t.Run("simple config", func(t *testing.T) {
		src, _ := url.Parse("https://w3.org/")
		cf, err := NewConfigForURL(src, folders)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, []string{
			"standard/w3.org.json",
			"standard/global.json",
		}, cf.Files)
	})

	t.Run("custom config", func(t *testing.T) {
		src, _ := url.Parse("http://www.longform.org/")
		cf, err := NewConfigForURL(src, folders)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, []string{
			"custom/longform.org.toml",
			"standard/longform.org.json",
			"standard/global.json",
		}, cf.Files)
	})

	t.Run("wildcard", func(t *testing.T) {
		src, _ := url.Parse("http://blogs.reuters.com/")
		cf, err := NewConfigForURL(src, folders)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, []string{
			"standard/blogs.reuters.com.json",
			"standard/.reuters.com.json",
			"standard/global.json",
		}, cf.Files)
	})

	t.Run("only wildcard", func(t *testing.T) {
		src, _ := url.Parse("http://whatever.reuters.com/")
		cf, err := NewConfigForURL(src, folders)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, []string{
			"standard/.reuters.com.json",
			"standard/global.json",
		}, cf.Files)
	})

	t.Run("http_headers", func(t *testing.T) {
		src, _ := url.Parse("http://voices.washingtonpost.com/nn")
		cf, err := NewConfigForURL(src, folders)
		if err != nil {
			t.Fatal(err)
		}

		assert.Contains(t, cf.HTTPHeaders["Cookie"], "osfam=0;")
	})

	t.Run("IDNA", func(t *testing.T) {
		src, _ := url.Parse("http://p??rotin.com/")
		cf, err := NewConfigForURL(src, folders)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, []string{
			"standard/xn--protin-bva.com.json",
			"standard/global.json",
		}, cf.Files)
	})

	t.Run("autodetect_on_failure", func(t *testing.T) {
		src, _ := url.Parse("http://example.net/")
		cf, err := NewConfigForURL(src, folders)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, []string{
			"fixtures/example.net.toml",
		}, cf.Files)
	})

	t.Run("decode error", func(t *testing.T) {
		src, _ := url.Parse("http://error.example.net/")
		cf, err := NewConfigForURL(src, folders)
		assert.Nil(t, cf)
		if assert.Error(t, err) {
			assert.Contains(t, err.Error(), "cannot unmarshal")
		}
	})
}
