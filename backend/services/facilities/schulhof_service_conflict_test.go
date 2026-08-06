package facilities

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/constants"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	activityModels "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/base"
	facilityModels "github.com/moto-nrw/project-phoenix/models/facilities"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	activitySvc "github.com/moto-nrw/project-phoenix/services/activities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type schulhofConflictFacilityService struct {
	Service
	room *facilityModels.Room
	err  error
}

func (s *schulhofConflictFacilityService) FindRoomByName(context.Context, string) (*facilityModels.Room, error) {
	return s.room, s.err
}

func TestSchulhofStatusPropagatesRoomLookupErrors(t *testing.T) {
	lookupErr := errors.New("room database unavailable")
	service := NewSchulhofService(
		&schulhofConflictFacilityService{err: lookupErr},
		&schulhofConflictActivityService{},
		&schulhofConflictActiveService{},
		slog.New(slog.DiscardHandler),
	)

	status, err := service.GetSchulhofStatus(context.Background(), 1)

	require.ErrorIs(t, err, lookupErr)
	assert.Nil(t, status)
}

type schulhofConflictActivityService struct {
	activitySvc.ActivityService
	group  *activityModels.Group
	groups []*activityModels.Group
	err    error
}

func (s *schulhofConflictActivityService) ListGroups(context.Context, *base.QueryOptions) ([]*activityModels.Group, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.groups != nil {
		return s.groups, nil
	}
	if s.group == nil {
		return []*activityModels.Group{}, nil
	}
	return []*activityModels.Group{s.group}, nil
}

func TestSchulhofServiceSelectsDedicatedActivityAmongSameNameActivities(t *testing.T) {
	room := &facilityModels.Room{
		Model:    base.Model{ID: 42},
		Name:     constants.SchulhofRoomName,
		IsSystem: true,
	}
	normalActivity := &activityModels.Group{
		Model:         base.Model{ID: 76},
		Name:          constants.SchulhofActivityName,
		PlannedRoomID: &room.ID,
		IsSystem:      false,
	}
	dedicatedActivity := &activityModels.Group{
		Model:         base.Model{ID: 77},
		Name:          constants.SchulhofActivityName,
		PlannedRoomID: &room.ID,
		IsSystem:      true,
	}
	service := NewSchulhofService(
		&schulhofConflictFacilityService{room: room},
		&schulhofConflictActivityService{
			groups: []*activityModels.Group{normalActivity, dedicatedActivity},
		},
		&schulhofConflictActiveService{},
		slog.New(slog.DiscardHandler),
	)

	ensured, err := service.EnsureInfrastructure(context.Background(), 1)
	require.NoError(t, err)
	assert.Same(t, dedicatedActivity, ensured)

	status, err := service.GetSchulhofStatus(context.Background(), 1)
	require.NoError(t, err)
	require.NotNil(t, status.ActivityGroupID)
	assert.Equal(t, dedicatedActivity.ID, *status.ActivityGroupID)
}

type schulhofConflictActiveService struct {
	activeSvc.Service

	findGroupsByCall [][]*active.Group
	findErrorsByCall []error
	createErr        error
	supervisorsErr   error
	visitsErr        error
	supervisors      map[int64][]*active.GroupSupervisor

	createCalls int
	findCalls   int
	endedIDs    []int64
}

func (s *schulhofConflictActiveService) FindSupervisorsByActiveGroupID(context.Context, int64) ([]*active.GroupSupervisor, error) {
	return []*active.GroupSupervisor{}, s.supervisorsErr
}

func (s *schulhofConflictActiveService) FindSupervisorsByActiveGroupIDs(_ context.Context, groupIDs []int64) ([]*active.GroupSupervisor, error) {
	if s.supervisorsErr != nil {
		return nil, s.supervisorsErr
	}

	var supervisors []*active.GroupSupervisor
	for _, groupID := range groupIDs {
		supervisors = append(supervisors, s.supervisors[groupID]...)
	}
	return supervisors, nil
}

