package schedule

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
)

// Fakes embed the repository interfaces so only the handful of methods the
// bridge calls need bodies; anything else would panic loudly rather than
// silently returning a zero value.

type bridgeInstanceRepoStub struct {
	scheduleModel.ActivityInstanceRepository

	byActiveGroup map[int64]*scheduleModel.ActivityInstance
	completed     int64
	calls         *[]string
}

func (f *bridgeInstanceRepoStub) FindByActiveGroupID(_ context.Context, activeGroupID int64) (*scheduleModel.ActivityInstance, error) {
	return f.byActiveGroup[activeGroupID], nil
}

func (f *bridgeInstanceRepoStub) CompleteActiveByActiveGroupIDs(_ context.Context, _ []int64, _ time.Time) (int64, error) {
	*f.calls = append(*f.calls, "complete")
	return f.completed, nil
}

type bridgeStudentRepoStub struct {
	scheduleModel.InstanceStudentRepository

	candidates    []*scheduleModel.InstanceStudent
	markErr       error
	gotExclusions []scheduleModel.StudentInstanceRef
	gotMarked     []scheduleModel.StudentInstanceRef
	calls         *[]string
}

func (f *bridgeStudentRepoStub) FindNotScheduledCandidatesByInstanceIDs(context.Context, []int64) ([]*scheduleModel.InstanceStudent, error) {
	return f.candidates, nil
}

func (f *bridgeStudentRepoStub) MarkNotScheduled(_ context.Context, refs []scheduleModel.StudentInstanceRef) error {
	*f.calls = append(*f.calls, "mark_not_scheduled")
	f.gotMarked = refs
	return nil
}

func (f *bridgeStudentRepoStub) MarkExpectedAbsentByActiveGroupIDs(
	_ context.Context, _ []int64, _ time.Time, exclusions []scheduleModel.StudentInstanceRef,
) error {
	*f.calls = append(*f.calls, "mark_absent")
	f.gotExclusions = exclusions
	return f.markErr
}

type bridgeCareDayStub struct {
	byStudent map[int64]CareDayStatus
}

func (f *bridgeCareDayStub) ResolveForDate(_ context.Context, _ []int64, _ timezone.Date) (map[int64]CareDayStatus, error) {
	return f.byStudent, nil
}

func (f *bridgeCareDayStub) ResolveForRange(
	_ context.Context, studentIDs []int64, from, to timezone.Date,
) (map[int64]map[timezone.Date]CareDayStatus, error) {
	out := map[int64]map[timezone.Date]CareDayStatus{}
	for _, studentID := range studentIDs {
		byDate := map[timezone.Date]CareDayStatus{}
		for date := from; !date.After(to); date = date.AddDays(1) {
			byDate[date] = f.byStudent[studentID]
		}
		out[studentID] = byDate
	}
	return out, nil
}

// The whole point of routing every completion path through the bridge: a
// completed instance must never keep a genuinely expected row, because readers
// take "completed + expected" as the frozen "war an dem Tag nicht eingeplant"
// marker (#1747). Force-start used to stamp the instance completed without
// finalizing attendance, which relabelled real children as never booked and
// dropped them out of the attendance history.
func TestTimetableBridgeCompletesOnlyAfterFinalizingAttendance(t *testing.T) {
	const (
		activeGroupID   int64 = 8801
		instanceID      int64 = 7701
		bookedStudent   int64 = 5501
		notBookedChild  int64 = 5502
		cancelledChild  int64 = 5503
		completedResult int64 = 1
	)
	date := timezone.NewDate(2026, 4, 20)
	calls := make([]string, 0, 2)

	instances := &bridgeInstanceRepoStub{
		byActiveGroup: map[int64]*scheduleModel.ActivityInstance{
			activeGroupID: {Date: date, Title: "Hausaufgaben"},
		},
		completed: completedResult,
		calls:     &calls,
	}
	instances.byActiveGroup[activeGroupID].ID = instanceID

	students := &bridgeStudentRepoStub{
		candidates: []*scheduleModel.InstanceStudent{
			{InstanceID: instanceID, StudentID: bookedStudent},
			{InstanceID: instanceID, StudentID: notBookedChild},
			{InstanceID: instanceID, StudentID: cancelledChild},
		},
		calls: &calls,
	}

	svc := NewTimetableBridgeService(TimetableBridgeDependencies{
		Instances:        instances,
		InstanceStudents: students,
		CareDays: &bridgeCareDayStub{byStudent: map[int64]CareDayStatus{
			bookedStudent:  CareDayScheduled,
			notBookedChild: CareDayNotScheduled,
			cancelledChild: CareDayCancelled,
		}},
	})

	completed, err := svc.CompleteActiveByActiveGroupIDs(
		context.Background(), []int64{activeGroupID}, time.Now(),
	)
	require.NoError(t, err)
	assert.Equal(t, completedResult, completed)

	// Attendance is final BEFORE the status flips — the bulk absent update only
	// matches instances that are still active.
	assert.Equal(t, []string{"mark_not_scheduled", "mark_absent", "complete"}, calls)

	// Only the non-booking is spared. A cancelled day is a reported absence and
	// must still be written, or it vanishes from history and exports.
	assert.Equal(t, []scheduleModel.StudentInstanceRef{
		{StudentID: notBookedChild, InstanceID: instanceID},
	}, students.gotExclusions)

	// The spared row carries the reason itself. Without the persisted marker
	// it is indistinguishable from an ordinary expected row, and the next
	// writer of `status` would silently create or destroy the fact.
	assert.Equal(t, students.gotExclusions, students.gotMarked)
}

