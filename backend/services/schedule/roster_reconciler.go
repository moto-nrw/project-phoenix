package schedule

import (
	"cmp"
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/schedule"
)

// RosterReconciler adjusts already-materialized FUTURE timetable rosters when a
// grade transition changes a student's lifecycle status.
//
// The materializer is insert-only: it never mutates existing instance_students
// rows, and it copies enrollments for a new instance based on the student's
// status AT MATERIALIZATION TIME. So a graduation applied after upcoming
// instances were already materialized leaves the departed children on those
// rosters (inflating counts and staffing ratios), and a revert cannot re-add a
// child to an instance that was materialized while they were an alumnus (the
// materializer skipped them, and it will never revisit an existing instance).
// This service closes both gaps inside the same transaction as the apply/revert
// (#405 review).
type RosterReconciler struct {
	instanceRepo        schedule.ActivityInstanceRepository
	instanceStudentRepo schedule.InstanceStudentRepository
	enrollmentRepo      activities.StudentEnrollmentRepository
	logger              *slog.Logger
}

// NewRosterReconciler constructs a RosterReconciler. logger may be nil (a
// package default is used).
func NewRosterReconciler(
	instanceRepo schedule.ActivityInstanceRepository,
	instanceStudentRepo schedule.InstanceStudentRepository,
	enrollmentRepo activities.StudentEnrollmentRepository,
	logger *slog.Logger,
) *RosterReconciler {
	return &RosterReconciler{
		instanceRepo:        instanceRepo,
		instanceStudentRepo: instanceStudentRepo,
		enrollmentRepo:      enrollmentRepo,
		logger:              logger,
	}
}

func (s *RosterReconciler) getLogger() *slog.Logger {
	return cmp.Or(s.logger, slog.Default())
}

// RemoveStudentsFromFutureRosters deletes still-planned attendance rows for the
// given (now graduated) students on non-cancelled instances dated strictly after
// today. Past and today's rows are kept as a historical record.
//
// "Still planned" covers more than status 'expected': a future status day
// (planned sickness, excusal, class trip) has already rewritten such a row to
// 'absent', and slot-list Plan/Abgleich reads load every instance row regardless
// of status — eligibleOn filters on the enrollment interval, not on alumnus — so
// a row left behind keeps the departed child visible and counted in future
// exports. Only rows recording an actual event (observed presence or a stamped
// check-in/checkout) survive (#405 review).
func (s *RosterReconciler) RemoveStudentsFromFutureRosters(ctx context.Context, studentIDs []int64) error {
	if len(studentIDs) == 0 {
		return nil
	}
	after := timezone.TodayDate()
	removed, err := s.instanceStudentRepo.DeletePlannedByStudentIDsAfter(ctx, studentIDs, after)
	if err != nil {
		return &ScheduleError{Op: "reconcile roster: remove graduated students", Err: err}
	}
	s.getLogger().Info("reconciled future rosters after graduation",
		slog.Int("students", len(studentIDs)),
		slog.Int("rows_removed", removed),
	)
	return nil
}

// RestoreStudentsToFutureRosters re-adds the given (now reactivated) students to
// future planned template-backed instances their enrollment still covers but
// which were materialized while they were alumni. Existing rows are never
// duplicated (the UNIQUE (instance_id, student_id) constraint and the in-memory
// dedup both guard it), and only enrollment-valid (instance, student) pairs are
// inserted — reusing the same validity predicate the materializer applies.
//
// Restored rows go in as 'expected' and are then handed to
// ApplyActiveStatusDaysForInstance, exactly as the materializer does after
// copying enrollments: a child whose future date already carries an active
// sickness / excusal / class-trip status day must come back as absent for that
// date, not as expected (#405 review).
func (s *RosterReconciler) RestoreStudentsToFutureRosters(ctx context.Context, studentIDs []int64) error {
	if len(studentIDs) == 0 {
		return nil
	}
	after := timezone.TodayDate()

	instances, err := s.instanceRepo.FindFuturePlannedTemplateBacked(ctx, after)
	if err != nil {
		return &ScheduleError{Op: "reconcile roster: load future instances", Err: err}
	}
	if len(instances) == 0 {
		return nil
	}

	byGroup := make(map[int64][]*schedule.ActivityInstance)
	instanceIDs := make([]int64, 0, len(instances))
	for _, inst := range instances {
		if inst.ActivityGroupID == nil {
			continue // defensive: the query already excludes NULL group IDs
		}
		byGroup[*inst.ActivityGroupID] = append(byGroup[*inst.ActivityGroupID], inst)
		instanceIDs = append(instanceIDs, inst.ID)
	}

	existingRows, err := s.instanceStudentRepo.FindByInstanceIDs(ctx, instanceIDs)
	if err != nil {
		return &ScheduleError{Op: "reconcile roster: load existing rows", Err: err}
	}
	existing := make(map[instanceStudentPair]struct{}, len(existingRows))
	for _, row := range existingRows {
		existing[instanceStudentPair{instanceID: row.InstanceID, studentID: row.StudentID}] = struct{}{}
	}

	restored := 0
	// Instances that received at least one restored row, so the status-day pass
	// below runs once per instance instead of once per inserted row.
	touched := make(map[int64]timezone.Date)
	for _, sid := range studentIDs {
		enrollments, err := s.enrollmentRepo.FindByStudentID(ctx, sid)
		if err != nil {
			return &ScheduleError{Op: "reconcile roster: load enrollments", Err: err}
		}
		for _, e := range enrollments {
			for _, inst := range byGroup[e.ActivityGroupID] {
				periodID := int64(0)
				if inst.CalendarPeriodID != nil {
					periodID = *inst.CalendarPeriodID
				}
				if !isEnrollmentValidOn(e, inst.Date, periodID) {
					continue
				}
				key := instanceStudentPair{instanceID: inst.ID, studentID: sid}
				if _, seen := existing[key]; seen {
					continue
				}
				row := &schedule.InstanceStudent{
					InstanceID: inst.ID,
					StudentID:  sid,
					Status:     schedule.AttendanceStatusExpected,
				}
				if err := s.instanceStudentRepo.Create(ctx, row); err != nil {
					return &ScheduleError{Op: "reconcile roster: restore student", Err: err}
				}
				existing[key] = struct{}{}
				touched[inst.ID] = inst.Date
				restored++
			}
		}
	}

	// Reapply broad day statuses to the rows just inserted. The row went in as
	// 'expected'; if the restored child has an active (non-cleared) status day on
	// that date the row must read absent + the matching substatus instead. This
	// mirrors the materializer's post-copy pass — without it a revert shows a
	// sick / excused / class-trip child as expected for a future date, because
	// the original materialization skipped the row entirely while they were an
	// alumnus and never got the chance to stamp the status day (#405 review).
	statusApplied := 0
	for instanceID, date := range touched {
		n, err := s.instanceStudentRepo.ApplyActiveStatusDaysForInstance(ctx, instanceID, date)
		if err != nil {
			return &ScheduleError{Op: "reconcile roster: apply student status days", Err: err}
		}
		statusApplied += n
	}

	s.getLogger().Info("reconciled future rosters after revert",
		slog.Int("students", len(studentIDs)),
		slog.Int("rows_restored", restored),
		slog.Int("status_days_applied", statusApplied),
	)
	return nil
}

// instanceStudentPair keys the existing-row set so a (instance, student) is
// inserted at most once even when several enrollments of the same group are
// valid on the same instance date.
type instanceStudentPair struct {
	instanceID int64
	studentID  int64
}
