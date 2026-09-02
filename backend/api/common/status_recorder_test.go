package common

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

type flushRecorder struct {
	*httptest.ResponseRecorder
	flushed bool
}

func (r *flushRecorder) Flush() { r.flushed = true }

func TestStatusRecorderPreservesResponseWriterCapabilities(t *testing.T) {
	t.Parallel()
	base := &flushRecorder{ResponseRecorder: httptest.NewRecorder()}
	recorder := NewStatusRecorder(base)
	assert.Same(t, base, recorder.Unwrap())
	_, ok := recorder.Writer().(http.Flusher)
	assert.True(t, ok)
	recorder.Writer().(http.Flusher).Flush()
	assert.True(t, base.flushed)
}

func TestStatusRecorderKeepsFirstStatusAndReportsHeaderWritten(t *testing.T) {
	t.Parallel()
	recorder := NewStatusRecorder(httptest.NewRecorder())
	assert.False(t, recorder.HeaderWritten())
	assert.Equal(t, http.StatusOK, recorder.Status())

	recorder.WriteHeader(http.StatusNotFound)
	recorder.WriteHeader(http.StatusOK)

	assert.True(t, recorder.HeaderWritten())
	assert.Equal(t, http.StatusNotFound, recorder.Status())

	implicit := NewStatusRecorder(httptest.NewRecorder())
	_, _ = implicit.Write([]byte("body"))
	assert.True(t, implicit.HeaderWritten())
	assert.Equal(t, http.StatusOK, implicit.Status())
}

func TestStatusRecorderKeepsFinalStatusAfterInformationalResponse(t *testing.T) {
	t.Parallel()
	recorder := NewStatusRecorder(httptest.NewRecorder())

	recorder.WriteHeader(http.StatusEarlyHints)
	assert.False(t, recorder.HeaderWritten())
	assert.Equal(t, http.StatusOK, recorder.Status())

	recorder.WriteHeader(http.StatusInternalServerError)
	assert.True(t, recorder.HeaderWritten())
	assert.Equal(t, http.StatusInternalServerError, recorder.Status())
}
