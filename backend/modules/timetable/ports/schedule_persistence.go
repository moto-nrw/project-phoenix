package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type ActivityInstanceRecords interface {
	Create(context.Context, *schedulerecords.ActivityInstance) error
	FindByID(context.Context, int64) (*schedulerecords.ActivityInstance, error)
	Update(context.Context, *schedulerecords.ActivityInstance) error
	Delete(context.Context, int64) error

	// CreateTemplateBackedIfAbsent inserts a template-backed instance and
	// returns inserted=false when the unique template slot already exists.
	CreateTemplateBackedIfAbsent(ctx context.Context, instance *schedulerecords.ActivityInstance) (inserted bool, err error)

	// FindByTenantAndDate returns all instances for the current tenant on the given date.
	FindByTenantAndDate(ctx context.Context, date calendar.Date) ([]*schedulerecords.ActivityInstance, error)

	// FindByTenantAndDateRange returns all instances within an inclusive date range.
	FindByTenantAndDateRange(ctx context.Context, from, to calendar.Date) ([]*schedulerecords.ActivityInstance, error)

	// FindByIDs returns the instances matching the given IDs in one
	// tenant-scoped IN query, ordered by date then start time. Empty input
	// returns an empty slice without hitting the DB, matching the sibling
	// bulk helpers. Used by the self-service assignment read (#1844) to load
	// only the instances a staff member is actually assigned to.
	FindByIDs(ctx context.Context, ids []int64) ([]*schedulerecords.ActivityInstance, error)

	// FindByActivityGroupAndDate returns instances for a specific template on a date.
	// There can be multiple rows when a template schedule defines several start
	// times on the same weekday.
	FindByActivityGroupAndDate(ctx context.Context, activityGroupID int64, date calendar.Date) ([]*schedulerecords.ActivityInstance, error)

	// FindByActivityGroupAndDateRange returns one template's instances within
	// an inclusive date range in one tenant-scoped query.
	FindByActivityGroupAndDateRange(ctx context.Context, activityGroupID int64, from, to calendar.Date) ([]*schedulerecords.ActivityInstance, error)

	// FindByActiveGroupID returns the instance that is currently bridged to the
	// given active.group, or nil if none.
	FindByActiveGroupID(ctx context.Context, activeGroupID int64) (*schedulerecords.ActivityInstance, error)

	// FindPlannedTemplateBackedFrom returns every planned instance dated on or
	// after `from` that the MATERIALIZER produced (activity_group_id and
	// calendar_period_id both set, is_spontaneous false), tenant-scoped and
	// ordered by date then start time. Used to reconcile already-materialized
	// rosters when a reverted grade transition restores students the
	// insert-only materializer had skipped while they were alumni (#405
	// review).
	//
	// Hand-created blocks are excluded even when they link a template for its
	// metadata: their roster is the list of students the planner submitted, so
	// refilling it from the template's enrollments would add children nobody
	// assigned. Only the materializer stamps calendar_period_id (#405 review).
	//
	// The bound is INCLUSIVE of `from` (today, at revert time). An instance
	// materialized after the apply but dated today has no archive row — the
	// materializer skipped the alumnus outright — so excluding the boundary date
	// would leave a same-day apply-then-revert child permanently missing from
	// today's roster with nothing left to repair it. Today's instances that
	// already started or finished are excluded by the planned-status filter.
	FindPlannedTemplateBackedFrom(ctx context.Context, from calendar.Date) ([]*schedulerecords.ActivityInstance, error)

	// MaxID returns the highest instance id currently visible to this tenant, or
	// 0 when it has none. It is an ORDERING MARKER, not a count: a grade
	// transition records it while applying so its revert can tell instances that
	// already existed from instances materialized afterwards, during the alumnus
	// window. created_at cannot answer that — it defaults to the transaction
	// start time, so a materialization that started earlier and then blocked on
	// the tenant transition lock backdates rows it inserts after the apply
	// commits. Sequence ids are assigned at INSERT and do reflect that order
	// (#405 review).
	MaxID(ctx context.Context) (int64, error)

	// MarkCompleted updates only lifecycle columns. Do not use a full-row
	// Update for DB-loaded instances because SQL TIME columns do not round-trip
	// safely through Bun.
	MarkCompleted(ctx context.Context, instanceID int64, completedAt time.Time) error

	// CompleteActiveByActiveGroupIDs marks every still-active instance bridged
	// to one of the given active.groups as completed and returns the number of
	// rows changed. Used by the scheduler's daily session-end bridge and by
	// kiosk/session end. This path is outside the five-minute reopen window:
	// it does not write completed_by, reopen_until, or a completion snapshot,
	// so CanReopenInstance stays false.
	CompleteActiveByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, completedAt time.Time) (int64, error)

	// DeletePlannedNonSpontaneousInWindow removes still-planned,
	// template-backed instances in the inclusive [from, to] window and
	// returns the number of rows deleted. A nil `to` leaves the window
	// open-ended (everything from `from` onward) — the template split uses
	// that so no planned old-template instance can survive past the
	// effective date regardless of the materialization window.
	// activityGroupID narrows the delete to one template's instances; nil
	// deletes across all templates. Used by ReplanWeek and the template
	// split (WP-B3). preserveDeviations keeps Vertretungsplan overrides
	// (#1840): true for re-plan, false for the destructive template
	// split/end series operation — see the implementation for why.
	DeletePlannedNonSpontaneousInWindow(ctx context.Context, from calendar.Date, to *calendar.Date, activityGroupID *int64, preserveDeviations bool) (int64, error)

	// PropagateListKindToFutureInstances re-classifies future template-backed
	// planned instances of one template whose list_kind still equals the
	// series' previous value (NULL and '' treated alike), returning the number
	// of rows changed. It carries a series Listenart edit onto already
	// materialized future occurrences so the classified daily lists (#1565)
	// reflect it without a manual re-plan, while leaving today/past rows,
	// non-planned/spontaneous rows, and per-occurrence classification overrides
	// untouched. `after` is today; only rows dated strictly after it change.
	PropagateListKindToFutureInstances(ctx context.Context, activityGroupID int64, previousKind, newKind *string, after calendar.Date) (int64, error)

	// UpdateColumns is the generic partial-update helper promoted from the
	// embedded base repository: updates only the named columns by primary
	// key. Lifecycle transitions use it because SQL TIME columns do not
	// round-trip safely through a full-row Update.
	UpdateColumns(ctx context.Context, instance *schedulerecords.ActivityInstance, columns ...string) (int64, error)

	// Generic query helpers promoted from the embedded base repository.
	// Used by the timetable retention cleanup.
	OldestBefore(ctx context.Context, dateColumn string, cutoff *calendar.Date) (*calendar.Date, error)
	DeleteOlderThan(ctx context.Context, dateColumn string, cutoff calendar.Date) (int64, error)
}

