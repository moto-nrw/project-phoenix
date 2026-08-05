package checkin

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/constants"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	activitiesService "github.com/moto-nrw/project-phoenix/services/activities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type rolloverActiveService struct {
	activeService.Service
	groups        []*active.Group
	endedGroupIDs []int64
}

func (s *rolloverActiveService) FindActiveGroupsByRoomID(_ context.Context, _ int64) ([]*active.Group, error) {
	return s.groups, nil
}

func (s *rolloverActiveService) CreateActiveGroup(_ context.Context, group *active.Group) error {
	if len(s.endedGroupIDs) == 0 {
		return activeService.ErrRoomConflict
	}
	group.ID = 200
	return nil
}

func (s *rolloverActiveService) EndActiveGroupSession(_ context.Context, groupID int64) error {
	s.endedGroupIDs = append(s.endedGroupIDs, groupID)
	return nil
}

type rolloverActivityService struct {
	activitiesService.ActivityService
	group *activities.Group
}

func (s *rolloverActivityService) ListGroups(_ context.Context, _ *base.QueryOptions) ([]*activities.Group, error) {
	return []*activities.Group{s.group}, nil
}

func TestUseExistingActiveGroupCannotSelectPreviousDaySession(t *testing.T) {
	now := time.Date(2026, time.August, 5, 9, 0, 0, 0, timezone.Berlin)
	deviceID := int64(91)
	room := &facilities.Room{Model: base.Model{ID: 42}, Name: "Schulhof"}
	stale := &active.Group{
		Model:     base.Model{ID: 100},
		RoomID:    room.ID,
		DeviceID:  &deviceID,
		StartTime: now.AddDate(0, 0, -1),
	}
	current := &active.Group{
		Model:     base.Model{ID: 101},
		RoomID:    room.ID,
		StartTime: now.Add(-time.Hour),
	}

	groups := activeGroupsStartedToday([]*active.Group{stale, current}, now)
	selection := (&CheckinService{}).useExistingActiveGroup(context.Background(), groups, room, deviceID)

	require.NotNil(t, selection)
	assert.Equal(t, current.ID, selection.Group.ID)
	assert.False(t, selection.DeviceScoped, "yesterday's device match must not win")
}

func TestPreviousDaySpecialRoomGroupsAreEndedBeforeReplacement(t *testing.T) {
	now := time.Date(2026, time.August, 5, 9, 0, 0, 0, timezone.Berlin)
	endedAt := now.Add(-time.Hour)
	activeStub := &rolloverActiveService{}
	service := &CheckinService{active: activeStub}
	groups := []*active.Group{
		{Model: base.Model{ID: 100}, StartTime: now.AddDate(0, 0, -1)},
		{Model: base.Model{ID: 101}, StartTime: now.Add(-time.Hour)},
		{Model: base.Model{ID: 102}, StartTime: now.AddDate(0, 0, -2), EndTime: &endedAt},
		{Model: base.Model{ID: 103}, StartTime: now.AddDate(0, 0, 1)},
	}

	err := service.endPreviousDayActiveGroups(context.Background(), groups, now)

	require.NoError(t, err)
	assert.Equal(t, []int64{100}, activeStub.endedGroupIDs)
}

func TestFindOrCreateSpecialRoomGroupEndsStaleConflictBeforeCreate(t *testing.T) {
	now := time.Date(2026, time.August, 5, 9, 0, 0, 0, timezone.Berlin)
	stale := &active.Group{
		Model:     base.Model{ID: 100},
		RoomID:    42,
		StartTime: now.AddDate(0, 0, -1),
	}
	activeStub := &rolloverActiveService{groups: []*active.Group{stale}}
	activity := &activities.Group{Model: base.Model{ID: 300}}
	service := &CheckinService{
		active:     activeStub,
		activities: &rolloverActivityService{group: activity},
		logger:     slog.New(slog.DiscardHandler),
	}
	room := &facilities.Room{Model: base.Model{ID: 42}, Name: constants.WCRoomName}
	originalTimeNow := timeNow
	timeNow = func() time.Time { return now }
	t.Cleanup(func() { timeNow = originalTimeNow })

	selection, err := service.findOrCreateActiveGroupForRoom(context.Background(), room, 91)

	require.NoError(t, err)
	require.NotNil(t, selection)
	assert.Equal(t, []int64{stale.ID}, activeStub.endedGroupIDs)
	assert.Equal(t, int64(200), selection.Group.ID)
	assert.Equal(t, activity.ID, *selection.Group.GroupID)
}
