package server

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"

	"codeberg.org/readeck/readeck/pkg/form"
)

type PaginationForm struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

func (pf *PaginationForm) Validate(f *form.Form) {
	if pf.Offset < 0 {
		f.Fields["offset"].Errors.Add(errors.New("Must be a positive number"))
	}
	if pf.Limit < 0 {
		f.Fields["limit"].Errors.Add(errors.New("Must be a positive number"))
	}
	if pf.Limit > 100 {
		f.Fields["limit"].Errors.Add(errors.New("Must be inferior or equal to 100"))
	}
}

// GetPageParams returns the pagination parameters from the query string
func (s *Server) GetPageParams(r *http.Request) (*PaginationForm, *form.Form) {
	params := &PaginationForm{}
	f := form.NewForm(params)
	f.BindValues(r.URL.Query())
	f.Validate()

	if !f.IsValid() {
		return nil, f
	}

	return params, f
}

type Pagination struct {
	URL          *url.URL
	Limit        int
	Offset       int
	TotalCount   int
	TotalPages   int
	CurrentPage  int
	First        int
	Last         int
	Next         int
	Previous     int
	FirstPage    string
	LastPage     string
	NextPage     string
	PreviousPage string
	PageLinks    []PageLink
}

type PageLink struct {
	Index int
	URL   string
}

func (p Pagination) GetLink(offset int) string {
	var u url.URL
	u = *p.URL
	q := u.Query()
	q.Set("limit", strconv.Itoa(p.Limit))
	q.Set("offset", strconv.Itoa(offset))
	u.RawQuery = q.Encode()
	return u.String()
}

func (p Pagination) GetPageLinks() []PageLink {
	res := []PageLink{
		{1, p.GetLink(0)},
	}

	max := func(x, y int) int {
		if x < y {
			return y
		}
		return x
	}
	min := func(x, y int) int {
		if x > y {
			return y
		}
		return x
	}

	prevLinks := []PageLink{}
	for i := p.CurrentPage - 1; i > max(1, p.CurrentPage-3); i-- {
		prevLinks = append([]PageLink{{i, p.GetLink((i - 1) * p.Limit)}}, prevLinks...)
	}
	if len(prevLinks) > 0 && prevLinks[0].Index > 2 {
		res = append(res, PageLink{})
	}
	res = append(res, prevLinks...)

	if p.CurrentPage > 1 {
		res = append(res, PageLink{p.CurrentPage, p.GetLink((p.CurrentPage - 1) * p.Limit)})
	}

	for i := p.CurrentPage + 1; i < min(p.TotalPages, p.CurrentPage+3); i++ {
		res = append(res, PageLink{i, p.GetLink((i - 1) * p.Limit)})
	}

	if len(res) > 0 && res[len(res)-1].Index < p.TotalPages-1 {
		res = append(res, PageLink{})
	}

	if p.CurrentPage < p.TotalPages {
		res = append(res, PageLink{p.TotalPages, p.GetLink(p.Last)})
	}

	return res
}

func (s *Server) NewPagination(r *http.Request, count, limit, offset int) Pagination {
	p := Pagination{
		URL:         s.AbsoluteURL(r),
		Limit:       limit,
		Offset:      offset,
		TotalCount:  count,
		TotalPages:  int(math.Ceil(float64(count) / float64(limit))),
		CurrentPage: int(math.Floor(float64(offset)/float64(limit))) + 1,
		First:       0,
	}
	p.Last = (p.TotalPages - 1) * p.Limit

	if n := p.Offset + p.Limit; n <= p.Last {
		p.Next = p.Offset + p.Limit
		p.NextPage = p.GetLink(p.Next)
	}
	if n := p.Offset - p.Limit; n >= 0 {
		p.Previous = p.Offset - p.Limit
		p.PreviousPage = p.GetLink(p.Previous)
	}

	p.FirstPage = p.GetLink(p.First)
	p.LastPage = p.GetLink(p.Last)
	p.PageLinks = p.GetPageLinks()

	return p
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
