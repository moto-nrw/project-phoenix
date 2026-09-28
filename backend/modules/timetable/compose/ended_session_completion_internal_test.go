package compose

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The fakes answer only what the completion asks; every call is recorded in
// order so the tests can pin that attendance is final before the status
// flips.

type endedSessionInstancesStub struct {
	instances []*scheduleModels.ActivityInstance
	completed int64
	calls     *[]string
}

func (f *endedSessionInstancesStub) List(context.Context, *ActivityInstanceQueryOptions) ([]*scheduleModels.ActivityInstance, error) {
	return f.instances, nil
}

func (f *endedSessionInstancesStub) CompleteActiveByActiveGroupIDs(context.Context, []int64, time.Time) (int64, error) {
	*f.calls = append(*f.calls, "complete")
	return f.completed, nil
}

type endedSessionParticipantsStub struct {
	candidates    []*scheduleModels.InstanceStudent
	markErr       error
	gotExclusions []scheduleModels.StudentInstanceRef
	gotMarked     []scheduleModels.StudentInstanceRef
	calls         *[]string
}

func (f *endedSessionParticipantsStub) FindNotScheduledCandidatesByInstanceIDs(context.Context, []int64) ([]*scheduleModels.InstanceStudent, error) {
	return f.candidates, nil
}

func (f *endedSessionParticipantsStub) MarkNotScheduled(_ context.Context, refs []scheduleModels.StudentInstanceRef) error {
	*f.calls = append(*f.calls, "mark_not_scheduled")
	f.gotMarked = refs
	return nil
}

func (f *endedSessionParticipantsStub) MarkExpectedAbsentByActiveGroupIDs(_ context.Context, _ []int64, _ time.Time, exclusions []scheduleModels.StudentInstanceRef) error {
	*f.calls = append(*f.calls, "mark_absent")
	f.gotExclusions = exclusions
	return f.markErr
}

// careDaysStub answers the plan verdict per child for every date and applies
// Care Plan's absence exemption: only a non-booking is spared.
type careDaysStub struct {
	byStudent map[int64]timetable.CareDayStatus
}

func (f careDaysStub) ResolveForDate(_ context.Context, _ []int64, _ timezone.Date) (map[int64]timetable.CareDayStatus, error) {
	return f.byStudent, nil
}

func (f careDaysStub) ResolveForRange(_ context.Context, studentIDs []int64, from, to timezone.Date) (map[int64]map[timezone.Date]timetable.CareDayStatus, error) {
	out := map[int64]map[timezone.Date]timetable.CareDayStatus{}
	for _, studentID := range studentIDs {
		byDate := map[timezone.Date]timetable.CareDayStatus{}
		for date := from; !date.After(to); date = date.AddDays(1) {
			byDate[date] = f.byStudent[studentID]
		}
		out[studentID] = byDate
	}
	return out, nil
}

func (careDaysStub) AttendanceRowCareDay(_ bool, _ timetable.CareDayAttendance, planVerdict timetable.CareDayStatus) timetable.CareDayStatus {
	return planVerdict
}

func (careDaysStub) Expected(status timetable.CareDayStatus) bool {
	return status != timetable.CareDayNotScheduled && status != timetable.CareDayCancelled
}

func (careDaysStub) ExemptFromAbsence(status timetable.CareDayStatus) bool {
	return status == timetable.CareDayNotScheduled
}

func endedInstance(id int64, date timezone.Date) *scheduleModels.ActivityInstance {
	instance := &scheduleModels.ActivityInstance{Date: scheduleModels.Date(date), Title: "Hausaufgaben"}
	instance.ID = id
	return instance
}

func newEndedSessionCompletion(t *testing.T, deps EndedSessionCompletionDependencies) timetable.EndedSessionCompletion {
	t.Helper()
	completion, err := NewEndedSessionCompletion(deps)
	require.NoError(t, err)
	return completion
}

