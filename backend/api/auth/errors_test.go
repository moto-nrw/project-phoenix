package auth_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/auth"
	apiCommon "github.com/moto-nrw/project-phoenix/api/common"
	"github.com/stretchr/testify/assert"
)

func TestAuthErrorInvalidRequest(t *testing.T) {
	err := errors.New("bad")
	renderer := auth.ErrorInvalidRequest(err)
	resp, ok := renderer.(*apiCommon.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusBadRequest, resp.HTTPStatusCode)
}

func TestAuthErrorUnauthorized(t *testing.T) {
	err := errors.New("no auth")
	renderer := auth.ErrorUnauthorized(err)
	resp, ok := renderer.(*apiCommon.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusUnauthorized, resp.HTTPStatusCode)
}

func TestAuthErrorInternalServer(t *testing.T) {
	err := errors.New("broken")
	renderer := auth.ErrorInternalServer(err)
	resp, ok := renderer.(*apiCommon.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusInternalServerError, resp.HTTPStatusCode)
}

func TestAuthErrorNotFound(t *testing.T) {
	err := errors.New("missing")
	renderer := auth.ErrorNotFound(err)
	resp, ok := renderer.(*apiCommon.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusNotFound, resp.HTTPStatusCode)
}
