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
	"codeberg.org/readeck/readeck/pkg/zipfs"
)

var validSchemes = map[string]bool{"http": true, "https": true}

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
	r.Get("/", api.bookmarkList)
	r.Post("/", api.bookmarkCreate)

	br := r.With(api.withBookmark)
	br.Get("/{uid}", api.bookmarkInfo)
	br.Get("/{uid}/article", api.bookmarkArticle)
	br.Get("/{uid}/x/*", api.bookmarkResource)
	br.Patch("/{uid}", api.bookmarkUpdate)
	br.Delete("/{uid}", api.bookmarkDelete)

	return api
}

// bookmarkList renders a paginated list of the connected
// user bookmarks in JSON.
func (api *bookmarkAPI) bookmarkList(w http.ResponseWriter, r *http.Request) {
	bl, err := api.getBookmarks(r, ".")
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			api.srv.TextMessage(w, r, http.StatusNotFound, "not found")
			return
		}
		api.srv.Error(w, r, err)
		return
	}

	api.srv.SendPaginationHeaders(w, r, bl.Pagination.TotalCount, bl.Pagination.Limit, bl.Pagination.Offset)
	api.srv.Render(w, r, http.StatusOK, bl.Items)
}

// bookmarkInfo renders a given bookmark items in JSON.
func (api *bookmarkAPI) bookmarkInfo(w http.ResponseWriter, r *http.Request) {
	b := r.Context().Value(ctxBookmarkKey).(*Bookmark)
	item := newBookmarkItem(api.srv, r, b, "./..")
	api.srv.Render(w, r, http.StatusOK, item)
}