type ActivityRecoveryRecords interface {
	// CompletionTimestamp returns PostgreSQL's stable time for the current
	// transaction. It is the baseline for distinguishing completion writes from
	// later attendance and supervisor edits during reopen.
	CompletionTimestamp(ctx context.Context) (time.Time, error)
	// LockOpenVisits takes FOR UPDATE locks on every still-open visit of the
	// group so completion can snapshot the same rows EndActivitySession closes.
	LockOpenVisits(ctx context.Context, activeGroupID int64) error
	// LockOpenSupervisors locks every still-open supervisor of the group so
	// completion snapshots the same rows EndActivitySession closes.
	LockOpenSupervisors(ctx context.Context, activeGroupID int64) error
	// LockSupervisors locks the snapshot supervisor rows during reopen so a
	// concurrent staffing change cannot hide from the unchanged-row check.
	LockSupervisors(ctx context.Context, supervisorIDs []int64) error
	// LockAttendance locks every instance_students row of the instance so
	// complete/cancel serialize with attendance PATCH and reopen can refuse
	// a restore after a post-completion attendance edit.
	LockAttendance(ctx context.Context, instanceID int64) error
	Restore(ctx context.Context, instanceID int64, snapshot schedulerecords.ActivityCompletionSnapshot, now time.Time) error
}

type CalendarPeriodRecords interface {
	Create(context.Context, *schedulerecords.CalendarPeriod) error
	FindByID(context.Context, int64) (*schedulerecords.CalendarPeriod, error)
	Update(context.Context, *schedulerecords.CalendarPeriod) error
	Delete(context.Context, int64) error

	// FindByTenantID finds all calendar periods for a tenant
	FindByTenantID(ctx context.Context) ([]*schedulerecords.CalendarPeriod, error)

	// FindActiveByTenantID finds all active calendar periods for a tenant
	FindActiveByTenantID(ctx context.Context) ([]*schedulerecords.CalendarPeriod, error)

	// FindByName finds a calendar period by name within the current tenant
	FindByName(ctx context.Context, name string) (*schedulerecords.CalendarPeriod, error)

	// CreateIfAbsent inserts the period unless a row with the same
	// (tenant_id, name) already exists. It reports whether the insert
	// happened (false = a period with that name was already present).
	CreateIfAbsent(ctx context.Context, p *schedulerecords.CalendarPeriod) (bool, error)

	// FindActiveOverlapping returns all active periods of the current tenant
	// whose [start_date, end_date] range overlaps [start, end] (inclusive on
	// both ends), excluding the period with excludeID.
	FindActiveOverlapping(ctx context.Context, start, end calendar.Date, excludeID int64) ([]*schedulerecords.CalendarPeriod, error)

	// FindActiveOverlappingByType behaves like FindActiveOverlapping but only
	// considers active periods of the given period_type. It backs the hard
	// same-type overlap rejection; the untyped variant stays advisory.
	FindActiveOverlappingByType(ctx context.Context, periodType string, start, end calendar.Date, excludeID int64) ([]*schedulerecords.CalendarPeriod, error)

	// UsageCounts returns, per calendar period of the current tenant, how many
	// rows reference it through nullable calendar_period_id FKs. Periods without
	// references are omitted from the map.
	UsageCounts(ctx context.Context) (map[int64]schedulerecords.CalendarPeriodUsage, error)
}

