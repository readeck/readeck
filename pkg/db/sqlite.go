package db

import (
	"database/sql"
	"net/url"

	_ "github.com/doug-martin/goqu/v9/dialect/sqlite3" // dialect
	_ "github.com/mattn/go-sqlite3"                    // driver
)

type sqliteConnector struct{}

func (c *sqliteConnector) Open(dsn string) (*sql.DB, error) {
	// We'll add some default options to the database dsn
	// before connecting.
	uri, err := url.Parse(dsn)
	if err != nil {
		return nil, err
	}

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

func init() {
	drivers["sqlite3"] = &sqliteConnector{}
}
