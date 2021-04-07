package bookmarks

import (
	"fmt"
	"strings"

	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"

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

// toSelectDataSet returns an augmented select dataset including the search query.
// Its implementation differs on database dialect.
func (st *searchString) toSelectDataSet(ds *goqu.SelectDataset) *goqu.SelectDataset {
	if len(st.terms) == 0 {
		return ds
	}

	switch ds.Dialect().Dialect() {
	case "postgres":
		return st.toPG(ds)
	case "sqlite3":
		return st.toSQLite(ds)
	}

	panic("dialect not implemented")
}

func (st *searchString) toPG(ds *goqu.SelectDataset) *goqu.SelectDataset {
	where := goqu.And()
	order := []exp.OrderedExpression{}

	// In order to use the GIN indexes, we build a fairly big but very efficient query.
	// For general search, we add a group of OR clauses to the main clauses list.
	for _, x := range st.terms {
		var fields = []string{"bs.title", "bs.description", "bs.text", "bs.site", "bs.author", "bs.label"}

		value := x.Value
		if x.Quotes {
			value = fmt.Sprintf(`"%s"`, value)
		}

		if x.Field != "" && allowedSearchFields[x.Field] {
			fields = []string{fmt.Sprintf("bs.%s", x.Field)}
		}

		w := goqu.Or()
		for _, f := range fields {
			w = w.Append(goqu.L(`? @@ websearch_to_tsquery('ts', ?)`, goqu.L(f), value))
			order = append(order, goqu.L(`ts_rank_cd(?, websearch_to_tsquery('ts', ?))`, goqu.L(f), value).Desc())
		}
		where = where.Append(w)
	}

	return ds.Prepared(false).Join(
		goqu.T("bookmark_search").As("bs"),
		goqu.On(goqu.Ex{"bs.bookmark_id": goqu.I("b.id")}),
	).
		Where(where).
		Order(order...)
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
