package bookmarks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gammazero/workerpool"
	log "github.com/sirupsen/logrus"

	"codeberg.org/readeck/readeck/pkg/archiver"
	"codeberg.org/readeck/readeck/pkg/extract"
	"codeberg.org/readeck/readeck/pkg/extract/contents"
	"codeberg.org/readeck/readeck/pkg/extract/fftr"
	"codeberg.org/readeck/readeck/pkg/extract/meta"
)

var workerPool *workerpool.WorkerPool

type ctxJobRequestID struct{}

// StartWorkerPool start the worker pool that performs
// page extraction.
func StartWorkerPool(workers int) {
	if workerPool != nil {
		panic("ExtractPool is already started")
	}

	workerPool = workerpool.New(workers)
}

// enqueueExtractPage sends a new bookmark to the extraction
// workers.
func enqueueExtractPage(ctx context.Context, b *Bookmark, html []byte) {
	workerPool.Submit(func() {
		// Always set state to loaded, even if there are errors
		saved := false
		defer func() {
			// Recover from any error that could have arose
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

		ex, err := extract.New(b.URL, html)
		if err != nil {
			log.WithError(err).Error()
			return
		}

		ex.LogFields = &log.Fields{"@id": ctx.Value(ctxJobRequestID{}).(string)}

		ex.AddProcessors(
			CheckIPProcessor,
			meta.ExtractMeta,
			meta.ExtractOembed,
			meta.SetDropProperties,
			meta.ExtractFavicon,
			meta.ExtractPicture,
			fftr.LoadConfiguration,
			fftr.ReplaceStrings,
		)

		if len(html) == 0 {
			ex.AddProcessors(fftr.FindContentPage, fftr.FindNextPage)
		}

		ex.AddProcessors(
			fftr.ExtractAuthor,
			fftr.ExtractDate,
			fftr.ExtractBody,
			fftr.StripTags,
			fftr.GoToNextPage,
			contents.Readability,
			CleanDomProcessor,
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
		b.Domain = drop.Domain
		b.Title = drop.Title
		b.Site = drop.URL.Hostname()
		b.SiteName = drop.Site
		b.Authors = Strings{}
		b.Lang = drop.Lang
		b.DocumentType = drop.DocumentType
		b.Description = drop.Description
		b.Text = ex.Text
		b.WordCount = len(strings.Fields(b.Text))

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

		// Run the archiver
		var arc *archiver.Archiver
		logEntry := log.NewEntry(ex.GetLogger()).WithFields(*ex.LogFields)
		if len(ex.HTML) > 0 && ex.Drop().IsHTML() {
			arc, err = newArchive(context.TODO(), ex)
			if err != nil {
				logEntry.WithError(err).Error("archiver error")
			}
		}

		// Create the zip file
		err = createZipFile(b, ex, arc)
		if err != nil {
			// If something goes really wrong, cleanup after ourselves
			b.Errors = append(b.Errors, err.Error())
			b.removeFiles()
			b.FilePath = ""
			b.Files = BookmarkFiles{}
		}

		// All good? Save now
		if err := b.Save(); err != nil {
			log.WithError(err).Error()
			return
		}
		saved = true
	})
}

func createZipFile(b *Bookmark, ex *extract.Extractor, arc *archiver.Archiver) error {
	// Fail fast
	fileURL, err := b.getBaseFileURL()
	if err != nil {
		return err
	}
	zipFile := filepath.Join(StoragePath(), fileURL+".zip")

	b.FilePath = fileURL
	b.Files = BookmarkFiles{}

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
		b.Files[k] = &BookmarkFile{name, p.Type, p.Size}
	}

	// Add HTML content
	if arc != nil && len(arc.Result) > 0 {
		if err = z.addCompressedFile("index.html", arc.Result); err != nil {
			return err
		}
		b.Files["article"] = &BookmarkFile{Name: "index.html"}
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
	b.Files["log"] = &BookmarkFile{Name: "log"}

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
	b.Files["props"] = &BookmarkFile{Name: "props.json"}

	return nil
}
