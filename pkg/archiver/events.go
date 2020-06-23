package archiver

import "context"

// SendEvent is the function used to send an archiver event.
func (arc *Archiver) SendEvent(ctx context.Context, event Event) {
	if arc.EventHandler != nil {
		arc.EventHandler(ctx, arc, event)
	}
}

type eventHandler func(context.Context, *Archiver, Event)

// Event is the interface for events emitted by the archiver.
type Event interface {
	Fields() map[string]interface{}
}

// EventInfo is a simple event for any type of data.
type EventInfo map[string]interface{}

// Fields returns the field map.
func (e EventInfo) Fields() map[string]interface{} {
	return e
}

// EventError is the event emitted when errors occur.
type EventError struct {
	Err error
	URI string
}

// Fields returns the field map.
func (e *EventError) Fields() map[string]interface{} {
	return map[string]interface{}{
		"error": e.Err,
		"uri":   e.URI,
	}
}

// EventStartHTML is the event emitted at the beginning of
// the archiving process.
type EventStartHTML string

// Fields returns the field map.
func (e EventStartHTML) Fields() map[string]interface{} {
	return map[string]interface{}{
		"uri": string(e),
	}
}

// EventFetchURL is the event emitted when the archiver loads
// a remote resource.
type EventFetchURL struct {
	uri    string
	parent string
	cached bool
}

// Fields returns the field map.
func (e *EventFetchURL) Fields() map[string]interface{} {
	return map[string]interface{}{
		"uri":    e.uri,
		"parent": e.parent,
		"cached": e.cached,
	}
}
