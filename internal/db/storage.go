package db

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"path"
	"time"

	"github.com/doug-martin/goqu/v9"
	log "github.com/sirupsen/logrus"

	"codeberg.org/readeck/readeck/internal/db/migrations"
)

// Connector is an interface for a database connector.
type Connector interface {
	// Dialect returns the connector's dialect
	Dialect() string

	// Open creates a new db connection.
	Open(*url.URL) (*sql.DB, error)

	// HasTable checks if a given table exists in the
	// database. It's used by the migration system.
	HasTable(string) (bool, error)
}

var (
	drivers = map[string]Connector{}
	driver  Connector
	db      *sql.DB
	qdb     *goqu.Database
)

type logger struct{}

func (l logger) Printf(format string, v ...interface{}) {
	log.Debugf(format, v...)
}

// Driver returns the SQL driver in use.
func Driver() Connector {
	if driver == nil {
		panic("database driver not initialized")
	}
	return driver
}

// Open opens a database connection and sets internal variables that can
// be retrieved using DB() (holding the sql.DB reference) and Q() (holding
// the goqu.Database reference).
func Open(dsn string) error {
	if driver != nil {
		return errors.New("a connection can only be opened once")
	}

	uri, err := url.Parse(dsn)
	if err != nil {
		return err
	}

	driverName := uri.Scheme

	var ok bool
	driver, ok = drivers[driverName]
	if !ok {
		return fmt.Errorf("database driver '%s' not found", driverName)
	}

	db, err = Driver().Open(uri)
	if err != nil {
		return err
	}

	qdb = goqu.New(Driver().Dialect(), db)
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

// Init creates the database schema by running all the needed migrations.
func Init() error {
	return applyMigrations()
}

// migration is a database migration entry
type migration struct {
	ID      int        `goqu:"id"`
	Name    string     `goqu:"name"`
	Applied *time.Time `goqu:"applied"`
}

// applyMigrations applies all the eligible migrations to the
// database. It reads all the files in migrations/{driver}
// and apply each one ordered by filename.
func applyMigrations() error {
	root := Driver().Dialect()
	files, err := migrations.Files.ReadDir(root)
	if err != nil {
		return err
	}

	last, err := getLastMigration()
	if err != nil {
		return err
	}

	log.WithField("last_id", last.ID).
		WithField("last_name", last.Name).
		Debug("schema migrations")

	for _, mf := range files {
		if mf.IsDir() {
			continue
		}

		var mid int
		var mname string
		fmt.Sscanf(mf.Name(), "%d-%s", &mid, &mname)

		if mid <= last.ID {
			continue
		}

		sql, err := migrations.Files.ReadFile(path.Join(root, mf.Name()))
		if err != nil {
			return err
		}
		if err := applyMigration(mid, mname, sql); err != nil {
			return err
		}
	}

	return nil
}

// applyMigration applies a given migration. In the same transaction
// we execute the SQL fetched from the file and add an entry in the
// migration table.
func applyMigration(id int, name string, sql []byte) error {
	log.WithField("id", id).WithField("name", name).Debug("migration")

	tx, err := Q().Begin()
	if err != nil {
		return err
	}
	return tx.Wrap(func() error {
		if _, err := tx.Exec(string(sql)); err != nil {
			return err
		}

		_, err := tx.Insert(goqu.T("migration")).Rows(map[string]interface{}{
			"id":      id,
			"name":    name,
			"applied": time.Now(),
		}).Executor().Exec()
		return err
	})
}

func getLastMigration() (m *migration, err error) {
	m = &migration{ID: 0}

	// Check if the migration table exists. If it doesn't
	// it means we start from the beginning.
	if ok, err := driver.HasTable("migration"); err != nil {
		return m, err
	} else if !ok {
		return m, nil
	}

	// Return the last migration, based on its ID.
	ds := Q().
		Select(goqu.C("id"), goqu.C("name"), goqu.C("applied")).
		From(goqu.T("migration")).Prepared(true).
		Order(goqu.C("id").Desc()).
		Limit(1).Offset(0)

	_, err = ds.ScanStruct(m)
	return
}
