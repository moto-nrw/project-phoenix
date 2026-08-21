package rooms_test

import (
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/api/rooms"
	"github.com/moto-nrw/project-phoenix/services/facilities"
	"github.com/stretchr/testify/assert"
)

func TestErrorRenderer_SystemRoomProtected(t *testing.T) {
	t.Parallel()

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

func TestErrorRenderer_SystemRoomNameReserved(t *testing.T) {
	t.Parallel()

	facErr := &facilities.FacilitiesError{
		Op:  "create room",
		Err: facilities.ErrSystemRoomNameReserved,
	}
	renderer := rooms.ErrorRenderer(facErr)
	resp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok, "expected *common.ErrResponse")
	assert.Equal(t, http.StatusBadRequest, resp.HTTPStatusCode)
	assert.Contains(t, resp.ErrorText, "Schulhof")
	assert.Contains(t, resp.ErrorText, "reserviert")
}

func TestErrorRenderer_RoomRequiredByCareOffering(t *testing.T) {
	t.Parallel()

	facErr := &facilities.FacilitiesError{
		Op:  "delete room",
		Err: facilities.ErrRoomRequiredByCareOffering,
	}
	renderer := rooms.ErrorRenderer(facErr)
	resp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok, "expected *common.ErrResponse")
	assert.Equal(t, http.StatusConflict, resp.HTTPStatusCode)
	assert.Contains(t, resp.ErrorText, "verknüpftes Betreuungsangebot")
}

func TestErrorRenderer_DuplicateToiletRoom(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
