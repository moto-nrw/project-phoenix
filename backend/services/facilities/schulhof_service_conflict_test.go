package facilities

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/constants"
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

	createCalls int
	findCalls   int
	endedIDs    []int64
}

func (s *schulhofConflictActiveService) FindSupervisorsByActiveGroupID(context.Context, int64) ([]*active.GroupSupervisor, error) {
	return []*active.GroupSupervisor{}, s.supervisorsErr
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

func newSchulhofConflictService(activeService *schulhofConflictActiveService, room *facilityModels.Room, activityGroup *activityModels.Group) *schulhofService {
	room.Name = constants.SchulhofRoomName
	room.IsSystem = true
	activityGroup.Name = constants.SchulhofActivityName
	activityGroup.IsSystem = true
	return &schulhofService{
		facilityService: &schulhofConflictFacilityService{room: room},
		activityService: &schulhofConflictActivityService{group: activityGroup},
		activeService:   activeService,
		logger:          slog.New(slog.DiscardHandler),
	}
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

func TestSchulhofService_GetOrCreateActiveGroup_ReusesConcurrentSchulhofGroupAfterRoomConflict(t *testing.T) {
	ctx := context.Background()
	room := &facilityModels.Room{Model: base.Model{ID: 42}}
	activityGroup := &activityModels.Group{Model: base.Model{ID: 77}, PlannedRoomID: &room.ID}
	concurrentGroup := &active.Group{
		Model:     base.Model{ID: 88},
		GroupID:   &activityGroup.ID,
		RoomID:    room.ID,
		StartTime: time.Now(),
	}
	activeService := &schulhofConflictActiveService{
		findGroupsByCall: [][]*active.Group{
			{},
			{},
			{concurrentGroup},
		},
		createErr: fmt.Errorf("create raced with another supervisor: %w", activeSvc.ErrRoomConflict),
	}
	service := newSchulhofConflictService(activeService, room, activityGroup)

	result, err := service.GetOrCreateActiveGroup(ctx, 123)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, concurrentGroup.ID, result.ID)
	assert.Equal(t, 1, activeService.createCalls)
	assert.Equal(t, 3, activeService.findCalls)
	assert.Empty(t, activeService.endedIDs)
}

func TestSchulhofService_GetOrCreateActiveGroup_ReturnsRefetchErrorAfterRoomConflict(t *testing.T) {
	ctx := context.Background()
	room := &facilityModels.Room{Model: base.Model{ID: 42}}
	activityGroup := &activityModels.Group{Model: base.Model{ID: 77}, PlannedRoomID: &room.ID}
	refetchErr := errors.New("refetch failed")
	activeService := &schulhofConflictActiveService{
		findGroupsByCall: [][]*active.Group{
			{},
			{},
			nil,
		},
		findErrorsByCall: []error{
			nil,
			nil,
			refetchErr,
		},
		createErr: fmt.Errorf("create raced with another supervisor: %w", activeSvc.ErrRoomConflict),
	}
	service := newSchulhofConflictService(activeService, room, activityGroup)

	result, err := service.GetOrCreateActiveGroup(ctx, 123)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, refetchErr)
	assert.Contains(t, err.Error(), "failed to refetch Schulhof active group after room conflict")
	assert.Equal(t, 1, activeService.createCalls)
	assert.Equal(t, 3, activeService.findCalls)
}

func TestSchulhofService_GetOrCreateActiveGroup_PropagatesRoomConflictWhenRefetchFindsNoGroup(t *testing.T) {
	ctx := context.Background()
	room := &facilityModels.Room{Model: base.Model{ID: 42}}
	activityGroup := &activityModels.Group{Model: base.Model{ID: 77}, PlannedRoomID: &room.ID}
	activeService := &schulhofConflictActiveService{
		findGroupsByCall: [][]*active.Group{
			{},
			{},
			{},
		},
		createErr: &activeSvc.ActiveError{Op: "CreateActiveGroup", Err: activeSvc.ErrRoomConflict},
	}
	service := newSchulhofConflictService(activeService, room, activityGroup)

	result, err := service.GetOrCreateActiveGroup(ctx, 123)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, activeSvc.ErrRoomConflict)
	assert.Contains(t, err.Error(), "failed to create Schulhof active group")
	assert.Equal(t, 1, activeService.createCalls)
	assert.Equal(t, 3, activeService.findCalls)
}
