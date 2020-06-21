package bookmarks

import (
	"archive/zip"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/net/idna"

	"github.com/doug-martin/goqu/v9"
	"github.com/lithammer/shortuuid/v3"
	log "github.com/sirupsen/logrus"
	"github.com/weppos/publicsuffix-go/publicsuffix"

	"github.com/readeck/readeck/configs"
	"github.com/readeck/readeck/internal/db"
)

// BookmarkState is the current bookmark state
type BookmarkState int

const (
	// StateLoaded when the page is fully loaded
	StateLoaded BookmarkState = iota

	// StateError when there was some unrecoverable
	// error during extraction
	StateError

	// StateLoading when the page is loading.
	StateLoading
)

const (
	// TableName is the bookmark table name in database.
	TableName = "bookmark"
)

var (
	// Bookmarks is the bookmark query manager
	Bookmarks = BookmarkManager{}

	// ErrNotFound is returned when a user record was not found.
	ErrNotFound = errors.New("not found")
)

// StoragePath returns the storage base directory for bookmark files
func StoragePath() string {
	return filepath.Join(configs.Config.Main.DataDirectory, "bookmarks")
}

// Bookmark is a bookmark record in database
type Bookmark struct {
	ID           int           `db:"id" goqu:"skipinsert,skipupdate"`
	UID          string        `db:"uid"`
	UserID       *int          `db:"user_id"`
	Created      time.Time     `db:"created" goqu:"skipupdate"`
	Updated      time.Time     `db:"updated"`
	State        BookmarkState `db:"state"`
	URL          string        `db:"url"`
	Title        string        `db:"title"`
	Site         string        `db:"site"`
	SiteName     string        `db:"site_name"`
	Published    *time.Time    `db:"published"`
	Authors      Strings       `db:"authors"`
	Lang         string        `db:"lang"`
	DocumentType string        `db:"type"`
	Description  string        `db:"description"`
	Text         string        `db:"text"`
	Embed        string        `db:"embed"`
	FilePath     string        `db:"file_path"`
	Files        BookmarkFiles `db:"files"`
	Errors       Strings       `db:"errors"`
	Tags         Strings       `db:"tags"`
	IsMarked     bool          `db:"is_marked"`
}

// BookmarkManager is a query helper for bookmark entries.
type BookmarkManager struct{}

// Create inserts a new bookmark in the database.
func (m *BookmarkManager) Create(bookmark *Bookmark) error {
	if bookmark.UserID == nil {
		return errors.New("no bookmark user")
	}

	bookmark.Created = time.Now()
	bookmark.Updated = bookmark.Created
	bookmark.UID = shortuuid.New()

	res, err := db.Q().Insert(TableName).
		Rows(bookmark).
		Prepared(true).Executor().Exec()
	if err != nil {
		return err
	}

	id, _ := res.LastInsertId()
	bookmark.ID = int(id)
	return nil
}

// Query returns a prepared goqu SelectDataset that can be extended later.
func (m *BookmarkManager) Query() *goqu.SelectDataset {
	return db.Q().From(goqu.T(TableName).As("b")).Prepared(true)
}

// GetOne executes the a select query and returns the first result or an error
// when there's no result.
func (m *BookmarkManager) GetOne(expressions ...goqu.Expression) (*Bookmark, error) {
	var b Bookmark
	found, err := m.Query().Where(expressions...).ScanStruct(&b)

	switch {
	case err != nil:
		return nil, err
	case !found:
		return nil, ErrNotFound
	}

	return &b, nil
}

// AddSearch adds the search query to an existing SelectDataset.
func (m *BookmarkManager) AddSearch(ds *goqu.SelectDataset, search string) *goqu.SelectDataset {
	return ds.Join(
		goqu.T("bookmark_idx").As("bi"),
		goqu.On(goqu.Ex{"bi.rowid": goqu.I("b.id")}),
	).
		Where(goqu.L("bookmark_idx match ?", search)).
		Order(goqu.L("bm25(bookmark_idx, 12.0, 6.0, 5.0, 2.0, 4.0)").Asc())
}

// Update updates some bookmark values.
func (b *Bookmark) Update(v interface{}) error {
	if b.ID == 0 {
		return errors.New("No ID")
	}

	_, err := db.Q().Update(TableName).Prepared(true).
		Set(v).
		Where(goqu.C("id").Eq(b.ID)).
		Executor().Exec()

	return err
}

