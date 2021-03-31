package admin

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/go-chi/chi/v5"

	"codeberg.org/readeck/readeck/internal/auth"
	"codeberg.org/readeck/readeck/internal/auth/users"
	"codeberg.org/readeck/readeck/internal/server"
	"codeberg.org/readeck/readeck/pkg/form"
)

type (
	ctxUserListKey struct{}
	ctxUserKey     struct{}
)

var (
	errSameUser = errors.New("same user as authenticated")
	errConflict = errors.New("conflicting user attributes")
)

type adminAPI struct {
	chi.Router
	srv *server.Server
}

func newAdminAPI(s *server.Server) *adminAPI {
	r := s.AuthenticatedRouter()
	api := &adminAPI{r, s}

	r.With(api.srv.WithPermission("read")).Group(func(r chi.Router) {
		r.With(api.withUserList).Get("/users", api.userList)
		r.With(api.withUser).Get("/users/{username}", api.userInfo)
	})

	r.With(api.srv.WithPermission("write")).Group(func(r chi.Router) {
		r.With(api.withUserList).Post("/users", api.userCreate)
		r.With(api.withUser).Patch("/users/{username}", api.userUpdate)
		r.With(api.withUser).Delete("/users/{username}", api.userDelete)
	})

	return api
}

func (api *adminAPI) withUserList(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := userList{}

		pf, _ := api.srv.GetPageParams(r)
		if pf == nil {
			api.srv.Status(w, r, http.StatusNotFound)
			return
		}
		if pf.Limit == 0 {
			pf.Limit = 50
		}

		ds := users.Users.Query().
			Order(goqu.I("username").Asc()).
			Limit(uint(pf.Limit)).
			Offset(uint(pf.Offset))

		var count int64
		var err error
		if count, err = ds.ClearOrder().ClearLimit().ClearOffset().Count(); err != nil {
			if errors.Is(err, users.ErrNotFound) {
				api.srv.TextMessage(w, r, http.StatusNotFound, "not found")
			} else {
				api.srv.Error(w, r, err)
			}
			return
		}

		res.items = []*users.User{}
		if err = ds.ScanStructs(&res.items); err != nil {
			api.srv.Error(w, r, err)
			return
		}

		res.Pagination = api.srv.NewPagination(r, int(count), pf.Limit, pf.Offset)

		ctx := context.WithValue(r.Context(), ctxUserListKey{}, res)
		next.ServeHTTP(w, r.Clone(ctx))
	})
}

func (api *adminAPI) withUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username := chi.URLParam(r, "username")

		u, err := users.Users.GetOne(
			goqu.C("username").Eq(username),
		)
		if err != nil {
			api.srv.Status(w, r, http.StatusNotFound)
			return
		}

		ctx := context.WithValue(r.Context(), ctxUserKey{}, u)
		next.ServeHTTP(w, r.Clone(ctx))
	})
}

func (api *adminAPI) createUser(uf *users.CreateForm) error {
	u := &users.User{
		Username: uf.Username,
		Email:    uf.Email,
		Password: uf.Password,
	}
	if uf.Group != nil {
		u.Group = *uf.Group
	} else {
		u.Group = "user"
	}

	c, err := users.Users.Query().Where(goqu.Or(
		goqu.C("email").Eq(uf.Email),
		goqu.C("username").Eq(uf.Username),
	)).Count()
	if err != nil {
		return err
	}
	if c > 0 {
		return errConflict
	}

	return users.Users.Create(u)
}

