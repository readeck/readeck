package bookmarks

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/go-chi/chi"
	log "github.com/sirupsen/logrus"

	"github.com/readeck/readeck/configs"
	"github.com/readeck/readeck/internal/server"
)

var validSchemes = map[string]bool{"http": true, "https": true}

// Routes returns the routes for the bookmarks domain.
func Routes(s *server.Server) http.Handler {
	r := chi.NewRouter()
	r.Use(s.WithSession(), s.WithAuth)

	// Start the job workers
	w := configs.Config.Extractor.NumWorkers
	StartExtractPool(w)
	log.WithField("workers", w).Info("Started extract workers")

	type resultFile struct {
		Src    string `json:"src"`
		Width  int    `json:"width,omitempty"`
		Height int    `json:"height,omitempty"`
	}

	type resultItem struct {
		ID           string                 `json:"id"`
		Href         string                 `json:"href"`
		Created      time.Time              `json:"created"`
		Updated      time.Time              `json:"updated"`
		State        BookmarkState          `json:"state"`
		URL          string                 `json:"url"`
		Title        string                 `json:"title"`
		SiteName     string                 `json:"site_name"`
		Site         string                 `json:"site"`
		Published    *time.Time             `json:"published,omitempty"`
		Authors      []string               `json:"authors"`
		Lang         string                 `json:"lang"`
		DocumentType string                 `json:"document_type"`
		Description  string                 `json:"description"`
		IsMarked     bool                   `json:"is_marked"`
		Tags         []string               `json:"tags"`
		Resources    map[string]*resultFile `json:"resources"`
		Embed        string                 `json:"embed,omitempty"`
		Logs         []string               `json:"logs,omitempty"`
		Errors       []string               `json:"errors,omitempty"`
	}

	serializeResult := func(b *Bookmark, r *http.Request) resultItem {
		res := resultItem{
			ID:           b.UID,
			Href:         s.AbsoluteURL(r, ".", b.UID).String(),
			Created:      b.Created,
			Updated:      b.Updated,
			State:        b.State,
			URL:          b.URL,
			Title:        b.Title,
			SiteName:     b.SiteName,
			Site:         b.Site,
			Published:    b.Published,
			Authors:      b.Authors,
			Lang:         b.Lang,
			DocumentType: b.DocumentType,
			Description:  b.Description,
			IsMarked:     b.IsMarked,
			Tags:         make([]string, 0),
			Resources:    make(map[string]*resultFile),
		}

		if b.Tags != nil {
			res.Tags = b.Tags
		}

		for k, v := range b.Files {
			f := &resultFile{Src: s.AbsoluteURL(r, "/media", v.Name).String()}
			if v.Size != [2]int{0, 0} {
				f.Width = v.Size[0]
				f.Height = v.Size[1]
			}
			res.Resources[k] = f
		}

		return res
	}

	type searchParams struct {
		Query string `json:"q" schema:"q"`
	}

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		pageParams, msg := s.GetPageParams(r)
		if msg != nil {
			s.Message(w, r, msg)
			return
		}
		if pageParams.Limit == 0 {
			pageParams.Limit = 30
		}

		search := &searchParams{}
		if msg := s.BindQueryString(r, search); msg != nil {
			s.Message(w, r, msg)
			return
		}

		items := []*Bookmark{}
		ds := Bookmarks.Query().
			Select(
				"b.id", "b.uid", "b.created", "b.updated", "b.state", "b.url", "b.title",
				"b.site_name", "b.site", "b.authors", "b.lang", "b.type",
				"b.is_marked", "b.tags", "b.description", "b.files").
			Where(goqu.C("user_id").Eq(s.GetUser(r).ID))

		ds = ds.Order(goqu.I("created").Desc())

		if strings.TrimSpace(search.Query) != "" {
			ds = Bookmarks.AddSearch(ds, search.Query)
		}

		ds = ds.
			Limit(uint(pageParams.Limit)).
			Offset(uint(pageParams.Offset))

		count, err := ds.ClearOrder().ClearLimit().ClearOffset().Count()
		if err != nil {
			s.Error(w, r, err)
			return
		}

		if err := ds.ScanStructs(&items); err != nil {
			s.Error(w, r, err)
			return
		}

		res := make([]resultItem, len(items))
		for i, item := range items {
			res[i] = serializeResult(item, r)
		}

		s.SendPaginationHeaders(w, r, int(count), pageParams.Limit, pageParams.Offset)
		s.Render(w, r, 200, res)
	})

	type createPayload struct {
		URL string `json:"url"`
	}

	r.Post("/", func(w http.ResponseWriter, r *http.Request) {
		var p createPayload
		if msg := s.LoadJSON(r, &p); msg != nil {
			s.Message(w, r, msg)
			return
		}

		uri, err := url.Parse(p.URL)
		if err != nil {
			s.TextMessage(w, r, 400, http.StatusText(400))
			return
		}

		if !validSchemes[uri.Scheme] {
			s.TextMessage(w, r, 400, "Invalid URL")
			return
		}

		b := &Bookmark{
			UserID:   &s.GetUser(r).ID,
			State:    StateLoading,
			URL:      uri.String(),
			Site:     uri.Hostname(),
			SiteName: uri.Hostname(),
		}

		if err := Bookmarks.Create(b); err != nil {
			panic(err)
		}

		// Start extraction job
		EnqueueExtractPage(b)

		// And tell the client we're all good!
		w.Header().Add(
			"location",
			s.AbsoluteURL(r, ".", b.UID).String(),
		)
		s.Render(w, r, 202, map[string]string{
			"message": "link submited",
		})
	})

	r.Get("/{uid}", func(w http.ResponseWriter, r *http.Request) {
		uid := chi.URLParam(r, "uid")

		b, err := Bookmarks.GetOne(
			goqu.C("uid").Eq(uid),
			goqu.C("user_id").Eq(s.GetUser(r).ID),
		)
		if err != nil {
			s.Status(w, r, 404)
			return
		}

		res := serializeResult(b, r)
		res.Href = s.AbsoluteURL(r).String()
		res.Embed = b.Embed
		res.Logs = b.Logs
		res.Errors = b.Errors

		s.Render(w, r, 200, res)
	})

	type updatePayload struct {
		Refresh  *bool   `json:"refresh"`
		IsMarked *bool   `json:"is_marked"`
		Tags     Strings `json:"tags"`
	}

	r.Patch("/{uid}", func(w http.ResponseWriter, r *http.Request) {
		uid := chi.URLParam(r, "uid")

		data := &updatePayload{}
		if msg := s.LoadJSON(r, data); msg != nil {
			s.Message(w, r, msg)
			return
		}

		b, err := Bookmarks.GetOne(
			goqu.C("uid").Eq(uid),
			goqu.C("user_id").Eq(s.GetUser(r).ID),
		)
		if err != nil {
			s.Status(w, r, 404)
			return
		}

		updated := map[string]interface{}{}
		if data.IsMarked != nil {
			b.IsMarked = *data.IsMarked
			updated["is_marked"] = b.IsMarked
		}
		if data.Tags != nil {
			b.Tags = data.Tags
			updated["tags"] = b.Tags
		}
		if data.Refresh != nil && *data.Refresh {
			b.State = StateLoading
			updated["state"] = b.State
		}

		if len(updated) > 0 {
			if err = b.Update(updated); err != nil {
				s.Error(w, r, err)
				return
			}
		}

		// Start the extraction job
		rspStatus := 200
		if b.State == StateLoading {
			EnqueueExtractPage(b)
			rspStatus = 202
		}

		updated["id"] = b.UID
		updated["href"] = s.AbsoluteURL(r).String()

		w.Header().Add(
			"location",
			s.AbsoluteURL(r).String(),
		)
		s.Render(w, r, rspStatus, updated)
	})

	r.Delete("/{uid}", func(w http.ResponseWriter, r *http.Request) {
		uid := chi.URLParam(r, "uid")

		b, err := Bookmarks.GetOne(
			goqu.C("uid").Eq(uid),
			goqu.C("user_id").Eq(s.GetUser(r).ID),
		)
		if err != nil {
			s.TextMessage(w, r, 404, http.StatusText(404))
			return
		}

		if err := b.Delete(); err != nil {
			s.Error(w, r, err)
			return
		}
		w.WriteHeader(204)
	})

	return r
}