// A completed block must never keep a genuinely expected row: readers take
// "completed + expected" as the frozen "war an dem Tag nicht eingeplant"
// marker (#1747). Force start used to stamp the block completed without
// finalizing attendance, relabelling real children as never booked.
func TestEndedSessionCompletionFinalizesAttendanceFirst(t *testing.T) {
	t.Parallel()
	const (
		activeGroupID  int64 = 8801
		instanceID     int64 = 7701
		bookedStudent  int64 = 5501
		notBookedChild int64 = 5502
		cancelledChild int64 = 5503
	)
	calls := make([]string, 0, 3)
	participants := &endedSessionParticipantsStub{
		candidates: []*scheduleModels.InstanceStudent{
			{InstanceID: instanceID, StudentID: bookedStudent},
			{InstanceID: instanceID, StudentID: notBookedChild},
			{InstanceID: instanceID, StudentID: cancelledChild},
		},
		calls: &calls,
	}
	completion := newEndedSessionCompletion(t, EndedSessionCompletionDependencies{
		Instances: &endedSessionInstancesStub{
			instances: []*scheduleModels.ActivityInstance{endedInstance(instanceID, timezone.NewDate(2026, 4, 20))},
			completed: 1,
			calls:     &calls,
		},
		Participants: participants,
		CareDays: careDaysStub{byStudent: map[int64]timetable.CareDayStatus{
			bookedStudent:  timetable.CareDayScheduled,
			notBookedChild: timetable.CareDayNotScheduled,
			cancelledChild: timetable.CareDayCancelled,
		}},
	})

	completed, err := completion.CompleteActiveByActiveGroupIDs(context.Background(), []int64{activeGroupID}, time.Now())
	require.NoError(t, err)
	assert.EqualValues(t, 1, completed)
	// Attendance is final BEFORE the status flips: the bulk absent update
	// only matches blocks that still run.
	assert.Equal(t, []string{"mark_not_scheduled", "mark_absent", "complete"}, calls)
	// Only the non-booking is spared. A cancelled day is a reported absence
	// and must still be written, or it vanishes from history and exports.
	assert.Equal(t, []scheduleModels.StudentInstanceRef{{StudentID: notBookedChild, InstanceID: instanceID}}, participants.gotExclusions)
	// The spared row carries the reason itself; without the marker the next
	// writer of `status` could not tell it from an ordinary expected row.
	assert.Equal(t, participants.gotExclusions, participants.gotMarked)
}

// A broad day status (sick / excused / class trip) can already have flipped
// a never-booked child's row to 'absent'. Ending the block resolves that,
// but only for rows the completion looks at (#1747 review).
func TestEndedSessionCompletionUndoesStatusDayAbsenceForUnbookedChild(t *testing.T) {
	t.Parallel()
	const (
		activeGroupID  int64 = 8803
		instanceID     int64 = 7703
		statusDayID    int64 = 3301
		notBookedChild int64 = 5504
	)
	calls := make([]string, 0, 3)
	statusDay := statusDayID
	participants := &endedSessionParticipantsStub{
		candidates: []*scheduleModels.InstanceStudent{{
			InstanceID: instanceID, StudentID: notBookedChild,
			Status: scheduleModels.AttendanceStatusAbsent, StudentStatusDayID: &statusDay,
		}},
		calls: &calls,
	}
	completion := newEndedSessionCompletion(t, EndedSessionCompletionDependencies{
		Instances: &endedSessionInstancesStub{
			instances: []*scheduleModels.ActivityInstance{endedInstance(instanceID, timezone.NewDate(2026, 4, 21))},
			completed: 1,
			calls:     &calls,
		},
		Participants: participants,
		CareDays:     careDaysStub{byStudent: map[int64]timetable.CareDayStatus{notBookedChild: timetable.CareDayNotScheduled}},
	})

	_, err := completion.CompleteActiveByActiveGroupIDs(context.Background(), []int64{activeGroupID}, time.Now())
	require.NoError(t, err)
	ref := scheduleModels.StudentInstanceRef{StudentID: notBookedChild, InstanceID: instanceID}
	assert.Equal(t, []scheduleModels.StudentInstanceRef{ref}, participants.gotMarked,
		"the status-day absence must reach MarkNotScheduled, or it stays in the history")
	// MarkNotScheduled resets the row to 'expected'; without the same
	// exclusion the bulk absent update would write the absence back at once.
	assert.Equal(t, []scheduleModels.StudentInstanceRef{ref}, participants.gotExclusions)
}

