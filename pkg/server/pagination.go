package server

import (
	"fmt"
	"math"
	"net/http"
	"strconv"

	validation "github.com/go-ozzo/ozzo-validation/v4"
)

// PaginatedQuery represents pagination parameters
type PaginatedQuery struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// Validate validates the pagination parameters
func (f PaginatedQuery) Validate() error {
	return validation.ValidateStruct(
		&f,
		validation.Field(&f.Limit, validation.Max(100)),
		validation.Field(&f.Offset, validation.Min(0)),
	)
}

// GetPageParams returns the pagination parameters from the query string
func (s *Server) GetPageParams(r *http.Request) (*PaginatedQuery, *Message) {
	params := &PaginatedQuery{}
	if msg := s.BindQueryString(r, params); msg != nil {
		return nil, msg
	}

	return params, nil
}

// SendPaginationHeaders compute and set the pagination headers
func (s *Server) SendPaginationHeaders(
	w http.ResponseWriter, r *http.Request,
	count, limit, offset int,
) {
	uri := s.AbsoluteURL(r)
	pages := int(math.Ceil(float64(count) / float64(limit)))
	page := int(math.Floor(float64(offset)/float64(limit))) + 1
	lastOffset := int(pages-1) * limit
	prevOffset := offset - limit
	nextOffset := offset + limit

	setHeader := func(w http.ResponseWriter, rel string, offset int) {
		u := *uri
		q := u.Query()
		q.Set("limit", strconv.Itoa(limit))
		q.Set("offset", strconv.Itoa(offset))
		u.RawQuery = q.Encode()
		w.Header().Add("Link", fmt.Sprintf(`<%s>; rel="%s"`, u.String(), rel))
	}

	if prevOffset >= 0 {
		setHeader(w, "prev", prevOffset)
	}
	if nextOffset <= lastOffset {
		setHeader(w, "next", nextOffset)
	}
	setHeader(w, "first", 0)
	setHeader(w, "last", lastOffset)

	w.Header().Set("Total-Count", strconv.Itoa(count))
	w.Header().Set("Total-Pages", strconv.Itoa(pages))
	w.Header().Set("Current-Page", strconv.Itoa(page))
}
