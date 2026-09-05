package checkin_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/constants"
	activityModels "github.com/moto-nrw/project-phoenix/models/activities"
	facilityModels "github.com/moto-nrw/project-phoenix/modules/facilities"
	activitySvc "github.com/moto-nrw/project-phoenix/services/activities"
	facilitiesSvc "github.com/moto-nrw/project-phoenix/services/facilities"
	checkin "github.com/moto-nrw/project-phoenix/services/iot/checkin"
)

type nonCanonicalSchulhofFacilityService struct {
	facilitiesSvc.Service
	room        *facilityModels.Room
	createCalls int
}

func (s *nonCanonicalSchulhofFacilityService) FindRoomByName(context.Context, string) (*facilityModels.Room, error) {
	return s.room, nil
}

func (s *nonCanonicalSchulhofFacilityService) CreateRoom(context.Context, *facilityModels.Room) error {
	s.createCalls++
	return nil
}

type existingSchulhofActivityService struct {
	activitySvc.ActivityService
	listCalls int
	groups    []*activityModels.Group
}

func (s *existingSchulhofActivityService) ListGroups(context.Context, *activityModels.GroupListQuery) ([]*activityModels.Group, error) {
	s.listCalls++
	if s.groups != nil {
		return s.groups, nil
	}
	return []*activityModels.Group{{Name: constants.SchulhofActivityName}}, nil
}

func TestSchulhofActivityGroupRejectsLegacyNonCanonicalRoomBeforeExistingActivityReturn(t *testing.T) {
	t.Parallel()

	facilityService := &nonCanonicalSchulhofFacilityService{
		room: &facilityModels.Room{Name: "schulhof", IsSystem: true},
	}
	activityService := &existingSchulhofActivityService{}
	svc := checkin.NewCheckinService(checkin.CheckinServiceDeps{
		Facilities: facilityService,
		Activities: activityService,
	})

	activityGroup, err := svc.SchulhofActivityGroupForTest(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-canonical room name")
	assert.Nil(t, activityGroup)
	assert.Zero(t, facilityService.createCalls, "legacy room must not be replaced or adopted")
	assert.Zero(t, activityService.listCalls, "room validation must happen before the existing-activity early return")
}

func TestSchulhofActivityGroupSelectsDedicatedActivityAmongSameNameActivities(t *testing.T) {
	t.Parallel()

	room := &facilityModels.Room{
		ID:       42,
		Name:     constants.SchulhofRoomName,
		IsSystem: true,
	}
	normalActivity := &activityModels.Group{
		Model:         activityModels.Model{ID: 76},
		Name:          constants.SchulhofActivityName,
		PlannedRoomID: &room.ID,
		IsSystem:      false,
	}
	dedicatedActivity := &activityModels.Group{
		Model:         activityModels.Model{ID: 77},
		Name:          constants.SchulhofActivityName,
		PlannedRoomID: &room.ID,
		IsSystem:      true,
	}
	activityService := &existingSchulhofActivityService{
		groups: []*activityModels.Group{normalActivity, dedicatedActivity},
	}
	svc := checkin.NewCheckinService(checkin.CheckinServiceDeps{
		Facilities: &nonCanonicalSchulhofFacilityService{
			room: room,
		},
		Activities: activityService,
	})

	activityGroup, err := svc.SchulhofActivityGroupForTest(context.Background())

	require.NoError(t, err)
	assert.Same(t, dedicatedActivity, activityGroup)
	assert.Equal(t, 1, activityService.listCalls)
}