// Sessions can straddle midnight: one run closes blocks of two dates, and a
// child not booked on one date may be expected on the other.
func TestEndedSessionCompletionSparesPerInstanceAcrossDates(t *testing.T) {
	t.Parallel()
	const (
		child         int64 = 5505
		firstBlock    int64 = 7704
		secondBlock   int64 = 7705
		activeGroupID int64 = 8804
	)
	calls := make([]string, 0, 3)
	participants := &endedSessionParticipantsStub{
		candidates: []*scheduleModels.InstanceStudent{
			{InstanceID: firstBlock, StudentID: child},
			{InstanceID: secondBlock, StudentID: child},
		},
		calls: &calls,
	}
	first, second := timezone.NewDate(2026, 4, 22), timezone.NewDate(2026, 4, 23)
	completion := newEndedSessionCompletion(t, EndedSessionCompletionDependencies{
		Instances: &endedSessionInstancesStub{
			instances: []*scheduleModels.ActivityInstance{endedInstance(firstBlock, first), endedInstance(secondBlock, second)},
			calls:     &calls,
		},
		Participants: participants,
		CareDays:     perDateCareDays{notBooked: map[timezone.Date]bool{first: true}},
	})

	_, err := completion.CompleteActiveByActiveGroupIDs(context.Background(), []int64{activeGroupID}, time.Now())
	require.NoError(t, err)
	assert.Equal(t, []scheduleModels.StudentInstanceRef{{StudentID: child, InstanceID: firstBlock}}, participants.gotExclusions)
}

// perDateCareDays books the child on every date except the listed ones.
type perDateCareDays struct {
	careDaysStub
	notBooked map[timezone.Date]bool
}

func (f perDateCareDays) ResolveForRange(_ context.Context, studentIDs []int64, from, to timezone.Date) (map[int64]map[timezone.Date]timetable.CareDayStatus, error) {
	out := map[int64]map[timezone.Date]timetable.CareDayStatus{}
	for _, studentID := range studentIDs {
		byDate := map[timezone.Date]timetable.CareDayStatus{}
		for date := from; !date.After(to); date = date.AddDays(1) {
			byDate[date] = timetable.CareDayScheduled
			if f.notBooked[date] {
				byDate[date] = timetable.CareDayNotScheduled
			}
		}
		out[studentID] = byDate
	}
	return out, nil
}

func TestEndedSessionCompletionDoesNotCompleteWhenFinalizationFails(t *testing.T) {
	t.Parallel()
	calls := make([]string, 0, 2)
	completion := newEndedSessionCompletion(t, EndedSessionCompletionDependencies{
		Instances: &endedSessionInstancesStub{
			instances: []*scheduleModels.ActivityInstance{endedInstance(7702, timezone.NewDate(2026, 4, 20))},
			calls:     &calls,
		},
		Participants: &endedSessionParticipantsStub{markErr: errors.New("update failed"), calls: &calls},
	})

	_, err := completion.CompleteActiveByActiveGroupIDs(context.Background(), []int64{8802}, time.Now())
	require.ErrorContains(t, err, "complete bridged instances: mark absent")
	assert.NotContains(t, calls, "complete",
		"a block must not reach 'completed' with unfinalized attendance rows")
}

// The system completion is not the planner's Complete: it may close a block
// before its planned end, and without Care Plan it spares nobody.
func TestEndedSessionCompletionAllowsSystemCompleteBeforePlannedEnd(t *testing.T) {
	t.Parallel()
	calls := make([]string, 0, 3)
	participants := &endedSessionParticipantsStub{calls: &calls}
	instance := endedInstance(7710, timezone.NewDate(2026, 5, 4))
	instance.StartTime = time.Date(1, 1, 1, 14, 0, 0, 0, time.UTC)
	instance.EndTime = time.Date(1, 1, 1, 18, 0, 0, 0, time.UTC)
	completion := newEndedSessionCompletion(t, EndedSessionCompletionDependencies{
		Instances:    &endedSessionInstancesStub{instances: []*scheduleModels.ActivityInstance{instance}, completed: 1, calls: &calls},
		Participants: participants,
	})

	now := time.Date(2026, 5, 4, 15, 0, 0, 0, timezone.Berlin)
	completed, err := completion.CompleteActiveByActiveGroupIDs(context.Background(), []int64{8810}, now)
	require.NoError(t, err)
	assert.EqualValues(t, 1, completed)
	assert.Equal(t, []string{"mark_not_scheduled", "mark_absent", "complete"}, calls)
	assert.Empty(t, participants.gotExclusions)
}

func TestEndedSessionCompletionSkipsWithoutSessions(t *testing.T) {
	t.Parallel()
	calls := make([]string, 0)
	completion := newEndedSessionCompletion(t, EndedSessionCompletionDependencies{
		Instances:    &endedSessionInstancesStub{calls: &calls},
		Participants: &endedSessionParticipantsStub{calls: &calls},
	})

	completed, err := completion.CompleteActiveByActiveGroupIDs(context.Background(), nil, time.Now())
	require.NoError(t, err)
	assert.Zero(t, completed)
	assert.Empty(t, calls)

	_, err = NewEndedSessionCompletion(EndedSessionCompletionDependencies{})
	require.EqualError(t, err, "ended session completion: instances and participants are required")
}
