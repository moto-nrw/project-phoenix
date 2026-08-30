package timetable

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	facilitiesModels "github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/services/users/userstest"
)

type dayTransitionRoomRepo struct {
	facilitiesModels.RoomRepository
	onFind func()
}

func (r dayTransitionRoomRepo) FindByID(context.Context, interface{}) (*facilitiesModels.Room, error) {
	r.onFind()
	return &facilitiesModels.Room{Name: "Lernraum"}, nil
}

func TestOperationsCreateAndStartSpontaneousRechecksWorkdayAfterRequestValidation(t *testing.T) {
	t.Parallel()

	roomChecked := false
	activityGroupRepo := &fakeOperationActivityGroupRepo{}
	activityCategoryRepo := &fakeOperationActivityCategoryRepo{}
	service := &fakeOperationsService{start: &scheduleSvc.StartInstanceResult{
		Instance: &schedule.ActivityInstance{Status: schedule.InstanceStatusActive},
	}}
	res := NewResource(Dependencies{
		TimetableData: operationTimetableData(scheduleSvc.TimetableDataDependencies{
			ActiveGroupRepo:      &fakeOperationActiveGroupRepo{},
			ActivityGroupRepo:    activityGroupRepo,
			ActivityCategoryRepo: activityCategoryRepo,
			RoomRepo: dayTransitionRoomRepo{
				onFind: func() { roomChecked = true },
			},
		}),
		OperationsService: service,
		PersonService: &userstest.PersonServiceMock{
			FindByAccountIDFn: func(context.Context, int64) (*userModels.Person, error) {
				person := &userModels.Person{}
				person.ID = 220
				return person, nil
			},
			GetStaffByPersonIDFn: func(context.Context, int64) (*userModels.Staff, error) {
				staff := &userModels.Staff{}
				staff.ID = 320
				return staff, nil
			},
		},
		SettingsService: &fakeOperationSettingsService{hasOverride: true, boolValue: true},
		Now: func() time.Time {
			if roomChecked {
				return time.Date(2026, time.May, 9, 0, 0, 0, 0, timezone.Berlin)
			}
			return time.Date(2026, time.May, 8, 23, 59, 59, 0, timezone.Berlin)
		},
	})
	router := operationRouter(http.MethodPost, "/spontaneous/start", res.operationsCreateAndStartSpontaneous)

	roomID := int64(70)
	rr := executeOperationRequest(t, router, http.MethodPost, "/spontaneous/start", map[string]any{
		"title":   "Freispiel",
		"room_id": roomID,
	})

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.True(t, roomChecked)
	assert.Empty(t, activityGroupRepo.lastFindByName, "the weekend recheck must run before activity resolution")
	assert.Nil(t, activityGroupRepo.createdGroup)
	assert.Nil(t, activityCategoryRepo.createdCategory)
	assert.Nil(t, service.lastSpontaneousInput, "a request crossing into Saturday must not mutate")
}
