package facilities

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

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
}

func (s *schulhofConflictFacilityService) FindRoomByName(context.Context, string) (*facilityModels.Room, error) {
	return s.room, nil
}

type schulhofConflictActivityService struct {
	activitySvc.ActivityService
	group *activityModels.Group
}

func (s *schulhofConflictActivityService) ListGroups(context.Context, *base.QueryOptions) ([]*activityModels.Group, error) {
	if s.group == nil {
		return []*activityModels.Group{}, nil
	}
	return []*activityModels.Group{s.group}, nil
}

type schulhofConflictActiveService struct {
	activeSvc.Service

	findGroupsByCall [][]*active.Group
	findErrorsByCall []error
	createErr        error

	createCalls int
	findCalls   int
	endedIDs    []int64
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
	return &schulhofService{
		facilityService: &schulhofConflictFacilityService{room: room},
		activityService: &schulhofConflictActivityService{group: activityGroup},
		activeService:   activeService,
		logger:          slog.New(slog.DiscardHandler),
	}
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
