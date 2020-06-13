package extract

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
)

type messageFormater struct {
	withPrefix bool
}

func (f *messageFormater) Format(entry *log.Entry) ([]byte, error) {
	data := make(log.Fields)
	for k, v := range entry.Data {
		data[k] = v
	}

	var b *bytes.Buffer
	if entry.Buffer != nil {
		b = entry.Buffer
	} else {
		b = &bytes.Buffer{}
	}

	if f.withPrefix {
		fmt.Fprintf(b, "[%s] ", strings.ToUpper(entry.Level.String())[0:4])
	}

	if entry.Message != "" {
		fmt.Fprintf(b, "%s ", entry.Message)
	}
	for k, v := range data {
		fmt.Fprintf(b, `%s="%v" `, k, v)
	}

	return b.Bytes(), nil
}

var messageLogFormat = messageFormater{withPrefix: true}
var errorLogFormat = messageFormater{withPrefix: false}

type messageLogHook struct {
	m *ProcessMessage
}

func (h *messageLogHook) Levels() []log.Level {
	return log.AllLevels
}
func (h *messageLogHook) Fire(entry *log.Entry) error {
	b, _ := messageLogFormat.Format(entry)
	h.m.Extractor.Logs = append(h.m.Extractor.Logs, strings.TrimSpace(string(b)))

	if entry.Level <= log.ErrorLevel {
		b, _ = errorLogFormat.Format(entry)
		h.m.Extractor.errors = append(h.m.Extractor.errors, errors.New(strings.TrimSpace(string(b))))
	}

	return nil
}
