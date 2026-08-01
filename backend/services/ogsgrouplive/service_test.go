package ogsgrouplive

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
)

type failingBoolSettings struct{ err error }

func (s failingBoolSettings) ResolveBool(context.Context, string) (bool, error) {
	return false, s.err
}

func TestSortGroupsUsesGermanCollation(t *testing.T) {
	groups := []*educationModels.Group{{Name: "Zebra"}, {Name: "Äpfel"}}

	sortGroups(groups)

	assert.Equal(t, "Äpfel", groups[0].Name)
	assert.Equal(t, "Zebra", groups[1].Name)
}

func TestRoomStatusesOmitCurrentRoomWithoutGroupRoom(t *testing.T) {
	students := []Student{{ID: 101}}
	locations := &activeService.StudentLocationSnapshot{
		Visits: map[int64]*activeModels.Visit{101: {ActiveGroupID: 201}},
		Groups: map[int64]*activeModels.Group{201: {RoomID: 301}},
	}

	statuses := roomStatuses(students, locations, nil)
	status, ok := statuses["101"]

	require.True(t, ok)
	assert.False(t, status.InGroupRoom)
	assert.Nil(t, status.CurrentRoomID)
}

func TestResolvePhotosEnabledPropagatesSettingsFailure(t *testing.T) {
	want := errors.New("settings unavailable")

	enabled, err := resolvePhotosEnabled(context.Background(), failingBoolSettings{err: want})

	assert.False(t, enabled)
	assert.ErrorIs(t, err, want)
}