type CareScheduleChangeRequestRecords interface {
	Create(ctx context.Context, req *schedulerecords.CareScheduleChangeRequest) error
	// FindByID takes `any` to match the embedded generic base.Repository.
	FindByID(ctx context.Context, id int64) (*schedulerecords.CareScheduleChangeRequest, error)

	// Decide moves a pending row to approved/rejected/withdrawn, stamping
	// decision_reason, reviewed_by, reviewed_at and (for approvals)
	// applied_at. Withdrawals carry no reviewer stamp.
	Decide(ctx context.Context, id int64, newStatus string, reason *string, reviewedBy *int64, applied bool) error
}

type ClassArrivalExceptionRecords interface {
	// Upsert stores the exception of one class and date, replacing what was
	// there.
	Upsert(ctx context.Context, row *schedulerecords.ClassArrivalException) error
}

type ClosingDayRecords interface {
	Create(context.Context, *schedulerecords.ClosingDay) error
	FindByID(context.Context, int64) (*schedulerecords.ClosingDay, error)
	Update(context.Context, *schedulerecords.ClosingDay) error
	Delete(context.Context, int64) error

	// FindByTenantID finds all closing days for the current tenant,
	// ordered by start_date.
	FindByTenantID(ctx context.Context) ([]*schedulerecords.ClosingDay, error)
}

type DateframeRecords interface {
	Create(context.Context, *schedulerecords.Dateframe) error
	FindByID(context.Context, int64) (*schedulerecords.Dateframe, error)
	Update(context.Context, *schedulerecords.Dateframe) error
	Delete(context.Context, int64) error

	// FindByName finds a dateframe by its name
	FindByName(ctx context.Context, name string) (*schedulerecords.Dateframe, error)
}

type InstanceStaffRecords interface {
	Create(context.Context, *schedulerecords.InstanceStaff) error
	FindByID(context.Context, int64) (*schedulerecords.InstanceStaff, error)
	Update(context.Context, *schedulerecords.InstanceStaff) error
	Delete(context.Context, int64) error

	// UpdateColumns surfaces the embedded generic repository's single-column
	// update: the #1843 sick cascade stamps/releases sick_absence_id without
	// whole-model writes.
	UpdateColumns(ctx context.Context, row *schedulerecords.InstanceStaff, columns ...string) (int64, error)

	// FindByInstanceID returns all staff assignments for an instance.
	FindByInstanceID(ctx context.Context, instanceID int64) ([]*schedulerecords.InstanceStaff, error)

	// FindByInstanceIDs returns every instance_staff row for any of the
	// given instance IDs in a single IN query, tenant-scoped. Returns an
	// empty slice (not nil) when instanceIDs is empty, matching the sibling
	// bulk helpers (FindExpectedByInstanceIDs, CountNonAbsentByInstanceIDs).
	// Used by the planned-conflict probe to intersect requested staff with
	// staff already assigned to overlapping instances without N+1.
	FindByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*schedulerecords.InstanceStaff, error)

	// FindByStaffAndDate returns all staff assignments for the given staff
	// member across all instances on a given date. Used by the one-click
	// substitute flow (E6) and gap detection.
	FindByStaffAndDate(ctx context.Context, staffID int64, date calendar.Date) ([]*schedulerecords.InstanceStaff, error)

	// FindByStaffIDsAndDate batches the joined date lookup for several staff
	// members. Empty input returns an empty slice without querying.
	FindByStaffIDsAndDate(ctx context.Context, staffIDs []int64, date calendar.Date) ([]*schedulerecords.InstanceStaff, error)

	// FindByStaffAndDateRange returns the staff member's assignments across
	// all instances dated within [from, to] inclusive, ordered by instance
	// date then start time. Used by the self-service assignment read (#1844)
	// so its cost stays proportional to one staff member's plan, never the
	// whole tenant window.
	FindByStaffAndDateRange(ctx context.Context, staffID int64, from, to calendar.Date) ([]*schedulerecords.InstanceStaff, error)

	// CountNonAbsentByInstanceIDs returns, for each instance id, the number of
	// instance_staff rows with is_absent=false. Single GROUP BY query; callers
	// must treat absent instance ids in the returned map as zero. Empty input
	// returns an empty map without hitting the DB.
	CountNonAbsentByInstanceIDs(ctx context.Context, instanceIDs []int64) (map[int64]int, error)

	// DeleteByInstanceID removes all staff assignments for an instance. The
	// CASCADE on the FK also does this on instance deletion; this method exists
	// for the "re-plan week" flow where the instance is kept but repopulated.
	DeleteByInstanceID(ctx context.Context, instanceID int64) error
}

