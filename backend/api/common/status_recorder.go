package common

import (
	"net/http"
	"sync/atomic"
)

// StatusRecorder remembers the status a handler wrote. An unwritten header
// counts as 200, matching net/http. Readers may sit on another goroutine
// than the handler (a runtime event from a background job), so its state is
// atomic.
type StatusRecorder struct {
	http.ResponseWriter
	state atomic.Uint32
}

func NewStatusRecorder(w http.ResponseWriter) *StatusRecorder {
	r := &StatusRecorder{ResponseWriter: w}
	r.state.Store(http.StatusOK)
	return r
}

const statusHeaderWritten uint32 = 1 << 31

func (r *StatusRecorder) Status() int { return int(r.state.Load() &^ statusHeaderWritten) }

// HeaderWritten reports whether the final response status has been sent to the
// client and can no longer change.
func (r *StatusRecorder) HeaderWritten() bool { return r.state.Load()&statusHeaderWritten != 0 }

func (r *StatusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

// Writer returns a response writer with the wrapped writer's flushing
// capability. Callers pass it to handlers while reading status from r.
func (r *StatusRecorder) Writer() http.ResponseWriter {
	flusher, ok := r.ResponseWriter.(http.Flusher)
	if !ok {
		return r
	}
	return &flushingStatusRecorder{StatusRecorder: r, flusher: flusher}
}

func (r *StatusRecorder) WriteHeader(status int) {
	r.commitStatus(status)
	r.ResponseWriter.WriteHeader(status)
}

func (r *StatusRecorder) Write(body []byte) (int, error) {
	r.commitStatus(http.StatusOK)
	return r.ResponseWriter.Write(body)
}

type flushingStatusRecorder struct {
	*StatusRecorder
	flusher http.Flusher
}

func (r *flushingStatusRecorder) Flush() {
	r.commitStatus(http.StatusOK)
	r.flusher.Flush()
}

func (r *StatusRecorder) commitStatus(status int) {
	if status >= http.StatusContinue && status < http.StatusOK {
		return
	}
	for {
		state := r.state.Load()
		if state&statusHeaderWritten != 0 {
			return
		}
		if r.state.CompareAndSwap(state, uint32(status)|statusHeaderWritten) {
			return
		}
	}
}