func (s *schulhofConflictActiveService) FindVisitsByActiveGroupID(context.Context, int64) ([]*active.Visit, error) {
	return []*active.Visit{}, s.visitsErr
}

func (s *schulhofConflictActiveService) FindActiveGroupsByRoomID(context.Context, int64) ([]*active.Group, error) {
	call := s.findCalls
	s.findCalls++

	if call < len(s.findErrorsByCall) && s.findErrorsByCall[call] != nil {
		return nil, s.findErrorsByCall[call]
	}
	if call < len(s.findGroupsByCall) {
		return s.findGroupsByCall[call], nil
	}
	return []*active.Group{}, nil
}

func (s *schulhofConflictActiveService) EndActiveGroupSession(_ context.Context, id int64) error {
	s.endedIDs = append(s.endedIDs, id)
	return nil
}

func (s *schulhofConflictActiveService) CreateActiveGroup(context.Context, *active.Group) error {
	s.createCalls++
	return s.createErr
}

func TestSchulhofStatusPropagatesInfrastructureReadErrors(t *testing.T) {
	room := &facilityModels.Room{
		Model:    base.Model{ID: 42},
		Name:     constants.SchulhofRoomName,
		IsSystem: true,
	}
	roomID := room.ID
	activityGroup := &activityModels.Group{
		Model:         base.Model{ID: 77},
		Name:          constants.SchulhofActivityName,
		PlannedRoomID: &roomID,
		IsSystem:      true,
	}
	todayGroup := &active.Group{
		Model:     base.Model{ID: 88},
		GroupID:   &activityGroup.ID,
		RoomID:    room.ID,
		StartTime: time.Now(),
	}

	tests := []struct {
		name            string
		activityService *schulhofConflictActivityService
		activeService   *schulhofConflictActiveService
		expectedError   error
	}{
		{
			name:            "activity lookup",
			activityService: &schulhofConflictActivityService{err: errors.New("activity database unavailable")},
			activeService:   &schulhofConflictActiveService{},
			expectedError:   errors.New("activity database unavailable"),
		},
		{
			name:            "active group lookup",
			activityService: &schulhofConflictActivityService{group: activityGroup},
			activeService: &schulhofConflictActiveService{
				findErrorsByCall: []error{errors.New("active group database unavailable")},
			},
			expectedError: errors.New("active group database unavailable"),
		},
		{
			name:            "supervisor lookup",
			activityService: &schulhofConflictActivityService{group: activityGroup},
			activeService: &schulhofConflictActiveService{
				findGroupsByCall: [][]*active.Group{{todayGroup}},
				supervisorsErr:   errors.New("supervisor database unavailable"),
			},
			expectedError: errors.New("supervisor database unavailable"),
		},
		{
			name:            "visit lookup",
			activityService: &schulhofConflictActivityService{group: activityGroup},
			activeService: &schulhofConflictActiveService{
				findGroupsByCall: [][]*active.Group{{todayGroup}},
				visitsErr:        errors.New("visit database unavailable"),
			},
			expectedError: errors.New("visit database unavailable"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewSchulhofService(
				&schulhofConflictFacilityService{room: room},
				tt.activityService,
				tt.activeService,
				slog.New(slog.DiscardHandler),
			)

			status, err := service.GetSchulhofStatus(context.Background(), 1)

			require.Error(t, err)
			assert.Nil(t, status)
			assert.Contains(t, err.Error(), tt.expectedError.Error())
		})
	}
}

func TestSchulhofEnsureInfrastructurePropagatesActivityLookupError(t *testing.T) {
	lookupErr := errors.New("activity database unavailable")
	room := &facilityModels.Room{
		Model:    base.Model{ID: 42},
		Name:     constants.SchulhofRoomName,
		IsSystem: true,
	}
	service := NewSchulhofService(
		&schulhofConflictFacilityService{room: room},
		&schulhofConflictActivityService{err: lookupErr},
		&schulhofConflictActiveService{},
		slog.New(slog.DiscardHandler),
	)

	activityGroup, err := service.EnsureInfrastructure(context.Background(), 1)

	require.ErrorIs(t, err, lookupErr)
	assert.Nil(t, activityGroup)
}