type InstanceStudentRecords interface {
	Create(context.Context, *schedulerecords.InstanceStudent) error
	FindByID(context.Context, int64) (*schedulerecords.InstanceStudent, error)
	Update(context.Context, *schedulerecords.InstanceStudent) error
	Delete(context.Context, int64) error

	// FindPresentInOtherActiveInstances returns, for the given students, rows
	// where the student is recorded status='present' in another instance
	// (id != excludeInstanceID) with status='active' on the given date,
	// tenant-scoped. Ordered by the other instance's start_time DESC, id DESC
	// so callers taking the first row per student get the latest running
	// block. Empty studentIDs returns an empty slice without hitting the DB.
	FindPresentInOtherActiveInstances(ctx context.Context, excludeInstanceID int64, date calendar.Date, studentIDs []int64) ([]schedulerecords.ParallelPresence, error)

	// FindByInstanceID returns all attendance rows for an instance.
	FindByInstanceID(ctx context.Context, instanceID int64) ([]*schedulerecords.InstanceStudent, error)

	// FindByInstanceIDs returns every attendance row (all statuses) for any
	// of the given instance IDs in one query, tenant-scoped. Empty input
	// returns an empty slice without hitting the DB (#1565 list options).
	FindByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*schedulerecords.InstanceStudent, error)

	// FindExpectedByInstanceIDs returns every instance_students row with
	// status='expected' for any of the given instance IDs, tenant-scoped.
	// Returns an empty slice (not nil) when instanceIDs is empty. Used by
	// the WP-B13 exception-conflict endpoint to resolve which students are
	// still flagged as expected on instances that have a cancellation or
	// modification exception attached.
	FindExpectedByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*schedulerecords.InstanceStudent, error)

	// FindNotScheduledCandidatesByInstanceIDs returns every row that ending a
	// block may still resolve: rows still 'expected', plus rows a broad day
	// status (sick / excused / class trip) already flipped to 'absent' and
	// still owns (student_status_day_id IS NOT NULL), excluding rows a human
	// decided by hand (manual_status_at). Tenant-scoped, empty slice for empty
	// input.
	//
	// It exists because 'expected' alone is not the candidate set. ApplyStatusDay
	// runs before anything knows whether the child was booked into care that
	// day, so a child who was never booked can already sit there as 'absent'.
	// The session-end bridge has to see those rows too, or MarkNotScheduled
	// never gets the chance to undo that false absence (#1747). The predicate
	// deliberately mirrors MarkNotScheduled's WHERE — a row this query omits is
	// a row that write could not have changed anyway.
	FindNotScheduledCandidatesByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*schedulerecords.InstanceStudent, error)

	// CountNonAbsentByInstanceIDs groups rows by instance_id and returns the
	// count of rows with status != 'absent' per instance, mirroring
	// InstanceStaffRepository.CountNonAbsentByInstanceIDs. Instances with
	// zero non-absent rows do not appear in the returned map - callers must
	// treat missing keys as zero. Feeds the Betreuungsplan capacity
	// computation (children count per block).
	CountNonAbsentByInstanceIDs(ctx context.Context, instanceIDs []int64) (map[int64]int, error)

	// FindByStudentAndDateRange returns attendance rows for a student across
	// all instances whose date falls in the inclusive range. Used by the
	// per-student day view (aggregation layer).
	FindByStudentAndDateRange(ctx context.Context, studentID int64, from, to calendar.Date) ([]*schedulerecords.InstanceStudent, error)

	// FindByStudentIDsAndDate is the multi-student form of
	// FindByStudentAndDateRange for a single day. One query for the whole
	// batch; empty input returns an empty slice without hitting the DB.
	FindByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date calendar.Date) ([]*schedulerecords.InstanceStudent, error)

	// FindCurrentCandidatesByStudentIDs is the multi-student form of
	// FindCurrentCandidates: every student's currently-running booked slots
	// in one query. Callers group per student and apply the same
	// exactly-one-candidate rule as the single-student path.
	FindCurrentCandidatesByStudentIDs(ctx context.Context, studentIDs []int64, date calendar.Date, at time.Time) ([]*schedulerecords.InstanceStudent, error)

	// UpdateAttendanceFromCheckinBatch applies UpdateAttendanceFromCheckin's
	// SET and guards to the given (instance, student) pairs in one statement
	// for one shared check-in instant. Guard-failing rows are skipped like
	// the single-row zero-rows race path.
	UpdateAttendanceFromCheckinBatch(ctx context.Context, keys []schedulerecords.InstanceStudentKey, checkedInAt time.Time) error

	// UpdateAttendanceCheckoutBatch applies UpdateAttendanceCheckout's SET
	// and guards to the given (instance, student) pairs in one statement for
	// one shared checkout instant.
	UpdateAttendanceCheckoutBatch(ctx context.Context, keys []schedulerecords.InstanceStudentKey, checkedOutAt time.Time) error

	// FindByInstanceAndStudent returns a single attendance row, or nil if the
	// student is not expected at the instance.
	FindByInstanceAndStudent(ctx context.Context, instanceID, studentID int64) (*schedulerecords.InstanceStudent, error)

	// DeleteByInstanceID removes all attendance rows for an instance.
	DeleteByInstanceID(ctx context.Context, instanceID int64) error

	// ArchivePlannedByStudentIDsFrom removes still-planned attendance rows for
	// the given students on non-cancelled instances dated on or after `from`,
	// snapshotting each removed row under `transitionID` so the transition's
	// revert can replay it verbatim.
	//
	// Past rows are kept as a historical record, and so is any row that records
	// an actual event — an observed presence, a stamped check-in/checkout, or a
	// hand-set status (manual_status_at) on an occurrence that has already
	// started, judged against `at`. Rows a status day rewrote to 'absent'
	// (planned sickness, excusal, class trip) ARE removed, and so are hand-set
	// rows on occurrences that have NOT started yet: every timetable and
	// slot-list reader loads instance rows irrespective of status, so leaving
	// either behind keeps a departed child visible and counted (#405 review).
	// The bound is inclusive so a graduation applied during the school day also
	// clears the child's still-planned rows on later blocks of that same day.
	//
	// Used when a grade transition graduates a cohort after the scheduler has
	// already materialized upcoming instances, so departed children stop
	// counting on current and future timetables and staffing ratios.
	// Tenant-scoped; returns the rows removed.
	ArchivePlannedByStudentIDsFrom(ctx context.Context, transitionID int64, studentIDs []int64, from calendar.Date, at time.Time) (int, error)

	// RestoreArchivedByTransition replays the rows
	// ArchivePlannedByStudentIDsFrom removed for `transitionID` (restricted to
	// the given students) and consumes the archive entries. This is what makes a
	// revert the exact inverse of the apply instead of a reconstruction from
	// enrollments, which would resurrect occurrences a supervisor had removed by
	// hand and could never recreate hand-added rows with no enrollment behind
	// them (#405 review).
	//
	// Only still-actionable instances are replayed — dated on or after `from`
	// (today at revert time) and neither completed nor cancelled. An alumnus
	// window spanning weeks leaves archived rows describing occurrences that are
	// now frozen history; re-inserting an expected/absent child there would
	// rewrite attendance nobody can still record. Those ledger entries are
	// consumed all the same, so a later re-apply starts from a clean snapshot
	// (#405 review). Structural fields come back verbatim and attendance state
	// is re-derived from today's status days, except for non-bookings and
	// hand-set statuses, which are replayed as archived. Tenant-scoped; returns
	// the rows restored.
	RestoreArchivedByTransition(ctx context.Context, transitionID int64, studentIDs []int64, from calendar.Date) (int, error)

	// UpdateAttendanceFromCheckin opens observed presence for an expected row,
	// a broad-status absence, or a checked-out present row. It stamps the first
	// checked_in_at and re-stamps it on re-entry; already-open presence
	// and independent manual decisions are never clobbered. Returns
	// (updated=true) only when a row was actually changed.
	UpdateAttendanceFromCheckin(ctx context.Context, instanceID, studentID int64, checkedInAt time.Time) (bool, error)
	CreateUnplannedPresentIfAbsent(ctx context.Context, instanceID, studentID int64, checkedInAt time.Time) (*schedulerecords.InstanceStudent, error)
	UpdateAttendanceCheckout(ctx context.Context, instanceID, studentID int64, checkedOutAt time.Time) error
	// ReconcileAttendanceInterval replaces the observed interval only when the
	// row still carries the caller's previous check-in and checkout. A missing
	// historical checkout may be repaired for a previously closed visit. The
	// check-in guard protects a later re-entry from an edit to an older visit.
	ReconcileAttendanceInterval(ctx context.Context, instanceID, studentID int64, previousCheckIn time.Time, previousCheckOut *time.Time, updatedCheckIn time.Time, updatedCheckOut *time.Time) (bool, error)
	FindCurrentCandidates(ctx context.Context, studentID int64, date calendar.Date, at time.Time) ([]*schedulerecords.InstanceStudent, error)
	ApplyStatusDay(ctx context.Context, studentID int64, date calendar.Date, statusDayID int64, substatus string) (int, error)
	ReleaseStatusDay(ctx context.Context, statusDayID int64) (int, error)
	ApplyActiveStatusDaysForInstance(ctx context.Context, instanceID int64, date calendar.Date) (int, error)
	ApplyPartialAbsence(ctx context.Context, pickupExceptionID int64) (int, error)
	ReleasePartialAbsence(ctx context.Context, pickupExceptionID int64) (int, error)
	ApplyActivePartialAbsencesForInstance(ctx context.Context, instanceID int64, date calendar.Date) (int, error)

	// UpdateAttendanceFields writes only the fields carried by the patch.
	// Callers (the PATCH handler) must validate cross-field invariants
	// BEFORE invoking — the repo does not re-check them.
	UpdateAttendanceFields(ctx context.Context, id int64, patch schedulerecords.AttendanceFieldPatch) error

	// BulkUpdateStatus flips every attendance row for the given instance whose
	// current status equals fromStatus to toStatus, and returns the number of
	// rows changed. Tenant-scoped via the caller's transaction context.
	//
	// Used by Complete() to mark remaining 'expected' students as 'absent'
	// when an instance ends (WP-B10 plan §6.2). Intentionally NOT used by
	// Cancel(): a cancelled instance never ran, so "absent" would be a
	// semantic misclaim.
	//
	// excludeStudentIDs is skipped by the update. Complete() passes the
	// children the care plan does not place here today (#1747): they were
	// never expected, so recording them absent would be the same kind of
	// misclaim — and it would leak into attendance statistics and exports.
	// Callers must stamp those same children via MarkNotScheduled, or the
	// spared rows carry no record of why they were spared.
	BulkUpdateStatus(ctx context.Context, instanceID int64, fromStatus, toStatus string, excludeStudentIDs []int64) (int, error)

	// MarkNotScheduled persists the #1747 non-booking marker on the given
	// (instance, student) rows: the care plan did not place the child in the
	// OGS on that instance's date, so ending the block wrote no absence for
	// them. Rows still 'expected' are stamped as they are. A row a broad day
	// status (sick / excused / class trip) already flipped to 'absent' is
	// stamped too and reset to 'expected', dropping substatus and status-day
	// provenance: those statuses are written before anything knows whether the
	// child was booked, and a child owed no care that day cannot be absent from
	// it. A child who was checked in keeps their outcome, and so does any row
	// carrying manual_status_at — staff can set an unbooked slot back to
	// 'expected', and stamping over that would claim they were never booked.
	//
	// Idempotent, tenant-scoped, and a no-op on an empty slice.
	MarkNotScheduled(ctx context.Context, refs []schedulerecords.StudentInstanceRef) error

	// FindInstancesWithAttendanceByStudentAndDateRange returns one row per
	// (instance_students, activity_instance) pair for the student across the
	// inclusive date range, sorted by date then start time. Tenant-scoped via
	// the caller's context. Single query — no N+1.
	//
	// Instances the student has no attendance row for (e.g. a spontaneous
	// instance they dropped into without being enrolled) are NOT included;
	// the handler layer enriches those via the visits-side lookup.
	//
	// Rows carrying the not_scheduled marker while still 'expected' are
	// excluded too: the care plan did not book the child that day (#1747), so
	// reporting them as expected attendance would claim care that never
	// existed. A marked row that somebody later checked in or decided by hand
	// stays visible — that decision outranks the marker. 'Expected' rows on
	// cancelled instances are never marked; the instance status carries that
	// story.
	FindInstancesWithAttendanceByStudentAndDateRange(ctx context.Context, studentID int64, from, to calendar.Date) ([]*schedulerecords.ScheduledInstanceRow, error)

	// HasPlannedSlotsInRange reports whether the tenant has at least one
	// planned slot assignment (is_unplanned = FALSE) on a non-cancelled
	// instance dated within the inclusive range. This is the tenant-level
	// "care plan is used" signal behind the attendance history's assignment
	// hints: a single student's slot rows cannot answer it (a student may
	// simply not be booked anywhere), and walk-in rows must not, because a
	// spontaneous drop-in also happens at schools that plan nothing.
	// Cancelled instances keep their assignment rows but count as no
	// evidence either — a cancelled-only occurrence was never a usable
	// slot. Tenant-scoped via the caller's context.
	HasPlannedSlotsInRange(ctx context.Context, from, to calendar.Date) (bool, error)

	// FindPlannedStudentIDsByDate returns unique student IDs that have a
	// non-cancelled materialized timetable row on the given date. Used by
	// student search's "kommt heute" heuristic as an additive planning signal.
	FindPlannedStudentIDsByDate(ctx context.Context, studentIDs []int64, date calendar.Date) ([]int64, error)

	// MarkExpectedAbsentByActiveGroupIDs flips status 'expected' → 'absent'
	// for students on still-active instances bridged to the given
	// active.groups. Used by the scheduler's daily session-end bridge.
	//
	// exclusions are (instance, student) pairs skipped by the update,
	// carrying the same rule as BulkUpdateStatus: a child the care plan does
	// not place there THAT day was never expected and must not be recorded
	// absent (#1747), and the same pairs must be stamped via
	// MarkNotScheduled. The pairs are per-instance on purpose — one nightly run
	// can close instances from several dates, and a child not booked on one
	// date may well be booked on another.
	MarkExpectedAbsentByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, updatedAt time.Time, exclusions []schedulerecords.StudentInstanceRef) error

	// CloseOpenCheckoutsByActiveGroupIDs stamps checked_out_at on open
	// present rows whose instance is bridged to the given active.groups.
	// Mirrors the daily session-end bulk visit close into slot attendance.
	CloseOpenCheckoutsByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, checkedOutAt time.Time) (int, error)

	// ListStudentInstanceRefsBefore returns (student_id, instance_id) pairs
	// for attendance rows whose instance date is before the cutoff, ordered
	// by student then instance. Custom projection (join on activity_instances)
	// the generic filter shape cannot express; feeds the per-student audit
	// rows of the timetable retention cleanup.
	ListStudentInstanceRefsBefore(ctx context.Context, cutoff calendar.Date) ([]schedulerecords.StudentInstanceRef, error)
}