// A broad day status (sick / excused / class trip) is reported before anything
// knows whether the child was booked into care, so ApplyStatusDay can already
// have flipped a never-booked child's row to 'absent'. Ending the block is what
// resolves that — but only for rows the bridge actually looks at. Reading only
// 'expected' rows left the false absence standing in the history and the
// exports (#1747 review).
func TestTimetableBridgeUndoesStatusDayAbsenceForUnbookedChild(t *testing.T) {
	const (
		activeGroupID  int64 = 8803
		instanceID     int64 = 7703
		statusDayID    int64 = 3301
		notBookedChild int64 = 5504
	)
	date := timezone.NewDate(2026, 4, 21)
	calls := make([]string, 0, 3)

	instances := &bridgeInstanceRepoStub{
		byActiveGroup: map[int64]*scheduleModel.ActivityInstance{
			activeGroupID: {Date: date, Title: "Hausaufgaben"},
		},
		completed: 1,
		calls:     &calls,
	}
	instances.byActiveGroup[activeGroupID].ID = instanceID

	statusDay := statusDayID
	students := &bridgeStudentRepoStub{
		candidates: []*scheduleModel.InstanceStudent{{
			InstanceID:         instanceID,
			StudentID:          notBookedChild,
			Status:             scheduleModel.AttendanceStatusAbsent,
			StudentStatusDayID: &statusDay,
		}},
		calls: &calls,
	}

	svc := NewTimetableBridgeService(TimetableBridgeDependencies{
		Instances:        instances,
		InstanceStudents: students,
		CareDays: &bridgeCareDayStub{byStudent: map[int64]CareDayStatus{
			notBookedChild: CareDayNotScheduled,
		}},
	})

	_, err := svc.CompleteActiveByActiveGroupIDs(
		context.Background(), []int64{activeGroupID}, time.Now(),
	)
	require.NoError(t, err)

	ref := scheduleModel.StudentInstanceRef{StudentID: notBookedChild, InstanceID: instanceID}
	assert.Equal(t, []scheduleModel.StudentInstanceRef{ref}, students.gotMarked,
		"the status-day absence must reach MarkNotScheduled, or it stays in the history")
	// MarkNotScheduled resets the row to 'expected'; without the same exclusion
	// the bulk absent update would immediately write the absence back.
	assert.Equal(t, []scheduleModel.StudentInstanceRef{ref}, students.gotExclusions)
}

func TestTimetableBridgeDoesNotCompleteWhenAttendanceFinalizationFails(t *testing.T) {
	const (
		activeGroupID int64 = 8802
		instanceID    int64 = 7702
	)
	calls := make([]string, 0, 1)

	instances := &bridgeInstanceRepoStub{
		byActiveGroup: map[int64]*scheduleModel.ActivityInstance{
			activeGroupID: {Date: timezone.NewDate(2026, 4, 20)},
		},
		calls: &calls,
	}
	instances.byActiveGroup[activeGroupID].ID = instanceID

	svc := NewTimetableBridgeService(TimetableBridgeDependencies{
		Instances: instances,
		InstanceStudents: &bridgeStudentRepoStub{
			markErr: errors.New("update failed"),
			calls:   &calls,
		},
	})

	_, err := svc.CompleteActiveByActiveGroupIDs(
		context.Background(), []int64{activeGroupID}, time.Now(),
	)
	require.Error(t, err)
	assert.NotContains(t, calls, "complete",
		"an instance must not reach 'completed' with unfinalized attendance rows")
}

func TestTimetableBridgeAllowsSystemCompleteBeforePlannedEnd(t *testing.T) {
	const activeGroupID int64 = 8810
	date := timezone.NewDate(2026, 5, 4)
	inst := &scheduleModel.ActivityInstance{
		Date:      date,
		StartTime: time.Date(1, 1, 1, 14, 0, 0, 0, time.UTC),
		EndTime:   time.Date(1, 1, 1, 18, 0, 0, 0, time.UTC),
	}
	inst.ID = 7710
	calls := make([]string, 0, 3)
	svc := NewTimetableBridgeService(TimetableBridgeDependencies{
		Instances: &bridgeInstanceRepoStub{
			byActiveGroup: map[int64]*scheduleModel.ActivityInstance{activeGroupID: inst},
			completed:     1,
			calls:         &calls,
		},
		InstanceStudents: &bridgeStudentRepoStub{calls: &calls},
	})

	now := time.Date(2026, 5, 4, 15, 0, 0, 0, timezone.Berlin)
	completed, err := svc.CompleteActiveByActiveGroupIDs(context.Background(), []int64{activeGroupID}, now)
	require.NoError(t, err)
	assert.Equal(t, int64(1), completed)
	assert.Contains(t, calls, "complete")
}
