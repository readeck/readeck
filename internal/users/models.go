package users

import (
	"errors"
	"strings"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/hlandau/passlib"

	"github.com/readeck/readeck/internal/db"
)

func init() {
	passlib.UseDefaults(passlib.Defaults20180601)
}

const (
	// TableName is the user table name in database.
	TableName = "user"
)

var (
	// Users is the user manager.
	Users = Manager{}

	// ErrNotFound is returned when a user record was not found.
	ErrNotFound = errors.New("not found")
)

// User is a user record in database
type User struct {
	ID       int       `db:"id" goqu:"skipinsert,skipupdate"`
	Created  time.Time `db:"created" goqu:"skipupdate"`
	Updated  time.Time `db:"updated"`
	Username string    `db:"username"`
	Email    string    `db:"email"`
	Password string    `db:"password"`
}

// Manager is a query helper for user entries.
type Manager struct{}

// Query returns a prepared goqu SelectDataset that can be extended later.
func (m *Manager) Query() *goqu.SelectDataset {
	return db.Q().From(goqu.T(TableName).As("u")).Prepared(true)
}

// GetOne executes the a select query and returns the first result or an error
// when there's no result.
func (m *Manager) GetOne(expressions ...goqu.Expression) (*User, error) {
	var u User
	found, err := m.Query().Where(expressions...).ScanStruct(&u)

	switch {
	case err != nil:
		return nil, err
	case !found:
		return nil, ErrNotFound
	}

	return &u, nil
}

// Create insert a new user in the database. The password
// must be present. It will be hashed and updated before insertion.
func (m *Manager) Create(user *User) error {
	if strings.TrimSpace(user.Password) == "" {
		return errors.New("password is empty")
	}
	hash, err := passlib.Hash(user.Password)
	if err != nil {
		return err
	}
	user.Password = hash

	user.Created = time.Now()
	user.Updated = user.Created

	res, err := db.Q().Insert(TableName).
		Rows(user).
		Prepared(true).Executor().Exec()
	if err != nil {
		panic(err)
	}

	id, _ := res.LastInsertId()
	user.ID = int(id)
	return nil
}

// Update updates some user values.
func (u *User) Update(v interface{}) error {
	if u.ID == 0 {
		return errors.New("no ID")
	}

	_, err := db.Q().Update(TableName).Prepared(true).
		Set(v).
		Where(goqu.C("id").Eq(u.ID)).
		Executor().Exec()

	return err
}

// Save updates all the user values.
func (u *User) Save() error {
	u.Updated = time.Now()
	return u.Update(u)
}

// CheckPassword checks if the given password matches the
// current user password.
func (u *User) CheckPassword(password string) bool {
	newhash, err := passlib.Verify(password, u.Password)
	if err != nil {
		return false
	}

	// Update the password when needed
	if newhash != "" {
		u.Update(goqu.Record{"password": newhash, "updated": time.Now()})
	}

	return true
}

// SetPassword set a new user password
func (u *User) SetPassword(password string) error {
	var err error
	if u.Password, err = passlib.Hash(password); err != nil {
		return err
	}

	return u.Update(goqu.Record{"password": u.Password, "updated": time.Now()})
}
