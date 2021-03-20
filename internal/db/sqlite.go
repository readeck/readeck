// +build !without_sqlite

package db

import (
	"database/sql"
	"net/url"

	"github.com/doug-martin/goqu/v9"
	_ "github.com/doug-martin/goqu/v9/dialect/sqlite3" // dialect
	_ "github.com/mattn/go-sqlite3"                    // driver
)

func init() {
	drivers["sqlite3"] = &sqliteConnector{}
}

type sqliteConnector struct{}

func (c *sqliteConnector) Dialect() string {
	return "sqlite3"
}

func (c *sqliteConnector) Open(dsn *url.URL) (*sql.DB, error) {
	uri := *dsn

	// Remove scheme
	uri.Scheme = ""

	// Set default options
	q := uri.Query()
	q.Set("_foreign_keys", "on")
	q.Set("_journal", "WAL")
	uri.RawQuery = q.Encode()

	db, err := sql.Open("sqlite3", uri.String())
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(2)
	return db, nil
}

func (c *sqliteConnector) HasTable(name string) (bool, error) {
	ds := Q().Select(goqu.C("name")).
		From(goqu.T("sqlite_master")).
		Where(
			goqu.C("type").Eq("table"),
			goqu.C("name").Eq(name),
		)
	var res string

	if _, err := ds.ScanVal(&res); err != nil {
		return false, err
	}

	return res == name, nil
}
