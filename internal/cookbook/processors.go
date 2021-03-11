package cookbook

import (
	"bytes"
	"context"
	"time"

	log "github.com/sirupsen/logrus"

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
		Input:  bytes.NewReader(m.Extractor.HTML),
		URL:    m.Extractor.Drop().URL,
	}
	arc, err := archiver.New(req)
	if err != nil {
		m.Log.WithError(err).Error("archive error")
		return next
	}

	arc.MaxConcurrentDownload = 5
	arc.Flags = archiver.EnableImages
	arc.RequestTimeout = 45 * time.Second
	arc.EventHandler = eventHandler

	ctx := context.WithValue(context.Background(), ctxLogger, m.Log)

	if err := arc.Archive(ctx); err != nil {
		m.Log.WithError(err).Error("archive error")
		return next
	}

	m.Extractor.HTML = arc.Result

	return next
}

func eventHandler(ctx context.Context, arc *archiver.Archiver, evt archiver.Event) {
	log := ctx.Value(ctxLogger).(*log.Entry)
	switch evt.(type) {
	case *archiver.EventError:
		log.WithFields(evt.Fields()).Warn("archive error")
	case archiver.EventStartHTML:
		log.WithFields(evt.Fields()).Info("start archive")
	case *archiver.EventFetchURL:
		log.WithFields(evt.Fields()).Debug("load archive resource")
	default:
		log.WithFields(evt.Fields()).Debug("archiver")
	}
}

var ctxLogger = struct{}{}
