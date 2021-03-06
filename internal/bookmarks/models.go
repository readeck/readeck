package bookmarks

import (
	"archive/zip"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/lithammer/shortuuid/v3"
	log "github.com/sirupsen/logrus"

	"github.com/readeck/readeck/configs"
	"github.com/readeck/readeck/internal/auth/users"
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

// StateNames returns a string with the state name.
var StateNames = map[BookmarkState]string{
	StateLoaded:  "loaded",
	StateError:   "error",
	StateLoading: "loading",
}

var (
	// Bookmarks is the bookmark query manager
	Bookmarks = BookmarkManager{}

	// ErrNotFound is returned when a bookmark record was not found.
	ErrNotFound = errors.New("not found")

	rxHTMLStart = regexp.MustCompile(`^(.*?)<body>`)
	rxHTMLEnd   = regexp.MustCompile(`</body>\s*</html>\s*$`)
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
	Domain       string        `db:"domain"`
	Site         string        `db:"site"`
	SiteName     string        `db:"site_name"`
	Published    *time.Time    `db:"published"`
	Authors      Strings       `db:"authors"`
	Lang         string        `db:"lang"`
	DocumentType string        `db:"type"`
	Description  string        `db:"description"`
	Text         string        `db:"text"`
	WordCount    int           `db:"word_count"`
	Embed        string        `db:"embed"`
	FilePath     string        `db:"file_path"`
	Files        BookmarkFiles `db:"files"`
	Errors       Strings       `db:"errors"`
	Labels       Strings       `db:"labels"`
	IsArchived   bool          `db:"is_archived"`
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

	ds := db.Q().Insert(TableName).
		Rows(bookmark).
		Prepared(true)

	id, err := db.InsertWithID(ds, "id")
	if err != nil {
		return err
	}

	bookmark.ID = id
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

// DeleteUserBookmakrs remove all bookmarks for a given user.
// Normally we don't need such a process but since, a bookmark
// holds a file, we can't only rely on the foreign key cascade
// deletion. Hence this.
func (m *BookmarkManager) DeleteUserBookmakrs(u *users.User) error {
	ds := Bookmarks.Query().
		Where(goqu.C("user_id").Eq(u.ID))

	items := []*Bookmark{}
	if err := ds.ScanStructs(&items); err != nil {
		return err
	}

	for _, b := range items {
		if err := b.Delete(); err != nil {
			return err
		}
	}

	return nil
}

// Update updates some bookmark values.
func (b *Bookmark) Update(v interface{}) error {
	if b.ID == 0 {
		return errors.New("No ID")
	}

	switch v := v.(type) {
	case map[string]interface{}:
		v["updated"] = time.Now()
	default:
		//
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

// StateName returns the current bookmark state name.
func (b *Bookmark) StateName() string {
	return StateNames[b.State]
}

// ReadingTime returns the aproximated reading time
func (b *Bookmark) ReadingTime() int {
	return b.WordCount / 200
}

func (b *Bookmark) getBaseFileURL() (string, error) {
	return path.Join(b.UID[:2], b.UID), nil
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

// getInnerFile returns the content of a file in the
func (b *Bookmark) getInnerFile(name string) ([]byte, error) {
	p := b.getFilePath()
	if p == "" {
		return nil, os.ErrNotExist
	}

	z, err := zip.OpenReader(p)
	if err != nil {
		return nil, err
	}
	defer z.Close()

	fd, err := z.Open(name)
	if err != nil {
		return nil, err
	}

	return ioutil.ReadAll(fd)
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

	replaceArgs := []string{}
	buf := new(strings.Builder)

	for _, entry := range z.File {
		// Build the resource url replacement list
		if !strings.HasSuffix(entry.Name, "/") && strings.HasPrefix(entry.Name, resourceDirName) {
			replaceArgs = append(replaceArgs,
				"./"+entry.Name,
				baseURL+"/"+entry.Name)
			continue
		}

		// Extract document
		if entry.Name == a.Name {
			fp, err := entry.Open()
			if err != nil {
				return nil, err
			}
			defer fp.Close()
			if _, err := io.Copy(buf, fp); err != nil {
				return nil, err
			}
		}
	}

	// Replace resource links
	replacer := strings.NewReplacer(replaceArgs...)
	res := replacer.Replace(buf.String())

	// Extract the content body by removing the outter parts
	res = rxHTMLStart.ReplaceAllString(res, "")
	res = rxHTMLEnd.ReplaceAllString(res, "")

	return strings.NewReader(res), nil
}

// GetSumStrings returns the string used to generate the etag
// of the bookmark(s)
func (b *Bookmark) GetSumStrings() []string {
	return []string{b.UID, b.Updated.String()}
}

// GetLastModified returns the last modified times
func (b *Bookmark) GetLastModified() []time.Time {
	return []time.Time{b.Updated}
}

// Strings is a list of strings stored in a column.
type Strings []string

// Scan loads a Strings instance from a column.
func (s *Strings) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	v, err := db.JSONBytes(value)
	if err != nil {
		return err
	}
	json.Unmarshal(v, s)
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

	v, err := db.JSONBytes(value)
	if err != nil {
		return err
	}
	json.Unmarshal(v, f)
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
