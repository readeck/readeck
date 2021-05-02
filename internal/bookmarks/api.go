package bookmarks

import (
	"context"
	"errors"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/go-chi/chi/v5"
	"github.com/leebenson/conform"
	log "github.com/sirupsen/logrus"
	"github.com/thoas/go-funk"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/internal/auth"
	"codeberg.org/readeck/readeck/internal/server"
	"codeberg.org/readeck/readeck/pkg/form"
	"codeberg.org/readeck/readeck/pkg/timers"
	"codeberg.org/readeck/readeck/pkg/zipfs"
)

var validSchemes = map[string]bool{"http": true, "https": true}

var deleteTimer = timers.NewTimerStore()

type (
	ctxBookmarkKey     struct{}
	ctxBookmarkListKey struct{}
	ctxSearchString    struct{}
	ctxFilterForm      struct{}
	ctxDefaultLimit    struct{}
)

// bookmarkAPI is the base bookmark API router.
type bookmarkAPI struct {
	chi.Router
	srv *server.Server
}

// newBookmarkAPI returns a BookmarkApi with all the routes
// set up.
func newBookmarkAPI(s *server.Server) *bookmarkAPI {
	// Start the job workers
	w := configs.Config.Extractor.NumWorkers
	StartWorkerPool(w)
	log.WithField("workers", w).Info("Started extract workers")

	r := s.AuthenticatedRouter()

	api := &bookmarkAPI{r, s}

	r.With(api.srv.WithPermission("read")).Group(func(r chi.Router) {
		r.With(api.withBookmarkList).Get("/", api.bookmarkList)
		r.With(api.withBookmark).Group(func(r chi.Router) {
			r.Get("/{uid:[a-zA-Z0-9]{18,22}}", api.bookmarkInfo)
			r.Get("/{uid:[a-zA-Z0-9]{18,22}}/article", api.bookmarkArticle)
			r.Get("/{uid:[a-zA-Z0-9]{18,22}}/x/*", api.bookmarkResource)
		})

	})

	r.With(api.srv.WithPermission("write")).Group(func(r chi.Router) {
		r.Post("/", api.bookmarkCreate)
		r.With(api.withBookmark).Group(func(r chi.Router) {
			r.Patch("/{uid:[a-zA-Z0-9]{18,22}}", api.bookmarkUpdate)
			r.Delete("/{uid:[a-zA-Z0-9]{18,22}}", api.bookmarkDelete)
		})
	})

	return api
}

// bookmarkList renders a paginated list of the connected
// user bookmarks in JSON.
func (api *bookmarkAPI) bookmarkList(w http.ResponseWriter, r *http.Request) {
	bl := r.Context().Value(ctxBookmarkListKey{}).(bookmarkList)

	bl.Items = make([]bookmarkItem, len(bl.items))
	for i, item := range bl.items {
		bl.Items[i] = newBookmarkItem(api.srv, r, item, ".")
	}

	api.srv.SendPaginationHeaders(w, r, bl.Pagination.TotalCount, bl.Pagination.Limit, bl.Pagination.Offset)
	api.srv.Render(w, r, http.StatusOK, bl.Items)
}

// bookmarkInfo renders a given bookmark items in JSON.
func (api *bookmarkAPI) bookmarkInfo(w http.ResponseWriter, r *http.Request) {
	b := r.Context().Value(ctxBookmarkKey{}).(*Bookmark)
	item := newBookmarkItem(api.srv, r, b, "./..")

	if api.srv.IsTurboRequest(r) {
		api.srv.RenderTurboStream(w, r,
			"/bookmarks/components/card", "replace",
			"bookmark-card-"+b.UID, item)
		return
	}

	api.srv.Render(w, r, http.StatusOK, item)
}

// bookmarkArticle renders the article HTML content of a bookmark.
// Note that only the body's content is rendered.
func (api *bookmarkAPI) bookmarkArticle(w http.ResponseWriter, r *http.Request) {
	b := r.Context().Value(ctxBookmarkKey{}).(*Bookmark)

	bi := newBookmarkItem(api.srv, r, b, "")
	buf, err := api.getBookmarkArticle(&bi)
	if err != nil {
		api.srv.Error(w, r, err)
		return
	}

	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(200)
	io.Copy(w, buf)
}

