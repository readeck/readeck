package bookmarks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gammazero/workerpool"
	log "github.com/sirupsen/logrus"

	"github.com/readeck/readeck/pkg/archiver"
	"github.com/readeck/readeck/pkg/extract"
	"github.com/readeck/readeck/pkg/extract/contents"
	"github.com/readeck/readeck/pkg/extract/fftr"
	"github.com/readeck/readeck/pkg/extract/meta"
)

var extractPool *workerpool.WorkerPool

// StartExtractPool start the worker pool that performs
// page extraction.
func StartExtractPool(workers int) {
	if extractPool != nil {
		panic("ExtractPool is already started")
	}

	extractPool = workerpool.New(workers)
}

// EnqueueExtractPage sends a new bookmark to the extraction
// workers.
func EnqueueExtractPage(b *Bookmark) {
	extractPool.Submit(func() {
		// Always set state to loaded, even if there are errors
		saved := false
		defer func() {
			// Recover from any error that could have arised
			if r := recover(); r != nil {
				log.WithField("recover", r).Error("error during extraction")
				b.State = StateError
				b.Errors = append(b.Errors, fmt.Sprintf("%v", r))
				saved = false

			}

			// Never stay hanging
			if b.State == StateLoading {
				b.State = StateLoaded
				saved = false
			}

			// Then save the whole thing
			if !saved {
				b.Save()
			}
			runtime.GC()
		}()

		ex, err := extract.New(b.URL)
		if err != nil {
			log.WithError(err).Error()
			return
		}

		ex.AddProcessors(
			meta.ExtractMeta,
			meta.ExtractOembed,
			meta.SetDropProperties,
			meta.ExtractFavicon,
			meta.ExtractPicture,
			fftr.LoadConfiguration,
			fftr.ReplaceStrings,
			fftr.FindContentPage,
			fftr.ExtractAuthor,
			fftr.ExtractDate,
			fftr.FindNextPage,
			fftr.ExtractBody,
			fftr.StripTags,
			fftr.GoToNextPage,
			contents.Readability,
			contents.Text,
		)

		ex.Run()
		drop := ex.Drop()
		if drop == nil {
			return
		}

		b.Updated = time.Now()
		b.URL = drop.UnescapedURL()
		b.State = StateLoaded
		b.Title = drop.Title
		b.Site = drop.URL.Hostname()
		b.SiteName = drop.Site
		b.Authors = Strings{}
		b.Lang = drop.Lang
		b.DocumentType = drop.DocumentType
		b.Description = drop.Description
		b.Text = ex.Text

		for _, x := range drop.Authors {
			b.Authors = append(b.Authors, x)
		}

		for _, x := range ex.Errors() {
			b.Errors = append(b.Errors, x.Error())
		}

		if !drop.Date.IsZero() {
			b.Published = &drop.Date
		}

		if drop.IsMedia() {
			b.Embed = drop.Meta.LookupGet("oembed.html")
		}

		// Run archiver & zipper
		err = jobPostProcess(b, ex)
		if err != nil {
			log.WithError(err).Error()
			b.Errors = append(b.Errors, err.Error())
		}

		// All good? Save now
		if err := b.Save(); err != nil {
			log.WithError(err).Error()
			return
		}
		saved = true
	})
}

func jobPostProcess(b *Bookmark, ex *extract.Extractor) error {
	var (
		err error
		arc *archiver.Archiver
	)

	// Fail fast
	fileURL, err := b.getBaseFileURL()
	if err != nil {
		return err
	}
	zipFile := filepath.Join(StoragePath(), fileURL+".zip")

	files := BookmarkFiles{}

	// Create the zip file
	z, err := newZipper(zipFile)
	if err != nil {
		return err
	}
	defer func() {
		err := z.close()
		if err != nil {
			panic(err)
		}
	}()

	// Add images to the zipfile
	if err = z.addDirectory("img"); err != nil {
		return err
	}

	for k, p := range ex.Drop().Pictures {
		name := path.Join("img", p.Name(k))
		if err = z.addFile(name, p.Bytes()); err != nil {
			return err
		}
		files[k] = &BookmarkFile{name, p.Type, p.Size}
	}

	// Run the archiver
	if len(ex.HTML) > 0 && ex.Drop().IsHTML() {
		arc, err = NewArchive(ex, log.NewEntry(log.StandardLogger()))
		if err != nil {
			return err
		}
	}

	// Add HTML content
	if arc != nil && len(arc.Result) > 0 {
		if err = z.addCompressedFile("index.html", arc.Result); err != nil {
			return err
		}
		files["article"] = &BookmarkFile{Name: "index.html"}
	}

	// Add assets
	if arc != nil && len(arc.Cache) > 0 {
		if err = z.addDirectory(resourceDirName); err != nil {
			return err
		}

		for uri, asset := range arc.Cache {
			fname := path.Join(resourceDirName, getURLfilename(uri, asset.ContentType))
			if err = z.addFile(fname, asset.Data); err != nil {
				return err
			}
		}
	}

	// Add the log
	if err = z.addCompressedFile("log", []byte(strings.Join(ex.Logs, "\n"))); err != nil {
		return err
	}
	files["log"] = &BookmarkFile{Name: "log"}

	// Add the metadata
	buf := new(bytes.Buffer)
	enc := json.NewEncoder(buf)
	enc.SetIndent("", "  ")
	if err = enc.Encode(ex.Drop()); err != nil {
		return err
	}
	if err = z.addCompressedFile("props.json", buf.Bytes()); err != nil {
		return err
	}
	files["props"] = &BookmarkFile{Name: "props.json"}

	b.FilePath = fileURL
	b.Files = files
	return nil
}
