package bookmarks

import (
	"fmt"
	"strings"

	"github.com/doug-martin/goqu/v9"

	"codeberg.org/readeck/readeck/internal/searchstring"
)

var allowedSearchFields = map[string]bool{
	"author": true,
	"label":  true,
	"title":  true,
}

type searchString struct {
	terms []searchstring.SearchTerm
}

func newSearchString(input string) (res *searchString) {
	res = &searchString{}
	st, err := searchstring.Parse(input)
	if err != nil {
		return
	}
	res.terms = st
	return
}

func (st *searchString) toSQLite(ds *goqu.SelectDataset) *goqu.SelectDataset {
	q := strings.Builder{}
	for i, x := range st.terms {
		if x.Field != "" && allowedSearchFields[x.Field] {
			q.WriteString(x.Field)
			q.WriteRune(':')
		}
		s := strings.ReplaceAll(x.Value, `"`, `""`)
		if x.Quotes {
			s = fmt.Sprintf(`"%s"`, s)
		}
		q.WriteString(s)
		if i+1 < len(st.terms) {
			q.WriteRune(' ')
		}
	}

	return ds.Join(
		goqu.T("bookmark_idx").As("bi"),
		goqu.On(goqu.Ex{"bi.rowid": goqu.I("b.id")}),
	).
		Where(goqu.L("bookmark_idx match ?", q.String())).
		Order(goqu.L("bm25(bookmark_idx, 12.0, 6.0, 5.0, 2.0, 4.0)").Asc())
}
