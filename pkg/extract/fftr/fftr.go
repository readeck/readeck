package fftr

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"path"
	"strings"

	"github.com/komkom/toml"
	"golang.org/x/net/idna"
)

// ConfigFolder is an http.FileSystem with a name.
type ConfigFolder struct {
	fs.FS
	Name string
}

// ConfigFolderList is a list of configuration folders.
type ConfigFolderList []*ConfigFolder

// DefaultConfigurationFolders is a list of default locations with
// configuration files.
var DefaultConfigurationFolders = ConfigFolderList{
	{siteConfigFS("custom"), "custom"},
	{siteConfigFS("standard"), "standard"},
}

// Config holds the fivefilters configuration.
type Config struct {
	Files []string `json:"-"`

	TitleSelectors          []string          `json:"title_selectors"`
	BodySelectors           []string          `json:"body_selectors"`
	DateSelectors           []string          `json:"date_selectors"`
	AuthorSelectors         []string          `json:"author_selectors"`
	StripSelectors          []string          `json:"strip_selectors"`
	StripIDOrClass          []string          `json:"strip_id_or_class"`
	StripImageSrc           []string          `json:"strip_image_src"`
	NativeAdSelectors       []string          `json:"native_ad_selectors"`
	Tidy                    bool              `json:"tidy"`
	Prune                   bool              `json:"prune"`
	AutoDetectOnFailure     bool              `json:"autodetect_on_failure"`
	SinglePageLinkSelectors []string          `json:"single_page_link_selectors"`
	NextPageLinkSelectors   []string          `json:"next_page_link_selectors"`
	ReplaceStrings          [][2]string       `json:"replace_strings"`
	HTTPHeaders             map[string]string `json:"http_headers"`
	Tests                   []FilterTest      `json:"tests"`
}

// FilterTest holds the values for a filter's test.
type FilterTest struct {
	URL      string   `json:"url"`
	Contains []string `json:"contains"`
}

// NewConfig loads a configuration file from an io.Reader.
func NewConfig(r io.Reader, format string) (*Config, error) {
	cf := &Config{}
	cf.AutoDetectOnFailure = true
	switch format {
	case "toml":
		dec := json.NewDecoder(toml.New(r))
		if err := dec.Decode(cf); err != nil {
			return nil, err
		}
	case "json":
		dec := json.NewDecoder(r)
		if err := dec.Decode(cf); err != nil {
			return nil, err
		}
	}

	return cf, nil
}

// NewConfigForURL loads site config configuration file(s) for
// a given URL.
func NewConfigForURL(src *url.URL, folders ConfigFolderList) (*Config, error) {
	res := &Config{}
	res.HTTPHeaders = map[string]string{}
	res.AutoDetectOnFailure = true

	hostname := src.Hostname()
	if strings.HasPrefix(hostname, "www.") {
		hostname = hostname[4:]
	}
	hostname, _ = idna.ToASCII(hostname)

	fileList := folders.findHostFile(hostname)
	fileList = append(fileList, folders.findHostWildcard(hostname)...)
	fileList = append(fileList, folders.findHostFile("global")...)

	for _, x := range fileList {
		fp, _ := x.cf.Open(x.name)
		defer fp.Close()

		cf, err := NewConfig(fp, x.format)
		if err != nil {
			return nil, err
		}
		if !res.AutoDetectOnFailure {
			break
		}
		cf.Files = []string{path.Join(x.cf.Name, x.name)}
		res.Merge(cf)
	}

	return res, nil
}

// Merge merges a new configuration in the current one.
func (cf *Config) Merge(new *Config) {
	cf.Files = append(cf.Files, new.Files...)
	cf.TitleSelectors = append(cf.TitleSelectors, new.TitleSelectors...)
	cf.BodySelectors = append(cf.BodySelectors, new.BodySelectors...)
	cf.DateSelectors = append(cf.DateSelectors, new.DateSelectors...)
	cf.AuthorSelectors = append(cf.AuthorSelectors, new.AuthorSelectors...)
	cf.StripSelectors = append(cf.StripSelectors, new.StripSelectors...)
	cf.StripIDOrClass = append(cf.StripIDOrClass, new.StripIDOrClass...)
	cf.StripImageSrc = append(cf.StripImageSrc, new.StripImageSrc...)
	cf.NativeAdSelectors = append(cf.NativeAdSelectors, new.NativeAdSelectors...)
	cf.Tidy = new.Tidy
	cf.Prune = new.Prune
	cf.AutoDetectOnFailure = new.AutoDetectOnFailure
	cf.SinglePageLinkSelectors = append(cf.SinglePageLinkSelectors, new.SinglePageLinkSelectors...)
	cf.NextPageLinkSelectors = append(cf.NextPageLinkSelectors, new.NextPageLinkSelectors...)
	cf.ReplaceStrings = append(cf.ReplaceStrings, new.ReplaceStrings...)
	cf.Tests = append(cf.Tests, new.Tests...)

	for k, v := range new.HTTPHeaders {
		cf.HTTPHeaders[k] = v
	}
}

func (cf *ConfigFolder) fileExists(name string) bool {
	s, err := fs.Stat(cf.FS, name)
	if err != nil {
		return false
	}
	return !s.IsDir()
}

func (cf *ConfigFolder) fileLookup(name string) (string, string, bool) {
	extensions := []string{"json", "toml"}
	for _, ext := range extensions {
		fname := fmt.Sprintf("%s.%s", name, ext)
		if cf.fileExists(fname) {
			return fname, ext, true
		}
	}

	return "", "", false
}

type lookupResult struct {
	name   string
	format string
	cf     *ConfigFolder
}

func (cf ConfigFolderList) findHostFile(name string) []lookupResult {
	res := []lookupResult{}

	for _, folder := range cf {
		if fname, ext, ok := folder.fileLookup(name); ok {
			res = append(res, lookupResult{fname, ext, folder})
		}
	}

	return res
}

func (cf ConfigFolderList) findHostWildcard(name string) []lookupResult {
	res := []lookupResult{}
	parts := strings.Split(name, ".")
	for _, folder := range cf {
		for i := range parts {
			n := fmt.Sprintf(".%s", strings.Join(parts[i:], "."))
			if fname, ext, ok := folder.fileLookup(n); ok {
				res = append(res, lookupResult{fname, ext, folder})
				break
			}
		}
	}

	return res
}