// bookmarkArticle renders the article HTML content of a bookmark.
// Note that only the body's content is rendered.
func (api *bookmarkAPI) bookmarkArticle(w http.ResponseWriter, r *http.Request) {
	b := r.Context().Value(ctxBookmarkKey).(*Bookmark)

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

	b := r.Context().Value(ctxBookmarkKey).(*Bookmark)

	updated, err := api.updateBookmark(b, uf)
	if err != nil {
		api.srv.Error(w, r, err)
		return
	}

	updated["href"] = api.srv.AbsoluteURL(r).String()

	if api.srv.IsTurboRequest(r) {
		api.srv.RenderTemplate(w, r, 200, "bookmarks/_turbo.gohtml", server.TC{
			"item": newBookmarkItem(api.srv, r, b, "./.."),
		})
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
	b := r.Context().Value(ctxBookmarkKey).(*Bookmark)

	if err := api.deleteBookmark(b); err != nil {
		api.srv.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// bookmarkResource is the route returning any resource
// from a given bookmark. The resource is extracted from
// the sidecar zip file of a bookmark.
// Note that for images, we'll use another route that is not
// authenticated and thus, much faster.
func (api *bookmarkAPI) bookmarkResource(w http.ResponseWriter, r *http.Request) {
	b := r.Context().Value(ctxBookmarkKey).(*Bookmark)
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

		dates := []time.Time{configs.BuildTime(), b.Updated}
		if r.Method == "GET" && len(r.URL.Query()) == 0 && api.srv.CheckIfModifiedSince(
			r, dates...,
		) {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		ctx := context.WithValue(r.Context(), ctxBookmarkKey, b)

		if b.State == StateLoaded {
			api.srv.SetLastModified(w, dates...)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// getBookmarks returns a paginated list of BookmarkItem
// (BookmarkList). It applies pagination and any filter
// parameters that are present in the request.
func (api *bookmarkAPI) getBookmarks(r *http.Request, base string) (bookmarkList, error) {
	res := bookmarkList{}

	pf, _ := api.srv.GetPageParams(r)
	if pf == nil {
		return res, ErrNotFound
	}
	if pf.Limit == 0 {
		pf.Limit = 30
	}

	ds := Bookmarks.Query().
		Select(
			"b.id", "b.uid", "b.created", "b.updated", "b.state", "b.url", "b.title",
			"b.site_name", "b.site", "b.authors", "b.lang", "b.type",
			"b.is_deleted", "b.is_read", "b.is_marked", "b.is_archived",
			"b.tags", "b.description", "b.file_path", "b.files").
		Where(
			goqu.C("user_id").Eq(auth.GetRequestUser(r).ID),
		)

	ds = ds.Order(goqu.I("created").Desc())

	// if strings.TrimSpace(search.Query) != "" {
	// 	ds = Bookmarks.AddSearch(ds, search.Query)
	// }

	ds = ds.
		Limit(uint(pf.Limit)).
		Offset(uint(pf.Offset))

	count, err := ds.ClearOrder().ClearLimit().ClearOffset().Count()
	if err != nil {
		return res, err
	}

	items := []*Bookmark{}
	if err := ds.ScanStructs(&items); err != nil {
		return res, err
	}

	res.Pagination = api.srv.NewPagination(r, int(count), pf.Limit, pf.Offset)

	res.Items = make([]bookmarkItem, len(items))
	for i, item := range items {
		res.Items[i] = newBookmarkItem(api.srv, r, item, base)
	}

	return res, nil
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
		ctxJobRequestID,
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
func (api *bookmarkAPI) updateBookmark(b *Bookmark, uf *updateForm) (map[string]interface{}, error) {
	updated := map[string]interface{}{}
	if uf.IsRead != nil {
		b.IsRead = *uf.IsRead
		updated["is_read"] = b.IsRead
	}
	if uf.IsMarked != nil {
		b.IsMarked = *uf.IsMarked
		updated["is_marked"] = b.IsMarked
	}
	if uf.IsArchived != nil {
		b.IsArchived = *uf.IsArchived
		updated["is_archived"] = b.IsArchived
	}
	if uf.IsDeleted != nil {
		b.IsDeleted = *uf.IsDeleted
		updated["is_deleted"] = b.IsDeleted
	}

	// Set tags
	tagsChanged := false
	if uf.Tags != nil {
		b.Tags = funk.UniqString(uf.Tags)
		tagsChanged = true
	}

	// Add tags
	if uf.AddTags != nil {
		b.Tags = funk.UniqString(append(b.Tags, uf.AddTags...))
		tagsChanged = true
	}

	// Remove has the last say
	if uf.RemoveTags != nil {
		_, b.Tags = funk.DifferenceString(uf.RemoveTags, b.Tags)
		tagsChanged = true
	}

	if tagsChanged {
		sort.Strings(b.Tags)
		updated["tags"] = b.Tags
	}

	if len(updated) > 0 {
		updated["updated"] = time.Now()
		if err := b.Update(updated); err != nil {
			return updated, err
		}
		if updated["is_deleted"] == true {
			api.launchDelete(b)
		}
	}

	updated["id"] = b.UID
	return updated, nil
}

// deleteBookmark removes a given bookmark.
func (api *bookmarkAPI) deleteBookmark(b *Bookmark) error {
	// Mark as removed
	err := b.Update(map[string]interface{}{
		"is_deleted": true,
	})
	if err != nil {
		return err
	}

	api.launchDelete(b)
	return nil
}

func (api *bookmarkAPI) launchDelete(b *Bookmark) {
	uid := b.UID
	time.AfterFunc(30*time.Second, func() {
		l := log.WithField("id", uid)
		b, err := Bookmarks.GetOne(
			goqu.C("is_deleted").Eq(true),
			goqu.C("uid").Eq(uid),
		)
		if err != nil {
			if !errors.Is(err, ErrNotFound) {
				l.WithError(err).Error("Error retrieving bookmark")
			}
			return
		}

		if err := b.Delete(); err != nil {
			l.WithError(err).Error("Error deleting bookmark")
			return
		}
		l.Info("Bookmark deleted")
	})
}

func (api *bookmarkAPI) deleteBookmarkCancel(b *Bookmark) error {
	return b.Update(map[string]interface{}{
		"is_deleted": false,
	})
}

// bookmarkList is a paginated list of BookmarkItem instances.
type bookmarkList struct {
	Pagination server.Pagination
	Items      []bookmarkItem
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
	IsRead       bool                     `json:"is_read"`
	IsMarked     bool                     `json:"is_marked"`
	IsArchived   bool                     `json:"is_archived"`
	Tags         []string                 `json:"tags"`
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
		IsDeleted:    b.IsDeleted,
		IsRead:       b.IsRead,
		IsMarked:     b.IsMarked,
		IsArchived:   b.IsArchived,
		Tags:         make([]string, 0),
		Resources:    make(map[string]*bookmarkFile),

		mediaURL: s.AbsoluteURL(r, "/bm", b.FilePath),
	}

	if b.Tags != nil {
		res.Tags = b.Tags
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

type createForm struct {
	URL string `json:"url" conform:"trim"`
}

func (cf *createForm) Validate(f *form.Form) {
	// An empty value yields an error, so form.Required is
	// not needed in this case.
	form.IsValidURL(f.Fields["url"], validSchemes)
}

type updateForm struct {
	IsRead     *bool   `json:"is_read"`
	IsMarked   *bool   `json:"is_marked"`
	IsArchived *bool   `json:"is_archived"`
	IsDeleted  *bool   `json:"is_deleted"`
	Tags       Strings `json:"tags"`
	AddTags    Strings `json:"add_tags"`
	RemoveTags Strings `json:"remove_tags"`
	RedirTo    string  `json:"_to"`
}

type deleteForm struct {
	Cancel  bool   `json:"cancel"`
	RedirTo string `json:"_to"`
}
