package rooms_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/api/rooms"
	"github.com/moto-nrw/project-phoenix/services/facilities"
	"github.com/stretchr/testify/assert"
)

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

func TestErrorRenderer_SystemRoomProtected(t *testing.T) {
	facErr := &facilities.FacilitiesError{
		Op:  "delete room",
		Err: facilities.ErrSystemRoomProtected,
	}
	renderer := rooms.ErrorRenderer(facErr)
	resp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok, "expected *common.ErrResponse")
	assert.Equal(t, http.StatusForbidden, resp.HTTPStatusCode)
	assert.Contains(t, resp.ErrorText, "Systemraum")
}

func TestErrorRenderer_DuplicateToiletRoom(t *testing.T) {
	facErr := &facilities.FacilitiesError{
		Op:  "CreateRoom",
		Err: facilities.ErrDuplicateToiletRoom,
	}
	renderer := rooms.ErrorRenderer(facErr)
	resp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok, "expected *common.ErrResponse")
	assert.Equal(t, http.StatusConflict, resp.HTTPStatusCode)
	assert.Contains(t, resp.ErrorText, "Toilettenraum")
	assert.NotContains(t, resp.ErrorText, "facilities error during",
		"renderer must surface the inner sentinel, not the FacilitiesError wrapper prefix")
}

func TestErrorRenderer_DuplicateRoom(t *testing.T) {
	facErr := &facilities.FacilitiesError{
		Op:  "CreateRoom",
		Err: facilities.ErrDuplicateRoom,
	}
	renderer := rooms.ErrorRenderer(facErr)
	resp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok, "expected *common.ErrResponse")
	assert.Equal(t, http.StatusConflict, resp.HTTPStatusCode)
	assert.Contains(t, resp.ErrorText, "Ein Raum mit diesem Namen")
}
