package checkin_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	facilityModels "github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/models/users"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	checkin "github.com/moto-nrw/project-phoenix/services/iot/checkin"
)

// stubActiveService is a narrow active.Service double used to drive the
// error/occupancy branches of the CheckinService without a database.
type stubActiveService struct {
	activeSvc.Service
	findDeviceActiveGroupInRoomFn    func(ctx context.Context, roomID int64, deviceID int64) (*active.Group, error)
	countActiveVisitsByRoomIDFn      func(ctx context.Context, roomID int64) (int, error)
	countActiveVisitsByActiveGroupFn func(ctx context.Context, activeGroupID int64) (int, error)
}

func (s *stubActiveService) GetRoomsByIDs(_ context.Context, _ []int64) ([]*facilityModels.Room, error) {
	return nil, nil
}

func (s *stubActiveService) GetActiveGroupVisitsWithDisplay(_ context.Context, _ int64) ([]*active.VisitWithStudentDisplay, error) {
	return nil, nil
}

func (s *stubActiveService) HasOpenAttendanceOn(_ context.Context, _ timezone.Date) (bool, error) {
	return false, nil
}

func (s *stubActiveService) FindDeviceActiveGroupInRoom(ctx context.Context, roomID int64, deviceID int64) (*active.Group, error) {
	return s.findDeviceActiveGroupInRoomFn(ctx, roomID, deviceID)
}

func (s *stubActiveService) CountActiveVisitsByRoomID(ctx context.Context, roomID int64) (int, error) {
	return s.countActiveVisitsByRoomIDFn(ctx, roomID)
}

func (s *stubActiveService) CountActiveVisitsByActiveGroupID(ctx context.Context, activeGroupID int64) (int, error) {
	return s.countActiveVisitsByActiveGroupFn(ctx, activeGroupID)
}

// =============================================================================
// ShouldSkipCheckin TESTS
// =============================================================================

func TestShouldSkipCheckin_NilRoomID(t *testing.T) {
	t.Parallel()

	result := checkin.ShouldSkipCheckin(nil, true, &active.Visit{ActiveGroup: &active.Group{RoomID: 1}}, time.Now())
	assert.False(t, result)
}

func TestShouldSkipCheckin_NotCheckedOut(t *testing.T) {
	t.Parallel()

	roomID := int64(1)
	result := checkin.ShouldSkipCheckin(&roomID, false, &active.Visit{ActiveGroup: &active.Group{RoomID: 1}}, time.Now())
	assert.False(t, result)
}

func TestShouldSkipCheckin_NilCurrentVisit(t *testing.T) {
	t.Parallel()

	roomID := int64(1)
	result := checkin.ShouldSkipCheckin(&roomID, true, nil, time.Now())
	assert.False(t, result)
}

func TestShouldSkipCheckin_NilActiveGroup(t *testing.T) {
	t.Parallel()

	roomID := int64(1)
	result := checkin.ShouldSkipCheckin(&roomID, true, &active.Visit{}, time.Now())
	assert.False(t, result)
}

func TestShouldSkipCheckin_SameRoom(t *testing.T) {
	t.Parallel()

	roomID := int64(1)
	now := time.Now()
	result := checkin.ShouldSkipCheckin(&roomID, true, &active.Visit{ActiveGroup: &active.Group{RoomID: 1, StartTime: now}}, now)
	assert.True(t, result)
}

func TestShouldSkipCheckin_DifferentRoom(t *testing.T) {
	t.Parallel()

	roomID := int64(2)
	now := time.Now()
	result := checkin.ShouldSkipCheckin(&roomID, true, &active.Visit{ActiveGroup: &active.Group{RoomID: 1, StartTime: now}}, now)
	assert.False(t, result)
}

func TestShouldSkipCheckin_PreviousDaySameRoom(t *testing.T) {
	t.Parallel()

	roomID := int64(1)
	now := time.Now()
	result := checkin.ShouldSkipCheckin(&roomID, true, &active.Visit{
		ActiveGroup: &active.Group{RoomID: roomID, StartTime: now.AddDate(0, 0, -1)},
	}, now)
	assert.False(t, result)
}

// =============================================================================
// BuildCheckinResult TESTS
// =============================================================================