// bookmarkCreate creates a new bookmark.
func (api *bookmarkAPI) bookmarkCreate(w http.ResponseWriter, r *http.Request) {
	var uri string
	var html []byte
	var err error
	ct, _, _ := mime.ParseMediaType(r.Header.Get("content-type"))

	if ct == "multipart/form-data" {
		// A multipart form must provide a section with the url and
		// another one with the html source.
		uri, html, err = api.loadCreateParamsHTML(w, r)
		if err != nil {
			api.srv.Message(w, r, &server.Message{
				Status:  http.StatusBadRequest,
				Message: err.Error(),
			})
			return
		}
	} else {
		cf := &createForm{}
		f := form.NewForm(cf)

		form.Bind(f, r)
		if !f.IsValid() {
			api.srv.Render(w, r, http.StatusBadRequest, f)
			return
		}
		uri = cf.URL
	}

	b, err := api.createBookmark(r, uri, html)
	if err != nil {
		api.srv.Error(w, r, err)
		return
	}

	w.Header().Add(
		"Location",
		api.srv.AbsoluteURL(r, ".", b.UID).String(),
	)
	api.srv.TextMessage(w, r, http.StatusAccepted, "Link submited")
}

// bookmarkUpdate updates an existing bookmark.
func (api *bookmarkAPI) bookmarkUpdate(w http.ResponseWriter, r *http.Request) {
	uf := &updateForm{}
	f := form.NewForm(uf)
	form.Bind(f, r)

	if !f.IsValid() {
		api.srv.Render(w, r, http.StatusBadRequest, f)
		return
	}

	b := r.Context().Value(ctxBookmarkKey{}).(*Bookmark)

	updated, err := api.updateBookmark(b, uf, r)
	if err != nil {
		api.srv.Error(w, r, err)
		return
	}

	updated["href"] = api.srv.AbsoluteURL(r).String()

	if api.srv.IsTurboRequest(r) {
		item := newBookmarkItem(api.srv, r, b, "./..")

		_, withLabels := updated["labels"]
		_, withMarked := updated["is_marked"]
		_, withArchived := updated["is_archived"]
		_, withDeleted := updated["is_deleted"]

		if withLabels {
			api.srv.RenderTurboStream(w, r,
				"/bookmarks/components/labels", "replace",
				"bookmark-label-list-"+b.UID, item)
		}
		if withMarked || withArchived || withDeleted {
			api.srv.RenderTurboStream(w, r,
				"/bookmarks/components/actions", "replace",
				"bookmark-actions-"+b.UID, item)
			api.srv.RenderTurboStream(w, r,
				"/bookmarks/components/card", "replace",
				"bookmark-card-"+b.UID, item)
		}
		if withMarked || withArchived {
			api.srv.RenderTurboStream(w, r,
				"/bookmarks/components/bottom_actions", "replace",
				"bookmark-bottom-actions-"+b.UID, item)
		}
		return
	}

	w.Header().Add(
		"Location",
		updated["href"].(string),
	)
	api.srv.Render(w, r, http.StatusOK, updated)
}

// bookmarkDelete deletes a bookmark.
func (api *bookmarkAPI) bookmarkDelete(w http.ResponseWriter, r *http.Request) {
	b := r.Context().Value(ctxBookmarkKey{}).(*Bookmark)

	if err := b.Update(map[string]interface{}{}); err != nil {
		api.srv.Error(w, r, err)
		return
	}

	api.launchDelete(b, r)
	w.WriteHeader(http.StatusNoContent)
}

// bookmarkResource is the route returning any resource
// from a given bookmark. The resource is extracted from
// the sidecar zip file of a bookmark.
// Note that for images, we'll use another route that is not
// authenticated and thus, much faster.
func (api *bookmarkAPI) bookmarkResource(w http.ResponseWriter, r *http.Request) {
	b := r.Context().Value(ctxBookmarkKey{}).(*Bookmark)
	p := path.Clean(chi.URLParam(r, "*"))

	r2 := new(http.Request)
	*r2 = *r
	r2.URL = new(url.URL)
	*r2.URL = *r.URL
	r2.URL.Path = p

	fs := zipfs.HTTPZipFile(b.getFilePath())
	fs.ServeHTTP(w, r2)
}

// withBookmark returns a router that will fetch a bookmark and add it into the
// request's context. It also deals with if-modified-since header.
func (api *bookmarkAPI) withBookmark(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid := chi.URLParam(r, "uid")

		b, err := Bookmarks.GetOne(
			goqu.C("uid").Eq(uid),
			goqu.C("user_id").Eq(auth.GetRequestUser(r).ID),
		)
		if err != nil {
			api.srv.Status(w, r, http.StatusNotFound)
			return
		}
		ctx := context.WithValue(r.Context(), ctxBookmarkKey{}, b)

		if b.State == StateLoaded {
			api.srv.WriteLastModified(w, b)
			api.srv.WriteEtag(w, b)
		}

		api.srv.WithCaching(next).ServeHTTP(w, r.WithContext(ctx))
	})
}

