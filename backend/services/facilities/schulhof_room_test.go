package facilities

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/constants"
	activityModels "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/base"
	facilityModels "github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateSchulhofActivityRoomRejectsNormalSameNameActivity(t *testing.T) {
	room := &facilityModels.Room{
		Model:    base.Model{ID: 42},
		Name:     constants.SchulhofRoomName,
		IsSystem: true,
	}
	activityGroup := &activityModels.Group{
		Name:          constants.SchulhofActivityName,
		PlannedRoomID: &room.ID,
		IsSystem:      false,
	}

	err := ValidateSchulhofActivityRoom(activityGroup, room)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not marked as a system activity")
}

func TestFindCanonicalSchulhofRoomRejectsNormalSameNameRoom(t *testing.T) {
	service := &schulhofConflictFacilityService{
		room: &facilityModels.Room{
			Name:     constants.SchulhofRoomName,
			IsSystem: false,
		},
	}

	room, err := FindCanonicalSchulhofRoom(t.Context(), service)

	require.Error(t, err)
	assert.Nil(t, room)
	assert.Contains(t, err.Error(), "not marked as a system room")
}