// Save updates all the bookmark values.
func (b *Bookmark) Save() error {
	b.Updated = time.Now()
	return b.Update(b)
}

// Delete removes a bookmark from the database.
func (b *Bookmark) Delete() error {
	_, err := db.Q().Delete(TableName).Prepared(true).
		Where(goqu.C("id").Eq(b.ID)).
		Executor().Exec()

	if err != nil {
		return err
	}

	b.removeFiles()
	return nil
}

func (b *Bookmark) getBaseFileURL() (string, error) {
	var res string
	var err error

	if res, err = publicsuffix.Domain(b.Site); err != nil {
		res = b.Site
	}

	if res, err = idna.ToASCII(res); err != nil {
		return "", err
	}

	return path.Join(res, b.Created.Format("20060102"), b.UID), nil
}

func (b *Bookmark) removeFiles() {
	filename := b.getFilePath()
	if filename == "" {
		return
	}

	l := log.WithField("path", filename)
	if err := os.Remove(filename); err != nil {
		l.WithError(err).Error()
	} else {
		l.Debug("file removed")
	}

	// Remove empty directories up to the base
	dirname := path.Dir(filename)
	if stat, _ := os.Stat(dirname); stat == nil {
		return
	}
	for dirname != "." {
		// Just try to remove and if it's not empty it will complain
		d := dirname
		if err := os.Remove(d); err != nil {
			break
		}
		log.WithField("dir", dirname).Debug("directory removed")
		dirname = path.Dir(dirname)
	}
}

func (b *Bookmark) getFilePath() string {
	if b.FilePath == "" {
		return ""
	}
	return filepath.Join(StoragePath(), b.FilePath+".zip")
}

// getArticle returns the article content
func (b *Bookmark) getArticle(baseURL string) (*strings.Reader, error) {
	a, ok := b.Files["article"]
	if !ok {
		return nil, os.ErrNotExist
	}

	p := b.getFilePath()
	if p == "" {
		return nil, os.ErrNotExist
	}

	z, err := zip.OpenReader(p)
	if err != nil {
		return nil, err
	}
	defer z.Close()

	resourceList := []string{}
	buf := new(strings.Builder)

	for _, entry := range z.File {
		if !strings.HasSuffix(entry.Name, "/") && strings.HasPrefix(entry.Name, resourceDirName) {
			resourceList = append(resourceList, entry.Name)
			continue
		}

		if entry.Name == a.Name {
			fp, err := entry.Open()
			if err != nil {
				return nil, err
			}
			if _, err := io.Copy(buf, fp); err != nil {
				return nil, err
			}
		}
	}

	args := []string{}
	for _, x := range resourceList {
		args = append(args, "./"+x, baseURL+"/"+x)
	}

	replacer := strings.NewReplacer(args...)
	res := replacer.Replace(buf.String())

	return strings.NewReader(res), nil
}

// Strings is a list of strings stored in a column.
type Strings []string

// Scan loads a Strings instance from a column.
func (s *Strings) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	v, ok := value.(string)
	if !ok {
		return fmt.Errorf("Can't cast %+v", value)
	}
	json.Unmarshal([]byte(v), s)
	return nil
}

// Value encode a Strings instance for storage.
func (s Strings) Value() (driver.Value, error) {
	v, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	return string(v), nil
}

// BookmarkFiles is a map of BookmarkFile instances.
type BookmarkFiles map[string]*BookmarkFile

// BookmarkFile represents a stored file (attachment) for a bookmark.
// The Size property is ony useful for images.
type BookmarkFile struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Size [2]int `json:"size,omitempty"`
}

// Scan loads a BookmarkFiles instance from a column.
func (f *BookmarkFiles) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	v, ok := value.(string)
	if !ok {
		return fmt.Errorf("Can't cast %+v", value)
	}
	json.Unmarshal([]byte(v), f)
	return nil
}

// Value encodes a BookmarkFiles instance for storage.
func (f BookmarkFiles) Value() (driver.Value, error) {
	v, err := json.Marshal(f)
	if err != nil {
		return "", err
	}
	return string(v), nil
}
