package timers

import (
	"sync"
	"time"
)

// TimerStore describes a timer store.
//
// A timer store keeps track of timers in a map. A finished timer
// is always removed from the map after its execution.
type TimerStore struct {
	sync.Mutex

	timers map[interface{}]*time.Timer
}

// NewTimerStore creates a new TimerStore.
func NewTimerStore() *TimerStore {
	return &TimerStore{
		timers: make(map[interface{}]*time.Timer),
	}
}

// Start starts a new timer that will execute the given function
// "f" after "duration". It stores the time.Timer in the store and
// removes it when the function finishes.
func (ts *TimerStore) Start(id interface{}, duration time.Duration, f func()) {
	if ts.Exists(id) {
		return
	}

	ts.Lock()
	defer ts.Unlock()

	ts.timers[id] = time.AfterFunc(duration, func() {
		f()
		ts.Lock()
		delete(ts.timers, id)
		ts.Unlock()
	})
}

// Stop cancels a timer and removes it from the store.
func (ts *TimerStore) Stop(id interface{}) (res bool) {
	ts.Lock()
	defer ts.Unlock()

	if t := ts.timers[id]; t != nil {
		res = t.Stop()
		delete(ts.timers, id)
	}

	return
}

// Exists returns true if a timer is running for the given ID.
func (ts *TimerStore) Exists(id interface{}) bool {
	ts.Lock()
	defer ts.Unlock()

	_, ok := ts.timers[id]
	return ok
}