func TestBuildCheckinResult_CheckedOutAndCheckedIn_Transfer(t *testing.T) {
	t.Parallel()

	newVisitID := int64(123)
	checkoutVisitID := int64(100)

	input := &checkin.CheckinResultInput{
		Student:          &users.Student{Model: base.Model{ID: 1}},
		Person:           &users.Person{FirstName: "Max", LastName: "Test"},
		CheckedOut:       true,
		NewVisitID:       &newVisitID,
		CheckoutVisitID:  &checkoutVisitID,
		RoomName:         "Room B",
		PreviousRoomName: "Room A",
	}

	result := checkin.BuildCheckinResult(input)

	assert.Equal(t, "transferred", result.Action)
	assert.Equal(t, "Gewechselt von Room A zu Room B!", result.GreetingMsg)
	assert.Equal(t, &newVisitID, result.VisitID)
	assert.Equal(t, "Room B", result.RoomName)
	assert.Equal(t, "Room A", result.PreviousRoomName)
}

func TestBuildCheckinResult_CheckedOutAndCheckedIn_SameRoom(t *testing.T) {
	t.Parallel()

	newVisitID := int64(123)
	checkoutVisitID := int64(100)

	input := &checkin.CheckinResultInput{
		Student:          &users.Student{Model: base.Model{ID: 1}},
		Person:           &users.Person{FirstName: "Max", LastName: "Test"},
		CheckedOut:       true,
		NewVisitID:       &newVisitID,
		CheckoutVisitID:  &checkoutVisitID,
		RoomName:         "Room A",
		PreviousRoomName: "Room A", // Same room
	}

	result := checkin.BuildCheckinResult(input)

	assert.Equal(t, "checked_in", result.Action)
	assert.Equal(t, "Hallo Max!", result.GreetingMsg)
}

func TestBuildCheckinResult_CheckedOutOnly(t *testing.T) {
	t.Parallel()

	checkoutVisitID := int64(100)

	input := &checkin.CheckinResultInput{
		Student:         &users.Student{Model: base.Model{ID: 1}},
		Person:          &users.Person{FirstName: "Max", LastName: "Test"},
		CheckedOut:      true,
		NewVisitID:      nil, // No checkin
		CheckoutVisitID: &checkoutVisitID,
		RoomName:        "Room A",
	}

	result := checkin.BuildCheckinResult(input)

	assert.Equal(t, "checked_out", result.Action)
	assert.Equal(t, "Tschüss Max!", result.GreetingMsg)
	assert.Equal(t, &checkoutVisitID, result.VisitID)
}

func TestBuildCheckinResult_CheckedInOnly(t *testing.T) {
	t.Parallel()

	newVisitID := int64(123)

	input := &checkin.CheckinResultInput{
		Student:    &users.Student{Model: base.Model{ID: 1}},
		Person:     &users.Person{FirstName: "Max", LastName: "Test"},
		CheckedOut: false,
		NewVisitID: &newVisitID,
		RoomName:   "Room A",
	}

	result := checkin.BuildCheckinResult(input)

	assert.Equal(t, "checked_in", result.Action)
	assert.Equal(t, "Hallo Max!", result.GreetingMsg)
	assert.Equal(t, &newVisitID, result.VisitID)
}

func TestBuildCheckinResult_NoAction(t *testing.T) {
	t.Parallel()

	input := &checkin.CheckinResultInput{
		Student:    &users.Student{Model: base.Model{ID: 1}},
		Person:     &users.Person{FirstName: "Max", LastName: "Test"},
		CheckedOut: false,
		NewVisitID: nil,
	}

	result := checkin.BuildCheckinResult(input)

	// No action - empty action field
	assert.Equal(t, "", result.Action)
	assert.Equal(t, "", result.GreetingMsg)
}

func TestBuildCheckinResult_TransferNoPreviousRoom(t *testing.T) {
	t.Parallel()

	newVisitID := int64(123)
	checkoutVisitID := int64(100)

	input := &checkin.CheckinResultInput{
		Student:          &users.Student{Model: base.Model{ID: 1}},
		Person:           &users.Person{FirstName: "Max", LastName: "Test"},
		CheckedOut:       true,
		NewVisitID:       &newVisitID,
		CheckoutVisitID:  &checkoutVisitID,
		RoomName:         "Room B",
		PreviousRoomName: "", // No previous room
	}

	result := checkin.BuildCheckinResult(input)

	// No previous room, so treated as regular checkin
	assert.Equal(t, "checked_in", result.Action)
	assert.Equal(t, "Hallo Max!", result.GreetingMsg)
}