// Since #2161 the status read model surfaces the NEWEST open group in the
// room regardless of template backing: a started planned block (different
// template) or a spontaneous session counts just like the system activity's
// own daily group. Ended groups never win.
func TestSchulhofStatusPicksNewestOpenGroupRegardlessOfTemplate(t *testing.T) {
	room := &facilityModels.Room{
		Model:    base.Model{ID: 42},
		Name:     constants.SchulhofRoomName,
		IsSystem: true,
	}
	roomID := room.ID
	activityGroup := &activityModels.Group{
		Model:         base.Model{ID: 77},
		Name:          constants.SchulhofActivityName,
		PlannedRoomID: &roomID,
		IsSystem:      true,
	}
	// Anchor to today's Berlin midnight: the status filter drops groups from
	// other calendar days, and now-relative offsets would cross the boundary
	// when the test runs shortly after midnight.
	dayStart := timezone.TodayDate().BerlinMidnight()
	older := &active.Group{
		Model:     base.Model{ID: 88},
		GroupID:   &activityGroup.ID,
		RoomID:    room.ID,
		StartTime: dayStart.Add(1 * time.Minute),
	}
	plannedBlockTemplateID := int64(500)
	newer := &active.Group{
		Model:     base.Model{ID: 89},
		GroupID:   &plannedBlockTemplateID,
		RoomID:    room.ID,
		StartTime: dayStart.Add(10 * time.Minute),
	}
	endedAt := dayStart.Add(30 * time.Minute)
	ended := &active.Group{
		Model:     base.Model{ID: 90},
		RoomID:    room.ID,
		StartTime: dayStart.Add(20 * time.Minute),
		EndTime:   &endedAt,
	}
	activeService := &schulhofConflictActiveService{
		findGroupsByCall: [][]*active.Group{{older, ended, newer}},
	}
	service := NewSchulhofService(
		&schulhofConflictFacilityService{room: room},
		&schulhofConflictActivityService{group: activityGroup},
		activeService,
		slog.New(slog.DiscardHandler),
	)

	status, err := service.GetSchulhofStatus(context.Background(), 1)

	require.NoError(t, err)
	require.NotNil(t, status.ActiveGroupID)
	assert.Equal(t, newer.ID, *status.ActiveGroupID)
}

func TestSchulhofStatusPrefersCurrentUsersSupervisedOpenGroup(t *testing.T) {
	const staffID int64 = 123
	room := &facilityModels.Room{
		Model:    base.Model{ID: 42},
		Name:     constants.SchulhofRoomName,
		IsSystem: true,
	}
	roomID := room.ID
	activityGroup := &activityModels.Group{
		Model:         base.Model{ID: 77},
		Name:          constants.SchulhofActivityName,
		PlannedRoomID: &roomID,
		IsSystem:      true,
	}
	// Anchored to today's Berlin midnight — see
	// TestSchulhofStatusPicksNewestOpenGroupRegardlessOfTemplate.
	dayStart := timezone.TodayDate().BerlinMidnight()
	owned := &active.Group{
		Model:     base.Model{ID: 88},
		RoomID:    room.ID,
		StartTime: dayStart.Add(1 * time.Minute),
	}
	newer := &active.Group{
		Model:     base.Model{ID: 89},
		RoomID:    room.ID,
		StartTime: dayStart.Add(10 * time.Minute),
	}
	ownedSupervision := &active.GroupSupervisor{
		Model:   base.Model{ID: 99},
		GroupID: owned.ID,
		StaffID: staffID,
	}
	activeService := &schulhofConflictActiveService{
		findGroupsByCall: [][]*active.Group{{owned, newer}},
		supervisors: map[int64][]*active.GroupSupervisor{
			owned.ID: {ownedSupervision},
		},
	}
	service := NewSchulhofService(
		&schulhofConflictFacilityService{room: room},
		&schulhofConflictActivityService{group: activityGroup},
		activeService,
		slog.New(slog.DiscardHandler),
	)

	status, err := service.GetSchulhofStatus(context.Background(), staffID)

	require.NoError(t, err)
	require.NotNil(t, status.ActiveGroupID)
	assert.Equal(t, owned.ID, *status.ActiveGroupID)
	assert.True(t, status.IsUserSupervising)
	require.NotNil(t, status.SupervisionID)
	assert.Equal(t, ownedSupervision.ID, *status.SupervisionID)
}