func (api *bookmarkAPI) withBookmarkFilters(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		filter := chi.URLParam(r, "filter")
		filters := &filterForm{}

		switch filter {
		case "unread":
			filters.setArchived(false)
		case "archives":
			filters.setArchived(true)
		case "favorites":
			filters.setMarked(true)
		}

		ctx := context.WithValue(r.Context(), ctxFilterForm{}, filters)
		next.ServeHTTP(w, r.Clone(ctx))
	})
}

func (api *bookmarkAPI) withDefaultLimit(limit int) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), ctxDefaultLimit{}, limit)
			next.ServeHTTP(w, r.Clone(ctx))
		})
	}
}

func (api *bookmarkAPI) withBookmarkList(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := bookmarkList{}

		pf, _ := api.srv.GetPageParams(r)
		if pf == nil {
			api.srv.Status(w, r, http.StatusNotFound)
			return
		}
		if pf.Limit == 0 {
			if limit, ok := r.Context().Value(ctxDefaultLimit{}).(int); ok {
				pf.Limit = limit
			} else {
				pf.Limit = 50
			}
		}

		ds := Bookmarks.Query().
			Select(
				"b.id", "b.uid", "b.created", "b.updated", "b.state", "b.url", "b.title",
				"b.domain", "b.site", "b.site_name", "b.authors", "b.lang", "b.type",
				"b.is_marked", "b.is_archived",
				"b.labels", "b.description", "b.word_count", "b.file_path", "b.files").
			Where(
				goqu.C("user_id").Eq(auth.GetRequestUser(r).ID),
			)

		ds = ds.Order(goqu.I("created").Desc())

		// Search filter
		search := &searchForm{}
		sf := form.NewForm(search)
		sf.BindValues(r.URL.Query())
		if search.Query != "" {
			st := newSearchString(search.Query)
			ds = st.toSelectDataSet(ds)
		}

		// Status filters
		// status come first from request context and if nothing is found
		// we handle the querystring filters.
		var filters *filterForm
		filters, ok := r.Context().Value(ctxFilterForm{}).(*filterForm)
		if !ok {
			filters = &filterForm{}
			ff := form.NewForm(filters)
			ff.BindValues(r.URL.Query())
		}

		if filters.IsMarked != nil {
			ds = ds.Where(goqu.C("is_marked").Table("b").Eq(goqu.V(filters.IsMarked)))
		}
		if filters.IsArchived != nil {
			ds = ds.Where(goqu.C("is_archived").Table("b").Eq(goqu.V(filters.IsArchived)))
		}

		ds = ds.
			Limit(uint(pf.Limit)).
			Offset(uint(pf.Offset))

		var count int64
		var err error
		if count, err = ds.ClearOrder().ClearLimit().ClearOffset().Count(); err != nil {
			if errors.Is(err, ErrNotFound) {
				api.srv.TextMessage(w, r, http.StatusNotFound, "not found")
			} else {
				api.srv.Error(w, r, err)
			}
			return
		}

		res.items = []*Bookmark{}
		if err = ds.ScanStructs(&res.items); err != nil {
			api.srv.Error(w, r, err)
			return
		}

		res.Pagination = api.srv.NewPagination(r, int(count), pf.Limit, pf.Offset)

		ctx := context.WithValue(r.Context(), ctxBookmarkListKey{}, res)
		if search.Query != "" {
			ctx = context.WithValue(ctx, ctxSearchString{}, search.Query)
		}

		if r.Method == http.MethodGet {
			api.srv.WriteEtag(w, res)
		}
		api.srv.WithCaching(next).ServeHTTP(w, r.Clone(ctx))
	})
}

// getBookmarkArticle returns a strings.Reader containing the
// HTML content of a bookmark. Only the body is retrieved.
func (api *bookmarkAPI) getBookmarkArticle(b *bookmarkItem) (*strings.Reader, error) {
	return b.getArticle(b.mediaURL.String())
}

// createBookmark creates a new bookmark and starts the extraction process.
func (api *bookmarkAPI) createBookmark(r *http.Request, u string, html []byte) (*Bookmark, error) {
	uri, _ := url.Parse(u)

	b := &Bookmark{
		UserID:   &auth.GetRequestUser(r).ID,
		State:    StateLoading,
		URL:      uri.String(),
		Site:     uri.Hostname(),
		SiteName: uri.Hostname(),
	}

	if err := Bookmarks.Create(b); err != nil {
		return nil, err
	}

	// Start extraction job
	ctx := context.WithValue(
		context.Background(),
		ctxJobRequestID{},
		api.srv.GetReqID(r),
	)
	enqueueExtractPage(ctx, b, html)
	return b, nil
}

