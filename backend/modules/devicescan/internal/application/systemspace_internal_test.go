package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/facilities"
)

func canonicalSchulhof(id int64) facilities.Room {
	return facilities.Room{ID: id, Name: facilities.SchulhofRoomName, IsSystem: true, IsOpenRoom: true}
}

func TestSchulhofActivity_RejectsLegacyNonCanonicalRoomBeforeExistingActivityReturn(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.rooms.byName[facilities.SchulhofRoomName] = facilities.Room{ID: 7, Name: "schulhof", IsSystem: true}
	h.activities.byName[facilities.SchulhofActivityName] = []ports.Activity{{Name: facilities.SchulhofActivityName}}

	activity, err := h.service().schulhofActivity(context.Background())

	require.Error(t, err)
	assert.ErrorContains(t, err, "non-canonical room name")
	assert.Nil(t, activity)
	assert.Empty(t, h.rooms.created, "legacy room must not be replaced or adopted")
	assert.Zero(t, h.activities.listCalls, "room validation must happen before the existing-activity early return")
}

func TestSchulhofActivity_SelectsDedicatedActivityAmongSameNameActivities(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	room := canonicalSchulhof(42)
	h.rooms.byName[facilities.SchulhofRoomName] = room
	h.activities.byName[facilities.SchulhofActivityName] = []ports.Activity{
		{ID: 76, Name: facilities.SchulhofActivityName, PlannedRoomID: ptr(room.ID)},
		{ID: 77, Name: facilities.SchulhofActivityName, PlannedRoomID: ptr(room.ID), IsSystem: true},
	}

	activity, err := h.service().schulhofActivity(context.Background())

	require.NoError(t, err)
	assert.Equal(t, int64(77), activity.ID)
	assert.Equal(t, 1, h.activities.listCalls)
	assert.Empty(t, h.activities.created)
}

func TestSchulhofActivity_ProvisionsYardCategoryAndActivity(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	activity, err := h.service().schulhofActivity(context.Background())

	require.NoError(t, err)
	require.Len(t, h.rooms.created, 1)
	yard := h.rooms.created[0]
	assert.Equal(t, facilities.SchulhofRoomName, yard.Name)
	assert.True(t, yard.IsSystem)
	assert.True(t, yard.IsOpenRoom, "the provisioned yard starts released (#3064)")
	assert.Nil(t, yard.Color, "the yard color stays admin-configurable (#2405)")
	require.Len(t, h.activities.categories, 1)
	assert.Equal(t, facilities.SchulhofCategoryName, h.activities.categories[0].Name)
	require.Len(t, h.activities.created, 1)
	created := h.activities.created[0]
	assert.Equal(t, facilities.SchulhofActivityName, created.Name)
	assert.True(t, created.IsSystem)
	assert.True(t, created.IsOpen)
	assert.Equal(t, facilities.SchulhofMaxParticipants, created.MaxParticipants)
	assert.Equal(t, activity.ID, h.activities.byName[facilities.SchulhofActivityName][0].ID)
}

func TestSchulhofActivity_IsIdempotent(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	s := h.service()

	first, err := s.schulhofActivity(context.Background())
	require.NoError(t, err)
	second, err := s.schulhofActivity(context.Background())
	require.NoError(t, err)

	assert.Equal(t, first.ID, second.ID)
	assert.Len(t, h.rooms.created, 1)
	assert.Len(t, h.activities.created, 1)
}

func TestWCActivity_ProvisionsToiletCategoryAndActivity(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	activity, err := h.service().wcActivity(context.Background())

	require.NoError(t, err)
	require.Len(t, h.rooms.created, 1)
	toilet := h.rooms.created[0]
	assert.Equal(t, facilities.WCRoomName, toilet.Name)
	assert.False(t, toilet.IsOpenRoom, "toilets are never released")
	require.NotNil(t, toilet.Color)
	assert.Equal(t, facilities.WCActivityName, activity.Name)
}

func TestWCActivity_FindsExistingToiletAlias(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.rooms.toilet = &facilities.Room{ID: 9, Name: facilities.WCRoomAliasName}
	h.activities.byName[facilities.WCActivityName] = []ports.Activity{{ID: 310, Name: facilities.WCActivityName}}

	activity, err := h.service().wcActivity(context.Background())

	require.NoError(t, err)
	assert.Equal(t, int64(310), activity.ID)
	assert.Empty(t, h.rooms.created, "an existing alias room is reused, not duplicated")
}

func TestWCActivity_UnexpectedRoomLookupFailureAborts(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.rooms.toiletErr = errBoom

	_, err := h.service().wcActivity(context.Background())

	require.Error(t, err)
	assert.ErrorContains(t, err, "failed to look up WC room")
	assert.Empty(t, h.rooms.created)
}

func TestEnsureSystemRoom_RetriesLookupAfterCreateConflict(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.rooms.createErr = errBoom
	existing := facilities.Room{ID: 9, Name: facilities.WCRoomName}
	space := wcSpace
	calls := 0
	space.findRoom = func(context.Context, *Service) (*facilities.Room, error) {
		calls++
		if calls == 1 {
			return nil, nil
		}
		return &existing, nil
	}

	room, err := h.service().ensureSystemRoom(context.Background(), space)

	require.NoError(t, err)
	assert.Equal(t, existing.ID, room.ID, "a concurrent creation is adopted")
}

func TestSystemActivity_WithoutCatalogFails(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.activities = nil
	service := NewService(Dependencies{
		Fleet: h.fleet, Presence: h.presence, Rooms: h.rooms, Principals: h.principals, People: h.people,
		Visits: h.visits, Sessions: h.sessions, Attendance: h.attendance, Settings: h.settings,
		UnitOfWork: h.unit, Clock: h.clock, Logger: h.logger,
	})

	_, err := service.wcActivity(context.Background())

	require.Error(t, err)
}
