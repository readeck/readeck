package bookmarks

import (
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/readeck/readeck/internal/server"
	"github.com/readeck/readeck/pkg/form"
)

type bookmarkViews struct {
	chi.Router
	*bookmarkAPI
}

func newBookmarkViews(api *bookmarkAPI) *bookmarkViews {
	r := api.srv.AuthenticatedRouter()

	h := &bookmarkViews{r, api}
	r.HandleFunc("/", h.bookmarkList)

	br := r.With(api.withBookmark)
	br.Get("/{uid}", h.bookmarkInfo)
	br.Post("/{uid}", h.bookmarkUpdate)
	br.Post("/{uid}/delete", h.bookmarkDelete)

	return h
}

func (h *bookmarkViews) bookmarkList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	cf := &createForm{}
	f := form.NewForm(cf)

	// POST => create a new bookmark
	if r.Method == http.MethodPost {
		form.Bind(f, r)
		if f.IsValid() {
			if b, err := h.createBookmark(r, cf.URL, nil); err != nil {
				f.Errors.Add(err)
			} else {
				h.srv.Redirect(w, r, "/bookmarks", b.UID)
				return
			}
		}
	}

	// Retrieve the bookmark list
	bl, err := h.getBookmarks(r, ".")
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		h.srv.Error(w, r, err)
	}

	ctx := server.TC{
		"form":       f,
		"count":      bl.Pagination.TotalCount,
		"pagination": bl.Pagination,
		"bookmarks":  bl.Items,
	}

	h.srv.RenderTemplate(w, r, 200, "bookmarks/index.gohtml", ctx)
}

func (h *bookmarkViews) bookmarkInfo(w http.ResponseWriter, r *http.Request) {
	b := r.Context().Value(ctxBookmarkKey).(*Bookmark)
	item := newBookmarkItem(h.srv, r, b, "")
	item.Embed = b.Embed
	item.Errors = b.Errors

	ctx := server.TC{
		"item": item,
	}

	buf, err := h.getBookmarkArticle(&item)
	if err != nil {
		if os.IsNotExist(err) {
			ctx["html"] = strings.NewReader("")
		} else {
			panic(err)
		}
	} else {
		ctx["html"] = buf
	}

	ctx["out"] = w

	h.srv.RenderTemplate(w, r, 200, "bookmarks/bookmark.gohtml", ctx)
}

func (h *bookmarkViews) bookmarkUpdate(w http.ResponseWriter, r *http.Request) {
	uf := &updateForm{}
	f := form.NewForm(uf)
	form.Bind(f, r)

	if !f.IsValid() {
		h.srv.Render(w, r, http.StatusBadRequest, f)
		return
	}

	b := r.Context().Value(ctxBookmarkKey).(*Bookmark)

	if _, err := h.updateBookmark(b, uf); err != nil {
		h.srv.Error(w, r, err)
		return
	}

	redir := "/bookmarks/" + b.UID
	if uf.RedirTo != "" {
		redir = uf.RedirTo
	}

	h.srv.Redirect(w, r, redir)
}

func (h *bookmarkViews) bookmarkDelete(w http.ResponseWriter, r *http.Request) {
	df := &deleteForm{}
	f := form.NewForm(df)
	form.Bind(f, r)
	b := r.Context().Value(ctxBookmarkKey).(*Bookmark)

	var err error
	if df.Cancel {
		err = h.deleteBookmarkCancel(b)
	} else {
		err = h.deleteBookmark(b)
	}

	if err != nil {
		h.srv.Error(w, r, err)
		return
	}

	redir := "/bookmarks"
	if df.RedirTo != "" {
		redir = df.RedirTo
	}

	h.srv.Redirect(w, r, redir)
}