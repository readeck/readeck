package users

import (
	"errors"
	"hash/crc32"
	"strings"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/hlandau/passlib"

	"codeberg.org/readeck/readeck/internal/acls"
	"codeberg.org/readeck/readeck/internal/db"
)

func init() {
	if err := passlib.UseDefaults(passlib.Defaults20180601); err != nil {
		panic(err)
	}
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

	availableGroups = map[string]string{
		"none":  "none",
		"user":  "user",
		"staff": "staff",
		"admin": "admin",
	}
)

func AvailableGroups() map[string]string {
	return availableGroups
}

func ValidGroups() []string {
	r := make([]string, len(availableGroups))
	i := 0
	for k := range availableGroups {
		r[i] = k
		i++
	}
	return r
}

// User is a user record in database
type User struct {
	ID       int       `db:"id" goqu:"skipinsert,skipupdate"`
	Created  time.Time `db:"created" goqu:"skipupdate"`
	Updated  time.Time `db:"updated"`
	Username string    `db:"username"`
	Email    string    `db:"email"`
	Password string    `db:"password"`
	Group    string    `db:"group"`
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

	ds := db.Q().Insert(TableName).
		Rows(user).
		Prepared(true)

	id, err := db.InsertWithID(ds, "id")
	if err != nil {
		return err
	}

	user.ID = id
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

// Delete removes a user from the database
func (u *User) Delete() error {
	_, err := db.Q().Delete(TableName).Prepared(true).
		Where(goqu.C("id").Eq(u.ID)).
		Executor().Exec()

	return err
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
		_ = u.Update(goqu.Record{"password": newhash, "updated": time.Now()})
	}

	return true
}

// HashPassword returns a new hashed password
func (u *User) HashPassword(password string) (string, error) {
	return passlib.Hash(password)
}

// SetPassword set a new user password
func (u *User) SetPassword(password string) error {
	var err error
	if u.Password, err = u.HashPassword(password); err != nil {
		return err
	}

	return u.Update(goqu.Record{"password": u.Password, "updated": time.Now()})
}

// CheckCode returns a crc32 checksum of combined user information
// (username, email, password).
// This value is stored by the session and then validated on
// each request. This allows to invalidate every session when the user
// changes any of this information.
func (u *User) CheckCode() uint32 {
	return crc32.Checksum([]byte(
		u.Username+u.Email+u.Password,
	), crc32.IEEETable)
}

// IsAnonymous returns true when the instance is not set to any existing user
// (when ID is 0)
func (u *User) IsAnonymous() bool {
	return u.ID == 0
}

// Roles returns all the user's implicit roles.
func (u *User) Roles() []string {
	r, _ := acls.GetRoles(u.Group)
	return r
}

// HasPermission returns true if the user can perform "act" action
// on "obj" object.
func (u *User) HasPermission(obj, act string) bool {
	if u.Group == "" {
		return false
	}
	if r, err := acls.Check(u.Group, obj, act); err != nil {
		return false
	} else {
		return r
	}
}
