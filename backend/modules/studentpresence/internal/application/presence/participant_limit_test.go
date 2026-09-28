package presence

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// participantLimitSettings answers the #3632 question; every other question
// comes from the embedded fake and is never asked here.
type participantLimitSettings struct {
	*fakeSettingsResolver
	enforced bool
	err      error
	asked    int
}

func (s *participantLimitSettings) WebParticipantLimitEnforced(context.Context) (bool, error) {
	s.asked++
	return s.enforced, s.err
}

type participantLimitActivities struct {
	AttendanceActivityGroups
	activity *ports.SessionActivity
	err      error
	asked    int
}

func (r *participantLimitActivities) FindByID(context.Context, any) (*ports.SessionActivity, error) {
	r.asked++
	return r.activity, r.err
}

func participantLimitService(settings *participantLimitSettings, activities *participantLimitActivities, openVisits int) *service {
	return &service{
		ServiceDependencies: ServiceDependencies{
			PrincipalReader:   testAttendancePrincipal,
			ActivityGroupRepo: activities,
			SchoolPresence: &mockVisitRepository{countActiveByGroupIDFunc: func(context.Context, int64) (int, error) {
				return openVisits, nil
			}},
		},
		settings: settings,
	}
}

func activitySession(activityID int64) *ports.ActiveGroup {
	return &ports.ActiveGroup{ID: 70, RoomID: 12, GroupID: &activityID}
}

func TestEnsureActivityParticipantLimit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	limited := &ports.SessionActivity{ID: 5, Name: "Fußball", MaxParticipants: 45}

	t.Run("allows exceeding the limit while the school allows it", func(t *testing.T) {
		t.Parallel()
		settings := &participantLimitSettings{}
		activities := &participantLimitActivities{activity: limited}
		svc := participantLimitService(settings, activities, 63)

		require.NoError(t, svc.ensureActivityParticipantLimit(ctx, activitySession(5), 3))
		assert.Zero(t, activities.asked, "the default must not load the activity")
	})

	t.Run("never limits an activity without a limit", func(t *testing.T) {
		t.Parallel()
		activities := &participantLimitActivities{activity: &ports.SessionActivity{ID: 5, Name: "Freispiel"}}
		svc := participantLimitService(&participantLimitSettings{enforced: true}, activities, 500)

		require.NoError(t, svc.ensureActivityParticipantLimit(ctx, activitySession(5), 66))
	})

	t.Run("allows filling the last places", func(t *testing.T) {
		t.Parallel()
		svc := participantLimitService(&participantLimitSettings{enforced: true}, &participantLimitActivities{activity: limited}, 42)

		require.NoError(t, svc.ensureActivityParticipantLimit(ctx, activitySession(5), 3))
	})

	t.Run("refuses an assignment beyond the limit with the numbers", func(t *testing.T) {
		t.Parallel()
		svc := participantLimitService(&participantLimitSettings{enforced: true}, &participantLimitActivities{activity: limited}, 42)

		err := svc.ensureActivityParticipantLimit(ctx, activitySession(5), 4)

		require.ErrorIs(t, err, ErrActivityParticipantLimitExceeded)
		assert.NotErrorIs(t, err, ErrRoomCapacityExceeded, "the two refusals stay distinguishable")
		var limitErr *ActivityParticipantLimitError
		require.True(t, errors.As(err, &limitErr))
		assert.Equal(t, ActivityParticipantLimitError{
			ActivityID: 5, ActivityName: "Fußball", CurrentOccupancy: 42, MaxParticipants: 45, Incoming: 4,
		}, *limitErr)
		assert.Equal(t, studentpresence.ActivityParticipantLimitCode, limitErr.ErrorCode())
		assert.Equal(t, studentpresence.ActivityParticipantLimitDetails{
			ActivityID: 5, ActivityName: "Fußball", CurrentOccupancy: 42, MaxParticipants: 45, IncomingStudents: 4,
		}, limitErr.ErrorDetails())
	})

	t.Run("leaves terminal requests to the terminal's own check", func(t *testing.T) {
		t.Parallel()
		settings := &participantLimitSettings{enforced: true}
		svc := participantLimitService(settings, &participantLimitActivities{activity: limited}, 45)
		deviceCtx := context.WithValue(ctx, attendancePrincipalTestKey{}, RequestPrincipal{DeviceID: 9, IsIoT: true})

		require.NoError(t, svc.ensureActivityParticipantLimit(deviceCtx, activitySession(5), 1))
		assert.Zero(t, settings.asked, "a kiosk scan must not read the web setting")
	})

	t.Run("skips sessions without an activity", func(t *testing.T) {
		t.Parallel()
		svc := participantLimitService(&participantLimitSettings{enforced: true}, &participantLimitActivities{activity: limited}, 45)

		require.NoError(t, svc.ensureActivityParticipantLimit(ctx, &ports.ActiveGroup{ID: 70, RoomID: 12}, 1))
	})

	t.Run("keeps the registry default without a settings resolver", func(t *testing.T) {
		t.Parallel()
		activities := &participantLimitActivities{activity: limited}
		svc := participantLimitService(&participantLimitSettings{enforced: true}, activities, 63)
		svc.settings = nil

		require.NoError(t, svc.ensureActivityParticipantLimit(ctx, activitySession(5), 3))
		assert.Zero(t, activities.asked)
	})

	t.Run("fails when the setting cannot be read", func(t *testing.T) {
		t.Parallel()
		svc := participantLimitService(&participantLimitSettings{err: errors.New("settings down")}, &participantLimitActivities{activity: limited}, 0)

		err := svc.ensureActivityParticipantLimit(ctx, activitySession(5), 1)

		require.ErrorIs(t, err, ErrDatabaseOperation)
	})

	t.Run("fails when the activity cannot be read", func(t *testing.T) {
		t.Parallel()
		svc := participantLimitService(&participantLimitSettings{enforced: true}, &participantLimitActivities{err: errors.New("db down")}, 0)

		err := svc.ensureActivityParticipantLimit(ctx, activitySession(5), 1)

		require.ErrorIs(t, err, ErrDatabaseOperation)
	})
}

func TestCountStudentsEnteringSession(t *testing.T) {
	t.Parallel()
	target := &ports.ActiveGroup{ID: 70, RoomID: 12}
	attendance := map[int64]studentpresence.Attendance{1: {}, 2: {}, 3: {}}
	visits := map[int64]*studentpresence.Visit{
		1: {ActiveGroupID: 70}, // already in the target session
		2: {ActiveGroupID: 71}, // another session, possibly in the same room
	}

	// 3 has no visit and enters; 4 has no open attendance and is skipped.
	assert.Equal(t, 2, countStudentsEnteringSession(target, attendance, visits, []int64{1, 2, 3, 4}))
}
