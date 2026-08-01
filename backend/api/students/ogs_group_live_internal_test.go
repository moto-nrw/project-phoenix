package students

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/common"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
)

func TestSortOGSLiveGroupsUsesGermanCollation(t *testing.T) {
	groups := []*educationModels.Group{
		{Name: "Zebra"},
		{Name: "Äpfel"},
	}

	sortOGSLiveGroups(groups)

	assert.Equal(t, "Äpfel", groups[0].Name)
	assert.Equal(t, "Zebra", groups[1].Name)
}

func TestBuildOGSLiveRoomStatusOmitsCurrentRoomWithoutGroupRoom(t *testing.T) {
	responses := []StudentResponse{{ID: 101}}
	snapshot := &common.StudentDataSnapshot{
		LocationSnapshot: &common.StudentLocationSnapshot{
			Visits: map[int64]*activeModels.Visit{
				101: {ActiveGroupID: 201},
			},
			Groups: map[int64]*activeModels.Group{
				201: {RoomID: 301},
			},
		},
	}

	statuses := buildOGSLiveRoomStatus(responses, snapshot, nil)
	status, ok := statuses["101"]

	require.True(t, ok)
	assert.False(t, status.InGroupRoom)
	assert.Nil(t, status.CurrentRoomID)
}
