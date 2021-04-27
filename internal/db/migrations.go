package db

import (
	"io/fs"

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
var migrationList = []migrationEntry{}
