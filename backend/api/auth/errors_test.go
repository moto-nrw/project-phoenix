package auth_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/auth"
	apiCommon "github.com/moto-nrw/project-phoenix/api/common"
	"github.com/stretchr/testify/assert"
)

func TestAuthErrorVariables(t *testing.T) {
	assert.NotNil(t, auth.ErrInvalidRequest)
	assert.NotNil(t, auth.ErrInvalidLogin)
	assert.NotNil(t, auth.ErrUnauthorized)
	assert.NotNil(t, auth.ErrForbidden)
	assert.NotNil(t, auth.ErrInternalServer)
	assert.NotNil(t, auth.ErrResourceNotFound)
}

func TestAuthErrorVariables_DistinctMessages(t *testing.T) {
	errs := []error{
		auth.ErrInvalidRequest, auth.ErrInvalidLogin,
		auth.ErrUnauthorized, auth.ErrForbidden,
		auth.ErrInternalServer, auth.ErrResourceNotFound,
	}
	msgs := make(map[string]bool)
	for _, e := range errs {
		assert.False(t, msgs[e.Error()], "duplicate: %s", e.Error())
		msgs[e.Error()] = true
	}
}

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
