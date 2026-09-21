package application

import (
	"sync"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// demoRequestWindow admits at most limit requests per key within any
// DemoRequestWindow (#3466). It lives in the serving process: the demo
// environment runs one server, and a restart only forgives a window.
type demoRequestWindow struct {
	limit int
	mu    sync.Mutex
	seen  map[string][]time.Time
	swept time.Time
}

func newDemoRequestWindow(limit int) *demoRequestWindow {
	return &demoRequestWindow{limit: limit, seen: map[string][]time.Time{}}
}

// admit notes a request of key at now, or reports the rejection when key
// already made limit requests in the window. A rejected request is not
// noted, so the retry time stays where the answer said.
func (w *demoRequestWindow) admit(key string, now time.Time) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	since := now.Add(-domain.DemoRequestWindow)
	w.sweep(since)
	recent := inWindow(w.seen[key], since)
	if len(recent) >= w.limit {
		w.seen[key] = recent
		return &domain.DemoAccessRateLimitedError{RetryAt: recent[0].Add(domain.DemoRequestWindow)}
	}
	w.seen[key] = append(recent, now)
	return nil
}

// withdraw takes back the request admit noted for key at at, for a request
// the demo could not serve.
func (w *demoRequestWindow) withdraw(key string, at time.Time) {
	w.mu.Lock()
	defer w.mu.Unlock()
	requests := w.seen[key]
	for i := len(requests) - 1; i >= 0; i-- {
		if requests[i].Equal(at) {
			w.seen[key] = append(requests[:i:i], requests[i+1:]...)
			return
		}
	}
}

// sweep forgets keys without a request in the window, at most once per
// window, so keys that never return do not accumulate.
func (w *demoRequestWindow) sweep(since time.Time) {
	if w.swept.After(since) {
		return
	}
	for key, requests := range w.seen {
		if len(inWindow(requests, since)) == 0 {
			delete(w.seen, key)
		}
	}
	w.swept = since.Add(domain.DemoRequestWindow)
}

// inWindow drops the requests at or before since; requests are in order.
func inWindow(requests []time.Time, since time.Time) []time.Time {
	for i, at := range requests {
		if at.After(since) {
			return requests[i:]
		}
	}
	return nil
}
