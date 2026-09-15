package common

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStatusRecorderFlushCommitsImplicitSuccess(t *testing.T) {
	t.Parallel()
	recorder := NewStatusRecorder(httptest.NewRecorder())

	recorder.Writer().(http.Flusher).Flush()
	recorder.WriteHeader(http.StatusInternalServerError)

	assert.True(t, recorder.HeaderWritten())
	assert.Equal(t, http.StatusOK, recorder.Status())
}