// =============================================================================
// RoomNameByID / RoomNameForResponse TESTS (pure, no DB)
// =============================================================================

func TestRoomNameByID_WithRoomObject(t *testing.T) {
	t.Parallel()

	svc := checkin.NewCheckinService(checkin.CheckinServiceDeps{})
	room := &facilitiesModule.Room{Name: "Test Room"}
	name := svc.RoomNameByIDForTest(context.Background(), room, 1)
	assert.Equal(t, "Test Room", name)
}

func TestRoomNameForResponse_WithActiveGroupRoom(t *testing.T) {
	t.Parallel()

	svc := checkin.NewCheckinService(checkin.CheckinServiceDeps{})
	currentVisit := &active.Visit{
		ActiveGroup: &active.Group{
			Room: &facilityModels.Room{Name: "Library"},
		},
	}

	name := svc.RoomNameForResponseForTest(context.Background(), currentVisit, nil)
	assert.Equal(t, "Library", name)
}

func TestRoomNameForResponse_NilVisit_NilRoomID(t *testing.T) {
	t.Parallel()

	svc := checkin.NewCheckinService(checkin.CheckinServiceDeps{})

	name := svc.RoomNameForResponseForTest(context.Background(), nil, nil)
	assert.Equal(t, "", name)
}

// =============================================================================
// ProcessStudentCheckin TESTS (rewritten for the (result, error) signature)
// =============================================================================

func TestProcessStudentCheckin_NoRoomNotCheckedOut(t *testing.T) {
	t.Parallel()

	svc := checkin.NewCheckinService(checkin.CheckinServiceDeps{})

	student := &users.Student{Model: base.Model{ID: 1}}
	person := &users.Person{FirstName: "Test", LastName: "User"}

	input := &checkin.CheckinProcessingInput{
		RoomID:      nil,
		SkipCheckin: false,
		CheckedOut:  false,
	}

	result, err := svc.ProcessStudentCheckin(context.Background(), student, person, input)

	require.Error(t, err)
	assert.Nil(t, result)
	// The renderer-agnostic error carries the invalid-request classification and
	// the same message the handler previously rendered.
	var ce *checkin.CheckinError
	require.True(t, errors.As(err, &ce), "expected a *checkin.CheckinError")
	assert.Equal(t, checkin.KindInvalidRequest, ce.Kind)
	assert.Contains(t, err.Error(), "room_id is required")
}

func TestProcessStudentCheckin_SkippedCheckin_GetsRoomName(t *testing.T) {
	t.Parallel()

	svc := checkin.NewCheckinService(checkin.CheckinServiceDeps{})

	student := &users.Student{Model: base.Model{ID: 1}}
	person := &users.Person{FirstName: "Test", LastName: "User"}

	roomID := int64(1)
	input := &checkin.CheckinProcessingInput{
		RoomID:      &roomID,
		SkipCheckin: true,
		CheckedOut:  true,
		CurrentVisit: &active.Visit{
			ActiveGroup: &active.Group{
				Room: &facilityModels.Room{Name: "SkipCheckinRoom"},
			},
		},
	}

	result, err := svc.ProcessStudentCheckin(context.Background(), student, person, input)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "SkipCheckinRoom", result.RoomName)
	assert.Nil(t, result.NewVisitID, "Should not create a new visit when skipping checkin")
}

// =============================================================================
// Struct-shape TESTS
// =============================================================================

func TestCheckinProcessingResult_Struct(t *testing.T) {
	t.Parallel()

	visitID := int64(123)
	result := &checkin.CheckinProcessingResult{
		NewVisitID: &visitID,
		RoomName:   "Test Room",
	}

	assert.Equal(t, &visitID, result.NewVisitID)
	assert.Equal(t, "Test Room", result.RoomName)
}

func TestCheckinProcessingInput_Struct(t *testing.T) {
	t.Parallel()

	roomID := int64(1)
	input := &checkin.CheckinProcessingInput{
		RoomID:       &roomID,
		SkipCheckin:  false,
		CheckedOut:   true,
		CurrentVisit: nil,
	}

	assert.Equal(t, &roomID, input.RoomID)
	assert.False(t, input.SkipCheckin)
	assert.True(t, input.CheckedOut)
	assert.Nil(t, input.CurrentVisit)
}