type StaffNoticeRecords interface {
	Create(ctx context.Context, n *schedulerecords.StaffNotice) error
	Update(ctx context.Context, n *schedulerecords.StaffNotice) error
	FindByID(ctx context.Context, id int64) (*schedulerecords.StaffNotice, error)
	Delete(ctx context.Context, id int64) error
	// DeleteEndedBefore löscht die Hinweise, deren letzter gültiger Tag vor
	// cutoff liegt oder die vor dessen Berliner Mitternacht ausgeschaltet
	// wurden (#4045), und gibt ihre Zahl zurück.
	DeleteEndedBefore(ctx context.Context, cutoff calendar.Date) (int, error)
	// List gibt die Hinweise des Mandanten zurück, neueste zuerst.
	// includeInactive steuert, ob abgeschaltete Zeilen erscheinen.
	List(ctx context.Context, includeInactive bool) ([]*schedulerecords.StaffNotice, error)
	// ListValidOn gibt die aktiven Hinweise zurück, deren Zeitraum den Tag
	// enthält und deren Zielgruppe die Leserart einschließt. Wochentag und
	// Wochenmuster prüft der Service — die Datenbank grenzt nur grob ein.
	ListValidOn(ctx context.Context, date calendar.Date, reader string) ([]*schedulerecords.StaffNotice, error)
	// Acknowledge stempelt die Kenntnisnahme einer Person; ein zweiter Aufruf
	// ändert nichts.
	Acknowledge(ctx context.Context, noticeID, accountID int64) error
	// AcknowledgedAtFor gibt je Hinweis-Id den Zeitpunkt der eigenen
	// Kenntnisnahme zurück.
	AcknowledgedAtFor(ctx context.Context, accountID int64, noticeIDs []int64) (map[int64]time.Time, error)
	// AcknowledgedCounts gibt je Hinweis-Id die Zahl der Kenntnisnahmen zurück.
	AcknowledgedCounts(ctx context.Context, noticeIDs []int64) (map[int64]int, error)
	// Acknowledgements gibt alle Kenntnisnahmen eines Hinweises zurück,
	// neueste zuerst — ohne Namen, die löst der Service auf.
	Acknowledgements(ctx context.Context, noticeID int64) ([]*schedulerecords.StaffNoticeAck, error)
}

