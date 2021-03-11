package fftr

import (
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"path"
	"strings"

	"github.com/pelletier/go-toml"
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
var DefaultConfigurationFolders ConfigFolderList = ConfigFolderList{
	{siteConfigFS("custom"), "custom"},
	{siteConfigFS("standard"), "standard"},
}

// Config holds the fivefilters configuration.
type Config struct {
	Files []string `toml:"-"`

	TitleSelectors          []string          `toml:"title_selectors"`
	BodySelectors           []string          `toml:"body_selectors"`
	DateSelectors           []string          `toml:"date_selectors"`
	AuthorSelectors         []string          `toml:"author_selectors"`
	StripSelectors          []string          `toml:"strip_selectors"`
	StripIDOrClass          []string          `toml:"strip_id_or_class"`
	StripImageSrc           []string          `toml:"strip_image_src"`
	NativeAdSelectors       []string          `toml:"native_ad_selectors"`
	Tidy                    bool              `toml:"tidy"`
	Prune                   bool              `toml:"prune"`
	AutoDetectOnFailure     bool              `toml:"autodetect_on_failure"`
	SinglePageLinkSelectors []string          `toml:"single_page_link_selectors"`
	NextPageLinkSelectors   []string          `toml:"next_page_link_selectors"`
	ReplaceStrings          [][2]string       `toml:"replace_strings"`
	HTTPHeaders             map[string]string `toml:"http_headers"`
	Tests                   []FilterTest      `toml:"tests"`
}

// FilterTest holds the values for a filter's test.
type FilterTest struct {
	URL      string
	Contains []string
}

// NewConfig loads a configuration file from an io.Reader.
func NewConfig(r io.Reader) (*Config, error) {
	cf := &Config{}
	cf.AutoDetectOnFailure = true
	dec := toml.NewDecoder(r)
	if err := dec.Decode(cf); err != nil {
		return nil, err
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

		cf, err := NewConfig(fp)
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

type lookupResult struct {
	name string
	cf   *ConfigFolder
}

func (cf ConfigFolderList) findHostFile(name string) []lookupResult {
	res := []lookupResult{}

	for _, folder := range cf {
		fname := fmt.Sprintf("%s.toml", name)
		if folder.fileExists(fname) {
			res = append(res, lookupResult{fname, folder})
		}
	}

	return res
}

func (cf ConfigFolderList) findHostWildcard(name string) []lookupResult {
	res := []lookupResult{}
	parts := strings.Split(name, ".")
	for _, folder := range cf {
		for i := range parts {
			fname := fmt.Sprintf(".%s.toml", strings.Join(parts[i:], "."))
			if folder.fileExists(fname) {
				res = append(res, lookupResult{fname, folder})
				break
			}
		}
	}

	return res
}
