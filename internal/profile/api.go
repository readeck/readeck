package profile

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/go-chi/chi/v5"

	"codeberg.org/readeck/readeck/internal/auth"
	"codeberg.org/readeck/readeck/internal/auth/tokens"
	"codeberg.org/readeck/readeck/internal/auth/users"
	"codeberg.org/readeck/readeck/internal/server"
	"codeberg.org/readeck/readeck/pkg/form"
	"codeberg.org/readeck/readeck/pkg/timers"
)

type (
	ctxTokenListKey struct{}
	ctxtTokenKey    struct{}
)

// profileAPI is the base settings API router.
type profileAPI struct {
	chi.Router
	srv *server.Server
}

// Token deletion timers
var tokenTimers = timers.NewTimerStore()

// newProfileAPI returns a SettingAPI with its routes set up.
func newProfileAPI(s *server.Server) *profileAPI {
	r := s.AuthenticatedRouter()
	api := &profileAPI{r, s}

	r.With(api.srv.WithPermission("read")).Group(func(r chi.Router) {
		r.Get("/", api.profileInfo)
		r.With(api.withTokenList).Get("/tokens", api.tokenList)
	})

	r.With(api.srv.WithPermission("write")).Group(func(r chi.Router) {
		r.Patch("/", api.profileUpdate)
		r.Put("/password", api.passwordUpdate)
	})

	return api
}

// UpdateProfile updates the user profile information.
func (api *profileAPI) UpdateProfile(u *users.User, sf *users.ProfileForm) (map[string]interface{}, error) {
	updated := map[string]interface{}{}
	if sf.Email != nil {
		u.Email = *sf.Email
		updated["email"] = u.Email
	}
	if sf.Username != nil {
		u.Username = *sf.Username
		updated["username"] = u.Username
	}

	if len(updated) > 0 {
		updated["updated"] = time.Now()
		if err := u.Update(updated); err != nil {
			return updated, err
		}
	}

	updated["id"] = u.ID
	return updated, nil
}

// UpdatePassword updates the user password.
func (api *profileAPI) UpdatePassword(u *users.User, pf *users.PasswordForm) error {
	return u.SetPassword(pf.Password)
}

// userProfile is the mapping returned by the profileInfo route.
type profileInfoProvider struct {
	Name        string `json:"name"`
	Application string `json:"application"`
}
type profileInfoUser struct {
	Username string              `json:"username"`
	Email    string              `json:"email"`
	Created  time.Time           `json:"created"`
	Updated  time.Time           `json:"updated"`
	Settings *users.UserSettings `json:"settings"`
}
type profileInfo struct {
	Provider profileInfoProvider `json:"provider"`
	User     profileInfoUser     `json:"user"`
}

// profileInfo returns the current user information.
func (api *profileAPI) profileInfo(w http.ResponseWriter, r *http.Request) {
	info := auth.GetRequestAuthInfo(r)

	res := profileInfo{
		Provider: profileInfoProvider{
			Name:        info.Provider.Name,
			Application: info.Provider.Application,
		},
		User: profileInfoUser{
			Username: info.User.Username,
			Email:    info.User.Email,
			Created:  info.User.Created,
			Updated:  info.User.Updated,
			Settings: info.User.Settings,
		},
	}

	api.srv.Render(w, r, 200, res)
}

// profileUpdate updates the current user profile information.
func (api *profileAPI) profileUpdate(w http.ResponseWriter, r *http.Request) {
	uf := &users.ProfileForm{}
	f := form.NewForm(uf)
	form.Bind(f, r)

	if !f.IsValid() {
		api.srv.Render(w, r, http.StatusUnprocessableEntity, f)
		return
	}

	user := auth.GetRequestUser(r)
	updated, err := api.UpdateProfile(user, uf)
	if err != nil {
		api.srv.Error(w, r, err)
		return
	}

	api.srv.Render(w, r, 200, updated)
}

// passwordUpdate updates the current user's password.
func (api *profileAPI) passwordUpdate(w http.ResponseWriter, r *http.Request) {
	pf := &users.PasswordForm{}
	f := form.NewForm(pf)
	form.Bind(f, r)

	if !f.IsValid() {
		api.srv.Render(w, r, http.StatusUnprocessableEntity, f)
		return
	}

	user := auth.GetRequestUser(r)
	if err := api.UpdatePassword(user, pf); err != nil {
		api.srv.Error(w, r, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (api *profileAPI) withTokenList(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := tokenList{}

		pf, _ := api.srv.GetPageParams(r)
		if pf == nil {
			api.srv.Status(w, r, http.StatusNotFound)
			return
		}
		if pf.Limit == 0 {
			pf.Limit = 30
		}

		ds := tokens.Tokens.Query().
			Where(
				goqu.C("user_id").Eq(auth.GetRequestUser(r).ID),
			).
			Order(goqu.I("created").Desc()).
			Limit(uint(pf.Limit)).
			Offset(uint(pf.Offset))

		count, err := ds.ClearOrder().ClearLimit().ClearOffset().Count()
		if err != nil {
			if errors.Is(err, tokens.ErrNotFound) {
				api.srv.TextMessage(w, r, http.StatusNotFound, "not found")
			} else {
				api.srv.Error(w, r, err)
			}
			return
		}

		items := []*tokens.Token{}
		if err := ds.ScanStructs(&items); err != nil {
			api.srv.Error(w, r, err)
			return
		}

		res.Pagination = api.srv.NewPagination(r, int(count), pf.Limit, pf.Offset)

		res.Items = make([]tokenItem, len(items))
		for i, item := range items {
			res.Items[i] = newTokenItem(api.srv, r, item, ".")
		}

		ctx := context.WithValue(r.Context(), ctxTokenListKey{}, res)
		next.ServeHTTP(w, r.Clone(ctx))
	})
}

func (api *profileAPI) withToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid := chi.URLParam(r, "uid")
		t, err := tokens.Tokens.GetOne(
			goqu.C("uid").Eq(uid),
			goqu.C("user_id").Eq(auth.GetRequestUser(r).ID),
		)
		if err != nil {
			api.srv.Status(w, r, http.StatusNotFound)
			return
		}

		item := newTokenItem(api.srv, r, t, ".")
		ctx := context.WithValue(r.Context(), ctxtTokenKey{}, item)
		next.ServeHTTP(w, r.Clone(ctx))
	})
}

func (api *profileAPI) tokenList(w http.ResponseWriter, r *http.Request) {
	tl := r.Context().Value(ctxTokenListKey{}).(tokenList)

	api.srv.SendPaginationHeaders(w, r, tl.Pagination.TotalCount, tl.Pagination.Limit, tl.Pagination.Offset)
	api.srv.Render(w, r, http.StatusOK, tl.Items)
}

type tokenList struct {
	Pagination server.Pagination
	Items      []tokenItem
}

type tokenItem struct {
	*tokens.Token `json:"-"`

	ID        string     `json:"id"`
	Href      string     `json:"href"`
	Created   time.Time  `json:"created"`
	Expires   *time.Time `json:"expires"`
	IsEnabled bool       `json:"is_enabled"`
	IsDeleted bool       `json:"is_deleted"`
}

func newTokenItem(s *server.Server, r *http.Request, t *tokens.Token, base string) tokenItem {
	return tokenItem{
		Token:     t,
		ID:        t.UID,
		Href:      s.AbsoluteURL(r, base, t.UID).String(),
		Created:   t.Created,
		Expires:   t.Expires,
		IsEnabled: t.IsEnabled,
		IsDeleted: tokenTimers.Exists(t.ID),
	}
}