type StudentArrivalExceptionRecords interface {
	Create(context.Context, *schedulerecords.StudentArrivalException) error
	FindByID(context.Context, int64) (*schedulerecords.StudentArrivalException, error)
	Update(context.Context, *schedulerecords.StudentArrivalException) error

	// FindByStudentID finds all arrival exceptions for a student
	FindByStudentID(ctx context.Context, studentID int64) ([]*schedulerecords.StudentArrivalException, error)

	// FindUpcomingByStudentID finds upcoming arrival exceptions for a student (from today onwards)
	FindUpcomingByStudentID(ctx context.Context, studentID int64) ([]*schedulerecords.StudentArrivalException, error)

	// FindByStudentIDAndDate finds an arrival exception for a specific student and date
	FindByStudentIDAndDate(ctx context.Context, studentID int64, date calendar.Date) (*schedulerecords.StudentArrivalException, error)

	// FindByStudentIDsAndDate finds arrival exceptions for multiple students and a specific date (bulk query)
	FindByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date calendar.Date) ([]*schedulerecords.StudentArrivalException, error)

	// FindByStudentIDAndDateRange finds arrival exceptions for a student whose
	// exception_date falls within the inclusive [from, to] range, sorted by
	// date. Used by the timetable per-student week endpoint to pre-load all
	// exceptions in a single query.
	FindByStudentIDAndDateRange(ctx context.Context, studentID int64, from, to calendar.Date) ([]*schedulerecords.StudentArrivalException, error)

	// DeleteByStudentID deletes all arrival exceptions for a student
	DeleteByStudentID(ctx context.Context, studentID int64) error

	// DeletePastExceptions deletes all exceptions older than the given date
	DeletePastExceptions(ctx context.Context, beforeDate calendar.Date) (int64, error)
}