func TestSchulhofStatusIgnoresCallersEndedSupervisionWhenSelectingGroup(t *testing.T) {
	const staffID int64 = 123
	room := &facilityModels.Room{
		Model:    base.Model{ID: 42},
		Name:     constants.SchulhofRoomName,
		IsSystem: true,
	}
	roomID := room.ID
	activityGroup := &activityModels.Group{
		Model:         base.Model{ID: 77},
		Name:          constants.SchulhofActivityName,
		PlannedRoomID: &roomID,
		IsSystem:      true,
	}
	// Anchored to today's Berlin midnight — see
	// TestSchulhofStatusPicksNewestOpenGroupRegardlessOfTemplate.
	dayStart := timezone.TodayDate().BerlinMidnight()
	handedOff := &active.Group{
		Model:     base.Model{ID: 88},
		RoomID:    room.ID,
		StartTime: dayStart.Add(1 * time.Minute),
	}
	newer := &active.Group{
		Model:     base.Model{ID: 89},
		RoomID:    room.ID,
		StartTime: dayStart.Add(10 * time.Minute),
	}
	// The caller supervised the older group but already handed it off: the
	// supervisor row is ended, so it must not steer group selection.
	endedOn := timezone.TodayDate()
	endedSupervision := &active.GroupSupervisor{
		Model:   base.Model{ID: 99},
		GroupID: handedOff.ID,
		StaffID: staffID,
		EndDate: &endedOn,
	}
	activeService := &schulhofConflictActiveService{
		findGroupsByCall: [][]*active.Group{{handedOff, newer}},
		supervisors: map[int64][]*active.GroupSupervisor{
			handedOff.ID: {endedSupervision},
		},
	}
	service := NewSchulhofService(
		&schulhofConflictFacilityService{room: room},
		&schulhofConflictActivityService{group: activityGroup},
		activeService,
		slog.New(slog.DiscardHandler),
	)

	status, err := service.GetSchulhofStatus(context.Background(), staffID)

	require.NoError(t, err)
	require.NotNil(t, status.ActiveGroupID)
	assert.Equal(t, newer.ID, *status.ActiveGroupID)
	assert.False(t, status.IsUserSupervising)
	assert.Nil(t, status.SupervisionID)
}

func TestSchulhofStatusIgnoresOpenGroupsFromPreviousDays(t *testing.T) {
	const staffID int64 = 123
	room := &facilityModels.Room{
		Model:    base.Model{ID: 42},
		Name:     constants.SchulhofRoomName,
		IsSystem: true,
	}
	roomID := room.ID
	activityGroup := &activityModels.Group{
		Model:         base.Model{ID: 77},
		Name:          constants.SchulhofActivityName,
		PlannedRoomID: &roomID,
		IsSystem:      true,
	}
	stale := &active.Group{
		Model:     base.Model{ID: 88},
		RoomID:    room.ID,
		StartTime: time.Now().AddDate(0, 0, -1),
	}
	activeService := &schulhofConflictActiveService{
		findGroupsByCall: [][]*active.Group{{stale}},
		supervisors: map[int64][]*active.GroupSupervisor{
			stale.ID: {{GroupID: stale.ID, StaffID: staffID}},
		},
	}
	service := NewSchulhofService(
		&schulhofConflictFacilityService{room: room},
		&schulhofConflictActivityService{group: activityGroup},
		activeService,
		slog.New(slog.DiscardHandler),
	)

	status, err := service.GetSchulhofStatus(context.Background(), staffID)

	require.NoError(t, err)
	assert.Nil(t, status.ActiveGroupID)
	assert.False(t, status.IsUserSupervising)
	assert.Zero(t, status.SupervisorCount)
}
