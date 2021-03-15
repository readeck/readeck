package profile

import (
	"errors"
	"net/http"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/go-chi/chi/v5"

	"github.com/readeck/readeck/internal/auth"
	"github.com/readeck/readeck/internal/auth/tokens"
	"github.com/readeck/readeck/internal/auth/users"
	"github.com/readeck/readeck/internal/server"
	"github.com/readeck/readeck/pkg/form"
)

// profileAPI is the base settings API router.
type profileAPI struct {
	chi.Router
	srv *server.Server
}

// newProfileAPI returns a SettingAPI with its routes set up.
func newProfileAPI(s *server.Server) *profileAPI {
	r := s.AuthenticatedRouter()
	api := &profileAPI{r, s}

	r.Get("/", api.profileInfo)
	r.Patch("/", api.profileUpdate)
	r.Put("/password", api.passwordUpdate)
	r.Get("/tokens", api.tokenList)

	return api
}

// UpdateProfile updates the user profile information.
func (api *profileAPI) UpdateProfile(u *users.User, sf *profileForm) (map[string]interface{}, error) {
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
func (api *profileAPI) UpdatePassword(u *users.User, pf *passwordForm) error {
	return u.SetPassword(pf.Password)
}

// userProfile is the mapping returned by the profileInfo route.
type profileInfoProvider struct {
	Name        string `json:"name"`
	Application string `json:"application"`
}
type profileInfoUser struct {
	Username string    `json:"username"`
	Email    string    `json:"email"`
	Created  time.Time `json:"created"`
	Updated  time.Time `json:"updated"`
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
		},
	}

	api.srv.Render(w, r, 200, res)
}

// profileUpdate updates the current user profile information.
func (api *profileAPI) profileUpdate(w http.ResponseWriter, r *http.Request) {
	uf := &profileForm{}
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
	pf := &passwordForm{}
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

func (api *profileAPI) tokenList(w http.ResponseWriter, r *http.Request) {
	tl, err := api.getTokens(r, ".")
	if err != nil {
		if errors.Is(err, tokens.ErrNotFound) {
			api.srv.TextMessage(w, r, http.StatusNotFound, "not found")
			return
		}
		api.srv.Error(w, r, err)
		return
	}

	api.srv.SendPaginationHeaders(w, r, tl.Pagination.TotalCount, tl.Pagination.Limit, tl.Pagination.Offset)
	api.srv.Render(w, r, http.StatusOK, tl.Items)
}

func (api *profileAPI) getTokens(r *http.Request, base string) (tokenList, error) {
	res := tokenList{}

	pf, _ := api.srv.GetPageParams(r)
	if pf == nil {
		return res, tokens.ErrNotFound
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
		return res, err
	}

	items := []*tokens.Token{}
	if err := ds.ScanStructs(&items); err != nil {
		return res, err
	}

	res.Pagination = api.srv.NewPagination(r, int(count), pf.Limit, pf.Offset)

	res.Items = make([]tokenItem, len(items))
	for i, item := range items {
		res.Items[i] = newTokenItem(api.srv, r, item, base)
	}

	return res, nil
}

// profileForm is the form used by the profile update routes.
type profileForm struct {
	Username *string `json:"username" conform:"trim"`
	Email    *string `json:"email" conform:"trim"`
}

func (sf *profileForm) Validate(f *form.Form) {
	form.RequiredOrNull(f.Fields["username"])
	form.RequiredOrNull(f.Fields["email"])
	form.IsValidEmail(f.Fields["email"])
}

// passwordForm is the form used by the password update routes.
type passwordForm struct {
	Current  string `json:"current"`
	Password string `json:"password"`
}

func (pf *passwordForm) Validate(f *form.Form) {
	form.Required(f.Fields["password"])
}

// validateForView is the form validator used by the password update
// web view. It makes the "current" value mandatory and checks it
// against the current user password.
func (pf *passwordForm) validateForView(f *form.Form, u *users.User) bool {
	form.Required(f.Fields["current"])
	if !f.IsValid() {
		return false
	}
	if !u.CheckPassword(pf.Current) {
		f.Fields["current"].Errors.Add(errors.New("Invalid password"))
		return false
	}

	return true
}

type tokenList struct {
	Pagination server.Pagination
	Items      []tokenItem
}

type tokenItem struct {
	*tokens.Token `json:"-"`

	ID        string     `json:"id"`
	Href      string     `json:"href"`
	Created   time.Time  `json:"created" goqu:"skipupdate"`
	Expires   *time.Time `json:"expires"`
	IsEnabled bool       `json:"is_enabled"`
}

func newTokenItem(s *server.Server, r *http.Request, t *tokens.Token, base string) tokenItem {
	return tokenItem{
		Token:     t,
		ID:        t.UID,
		Href:      s.AbsoluteURL(r, base, t.UID).String(),
		Created:   t.Created,
		Expires:   t.Expires,
		IsEnabled: t.IsEnabled,
	}
}

type tokenForm struct {
	Expires   *time.Time `json:"expires"`
	IsEnabled bool       `json:"is_enabled"`
}
