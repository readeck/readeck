package timers

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/sessions"
)

// TimerStore describes a timer store.
//
// A timer store keeps track of timers in a map. A finished timer
// is always removed from the map after its execution.
type TimerStore struct {
	sync.Mutex

	timers    map[int]*time.Timer
	keyPrefix string
}

// NewTimerStore creates a new TimerStore.
func NewTimerStore(prefix string) *TimerStore {
	return &TimerStore{
		timers:    make(map[int]*time.Timer),
		keyPrefix: prefix,
	}
}

// Start starts a new timer that will execute the given function
// "f" after "duration". It stores the time.Timer in the store and
// removes it when the function finishes.
//
// It returns the timer ID so it can be used to save it in a
// convenient place.
func (ts *TimerStore) Start(duration time.Duration, f func()) int {
	ts.Lock()
	defer ts.Unlock()

	id := len(ts.timers) + 1
	ts.timers[id] = time.AfterFunc(duration, func() {
		f()
		ts.Lock()
		delete(ts.timers, id)
		ts.Unlock()
	})

	return id
}

// Stop cancels a timer and removes it from the store.
func (ts *TimerStore) Stop(id int) (res bool) {
	ts.Lock()
	defer ts.Unlock()

	if t := ts.timers[id]; t != nil {
		res = t.Stop()
		delete(ts.timers, id)
	}

	return
}

// Exists returns true if the given timer ID exists
func (ts *TimerStore) Exists(id int) bool {
	ts.Lock()
	defer ts.Unlock()

	_, ok := ts.timers[id]
	return ok
}

// Save saves the timer ID in the session. The suffix is added to the
// timer's session key. It can be, for instance, a object ID.
func (ts *TimerStore) Save(
	w http.ResponseWriter, r *http.Request,
	session *sessions.Session, suffix string, id int,
) {
	k := ts.keyPrefix + suffix
	if id < 0 {
		delete(session.Values, k)
	} else {
		session.Values[k] = id
	}
	session.Save(r, w)
}

// Get returns a timer ID for a given suffix. If no session key or no
// timer exists, it returns -1.
func (ts *TimerStore) Get(session *sessions.Session, suffix string) int {
	k := ts.keyPrefix + suffix
	res, ok := session.Values[k].(int)
	if !ok || !ts.Exists(res) {
		return -1
	}
	return res
}

// Clean removes all the non running timers in the session.
func (ts *TimerStore) Clean(w http.ResponseWriter, r *http.Request, session *sessions.Session) {
	save := false
	for k := range session.Values {
		key, ok := k.(string)
		if !ok {
			continue
		}
		if strings.HasPrefix(key, ts.keyPrefix) {
			key = strings.TrimPrefix(key, ts.keyPrefix)
			if ts.Get(session, key) == -1 {
				delete(session.Values, k)
				save = true
			}
		}
	}
	if save {
		session.Save(r, w)
	}
}
