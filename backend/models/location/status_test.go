package location_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	location "github.com/moto-nrw/project-phoenix/models/location"
)

func TestStatusJSONMarshalingWithRoom(t *testing.T) {
	t.Helper()

	room := &location.Room{
		ID:          int64(42),
		Name:        "Raum A",
		IsGroupRoom: true,
		OwnerType:   location.RoomOwnerGroup,
	}

	status := location.NewStatus(location.StatePresentInRoom, room)

	payload, err := json.Marshal(status)
	require.NoError(t, err)

	var decoded location.Status
	require.NoError(t, json.Unmarshal(payload, &decoded))

	assert.Equal(t, location.StatePresentInRoom, decoded.State)
	if assert.NotNil(t, decoded.Room) {
		assert.Equal(t, room.ID, decoded.Room.ID)
		assert.Equal(t, room.Name, decoded.Room.Name)
		assert.Equal(t, room.IsGroupRoom, decoded.Room.IsGroupRoom)
		assert.Equal(t, room.OwnerType, decoded.Room.OwnerType)
	}
}

func TestStatusJSONMarshalingWithoutRoom(t *testing.T) {
	t.Helper()

	status := location.NewStatus(location.StateTransit, nil)

	payload, err := json.Marshal(status)
	require.NoError(t, err)

	assert.NotContains(t, string(payload), `"room"`)

	var decoded location.Status
	require.NoError(t, json.Unmarshal(payload, &decoded))

	assert.Equal(t, location.StateTransit, decoded.State)
	assert.Nil(t, decoded.Room)
}
