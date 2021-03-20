package db

import (
	"fmt"

	"github.com/doug-martin/goqu/v9"
)

// InsertWithID executes an insert statement and returns the value
// of the field given by "r".
// Depending on the database, it uses different ways to do just that.
func InsertWithID(stmt *goqu.InsertDataset, r string) (id int, err error) {
	if Driver().Dialect() == "postgres" {
		_, err = stmt.Returning(goqu.C(r)).Executor().ScanVal(&id)
		return
	} else {
		res, err := stmt.Executor().Exec()
		if err != nil {
			return id, err
		}

		i, _ := res.LastInsertId()
		id = int(i)
	}

	return
}

// JsonBytes converts a string or a []uint8 to a []byte value.
// We need this with sqlite and postgresql not returning the same
// data type for their json fields.
func JsonBytes(value interface{}) ([]byte, error) {
	switch x := value.(type) {
	case string:
		return []byte(x), nil
	case []uint8:
		return x, nil
	}

	return []byte{}, fmt.Errorf("unknown data type for %+v", value)
}
