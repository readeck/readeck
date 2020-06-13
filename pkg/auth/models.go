package auth

import (
	"errors"
	"strings"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/hlandau/passlib"

	"github.com/readeck/readeck/pkg/db"
)

func init() {
	passlib.UseDefaults(passlib.Defaults20180601)
}

var (
	// Users is the user manager
	Users = UserManager{}
)

// User is a user record in database
type User struct {
	ID       int       `db:"id" goqu:"skipinsert"`
	Created  time.Time `db:"created" goqu:"skipupdate"`
	Updated  time.Time `db:"updated"`
	Username string    `db:"username"`
	Email    string    `db:"email"`
	Password string    `db:"password"`
}

// UserManager is a query helper for user entries.
type UserManager struct{}

// ByID returns a user by its id. If there's no such user,
// it will return nil and an error.
func (m *UserManager) ByID(id int) (*User, error) {
	var user User
	found, err := db.Q().From("user").
		Where(goqu.C("id").Eq(id)).
		Prepared(true).
		ScanStruct(&user)

	switch {
	case err != nil:
		return nil, err
	case !found:
		return nil, errors.New("not found")
	}

	return &user, nil
}

// ByUsername returns a user by its username. If there's no such user,
// it will return nil and an error.
func (m *UserManager) ByUsername(username string) (*User, error) {
	var user User
	found, err := db.Q().From("user").
		Where(goqu.C("username").Eq(username)).
		Prepared(true).
		ScanStruct(&user)

	switch {
	case err != nil:
		return nil, err
	case !found:
		return nil, errors.New("not found")
	}

	return &user, nil
}

// CreateUser insert a new user in the database. The password
// must be present. It will be hashed and updated before insertion.
func (m *UserManager) CreateUser(user *User) error {
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

	res, err := db.Q().Insert("user").
		Rows(user).
		Prepared(true).Executor().Exec()
	if err != nil {
		panic(err)
	}

	id, _ := res.LastInsertId()
	user.ID = int(id)
	return nil
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
		db.Q().Update("user").
			Set(goqu.Record{"password": newhash, "updated": time.Now()}).
			Where(goqu.C("id").Eq(u.ID)).
			Prepared(true).Executor().Exec()
	}

	return true
}

// SetPassword set a new user password
func (u *User) SetPassword(password string) error {
	var err error
	if u.Password, err = passlib.Hash(password); err != nil {
		return err
	}

	_, err = db.Q().Update("user").
		Set(goqu.Record{"password": u.Password, "updated": time.Now()}).
		Where(goqu.C("id").Eq(u.ID)).
		Prepared(true).Executor().Exec()

	return err
}
