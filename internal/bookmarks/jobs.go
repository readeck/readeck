package bookmarks

import (
	"fmt"
	"runtime"

	"github.com/gammazero/workerpool"
	log "github.com/sirupsen/logrus"

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
			contents.Archive,
		)

		ex.Run()
		drop := ex.Drop()
		if drop == nil {
			return
		}

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
		b.Meta = make(BookmarkMeta)
		b.Logs = ex.Logs

		for _, x := range drop.Authors {
			b.Authors = append(b.Authors, x)
		}

		for k, v := range drop.Meta {
			b.Meta[k] = v
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

		// Save HTML
		if len(ex.HTML) > 0 {
			if err := b.AddFile("article", &BookmarkFile{
				Name: "article.html",
				Type: "text/html",
			}, ex.HTML); err != nil {
				log.WithError(err).Error()
			}
		}

		// Save images
		for k, p := range drop.Pictures {
			name := p.Name(k)
			if err := b.AddFile(k, &BookmarkFile{
				Name: name,
				Type: p.Type,
				Size: p.Size,
			}, p.Bytes()); err != nil {
				log.WithError(err).Error()
			}
		}

		if err := b.Save(); err != nil {
			log.WithError(err).Error()
			return
		}
		saved = true
	})
}
