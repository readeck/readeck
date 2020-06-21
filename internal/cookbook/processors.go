package cookbook

import (
	"bytes"
	"context"
	"time"

	"github.com/readeck/readeck/pkg/archiver"
	"github.com/readeck/readeck/pkg/extract"
)

func archiveProcessor(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
	if m.Step() != extract.StepPostProcess {
		return next
	}

	if len(m.Extractor.HTML) == 0 {
		return next
	}
	if !m.Extractor.Drop().IsHTML() {
		return next
	}

	m.Log.Debug("create archive")

	req := &archiver.Request{
		Client: m.Extractor.Client(),
		Logger: m.Log,
		Input:  bytes.NewReader(m.Extractor.HTML),
		URL:    m.Extractor.Drop().URL,
	}
	arc, err := archiver.New(req)
	if err != nil {
		m.Log.WithError(err).Error("archive error")
		return next
	}
	arc.EnableLog = true
	arc.DebugLog = true
	arc.MaxConcurrentDownload = 5
	arc.Flags = archiver.EnableImages
	arc.RequestTimeout = 20 * time.Second

	if err := arc.Archive(context.Background()); err != nil {
		m.Log.WithError(err).Error("archive error")
		return next
	}

	m.Extractor.HTML = arc.Result

	return next
}
