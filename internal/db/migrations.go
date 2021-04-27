package db

import (
	"io/fs"
	"math/rand"

	"github.com/doug-martin/goqu/v9"
)

type migrationFunc func(*goqu.TxDatabase, fs.FS) error

type migrationEntry struct {
	id       int
	name     string
	funcList []migrationFunc
}

// newMigrationEntry creates a new migration which contains an id, a name and a list
// of functions performing the migration.
func newMigrationEntry(id int, name string, funcList ...migrationFunc) migrationEntry {
	res := migrationEntry{
		id:       id,
		name:     name,
		funcList: []migrationFunc{},
	}
	res.funcList = funcList
	return res
}

// migrationList is our full migration list
var migrationList = []migrationEntry{
	newMigrationEntry(1, "user_seed", func(tx *goqu.TxDatabase, _ fs.FS) (err error) {
		// Add a seed column to the user table
		sql := `ALTER TABLE "user" ADD COLUMN seed INTEGER NOT NULL DEFAULT 0;`

		if _, err = tx.Exec(sql); err != nil {
			return
		}

		// Set a new seed on every user
		var ids []int64
		if err = tx.From("user").Select("id").ScanVals(&ids); err != nil {
			return
		}
		for _, id := range ids {
			seed := rand.Intn(32767)
			_, err = tx.Update("user").
				Set(goqu.Record{"seed": seed}).
				Where(goqu.C("id").Eq(id)).
				Executor().Exec()
			if err != nil {
				return
			}
		}

		return
	}),
}
