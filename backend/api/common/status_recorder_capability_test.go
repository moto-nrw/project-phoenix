package common

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

type nonFlushingResponseWriter struct{ header http.Header }

func (w *nonFlushingResponseWriter) Header() http.Header     { return w.header }
func (*nonFlushingResponseWriter) Write([]byte) (int, error) { return 0, nil }
func (*nonFlushingResponseWriter) WriteHeader(int)           {}

func TestStatusRecorderDoesNotAdvertiseUnsupportedFlush(t *testing.T) {
	t.Parallel()
	recorder := NewStatusRecorder(&nonFlushingResponseWriter{header: make(http.Header)})

	_, ok := recorder.Writer().(http.Flusher)

	assert.False(t, ok)
}
