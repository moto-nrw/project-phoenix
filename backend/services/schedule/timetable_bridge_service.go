package schedule

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
)

// TimetableBridgeDependencies wires the repositories behind
// TimetableBridgeService.
type TimetableBridgeDependencies struct {
	Instances        scheduleModel.ActivityInstanceRepository
	InstanceStudents scheduleModel.InstanceStudentRepository
	// CareDays is optional. Without it no child is spared the absent stamp,
	// which is the behaviour that predates #1747 — never a silent skip.
	CareDays CareDayService
}

// TimetableBridgeService completes the schedule-side rows of active.groups that
// somebody else already ended: the nightly session-end job, and the force-start
// path that kicks a running session off a device or an activity.
//
// It exists so that no caller can stamp an instance completed without
// finalizing its attendance first: genuinely expected children flip to
// 'absent', children the care plan does not book that day keep their expected
// row and carry the frozen not_scheduled marker instead (#1747). A path that
// only flips the instance status leaves every expected row behind — an absence
// nobody recorded and a non-booking nobody wrote down, on a day that is over
// and that no later path revisits.
//
// Every caller that ends an active.group outside the instance lifecycle service
// goes through here: the nightly session-end job, the force-start path, and the
// kiosk's "Sitzung beenden" (api/iot/sessions).
type TimetableBridgeService struct {
	deps TimetableBridgeDependencies
}

// NewTimetableBridgeService creates the bridge used by every caller that ends
// active.groups outside the instance lifecycle service.
func NewTimetableBridgeService(deps TimetableBridgeDependencies) *TimetableBridgeService {
	return &TimetableBridgeService{deps: deps}
}

// CompleteActiveByActiveGroupIDs marks every still-active instance bridged to
// one of the given active.groups as completed and returns the number of rows
// changed — but only after its attendance is final: children the care plan
// genuinely expects flip expected → absent, children not booked into care that
// day keep their expected row as the marker (#1747).
//
// Session-end completions stay outside the five-minute reopen window. The
// planner/operations Complete path owns recovery snapshots and the planned-end
// gate. This bridge closes sessions that already have to end: timeout, sweep,
// force-start replacement, nightly session-end, and the kiosk session close.
//
// The name matches the repository method it wraps so callers keep one entry
// point, and the ordering lives in exactly one place: the bulk absent update
// only matches instances that are still active, so it has to run first.
func (s *TimetableBridgeService) CompleteActiveByActiveGroupIDs(
	ctx context.Context, activeGroupIDs []int64, completedAt time.Time,
) (int64, error) {
	if len(activeGroupIDs) == 0 {
		return 0, nil
	}

	notScheduled, err := s.notScheduledForEndedSessions(ctx, activeGroupIDs)
	if err != nil {
		return 0, err
	}
	// Stamp the non-booking marker before sparing those rows the absence: the
	// spared rows must carry the reason themselves, or later writers of
	// `status` cannot tell them from ordinary expected rows (#1747 review).
	if err := s.deps.InstanceStudents.MarkNotScheduled(ctx, notScheduled); err != nil {
		return 0, &ScheduleError{Op: "complete bridged instances: mark not scheduled", Err: err}
	}
	if err := s.deps.InstanceStudents.MarkExpectedAbsentByActiveGroupIDs(
		ctx, activeGroupIDs, completedAt, notScheduled,
	); err != nil {
		return 0, &ScheduleError{Op: "complete bridged instances: mark absent", Err: err}
	}

	return s.deps.Instances.CompleteActiveByActiveGroupIDs(ctx, activeGroupIDs, completedAt)
}

// notScheduledForEndedSessions collects the (instance, student) pairs where the
// child is not booked into care at all on that instance's date (#1747). An
// explicitly cancelled day is deliberately not in here — it is a reported
// absence and must still be written (CareDayStatus.ExemptFromAbsence). The
// exclusions are per-instance on purpose: one run can close instances from
// several dates, and a child not booked on one date may be genuinely expected
// on another — a global per-student list would spare both rows.
// Returns nil when the care-day service is not wired, which keeps the previous
// blanket behaviour rather than silently skipping absences.
func (s *TimetableBridgeService) notScheduledForEndedSessions(
	ctx context.Context, activeGroupIDs []int64,
) ([]scheduleModel.StudentInstanceRef, error) {
	if s.deps.CareDays == nil {
		return nil, nil
	}

	// One lookup per ended session — bounded by the sessions a single tenant
	// closes in a day, and the instance rows are already hot from the bridge.
	instanceIDs := make([]int64, 0, len(activeGroupIDs))
	datesByInstance := make(map[int64]timezone.Date, len(activeGroupIDs))
	for _, activeGroupID := range activeGroupIDs {
		instance, err := s.deps.Instances.FindByActiveGroupID(ctx, activeGroupID)
		if err != nil {
			return nil, &ScheduleError{
				Op:  "complete bridged instances: load instance",
				Err: fmt.Errorf("active group %d: %w", activeGroupID, err),
			}
		}
		if instance == nil {
			continue
		}
		instanceIDs = append(instanceIDs, instance.ID)
		datesByInstance[instance.ID] = instance.Date
	}
	if len(instanceIDs) == 0 {
		return nil, nil
	}

	// Not just the 'expected' rows: a broad day status (sick / excused / class
	// trip) is reported before anything knows whether the child was booked into
	// care, so a child the plan never booked can already sit here as 'absent'
	// with the status day owning the row. Ending the block is what resolves
	// that, and MarkNotScheduled can only undo the false absence on rows this
	// query hands it (#1747 review). Complete() has the same reach because it
	// loads the instance's full roster.
	rows, err := s.deps.InstanceStudents.FindNotScheduledCandidatesByInstanceIDs(ctx, instanceIDs)
	if err != nil {
		return nil, &ScheduleError{Op: "complete bridged instances: load candidate students", Err: err}
	}
	if len(rows) == 0 {
		return nil, nil
	}

	// Sessions can straddle midnight, so instances may span two dates. One
	// ResolveForRange over the covering window resolves every (student, date)
	// combination in the same four queries a single date would cost.
	seenStudent := make(map[int64]bool)
	studentIDs := make([]int64, 0, len(rows))
	var from, to timezone.Date
	first := true
	for _, row := range rows {
		date, ok := datesByInstance[row.InstanceID]
		if !ok {
			continue
		}
		if first || date.Before(from) {
			from = date
		}
		if first || date.After(to) {
			to = date
		}
		first = false
		if !seenStudent[row.StudentID] {
			seenStudent[row.StudentID] = true
			studentIDs = append(studentIDs, row.StudentID)
		}
	}
	if len(studentIDs) == 0 {
		return nil, nil
	}

	careDays, err := s.deps.CareDays.ResolveForRange(ctx, studentIDs, from, to)
	if err != nil {
		return nil, &ScheduleError{
			Op:  "complete bridged instances: resolve care days",
			Err: fmt.Errorf("%s..%s: %w", from, to, err),
		}
	}

	exclusions := make([]scheduleModel.StudentInstanceRef, 0)
	for _, row := range rows {
		date, ok := datesByInstance[row.InstanceID]
		if !ok {
			continue
		}
		if careDays[row.StudentID][date].ExemptFromAbsence() {
			exclusions = append(exclusions, scheduleModel.StudentInstanceRef{
				StudentID:  row.StudentID,
				InstanceID: row.InstanceID,
			})
		}
	}
	return exclusions, nil
}
