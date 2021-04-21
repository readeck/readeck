package bookmarks

import (
	"codeberg.org/readeck/readeck/pkg/extract"
)

// CleanDomProcessor is a last pass of cleaning on the resulting DOM node.
// It removes unwanted attributes, empty tags and set some defaults.
func CleanDomProcessor(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
	if m.Step() != extract.StepDom {
		return next
	}

	if m.Dom == nil {
		return next
	}

	m.Log.Debug("cleaning resulting DOM")

	bleach.clean(m.Dom)
	bleach.removeEmptyNodes(m.Dom)
	bleach.setLinkRel(m.Dom)

	return next
}
