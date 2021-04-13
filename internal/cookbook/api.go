package cookbook

import (
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"runtime"
	"time"

	"github.com/go-chi/chi/v5"
	log "github.com/sirupsen/logrus"

	"codeberg.org/readeck/readeck/internal/bookmarks"
	"codeberg.org/readeck/readeck/internal/server"
	"codeberg.org/readeck/readeck/pkg/extract"
	"codeberg.org/readeck/readeck/pkg/extract/contents"
	"codeberg.org/readeck/readeck/pkg/extract/fftr"
	"codeberg.org/readeck/readeck/pkg/extract/meta"
	"codeberg.org/readeck/readeck/pkg/extract/rules"
)

// cookbookAPI is the base cookbook api router.
type cookbookAPI struct {
	chi.Router
	srv  *server.Server
	urls map[string][]string
}

// newCookbookAPI returns a CookbokAPI with all the routes
// set up.
func newCookbookAPI(s *server.Server) *cookbookAPI {
	r := s.AuthenticatedRouter()

	api := &cookbookAPI{Router: r, srv: s}
	api.loadURLs()
	r.Get("/urls", api.urlList)
	r.Get("/extract", api.extract)

	return api
}

func (api *cookbookAPI) urlList(w http.ResponseWriter, r *http.Request) {
	api.srv.Render(w, r, 200, api.urls)
}

func (api *cookbookAPI) extract(w http.ResponseWriter, r *http.Request) {
	src := r.URL.Query().Get("url")
	if src == "" {
		http.Error(w, http.StatusText(400), 400)
		return
	}

	ex, err := extract.New(src, nil)
	if err != nil {
		panic(err)
	}

	if reqID := api.srv.GetReqID(r); reqID != "" {
		ex.LogFields = &log.Fields{"@id": reqID}
	}

	ex.AddProcessors(
		bookmarks.CheckIPProcessor,
		meta.ExtractMeta,
		meta.ExtractOembed,
		rules.ApplyRules,
		meta.SetDropProperties,
		meta.ExtractFavicon,
		meta.ExtractPicture,
		fftr.LoadConfiguration,
		fftr.ReplaceStrings,
		fftr.FindContentPage,
		fftr.ExtractAuthor,
		fftr.ExtractDate,
		fftr.FindNextPage,
		fftr.ExtractBody,
		fftr.StripTags,
		fftr.GoToNextPage,
		contents.Readability,
		bookmarks.CleanDomProcessor,
		contents.Text,
		archiveProcessor,
	)
	ex.Run()
	runtime.GC()

	// Very rough but good enough for our tests
	if r.Header.Get("accept") == "text/html" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write(ex.HTML)
		return
	}

	drop := ex.Drop()

	res := &extractResult{
		URL:          drop.UnescapedURL(),
		Logs:         ex.Logs,
		Errors:       []string{},
		Meta:         drop.Meta,
		Domain:       drop.Domain,
		Title:        drop.Title,
		Authors:      drop.Authors,
		Site:         drop.URL.Hostname(),
		SiteName:     drop.Site,
		Lang:         drop.Lang,
		Date:         &drop.Date,
		DocumentType: drop.DocumentType,
		Description:  drop.Description,
		HTML:         string(ex.HTML),
		Text:         ex.Text,
		Images:       map[string]*extractImg{},
	}

	if drop.IsMedia() {
		res.Embed = drop.Meta.LookupGet("oembed.html")
	}

	for _, x := range ex.Errors() {
		res.Errors = append(res.Errors, x.Error())
	}
	if res.Date.IsZero() {
		res.Date = nil
	}

	for k, p := range drop.Pictures {
		res.Images[k] = &extractImg{
			Encoded: fmt.Sprintf("data:%s;base64,%s", p.Type, p.Encoded()),
			Size:    p.Size,
		}
	}

	api.srv.Render(w, r, 200, res)
}

func (api *cookbookAPI) loadURLs() {
	api.urls = map[string][]string{}

	for i, configFS := range fftr.DefaultConfigurationFolders {
		files, err := fs.ReadDir(configFS, ".")
		if err != nil {
			panic(err)
		}

		// Parse config files
		for _, x := range files {
			if x.IsDir() || (path.Ext(x.Name()) != ".toml" && path.Ext(x.Name()) != ".json") {
				continue
			}

			f, err := configFS.Open(x.Name())
			if err != nil {
				panic(err)
			}
			cfg, err := fftr.NewConfig(f, path.Ext(x.Name())[1:])
			if err != nil {
				log.WithField(
					"cf", fmt.Sprintf("%s/%s", configFS.Name, x.Name()),
				).WithError(err).Error("error parsing file")
			}
			f.Close()

			if cfg != nil && len(cfg.Tests) > 0 {
				name := fmt.Sprintf("%d - %s - %s", i, configFS.Name,
					path.Base(x.Name()))
				api.urls[name] = make([]string, len(cfg.Tests))
				for i := range cfg.Tests {
					api.urls[name][i] = cfg.Tests[i].URL
				}
			}
		}
	}
}

type extractImg struct {
	Size    [2]int `json:"size"`
	Encoded string `json:"encoded"`
}

type extractResult struct {
	URL          string                 `json:"url"`
	Logs         []string               `json:"logs"`
	Errors       []string               `json:"errors"`
	Meta         extract.DropMeta       `json:"meta"`
	Domain       string                 `json:"domain"`
	Title        string                 `json:"title"`
	Authors      []string               `json:"authors"`
	Site         string                 `json:"site"`
	SiteName     string                 `json:"site_name"`
	Lang         string                 `json:"lang"`
	Date         *time.Time             `json:"date"`
	DocumentType string                 `json:"document_type"`
	Description  string                 `json:"description"`
	HTML         string                 `json:"html"`
	Text         string                 `json:"text"`
	Embed        string                 `json:"embed"`
	Images       map[string]*extractImg `json:"images"`
}
