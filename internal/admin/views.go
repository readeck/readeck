package admin

import (
	"fmt"
	"net/http"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/go-chi/chi/v5"

	"codeberg.org/readeck/readeck/internal/auth"
	"codeberg.org/readeck/readeck/internal/auth/users"
	"codeberg.org/readeck/readeck/internal/server"
	"codeberg.org/readeck/readeck/pkg/form"
	"codeberg.org/readeck/readeck/pkg/timers"
)

// adminViews is an HTTP handler for the user profile web views
type adminViews struct {
	chi.Router
	*adminAPI
}

// Token deletion timers
var userTimers = timers.NewTimerStore()

type deleteForm struct {
	Cancel bool `json:"cancel"`
}

func newAdminViews(api *adminAPI) *adminViews {
	r := api.srv.AuthenticatedRouter()
	h := &adminViews{r, api}

	r.With(api.srv.WithPermission("read")).Group(func(r chi.Router) {
		r.With(api.withUserList).Get("/", h.main)
		r.With(api.withUserList).Get("/users", h.userList)
		r.Get("/users/add", h.userCreate)
		r.With(api.withUser).Get("/users/{id:\\d+}", h.userInfo)
	})

	r.With(api.srv.WithPermission("write")).Group(func(r chi.Router) {
		r.With(api.withUserList).Post("/users", h.userList)
		r.Post("/users/add", h.userCreate)
		r.With(api.withUser).Post("/users/{id:\\d+}", h.userInfo)
		r.With(api.withUser).Post("/users/{id:\\d+}/delete", h.userDelete)
	})

	return h
}

func (h *adminViews) main(w http.ResponseWriter, r *http.Request) {
	h.srv.Redirect(w, r, "./users")
}

func (h *adminViews) userList(w http.ResponseWriter, r *http.Request) {
	ul := r.Context().Value(ctxUserListKey{}).(userList)
	ul.Items = make([]userItem, len(ul.items))
	for i, item := range ul.items {
		ul.Items[i] = newUserItem(h.srv, r, item, ".")
	}

	ctx := server.TC{
		"count":      ul.Pagination.TotalCount,
		"pagination": ul.Pagination,
		"users":      ul.Items,
	}

	h.srv.RenderTemplate(w, r, 200, "admin/user_list.gohtml", ctx)
}

func (h *adminViews) userCreate(w http.ResponseWriter, r *http.Request) {
	uf := &users.CreateForm{}
	f := form.NewForm(uf)

	if r.Method == http.MethodGet {
		defaultGroup := users.GroupChoice("user")
		uf.Group = &defaultGroup
	}

	if r.Method == http.MethodPost {
		form.Bind(f, r)
		if f.IsValid() {
			u, err := h.createUser(uf)
			if err == nil {
				h.srv.AddFlash(w, r, "info", "User created")
				h.srv.Redirect(w, r, "./..", fmt.Sprint(u.ID))
				return
			}
		}
	}

	ctx := server.TC{
		"Form": f,
	}
	h.srv.RenderTemplate(w, r, 200, "admin/user_create.gohtml", ctx)
}

func (h *adminViews) userInfo(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(ctxUserKey{}).(*users.User)
	item := newUserItem(h.srv, r, u, "./..")

	uf := &users.UpdateForm{}
	f := form.NewForm(uf)
	uf.SetUser(f, u)
	g := users.GroupChoice(u.Group)

	if r.Method == http.MethodGet {
		uf.Username = &u.Username
		uf.Email = &u.Email
		uf.Group = &g
	}

	if r.Method == http.MethodPost {
		form.Bind(f, r)

		if f.IsValid() {
			_, err := h.updateUser(u, uf)
			if err == nil {
				// Refresh session if same user
				if auth.GetRequestUser(r).ID == u.ID {
					nu, _ := users.Users.GetOne(goqu.C("id").Eq(u.ID))
					sess := h.srv.GetSession(r)
					sess.Values["check_code"] = nu.CheckCode()
				}
				h.srv.AddFlash(w, r, "info", "User updated")
				h.srv.Redirect(w, r, fmt.Sprint(u.ID))
				return
			}
		}
	}

	ctx := server.TC{
		"item": item,
		"Form": f,
	}

	h.srv.RenderTemplate(w, r, 200, "admin/user.gohtml", ctx)
}

func (h *adminViews) userDelete(w http.ResponseWriter, r *http.Request) {
	df := &deleteForm{}
	f := form.NewForm(df)
	form.Bind(f, r)

	u := r.Context().Value(ctxUserKey{}).(*users.User)
	if u.ID == auth.GetRequestUser(r).ID {
		h.srv.Error(w, r, errSameUser)
		return
	}

	defer func() {
		h.srv.Redirect(w, r, "..", fmt.Sprint(u.ID))
	}()

	if df.Cancel {
		userTimers.Stop(u.ID)
		return
	}

	userTimers.Start(u.ID, 20*time.Second, func() {
		log := h.srv.Log(r).WithField("id", u.ID)
		if err := u.Delete(); err != nil {
			log.WithError(err).Error("removing user")
			return
		}
		log.Debug("user removed")
	})
}
