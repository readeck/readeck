package cookbook

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

type cookbookViews struct {
	chi.Router
	*cookbookAPI
}

func newCookbookViews(api *cookbookAPI) *cookbookViews {
	r := api.srv.AuthenticatedRouter()
	v := &cookbookViews{r, api}

	r.With(api.srv.WithPermission("read")).Group(func(r chi.Router) {
		r.Get("/", v.typoView)
	})

	return v
}

func (v *cookbookViews) typoView(w http.ResponseWriter, r *http.Request) {
	v.srv.RenderTemplate(w, r, 200, "cookbook/prose", nil)
}
