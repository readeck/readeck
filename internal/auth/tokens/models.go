package tokens

import (
	"errors"
	"strings"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/lithammer/shortuuid/v3"
	"github.com/readeck/readeck/internal/auth/users"
	"github.com/readeck/readeck/internal/db"
)

const (
	// TableName is the user table name in database.
	TableName = "token"
)

var (
	// Tokens is the token manager.
	Tokens = Manager{}

	// ErrNotFound is returned when a user record was not found.
	ErrNotFound = errors.New("not found")
)

// Token is a token record in database
type Token struct {
	ID          int        `db:"id" goqu:"skipinsert,skipupdate"`
	UID         string     `db:"uid"`
	UserID      *int       `db:"user_id"`
	Created     time.Time  `db:"created" goqu:"skipupdate"`
	Expires     *time.Time `db:"expires"`
	IsEnabled   bool       `db:"is_enabled"`
	Application string     `db:"application"`
}

// Manager is a query helper for token entries.
type Manager struct{}

// Query returns a prepared goqu SelectDataset that can be extended later.
func (m *Manager) Query() *goqu.SelectDataset {
	return db.Q().From(goqu.T(TableName).As("t")).Prepared(true)
}

// GetOne executes the a select query and returns the first result or an error
// when there's no result.
func (m *Manager) GetOne(expressions ...goqu.Expression) (*Token, error) {
	var t Token
	found, err := m.Query().Where(expressions...).ScanStruct(&t)

	switch {
	case err != nil:
		return nil, err
	case !found:
		return nil, ErrNotFound
	}

	return &t, nil
}

// GetUser returns the token and user owning a given token uid.
func (m *Manager) GetUser(uid string) (*TokenAndUser, error) {
	var res TokenAndUser
	ds := m.Query().
		Join(
			goqu.T(users.TableName).As("u"),
			goqu.On(goqu.I("t.user_id").Eq(goqu.I("u.id"))),
		).
		Where(
			goqu.I("t.uid").Eq(uid),
			goqu.I("t.is_enabled").Eq(true),
		)

	found, err := ds.ScanStruct(&res)
	switch {
	case err != nil:
		return nil, err
	case !found:
		return nil, ErrNotFound
	}

	return &res, nil
}

// Create insert a new token in the database.
func (m *Manager) Create(token *Token) error {
	if token.UserID == nil {
		return errors.New("no token user")
	}
	if strings.TrimSpace(token.Application) == "" {
		return errors.New("no application")
	}

	token.Created = time.Now()
	token.UID = shortuuid.New()

	res, err := db.Q().Insert(TableName).
		Rows(token).
		Prepared(true).Executor().Exec()
	if err != nil {
		return err
	}

	id, _ := res.LastInsertId()
	token.ID = int(id)
	return nil
}

// Update updates some bookmark values.
func (t *Token) Update(v interface{}) error {
	if t.ID == 0 {
		return errors.New("No ID")
	}

	_, err := db.Q().Update(TableName).Prepared(true).
		Set(v).
		Where(goqu.C("id").Eq(t.ID)).
		Executor().Exec()

	return err
}

// Save updates all the bookmark values.
func (t *Token) Save() error {
	return t.Update(t)
}

// IsExpired returns true if the token has an expiration date and the
// current time is after the expiration.
func (t *Token) IsExpired() bool {
	if t.Expires == nil || t.Expires.IsZero() {
		return false
	}
	return time.Now().After(*t.Expires)
}

// TokenAndUser is a result of a joint query on user and token tables.
type TokenAndUser struct {
	Token *Token      `db:"t"`
	User  *users.User `db:"u"`
}
