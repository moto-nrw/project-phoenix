package schedule

import (
	"cmp"
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/schedule"
)

// RosterReconciler adjusts already-materialized timetable rosters when a grade
// transition changes a student's lifecycle status.
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
//
// The two directions are deliberately asymmetric in HOW they work:
//
//   - Apply archives every row it deletes, so the revert can replay exactly
//     those rows. Reconstructing a roster from enrollments is NOT the inverse of
//     the deletion — a child a supervisor had removed from one occurrence by
//     hand would come back, and a child hand-added to one occurrence without a
//     matching enrollment could never be recreated (#405 review).
//   - Only instances materialized DURING the alumnus window (inserted after the
//     transition was applied) are filled from enrollments, because those are
//     precisely the instances the materializer skipped the child on and for
//     which no archive entry can exist.
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
// given (now graduated) students on non-cancelled instances dated today or
// later, archiving every removed row under transitionID.
//
// "Still planned" covers more than status 'expected': a status day (planned
// sickness, excusal, class trip) has already rewritten such a row to 'absent',
// and slot-list Plan/Abgleich reads load every instance row regardless of
// status — eligibleOn filters on the enrollment interval, not on alumnus — so a
// row left behind keeps the departed child visible and counted in today's and
// future exports. The same holds for an attendance marker somebody set by hand
// on a block that has not started yet — a hand-set status OR a pre-marked
// 'present': nothing was observed, it is still a plan, and no reader filters
// alumni. Only rows recording an actual event survive — a stamped
// check-in/checkout, or a human-set status/presence on an occurrence that has
// already started — today's included (#405 review).
func (s *RosterReconciler) RemoveStudentsFromFutureRosters(ctx context.Context, transitionID int64, studentIDs []int64) error {
	if len(studentIDs) == 0 {
		return nil
	}
	// Inclusive of today: a graduation applied mid-day must also clear the
	// child's still-planned rows on the day's remaining blocks, which today's
	// slot-list reads would otherwise keep showing. `now` additionally separates
	// today's blocks that have already started (hand-set statuses there are
	// observations and stay) from the ones still ahead.
	now := timezone.Now()
	from := timezone.DateFromTime(now)
	removed, err := s.instanceStudentRepo.ArchivePlannedByStudentIDsFrom(ctx, transitionID, studentIDs, from, now)
	if err != nil {
		return &ScheduleError{Op: "reconcile roster: remove graduated students", Err: err}
	}
	s.getLogger().Info("reconciled rosters after graduation",
		slog.Int64("transition_id", transitionID),
		slog.Int("students", len(studentIDs)),
		slog.Int("rows_removed", removed),
	)
	return nil
}

// CurrentRosterBaseline returns the ordering marker an apply must record so its
// revert can tell instances that already existed from instances the materializer
// created afterwards, while the children were alumni. It is the highest instance
// id visible to the tenant inside the apply's transaction.
//
// A timestamp cannot serve as that marker: activity_instances.created_at
// defaults to current_timestamp, i.e. the TRANSACTION START time. A
// materialization transaction that began before the apply committed and then
// waited on the tenant grade-transition lock stamps rows it inserts afterwards
// with a pre-apply timestamp — and a created_at comparison would file exactly
// those instances under "existed before the transition", where neither the
// archive replay (no ledger row exists) nor the enrollment fill (excluded) puts
// the restored child back. Sequence ids are drawn at INSERT, after the lock is
// granted, so they order correctly (#405 review).
func (s *RosterReconciler) CurrentRosterBaseline(ctx context.Context) (int64, error) {
	maxID, err := s.instanceRepo.MaxID(ctx)
	if err != nil {
		return 0, &ScheduleError{Op: "reconcile roster: read instance baseline", Err: err}
	}
	return maxID, nil
}