type StudentArrivalNoteRecords interface {
	Create(context.Context, *schedulerecords.StudentArrivalNote) error
	FindByID(context.Context, int64) (*schedulerecords.StudentArrivalNote, error)
	Update(context.Context, *schedulerecords.StudentArrivalNote) error

	// FindByStudentID finds all arrival notes for a student
	FindByStudentID(ctx context.Context, studentID int64) ([]*schedulerecords.StudentArrivalNote, error)

	// FindByStudentIDAndDate finds all arrival notes for a student on a specific date
	FindByStudentIDAndDate(ctx context.Context, studentID int64, date calendar.Date) ([]*schedulerecords.StudentArrivalNote, error)

	// FindByStudentIDsAndDate finds all arrival notes for multiple students on a specific date (bulk query)
	FindByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date calendar.Date) ([]*schedulerecords.StudentArrivalNote, error)

	// DeleteByStudentID deletes all arrival notes for a student
	DeleteByStudentID(ctx context.Context, studentID int64) error

	// DeletePastNotes deletes all notes older than the given date
	DeletePastNotes(ctx context.Context, beforeDate calendar.Date) (int64, error)
}

type StudentArrivalScheduleRecords interface {
	Create(context.Context, *schedulerecords.StudentArrivalSchedule) error
	FindByID(context.Context, int64) (*schedulerecords.StudentArrivalSchedule, error)
	Update(context.Context, *schedulerecords.StudentArrivalSchedule) error

	// FindByStudentID finds all arrival schedules for a student
	FindByStudentID(ctx context.Context, studentID int64) ([]*schedulerecords.StudentArrivalSchedule, error)

	// FindByStudentIDAndWeekday finds an arrival schedule for a specific student and weekday
	FindByStudentIDAndWeekday(ctx context.Context, studentID int64, weekday int) (*schedulerecords.StudentArrivalSchedule, error)

	// FindByStudentIDsAndWeekday finds arrival schedules for multiple students and a specific weekday (bulk query)
	FindByStudentIDsAndWeekday(ctx context.Context, studentIDs []int64, weekday int) ([]*schedulerecords.StudentArrivalSchedule, error)

	// UpsertSchedule creates or updates an arrival schedule for a student and weekday
	UpsertSchedule(ctx context.Context, schedule *schedulerecords.StudentArrivalSchedule) error

	// DeleteByStudentID deletes all arrival schedules for a student
	DeleteByStudentID(ctx context.Context, studentID int64) error
}

