package admin

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/readeck/readeck/internal/auth"
	"github.com/readeck/readeck/internal/auth/users"
	"github.com/readeck/readeck/internal/server"
	"github.com/readeck/readeck/pkg/form"
	"github.com/readeck/readeck/pkg/timers"
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
		"Pagination": ul.Pagination,
		"Users":      ul.Items,
	}

	h.srv.RenderTemplate(w, r, 200, "/admin/user_list", ctx)
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
				h.srv.AddFlash(w, r, "success", "User created")
				h.srv.Redirect(w, r, "./..", fmt.Sprint(u.ID))
				return
			}
		}
	}

	ctx := server.TC{
		"Form": f,
	}
	h.srv.RenderTemplate(w, r, 200, "/admin/user_create", ctx)
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
					sess := h.srv.GetSession(r)
					sess.Values["u"] = u.ID
					sess.Values["s"] = u.Seed
				}
				h.srv.AddFlash(w, r, "success", "User updated")
				h.srv.Redirect(w, r, fmt.Sprint(u.ID))
				return
			} else {
				h.srv.Error(w, r, err)
				return
			}
		}
	}

	ctx := server.TC{
		"User": item,
		"Form": f,
	}

	h.srv.RenderTemplate(w, r, 200, "/admin/user", ctx)
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
		if err := h.deleteUser(r, u); err != nil {
			log.WithError(err).Error("removing user")
			return
		}
		log.Debug("user removed")
	})
}
