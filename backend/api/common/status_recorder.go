package common

import (
	"net/http"
	"sync/atomic"
)

// StatusRecorder remembers the status a handler wrote. An unwritten header
// counts as 200, matching net/http. Readers may sit on another goroutine
// than the handler (a runtime event from a background job), so both fields
// are atomic.
type StatusRecorder struct {
	http.ResponseWriter
	status        atomic.Int32
	headerWritten atomic.Bool
}

func NewStatusRecorder(w http.ResponseWriter) *StatusRecorder {
	r := &StatusRecorder{ResponseWriter: w}
	r.status.Store(http.StatusOK)
	return r
}

func (r *StatusRecorder) Status() int { return int(r.status.Load()) }

// HeaderWritten reports whether the status has been sent to the client and
// can no longer change.
func (r *StatusRecorder) HeaderWritten() bool { return r.headerWritten.Load() }

func (r *StatusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

func (r *StatusRecorder) WriteHeader(status int) {
	if r.headerWritten.CompareAndSwap(false, true) {
		r.status.Store(int32(status))
	}
	r.ResponseWriter.WriteHeader(status)
}

func (r *StatusRecorder) Write(body []byte) (int, error) {
	r.headerWritten.Store(true)
	return r.ResponseWriter.Write(body)
}

func (r *StatusRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}