// RestoreStudentsToFutureRosters undoes the apply's roster reconciliation for
// the given (now reactivated) students:
//
//  1. Every row the apply archived on a STILL-ACTIONABLE instance is replayed
//     with its structural fields intact — same note, room, unplanned /
//     non-booking / manual-status markers — while status, substatus and the
//     owning status day are re-derived from the day statuses active NOW, so a
//     sickness reported or cleared during the alumnus window wins over the
//     stale plan the archive captured (#405 review). This preserves
//     per-occurrence edits in both directions: a child deliberately removed
//     from one occurrence
//     before the transition is NOT resurrected (there is no archive entry for a
//     row that never existed), and a child hand-added to one occurrence without
//     an enrollment comes back. Archived rows whose occurrence has since become
//     past, completed or cancelled are dropped rather than replayed: that
//     attendance is frozen history now, and re-inserting an expected/absent
//     child into it would rewrite a day nobody can still observe (#405 review).
//
//  2. Instances materialized DURING the alumnus window (inserted after
//     baselineInstanceID, the apply's marker) get the enrollment-valid rows the
//     materializer skipped while the child was an alumnus, reusing the same
//     validity predicate the materializer applies. Those rows go in as
//     'expected' and are then handed to ApplyActiveStatusDaysForInstance exactly
//     as the materializer does after copying enrollments, so a child whose date
//     already carries an active sickness / excusal / class-trip status day comes
//     back as absent for that date rather than expected (#405 review).
//
// Instances that already existed when the transition was applied are NEVER
// reconstructed from enrollments — step 1 owns them.
//
// A nil baselineInstanceID means the apply recorded no marker (a transition
// applied before the column existed). Step 2 is then skipped entirely rather
// than guessed at: those applies removed nothing, so there is nothing to refill.
func (s *RosterReconciler) RestoreStudentsToFutureRosters(
	ctx context.Context, transitionID int64, studentIDs []int64, baselineInstanceID *int64,
) error {
	if len(studentIDs) == 0 {
		return nil
	}

	// Same "today" bounds both halves: the archive replay must not reach back
	// into instances that turned into history during the alumnus window, and the
	// enrollment fill starts at the same boundary date.
	from := timezone.TodayDate()

	replayed, err := s.instanceStudentRepo.RestoreArchivedByTransition(ctx, transitionID, studentIDs, from)
	if err != nil {
		return &ScheduleError{Op: "reconcile roster: replay archived rows", Err: err}
	}

	restored, statusApplied := 0, 0
	if baselineInstanceID != nil {
		restored, statusApplied, err = s.fillInstancesMaterializedDuringAlumnusWindow(ctx, studentIDs, from, *baselineInstanceID)
		if err != nil {
			return err
		}
	}

	s.getLogger().Info("reconciled rosters after revert",
		slog.Int64("transition_id", transitionID),
		slog.Int("students", len(studentIDs)),
		slog.Int("rows_replayed", replayed),
		slog.Int("rows_restored", restored),
		slog.Int("status_days_applied", statusApplied),
	)
	return nil
}

// fillInstancesMaterializedDuringAlumnusWindow adds enrollment-valid rows for
// the planned, materializer-produced instances dated on or after `from` whose id
// is above baselineInstanceID, i.e. the instances the materializer built while
// the students were alumni and therefore skipped. Hand-created blocks are out of
// scope even when they link a template — their roster was typed by a planner,
// not derived from enrollments (the repository filters them out).
// Existing rows are never duplicated
// (the UNIQUE (instance_id, student_id) constraint and the in-memory dedup both
// guard it). Returns the rows created and the status-day rows subsequently
// stamped.
//
// `from` is today, inclusively: an instance materialized after the apply and
// dated today has no archive entry to replay, so excluding the boundary date
// would leave a same-day apply-then-revert child off today's roster with nothing
// left to put them back (#405 review).
func (s *RosterReconciler) fillInstancesMaterializedDuringAlumnusWindow(
	ctx context.Context, studentIDs []int64, from timezone.Date, baselineInstanceID int64,
) (int, int, error) {
	all, err := s.instanceRepo.FindPlannedTemplateBackedFrom(ctx, from)
	if err != nil {
		return 0, 0, &ScheduleError{Op: "reconcile roster: load future instances", Err: err}
	}

	byGroup := make(map[int64][]*schedule.ActivityInstance)
	instanceIDs := make([]int64, 0, len(all))
	for _, inst := range all {
		if inst.ActivityGroupID == nil {
			continue // defensive: the query already excludes NULL group IDs
		}
		if inst.ID <= baselineInstanceID {
			// Existed before the transition was applied — the archive owns it.
			continue
		}
		byGroup[*inst.ActivityGroupID] = append(byGroup[*inst.ActivityGroupID], inst)
		instanceIDs = append(instanceIDs, inst.ID)
	}
	if len(instanceIDs) == 0 {
		return 0, 0, nil
	}

	existingRows, err := s.instanceStudentRepo.FindByInstanceIDs(ctx, instanceIDs)
	if err != nil {
		return 0, 0, &ScheduleError{Op: "reconcile roster: load existing rows", Err: err}
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
			return 0, 0, &ScheduleError{Op: "reconcile roster: load enrollments", Err: err}
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
					return 0, 0, &ScheduleError{Op: "reconcile roster: restore student", Err: err}
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
			return 0, 0, &ScheduleError{Op: "reconcile roster: apply student status days", Err: err}
		}
		statusApplied += n
	}

	return restored, statusApplied, nil
}

// instanceStudentPair keys the existing-row set so a (instance, student) is
// inserted at most once even when several enrollments of the same group are
// valid on the same instance date.
type instanceStudentPair struct {
	instanceID int64
	studentID  int64
}
