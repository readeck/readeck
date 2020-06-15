package db

import (
	"database/sql"
	"fmt"

	"github.com/doug-martin/goqu/v9"
	log "github.com/sirupsen/logrus"
)

// Connector is an interface for a database connector.
type Connector interface {
	Open(dsn string) (*sql.DB, error)
}

var (
	drivers = map[string]Connector{}
	db      *sql.DB
	qdb     *goqu.Database
)

type logger struct{}

func (l logger) Printf(format string, v ...interface{}) {
	log.Debugf(format, v...)
}

// Open opens a database connection and sets internal variables that can
// be retrived using DB() (holding the sql.DB reference) and Q() (holding
// the goqu.Database reference).
func Open(driver, dsn string) error {
	d, ok := drivers[driver]
	if !ok {
		return fmt.Errorf("driver '%s' not found", driver)
	}

	var err error
	db, err = d.Open(dsn)
	if err != nil {
		return err
	}

	qdb = goqu.New(driver, db)
	if log.IsLevelEnabled(log.DebugLevel) {
		qdb.Logger(logger{})
	}

	return nil
}

// Close closes the connection to the database.
func Close() error {
	if db != nil {
		return db.Close()
	}

	return nil
}

// DB returns the current sql.DB instance.
func DB() *sql.DB {
	return db
}

// Q returns the current goqu.Database instance.
func Q() *goqu.Database {
	return qdb
}

// Init creates the database schema. It is called during
// startup.
func Init() error {
	if _, err := db.Exec(sqlInit); err != nil {
		return err
	}

	return nil
}

var sqlInit = `
CREATE TABLE IF NOT EXISTS user (
  id       integer  PRIMARY KEY AUTOINCREMENT,
  created  datetime NOT NULL,
  updated  datetime NOT NULL,
  username text     UNIQUE NOT NULL,
  email    text     UNIQUE NOT NULL,
  password text     NOT NULL
);

CREATE TABLE IF NOT EXISTS bookmark (
  id          integer  PRIMARY KEY AUTOINCREMENT,
  uid         text     NOT NULL,
  user_id     integer  NOT NULL,
  created     datetime NOT NULL,
  updated     datetime NOT NULL,
  is_marked   integer  NOT NULL DEFAULT 0,
  is_archived integer  NOT NULL DEFAULT 0,
  is_read     integer  NOT NULL DEFAULT 0,
  state       integer  NOT NULL DEFAULT 0,
  url         text     NOT NULL,
  title       text     NOT NULL,
  site        text     NOT NULL DEFAULT "",
  site_name   text     NOT NULL DEFAULT "",
  published   datetime,
  authors     json     NOT NULL DEFAULT "",
  lang        text     NOT NULL DEFAULT "",
  type        text     NOT NULL DEFAULT "",
  description text     NOT NULL DEFAULT "",
  text        text     NOT NULL DEFAULT "",
  embed       text     NOT NULL DEFAULT "",
  meta        json     NOT NULL DEFAULT "",
  files       json     NOT NULL DEFAULT "",
  logs        json     NOT NULL DEFAULT "",
  errors      json     NOT NULL DEFAULT "",
  tags        json     NOT NULL DEFAULT "",

  CONSTRAINT fk_bookmark_user FOREIGN KEY (user_id) REFERENCES user(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_bookmark_uid ON bookmark(uid);

CREATE VIRTUAL TABLE IF NOT EXISTS bookmark_idx USING fts5(
	tokenize='unicode61 remove_diacritics 2',
	content='bookmark',
	content_rowid='id',
	title,
	description,
	text,
	site,
	authors
);

DROP TRIGGER IF EXISTS bookmark_ai;
CREATE TRIGGER bookmark_ai AFTER INSERT ON bookmark BEGIN
	INSERT INTO bookmark_idx (
		rowid, title, description, text, site, authors
	) VALUES (
		new.id, new.title, new.description, new.text, new.site_name || ' ' || new.site, new.authors
	);
END;

DROP TRIGGER IF EXISTS bookmark_au;
CREATE TRIGGER bookmark_au AFTER UPDATE ON bookmark BEGIN
	INSERT INTO bookmark_idx(
		bookmark_idx, rowid, title, description, text, site, authors
	) VALUES (
		'delete', old.id, old.title, old.description, old.text, old.site, old.authors
	);
	INSERT INTO bookmark_idx (
		rowid, title, description, text, site, authors
	) VALUES (
		new.id, new.title, new.description, new.text, new.site_name || ' ' || new.site, new.authors
	);
END;

DROP TRIGGER IF EXISTS bookmark_ad;
CREATE TRIGGER IF NOT EXISTS bookmark_ad AFTER DELETE ON bookmark BEGIN
	INSERT INTO bookmark_idx(
		bookmark_idx, rowid, title, description, text, site, authors
	) VALUES (
		'delete', old.id, old.title, old.description, old.text, old.site, old.authors
	);
END;
`