// loadCreateParamsHTML return the url and html passed in a multi-part form.
// The content is then passed to the extractor which won't fetch the HTML from
// the provided url.
func (api *bookmarkAPI) loadCreateParamsHTML(_ http.ResponseWriter, r *http.Request) (uri string, html []byte, err error) {
	const maxMemory = 8 << 20
	err = r.ParseMultipartForm(maxMemory)
	if err != nil {
		return
	}

	cf := &createForm{r.FormValue("url")}
	f := form.NewForm(cf)
	conform.Strings(cf)
	f.Validate()

	if !f.IsValid() {
		err = errors.New("Invalid URL")
		return
	}
	uri = cf.URL

	reader, _, err := r.FormFile("src")
	if errors.Is(err, http.ErrMissingFile) {
		err = errors.New(`File "src" not found`)
		return
	} else if err != nil {
		return
	}

	html, err = ioutil.ReadAll(reader)
	return
}

// updateBookmark update a bookmark and returns the fields
// that have been modified.
func (api *bookmarkAPI) updateBookmark(b *Bookmark, uf *updateForm, r *http.Request) (map[string]interface{}, error) {
	updated := map[string]interface{}{}
	var deleted interface{}

	if uf.IsMarked != nil {
		b.IsMarked = *uf.IsMarked
		updated["is_marked"] = b.IsMarked
	}
	if uf.IsArchived != nil {
		b.IsArchived = *uf.IsArchived
		updated["is_archived"] = b.IsArchived
	}
	if uf.IsDeleted != nil {
		deleted = *uf.IsDeleted
	}

	// Set labels
	labelsChanged := false
	if uf.Labels != nil {
		b.Labels = funk.UniqString(uf.Labels)
		labelsChanged = true
	}

	// Add labels
	if uf.AddLabels != nil {
		b.Labels = funk.UniqString(append(b.Labels, uf.AddLabels...))
		labelsChanged = true
	}

	// Remove has the last say
	if uf.RemoveLabels != nil {
		_, b.Labels = funk.DifferenceString(uf.RemoveLabels, b.Labels)
		labelsChanged = true
	}

	if labelsChanged {
		sort.Strings(b.Labels)
		updated["labels"] = b.Labels
	}

	if len(updated) > 0 || deleted != nil {
		updated["updated"] = time.Now()
		if err := b.Update(updated); err != nil {
			return updated, err
		}
		if d, ok := deleted.(bool); ok && d {
			api.launchDelete(b, r)
			updated["is_deleted"] = d
		} else if ok && !d {
			api.cancelDelete(b, r)
			updated["is_deleted"] = d
		}
	}

	updated["id"] = b.UID
	return updated, nil
}

func (api *bookmarkAPI) launchDelete(b *Bookmark, r *http.Request) {
	l := api.srv.Log(r).WithField("uid", b.UID).WithField("id", b.ID)
	l.Debug("launching bookmark removal")

	deleteTimer.Start(b.ID, 20*time.Second, func() {
		if err := b.Delete(); err != nil {
			l.WithError(err).Error("Error deleting bookmark")
			return
		}

		l.Debug("bookmark deleted")
	})
}

func (api *bookmarkAPI) cancelDelete(b *Bookmark, r *http.Request) {
	api.srv.Log(r).WithField("uid", b.UID).WithField("id", b.ID).
		Debug("removal canceled")

	deleteTimer.Stop(b.ID)
}

// bookmarkList is a paginated list of BookmarkItem instances.
type bookmarkList struct {
	items      []*Bookmark
	Pagination server.Pagination
	Items      []bookmarkItem
}

func (bl bookmarkList) GetSumStrings() []string {
	r := []string{}
	for i := range bl.items {
		r = append(r, bl.items[i].Updated.String(), bl.items[i].UID)
	}

	return r
}