type StudentPickupExceptionRecords interface {
	Create(context.Context, *schedulerecords.StudentPickupException) error
	FindByID(context.Context, int64) (*schedulerecords.StudentPickupException, error)
	Update(context.Context, *schedulerecords.StudentPickupException) error

	// FindByStudentID finds all pickup exceptions for a student
	FindByStudentID(ctx context.Context, studentID int64) ([]*schedulerecords.StudentPickupException, error)

	// FindUpcomingByStudentID finds upcoming pickup exceptions for a student (from today onwards)
	FindUpcomingByStudentID(ctx context.Context, studentID int64) ([]*schedulerecords.StudentPickupException, error)

	// FindByStudentIDAndDate finds a pickup exception for a specific student and date
	FindByStudentIDAndDate(ctx context.Context, studentID int64, date calendar.Date) (*schedulerecords.StudentPickupException, error)

	// FindByStudentIDsAndDate finds pickup exceptions for multiple students and a specific date (bulk query)
	FindByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date calendar.Date) ([]*schedulerecords.StudentPickupException, error)

	// FindByStudentIDAndDateRange finds pickup exceptions for a student whose
	// exception_date falls within the inclusive [from, to] range, sorted by
	// date. Used by the timetable per-student week endpoint to pre-load all
	// exceptions in a single query.
	FindByStudentIDAndDateRange(ctx context.Context, studentID int64, from, to calendar.Date) ([]*schedulerecords.StudentPickupException, error)

	// DeleteByStudentID deletes all pickup exceptions for a student
	DeleteByStudentID(ctx context.Context, studentID int64) error

	// DeletePastExceptions deletes all exceptions older than the given date
	DeletePastExceptions(ctx context.Context, beforeDate calendar.Date) (int64, error)
}

type StudentPickupNoteRecords interface {
	Create(context.Context, *schedulerecords.StudentPickupNote) error
	FindByID(context.Context, int64) (*schedulerecords.StudentPickupNote, error)
	Update(context.Context, *schedulerecords.StudentPickupNote) error

	// FindByStudentID finds all pickup notes for a student
	FindByStudentID(ctx context.Context, studentID int64) ([]*schedulerecords.StudentPickupNote, error)

	// FindByStudentIDAndDate finds all pickup notes for a student on a specific date
	FindByStudentIDAndDate(ctx context.Context, studentID int64, date calendar.Date) ([]*schedulerecords.StudentPickupNote, error)

	// FindByStudentIDsAndDate finds all pickup notes for multiple students on a specific date (bulk query)
	FindByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date calendar.Date) ([]*schedulerecords.StudentPickupNote, error)

	// DeleteByStudentID deletes all pickup notes for a student
	DeleteByStudentID(ctx context.Context, studentID int64) error

	// DeletePastNotes deletes all notes older than the given date
	DeletePastNotes(ctx context.Context, beforeDate calendar.Date) (int64, error)
}

type StudentPickupScheduleRecords interface {
	Create(context.Context, *schedulerecords.StudentPickupSchedule) error
	FindByID(context.Context, int64) (*schedulerecords.StudentPickupSchedule, error)
	Update(context.Context, *schedulerecords.StudentPickupSchedule) error

	// FindByStudentID finds all pickup schedules for a student
	FindByStudentID(ctx context.Context, studentID int64) ([]*schedulerecords.StudentPickupSchedule, error)

	// FindByStudentIDAndWeekday finds a pickup schedule for a specific student and weekday
	FindByStudentIDAndWeekday(ctx context.Context, studentID int64, weekday int) (*schedulerecords.StudentPickupSchedule, error)

	// FindByStudentIDsAndWeekday finds pickup schedules for multiple students and a specific weekday (bulk query)
	FindByStudentIDsAndWeekday(ctx context.Context, studentIDs []int64, weekday int) ([]*schedulerecords.StudentPickupSchedule, error)

	// UpsertSchedule creates or updates a pickup schedule for a student and weekday
	UpsertSchedule(ctx context.Context, schedule *schedulerecords.StudentPickupSchedule) error

	// DeleteByStudentID deletes all pickup schedules for a student
	DeleteByStudentID(ctx context.Context, studentID int64) error
}

// InstanceListing is the bounded instance read used by lifecycle and session completion.
type InstanceListing interface {
	ListInstances(context.Context, LegacyInstanceFilter) ([]*schedulerecords.ActivityInstance, error)
}

// ParticipantListingFilter pages planned rows before narrowing their execution status.
type ParticipantListingFilter struct {
	IDs         []int64
	InstanceIDs []int64
	StudentIDs  []int64
	Status      *string
	Limit       int
	Offset      int
}
