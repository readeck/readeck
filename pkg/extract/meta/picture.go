package meta

import (
	"codeberg.org/readeck/readeck/pkg/extract"
)

// ExtractPicture is a processor that extracts the picture from the document
// metadata. It has to come after ExtractMeta.
func ExtractPicture(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
	if m.Step() != extract.StepDom || m.Position > 0 {
		return next
	}

	d := m.Extractor.Drop()
	if d.Meta == nil {
		return next
	}

	href := d.Meta.LookupGet(
		"x.picture_url",
		"graph.image",
		"twitter.image",
		"oembed.thumbnail_url",
	)

	if href == "" {
		return next
	}

	size := uint(800)
	if d.DocumentType == "photo" {
		size = 1280
	}

	m.Log.WithField("href", href).Debug("loading picture")

	picture, err := extract.NewPicture(href, d.URL)
	if err != nil {
		m.Log.WithError(err).Warn()
		return next
	}

	if err = picture.Load(m.Extractor.Client(), size, ""); err != nil {
		m.Log.WithError(err).WithField("url", href).Warn("cannot load picture")
		return next
	}

	d.Pictures["image"] = picture
	m.Log.WithField("size", picture.Size[:]).Debug("picture loaded")

	thumbnail, err := picture.Copy(380, "")
	if err != nil {
		m.Log.WithError(err).Warn()
		return next
	}
	d.Pictures["thumbnail"] = thumbnail

	return next
}