func (api *adminAPI) updateUser(u *users.User, uf *users.UpdateForm) (map[string]interface{}, error) {
	updated := map[string]interface{}{}
	q := []goqu.Expression{}

	if uf.Username != nil {
		updated["username"] = uf.Username
		q = append(q, goqu.And(
			goqu.C("username").Eq(uf.Username),
			goqu.C("id").Neq(u.ID),
		))
	}
	if uf.Email != nil {
		updated["email"] = uf.Email
		q = append(q, goqu.And(
			goqu.C("email").Eq(uf.Email),
			goqu.C("id").Neq(u.ID),
		))
	}
	if uf.Group != nil {
		updated["group"] = uf.Group
	}
	if uf.Password != nil && *uf.Password != "" {
		var err error
		if updated["password"], err = u.HashPassword(*uf.Password); err != nil {
			return updated, nil
		}
	}

	if len(updated) == 0 {
		return updated, nil
	}

	if len(q) > 0 {
		c, err := users.Users.Query().Where(goqu.Or(q...)).Count()
		if err != nil {
			return updated, err
		}
		if c > 0 {
			return updated, errConflict
		}
	}

	updated["updated"] = time.Now()
	if err := u.Update(updated); err != nil {
		delete(updated, "password")
		return updated, err
	}

	if uf.Password != nil {
		updated["password"] = ""
	}

	return updated, nil
}

func (api *adminAPI) deleteUser(r *http.Request, u *users.User) error {
	if u.ID == auth.GetRequestUser(r).ID {
		return errSameUser
	}

	return u.Delete()
}

func (api *adminAPI) userList(w http.ResponseWriter, r *http.Request) {
	ul := r.Context().Value(ctxUserListKey{}).(userList)
	ul.Items = make([]userItem, len(ul.items))
	for i, item := range ul.items {
		ul.Items[i] = newUserItem(api.srv, r, item, ".")
	}

	api.srv.SendPaginationHeaders(w, r,
		ul.Pagination.TotalCount, ul.Pagination.Limit, ul.Pagination.Offset)
	api.srv.Render(w, r, http.StatusOK, ul.Items)
}

func (api *adminAPI) userInfo(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(ctxUserKey{}).(*users.User)
	item := newUserItem(api.srv, r, u, "./..")

	api.srv.Render(w, r, http.StatusOK, item)
}

func (api *adminAPI) userCreate(w http.ResponseWriter, r *http.Request) {
	uf := &users.CreateForm{}
	f := form.NewForm(uf)

	form.Bind(f, r)
	if !f.IsValid() {
		api.srv.Render(w, r, http.StatusBadRequest, f)
		return
	}

	err := api.createUser(uf)
	if err == nil {
		api.srv.TextMessage(w, r, http.StatusCreated, "User created")
		return
	}
	if errors.Is(err, errConflict) {
		api.srv.TextMessage(w, r, http.StatusConflict, err.Error())
		return
	}

	api.srv.Error(w, r, err)
}

func (api *adminAPI) userUpdate(w http.ResponseWriter, r *http.Request) {
	uf := &users.UpdateForm{}
	f := form.NewForm(uf)

	form.Bind(f, r)
	if !f.IsValid() {
		api.srv.Render(w, r, http.StatusBadRequest, f)
		return
	}

	u := r.Context().Value(ctxUserKey{}).(*users.User)
	updated, err := api.updateUser(u, uf)
	if err == nil {
		api.srv.Render(w, r, http.StatusOK, updated)
		return
	}
	if errors.Is(err, errConflict) {
		api.srv.TextMessage(w, r, http.StatusConflict, err.Error())
		return
	}
	api.srv.Error(w, r, err)
}

func (api *adminAPI) userDelete(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(ctxUserKey{}).(*users.User)

	err := api.deleteUser(r, u)
	if err == nil {
		api.srv.Status(w, r, http.StatusNoContent)
		return
	}
	if errors.Is(err, errSameUser) {
		api.srv.TextMessage(w, r, http.StatusConflict, err.Error())
		return
	}

	api.srv.Error(w, r, err)
}

type userList struct {
	items      []*users.User
	Pagination server.Pagination
	Items      []userItem
}

type userItem struct {
	Href     string    `json:"href"`
	Created  time.Time `json:"created"`
	Updated  time.Time `json:"updated"`
	Username string    `json:"username"`
	Email    string    `json:"email"`
	Group    string    `json:"group"`
}

func newUserItem(s *server.Server, r *http.Request, u *users.User, base string) userItem {
	return userItem{
		Href:     s.AbsoluteURL(r, base, u.Username).String(),
		Created:  u.Created,
		Updated:  u.Updated,
		Username: u.Username,
		Email:    u.Email,
		Group:    u.Group,
	}
}