func TestCheckinProcessingResult_ActiveGroupID(t *testing.T) {
	t.Parallel()

	activeGroupID := int64(42)
	visitID := int64(123)
	result := &checkin.CheckinProcessingResult{
		NewVisitID:    &visitID,
		RoomName:      "Test Room",
		ActiveGroupID: &activeGroupID,
	}

	assert.Equal(t, &activeGroupID, result.ActiveGroupID)
	assert.Equal(t, &visitID, result.NewVisitID)
	assert.Equal(t, "Test Room", result.RoomName)
}

func TestCheckinResult_AllFields(t *testing.T) {
	t.Parallel()

	visitID := int64(42)
	activeStudents := 7
	result := &checkin.CheckinResult{
		Action:                 "transferred",
		VisitID:                &visitID,
		RoomName:               "Room B",
		PreviousRoomName:       "Room A",
		GreetingMsg:            "Gewechselt!",
		DailyCheckoutAvailable: true,
		ActiveStudents:         &activeStudents,
	}

	assert.Equal(t, "transferred", result.Action)
	assert.Equal(t, &visitID, result.VisitID)
	assert.Equal(t, "Room B", result.RoomName)
	assert.Equal(t, "Room A", result.PreviousRoomName)
	assert.Equal(t, "Gewechselt!", result.GreetingMsg)
	assert.True(t, result.DailyCheckoutAvailable)
	assert.Equal(t, &activeStudents, result.ActiveStudents)
}

// =============================================================================
// Occupancy / device-group error paths (stub-driven, no DB)
// =============================================================================

func TestGetDeviceActiveGroupInRoom_ServiceError(t *testing.T) {
	t.Parallel()

	svc := checkin.NewCheckinService(checkin.CheckinServiceDeps{
		Active: &stubActiveService{
			findDeviceActiveGroupInRoomFn: func(context.Context, int64, int64) (*active.Group, error) {
				return nil, errors.New("database down")
			},
		},
	})

	result := svc.GetDeviceActiveGroupInRoom(context.Background(), 5, 7)

	assert.Nil(t, result, "Should return nil when active service fails")
}

func TestGetActiveStudentCountForRoom_ServiceError(t *testing.T) {
	t.Parallel()

	svc := checkin.NewCheckinService(checkin.CheckinServiceDeps{
		Active: &stubActiveService{
			countActiveVisitsByRoomIDFn: func(context.Context, int64) (int, error) {
				return 0, errors.New("count failed")
			},
			countActiveVisitsByActiveGroupFn: func(context.Context, int64) (int, error) {
				return 0, nil
			},
			findDeviceActiveGroupInRoomFn: func(context.Context, int64, int64) (*active.Group, error) {
				return nil, nil
			},
		},
	})

	result := svc.GetActiveStudentCountForRoom(context.Background(), 42)

	assert.Nil(t, result, "Should return nil when room occupancy lookup fails")
}

func TestGetActiveStudentCountForGroup(t *testing.T) {
	t.Parallel()

	svc := checkin.NewCheckinService(checkin.CheckinServiceDeps{
		Active: &stubActiveService{
			countActiveVisitsByRoomIDFn: func(context.Context, int64) (int, error) {
				return 0, nil
			},
			countActiveVisitsByActiveGroupFn: func(context.Context, int64) (int, error) {
				return 3, nil
			},
			findDeviceActiveGroupInRoomFn: func(context.Context, int64, int64) (*active.Group, error) {
				return nil, nil
			},
		},
	})

	result := svc.GetActiveStudentCountForGroup(context.Background(), 99)

	require.NotNil(t, result)
	assert.Equal(t, 3, *result)
}

func TestGetActiveStudentCountForGroup_ServiceError(t *testing.T) {
	t.Parallel()

	svc := checkin.NewCheckinService(checkin.CheckinServiceDeps{
		Active: &stubActiveService{
			countActiveVisitsByRoomIDFn: func(context.Context, int64) (int, error) {
				return 0, nil
			},
			countActiveVisitsByActiveGroupFn: func(context.Context, int64) (int, error) {
				return 0, errors.New("count failed")
			},
			findDeviceActiveGroupInRoomFn: func(context.Context, int64, int64) (*active.Group, error) {
				return nil, nil
			},
		},
	})

	result := svc.GetActiveStudentCountForGroup(context.Background(), 99)

	assert.Nil(t, result, "Should return nil when active-group occupancy lookup fails")
}