// bookmarkItem is a serialized bookmark instance that can
// be used directly on the API or by an HTML template.
type bookmarkItem struct {
	*Bookmark `json:"-"`

	ID           string                   `json:"id"`
	Href         string                   `json:"href"`
	Created      time.Time                `json:"created"`
	Updated      time.Time                `json:"updated"`
	State        BookmarkState            `json:"state"`
	Loaded       bool                     `json:"loaded"`
	URL          string                   `json:"url"`
	Title        string                   `json:"title"`
	SiteName     string                   `json:"site_name"`
	Site         string                   `json:"site"`
	Published    *time.Time               `json:"published,omitempty"`
	Authors      []string                 `json:"authors"`
	Lang         string                   `json:"lang"`
	DocumentType string                   `json:"document_type"`
	Type         string                   `json:"type"`
	Description  string                   `json:"description"`
	IsDeleted    bool                     `json:"is_deleted"`
	IsMarked     bool                     `json:"is_marked"`
	IsArchived   bool                     `json:"is_archived"`
	Labels       []string                 `json:"labels"`
	Resources    map[string]*bookmarkFile `json:"resources"`
	Embed        string                   `json:"embed,omitempty"`
	Errors       []string                 `json:"errors,omitempty"`

	mediaURL *url.URL
}

// bookmarkFile is a file attached to a bookmark. If the file is
// an image, the "Width" and "Height" values will be filled.
type bookmarkFile struct {
	Src    string `json:"src"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

// newBookmarkItem builds a BookmarkItem from a Bookmark instance.
func newBookmarkItem(s *server.Server, r *http.Request, b *Bookmark, base string) bookmarkItem {
	res := bookmarkItem{
		Bookmark:     b,
		ID:           b.UID,
		Href:         s.AbsoluteURL(r, base, b.UID).String(),
		Created:      b.Created,
		Updated:      b.Updated,
		State:        b.State,
		Loaded:       b.State != StateLoading,
		URL:          b.URL,
		Title:        b.Title,
		SiteName:     b.SiteName,
		Site:         b.Site,
		Published:    b.Published,
		Authors:      b.Authors,
		Lang:         b.Lang,
		DocumentType: b.DocumentType,
		Description:  b.Description,
		IsDeleted:    deleteTimer.Exists(b.ID),
		IsMarked:     b.IsMarked,
		IsArchived:   b.IsArchived,
		Labels:       make([]string, 0),
		Resources:    make(map[string]*bookmarkFile),

		mediaURL: s.AbsoluteURL(r, "/bm", b.FilePath),
	}

	if b.Labels != nil {
		res.Labels = b.Labels
	}

	switch res.DocumentType {
	case "video":
		res.Type = "video"
	case "image", "photo":
		res.Type = "photo"
	default:
		res.Type = "article"
	}

	for k, v := range b.Files {
		if path.Dir(v.Name) != "img" {
			continue
		}

		f := &bookmarkFile{
			Src: res.mediaURL.String() + "/" + v.Name,
		}

		if v.Size != [2]int{0, 0} {
			f.Width = v.Size[0]
			f.Height = v.Size[1]
		}
		res.Resources[k] = f
	}

	if v, ok := b.Files["props"]; ok {
		res.Resources["props"] = &bookmarkFile{Src: s.AbsoluteURL(r, base, b.UID, "x", v.Name).String()}
	}
	if v, ok := b.Files["log"]; ok {
		res.Resources["log"] = &bookmarkFile{Src: s.AbsoluteURL(r, base, b.UID, "x", v.Name).String()}
	}
	if _, ok := b.Files["article"]; ok {
		res.Resources["article"] = &bookmarkFile{Src: s.AbsoluteURL(r, base, b.UID, "article").String()}
	}

	return res
}

type searchForm struct {
	Query string `json:"q" conform:"trim"`
}

type filterForm struct {
	IsMarked   *bool `json:"is_marked"`
	IsArchived *bool `json:"is_archived"`
}

func (ff *filterForm) setMarked(v bool) {
	ff.IsMarked = &v
}
func (ff *filterForm) setArchived(v bool) {
	ff.IsArchived = &v
}

type createForm struct {
	URL string `json:"url" conform:"trim"`
}

func (cf *createForm) Validate(f *form.Form) {
	f.Fields["url"].Validate(
		form.IsRequired,
		form.IsValidURL(validSchemes),
	)
}

type updateForm struct {
	IsMarked     *bool   `json:"is_marked"`
	IsArchived   *bool   `json:"is_archived"`
	IsDeleted    *bool   `json:"is_deleted"`
	Labels       Strings `json:"labels"`
	AddLabels    Strings `json:"add_labels"`
	RemoveLabels Strings `json:"remove_labels"`
	RedirTo      string  `json:"_to"`
}

type deleteForm struct {
	Cancel  bool   `json:"cancel"`
	RedirTo string `json:"_to"`
}
