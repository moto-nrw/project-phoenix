package rooms_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/rooms"
	"github.com/stretchr/testify/assert"
)

func TestErrorVariables(t *testing.T) {
	assert.NotNil(t, rooms.ErrInvalidRequest)
	assert.NotNil(t, rooms.ErrInternalServer)
	assert.NotNil(t, rooms.ErrResourceNotFound)

	// Verify distinct messages
	errs := []error{rooms.ErrInvalidRequest, rooms.ErrInternalServer, rooms.ErrResourceNotFound}
	msgs := make(map[string]bool)
	for _, e := range errs {
		assert.False(t, msgs[e.Error()], "duplicate error message: %s", e.Error())
		msgs[e.Error()] = true
	}
}

func TestErrorInvalidRequest(t *testing.T) {
	err := errors.New("bad input")
	renderer := rooms.ErrorInvalidRequest(err)
	assert.NotNil(t, renderer)

	resp, ok := renderer.(interface {
		Render(http.ResponseWriter, *http.Request) error
	})
	assert.True(t, ok)
	assert.NotNil(t, resp)
}
