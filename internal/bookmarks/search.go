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
	"site":   true,
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
	// This is a huge mess. We must pass the search query as a full literal,
	// otherwise it fails on many edge cases.
	// /!\ HERE ARE DRAGONS!
	// We must absolutely properly escape the search value to avoid injections.
	matchQ := []string{}
	rpl := strings.NewReplacer(`"`, `""`, `'`, `''`)

	for _, x := range st.terms {
		q := fmt.Sprintf(`"%s"`, rpl.Replace(x.Value))

		if x.Field != "" && allowedSearchFields[x.Field] {
			q = fmt.Sprintf("%s:%s", x.Field, q)
		}

		matchQ = append(matchQ, q)
	}

	return ds.Join(
		goqu.T("bookmark_idx").As("bi"),
		goqu.On(goqu.Ex{"bi.rowid": goqu.I("b.id")}),
	).
		Where(goqu.L(`bookmark_idx match '?'`, goqu.L(strings.Join(matchQ, " ")))).
		Order(goqu.L("bm25(bookmark_idx, 12.0, 6.0, 5.0, 2.0, 4.0)").Asc())
}
