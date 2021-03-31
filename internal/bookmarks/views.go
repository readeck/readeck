package bookmarks

import (
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/internal/server"
	"codeberg.org/readeck/readeck/pkg/form"
)

type bookmarkViews struct {
	chi.Router
	*bookmarkAPI
}

func newBookmarkViews(api *bookmarkAPI) *bookmarkViews {
	r := api.srv.AuthenticatedRouter()

	h := &bookmarkViews{r, api}

	r.With(h.srv.WithPermission("read")).Group(func(r chi.Router) {
		r.With(api.withBookmarkList).Get("/", h.bookmarkList)
		r.With(api.withBookmark).Get("/{uid}", h.bookmarkInfo)
	})

	r.With(h.srv.WithPermission("write")).Group(func(r chi.Router) {
		r.With(api.withBookmarkList).Post("/", h.bookmarkList)
		r.With(api.withBookmark).Group(func(r chi.Router) {
			r.Post("/{uid}", h.bookmarkUpdate)
			r.Post("/{uid}/delete", h.bookmarkDelete)
		})
	})

	return h
}

func (h *bookmarkViews) bookmarkList(w http.ResponseWriter, r *http.Request) {
	cf := &createForm{}
	f := form.NewForm(cf)

	// POST => create a new bookmark
	if r.Method == http.MethodPost {
		form.Bind(f, r)
		if f.IsValid() {
			if b, err := h.createBookmark(r, cf.URL, nil); err != nil {
				f.Errors.Add(err)
			} else {
				redir := []string{"/bookmarks"}
				if !h.srv.IsTurboRequest(r) {
					redir = append(redir, b.UID)
				}
				h.srv.Redirect(w, r, redir...)
				return
			}
		}
	}

	// Retrieve the bookmark list
	bl := r.Context().Value(ctxBookmarkListKey{}).(bookmarkList)

	bl.Items = make([]bookmarkItem, len(bl.items))
	for i, item := range bl.items {
		bl.Items[i] = newBookmarkItem(h.srv, r, item, ".")
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
	b := r.Context().Value(ctxBookmarkKey{}).(*Bookmark)
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

	if configs.Config.Main.DevMode {
		for k, x := range map[string]string{
			"_props": "props.json",
			"_log":   "log",
		} {
			if r, err := b.getInnerFile(x); err != nil {
				ctx[k] = err.Error()
			} else {
				ctx[k] = string(r)
			}
		}
	}

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

	b := r.Context().Value(ctxBookmarkKey{}).(*Bookmark)

	if _, err := h.updateBookmark(b, uf, r); err != nil {
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
	b := r.Context().Value(ctxBookmarkKey{}).(*Bookmark)

	if err := b.Update(map[string]interface{}{}); err != nil {
		h.srv.Error(w, r, err)
		return
	}

	if df.Cancel {
		h.cancelDelete(b, r)
	} else {
		h.launchDelete(b, r)
	}

	redir := "/bookmarks"
	if df.RedirTo != "" {
		redir = df.RedirTo
	}

	h.srv.Redirect(w, r, redir)
}
