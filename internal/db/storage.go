package db

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"time"

	"codeberg.org/readeck/readeck/internal/db/migrations"
	"github.com/doug-martin/goqu/v9"
	log "github.com/sirupsen/logrus"
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
	sfs, err := fs.Sub(migrations.Files, root)
	if err != nil {
		return err
	}

	last, err := getLastMigration()
	if err != nil {
		return err
	}

	tx, err := Q().Begin()
	if err != nil {
		return err
	}
	return tx.Wrap(func() error {
		// When last.ID is -1, it means there's no schema, so we create it.
		// The schema is full and there's no need to apply any migration, only
		// to mark them.
		if last.ID < 0 {
			sql, err := fs.ReadFile(sfs, "schema.sql")
			if err != nil {
				return err
			}
			log.Debug("initial schema")
			if _, err = tx.Exec(string(sql)); err != nil {
				return err
			}
		}

		for _, m := range migrationList {
			if m.id <= last.ID {
				continue
			}

			if last.ID >= 0 { // Only apply migrations when there is a schema already
				for _, fn := range m.funcList {
					if err := fn(tx, sfs); err != nil {
						return err
					}
				}
			}

			if err = insertMigration(tx, m.id, m.name); err != nil {
				return err
			}
		}

		return nil
	})
}

// insertMigration adds an entry in the migration table.
func insertMigration(tx *goqu.TxDatabase, id int, name string) error {
	_, err := tx.Insert(goqu.T("migration")).Rows(map[string]interface{}{
		"id":      id,
		"name":    name,
		"applied": time.Now(),
	}).Executor().Exec()
	return err
}

// getLastMigration returns the last executed migration.
func getLastMigration() (m *migration, err error) {
	m = &migration{}

	// Check if the migration table exists. If it doesn't
	// it means we start from the beginning.
	if ok, err := driver.HasTable("migration"); err != nil {
		return m, err
	} else if !ok {
		m.ID = -1
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
