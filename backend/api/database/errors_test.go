package database_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/database"
	"github.com/stretchr/testify/assert"
)

func TestErrorInternalServer(t *testing.T) {
	testErr := errors.New("database connection failed")
	renderer := database.ErrorInternalServer(testErr)
	resp, ok := renderer.(*database.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusInternalServerError, resp.HTTPStatusCode)
	assert.Equal(t, "error", resp.StatusText)
	assert.Equal(t, "Internal server error", resp.ErrorText)
}

func TestErrResponse_Render(t *testing.T) {
	errResp := &database.ErrResponse{
		Err:            errors.New("test error"),
		HTTPStatusCode: http.StatusInternalServerError,
		StatusText:     "error",
		ErrorText:      "Internal server error",
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := errResp.Render(w, r)
	assert.NoError(t, err)
}
