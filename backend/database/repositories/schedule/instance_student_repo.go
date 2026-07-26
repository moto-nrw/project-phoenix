package schedule

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

const (
	tableInstanceStudents   = "schedule.instance_students"
	aliasInstanceStudent    = "instance_student"
	modelTblInstanceStudent = `schedule.instance_students AS "instance_student"`
)

// InstanceStudentRepository implements schedule.InstanceStudentRepository.
type InstanceStudentRepository struct {
	*base.Repository[*schedule.InstanceStudent]
	db *bun.DB
}

// NewInstanceStudentRepository creates a new InstanceStudentRepository.
func NewInstanceStudentRepository(db *bun.DB) schedule.InstanceStudentRepository {
	repo := base.NewRepository[*schedule.InstanceStudent](db, tableInstanceStudents, "InstanceStudent")
	repo.TenantScoped = true
	return &InstanceStudentRepository{
		Repository: repo,
		db:         db,
	}
}

// FindByID overrides the base method to ensure schema-qualified queries.
func (r *InstanceStudentRepository) FindByID(ctx context.Context, id any) (*schedule.InstanceStudent, error) {
	var row schedule.InstanceStudent
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&row).
		ModelTableExpr(modelTblInstanceStudent).
		Where(`"instance_student".id = ?`, id)

	query = base.WithTenantFilter(ctx, query, aliasInstanceStudent)

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  opFindByID,
			Err: err,
		}
	}
	return &row, nil
}

// List retrieves instance student rows matching the provided query options.
func (r *InstanceStudentRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*schedule.InstanceStudent, error) {
	return r.ListWithOptions(ctx, options)
}

// FindByInstanceID returns all attendance rows for an instance.
func (r *InstanceStudentRepository) FindByInstanceID(ctx context.Context, instanceID int64) ([]*schedule.InstanceStudent, error) {
	var rows []*schedule.InstanceStudent
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(modelTblInstanceStudent).
		Where(`"instance_student".instance_id = ?`, instanceID).
		Order(orderCreatedAtASC)

	query = base.WithTenantFilter(ctx, query, aliasInstanceStudent)

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by instance id",
			Err: err,
		}
	}
	return rows, nil
}

// FindByInstanceIDs returns every attendance row for any of the given
// instance IDs in one query (all statuses). Tenant-scoped; empty input
// returns an empty slice without hitting the DB. Bulk sibling of
// FindByInstanceID for callers that would otherwise loop per instance
// (#1565 list options).
func (r *InstanceStudentRepository) FindByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*schedule.InstanceStudent, error) {
	if len(instanceIDs) == 0 {
		return []*schedule.InstanceStudent{}, nil
	}
	var rows []*schedule.InstanceStudent
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(modelTblInstanceStudent).
		Where(`"instance_student".instance_id IN (?)`, bun.List(instanceIDs)).
		OrderExpr(`"instance_student".instance_id ASC, "instance_student".student_id ASC`)

	query = base.WithTenantFilter(ctx, query, aliasInstanceStudent)

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by instance ids",
			Err: err,
		}
	}
	return rows, nil
}

// FindExpectedByInstanceIDs returns every instance_students row with
// status='expected' for any of the given instance IDs. Tenant-scoped.
// Empty input returns an empty slice without hitting the DB, matching the
// sibling bulk helpers (see CountNonAbsentByInstanceIDs in instance_staff_repo).
func (r *InstanceStudentRepository) FindExpectedByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*schedule.InstanceStudent, error) {
	if len(instanceIDs) == 0 {
		return []*schedule.InstanceStudent{}, nil
	}
	var rows []*schedule.InstanceStudent
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(modelTblInstanceStudent).
		Where(`"instance_student".instance_id IN (?)`, bun.List(instanceIDs)).
		Where(`"instance_student".status = ?`, schedule.AttendanceStatusExpected).
		OrderExpr(`"instance_student".instance_id ASC, "instance_student".student_id ASC`)

	query = base.WithTenantFilter(ctx, query, aliasInstanceStudent)

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find expected by instance ids",
			Err: err,
		}
	}
	return rows, nil
}

// FindNotScheduledCandidatesByInstanceIDs implements
// schedule.InstanceStudentRepository: every row ending a block may still
// resolve — 'expected', or 'absent' while a broad day status still owns it.
//
// The WHERE mirrors MarkNotScheduled's row predicate on purpose: the session-end
// bridge feeds this result straight into that write, so a shape the write can
// change must not be missing here, and a shape it refuses to touch — a
// hand-decided row — must not be in here either. Reading only 'expected' rows
// hid children whose day status had already stamped a false absence on them
// (#1747).
func (r *InstanceStudentRepository) FindNotScheduledCandidatesByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*schedule.InstanceStudent, error) {
	if len(instanceIDs) == 0 {
		return []*schedule.InstanceStudent{}, nil
	}
	var rows []*schedule.InstanceStudent
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(modelTblInstanceStudent).
		Where(`"instance_student".instance_id IN (?)`, bun.List(instanceIDs)).
		Where(`"instance_student".manual_status_at IS NULL`).
		WhereGroup(" AND ", func(group *bun.SelectQuery) *bun.SelectQuery {
			return group.
				WhereOr(`"instance_student".status = ?`, schedule.AttendanceStatusExpected).
				WhereOr(`("instance_student".status = ? AND "instance_student".student_status_day_id IS NOT NULL)`,
					schedule.AttendanceStatusAbsent)
		}).
		OrderExpr(`"instance_student".instance_id ASC, "instance_student".student_id ASC`)

	query = base.WithTenantFilter(ctx, query, aliasInstanceStudent)

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find not scheduled candidates by instance ids",
			Err: err,
		}
	}
	return rows, nil
}

// CountNonAbsentByInstanceIDs groups instance_students by instance_id and
// returns the count of rows with status != 'absent' per instance. One query
// with GROUP BY, mirroring InstanceStaffRepository.CountNonAbsentByInstanceIDs
// (instance_staff_repo.go). Instances with zero non-absent rows do not appear
// in the returned map — callers must treat missing keys as zero.
func (r *InstanceStudentRepository) CountNonAbsentByInstanceIDs(ctx context.Context, instanceIDs []int64) (map[int64]int, error) {
	if len(instanceIDs) == 0 {
		return map[int64]int{}, nil
	}

	var rows []struct {
		InstanceID int64 `bun:"instance_id"`
		Cnt        int   `bun:"cnt"`
	}
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(modelTblInstanceStudent).
		ColumnExpr(`"instance_student".instance_id AS instance_id`).
		ColumnExpr(`COUNT(*)::int AS cnt`).
		Where(`"instance_student".instance_id IN (?)`, bun.List(instanceIDs)).
		Where(`"instance_student".status != ?`, schedule.AttendanceStatusAbsent).
		GroupExpr(`"instance_student".instance_id`)

	query = base.WithTenantFilter(ctx, query, aliasInstanceStudent)

	if err := query.Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "count non-absent by instance ids",
			Err: err,
		}
	}

	out := make(map[int64]int, len(rows))
	for _, row := range rows {
		out[row.InstanceID] = row.Cnt
	}
	return out, nil
}

// FindByStudentAndDateRange returns attendance rows for a student across all
// instances whose date falls within the inclusive range.
func (r *InstanceStudentRepository) FindByStudentAndDateRange(ctx context.Context, studentID int64, from, to timezone.Date) ([]*schedule.InstanceStudent, error) {
	var rows []*schedule.InstanceStudent
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(modelTblInstanceStudent).
		Join(`INNER JOIN schedule.activity_instances AS "activity_instance" ON "activity_instance".id = "instance_student".instance_id`).
		Where(`"instance_student".student_id = ?`, studentID).
		Where(`"activity_instance".date >= ?`, from).
		Where(`"activity_instance".date <= ?`, to).
		OrderExpr(`"activity_instance".date ASC, "activity_instance".start_time ASC`)

	query = base.WithTenantFilter(ctx, query, aliasInstanceStudent)

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by student and date range",
			Err: err,
		}
	}
	return rows, nil
}

// FindByInstanceAndStudent returns a single attendance row, or nil if the
// student is not expected at the instance.
func (r *InstanceStudentRepository) FindByInstanceAndStudent(ctx context.Context, instanceID, studentID int64) (*schedule.InstanceStudent, error) {
	var row schedule.InstanceStudent
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&row).
		ModelTableExpr(modelTblInstanceStudent).
		Where(`"instance_student".instance_id = ?`, instanceID).
		Where(`"instance_student".student_id = ?`, studentID)

	query = base.WithTenantFilter(ctx, query, aliasInstanceStudent)

	err := query.Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{
			Op:  "find by instance and student",
			Err: err,
		}
	}
	return &row, nil
}

// UpdateAttendanceFromCheckin flips an expected row, an absence owned by a
// broad day status, or a previously checked-out present row to observed open
// presence. Independent manual slot decisions and already-open present rows
// have no matching predicate and are never clobbered here.
//
// Reopening a checked-out present row re-stamps checked_in_at with the new
// check-in: (checked_in_at, checked_out_at) always describes the CURRENT
// presence interval. This is the session boundary that lets
// UpdateAttendanceCheckout reject superseded checkouts — a delayed checkout
// timestamped before the re-entry no longer closes the reopened slot.
//
// Returns (updated=true) when exactly one row was modified. A zero-rows
// result means either (a) the row doesn't exist in this tenant, or
// (b) it already moved out of 'expected' between the caller's read and
// this UPDATE — a benign race the mirror service handles by preserving
// the existing state.
func (r *InstanceStudentRepository) UpdateAttendanceFromCheckin(
	ctx context.Context, instanceID, studentID int64, checkedInAt time.Time,
) (bool, error) {
	q := base.GetDB(ctx, r.db).NewUpdate().
		Model((*schedule.InstanceStudent)(nil)).
		ModelTableExpr(modelTblInstanceStudent).
		Set(`status = ?`, schedule.AttendanceStatusPresent).
		Set(`substatus = CASE WHEN "instance_student".student_status_day_id IS NOT NULL THEN NULL ELSE "instance_student".substatus END`).
		Set(`student_status_day_id = NULL`).
		Set(`checked_in_at = CASE
			WHEN "instance_student".checked_out_at IS NOT NULL THEN ?
			ELSE COALESCE("instance_student".checked_in_at, ?) END`, checkedInAt, checkedInAt).
		Set(`checked_out_at = NULL`).
		Set(`updated_at = ?`, time.Now().UTC()).
		Where(`"instance_student".instance_id = ?`, instanceID).
		Where(`"instance_student".student_id = ?`, studentID).
		Where(`(
			"instance_student".status = ?
			OR "instance_student".student_status_day_id IS NOT NULL
			OR ("instance_student".status = ? AND "instance_student".checked_out_at IS NOT NULL)
		)`, schedule.AttendanceStatusExpected, schedule.AttendanceStatusPresent)

	q = base.WithTenantFilter(ctx, q, aliasInstanceStudent)

	res, err := q.Exec(ctx)
	if err != nil {
		return false, &modelBase.DatabaseError{
			Op:  "update attendance from checkin",
			Err: err,
		}
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (r *InstanceStudentRepository) CreateUnplannedPresentIfAbsent(
	ctx context.Context, instanceID, studentID int64, checkedInAt time.Time,
) (*schedule.InstanceStudent, error) {
	if _, err := base.GetDB(ctx, r.db).NewRaw(`
		INSERT INTO schedule.instance_students AS attendance
			(tenant_id, instance_id, student_id, status, checked_in_at, is_unplanned)
		VALUES (?, ?, ?, ?, ?, TRUE)
		ON CONFLICT (instance_id, student_id) DO UPDATE
		SET status = EXCLUDED.status,
			substatus = CASE
				WHEN attendance.student_status_day_id IS NOT NULL THEN NULL
				ELSE attendance.substatus
			END,
			student_status_day_id = NULL,
			checked_in_at = CASE
				WHEN attendance.checked_out_at IS NOT NULL THEN EXCLUDED.checked_in_at
				ELSE COALESCE(attendance.checked_in_at, EXCLUDED.checked_in_at)
			END,
			checked_out_at = NULL,
			updated_at = EXCLUDED.updated_at
		WHERE attendance.status = ?
			OR attendance.student_status_day_id IS NOT NULL
			OR (attendance.status = ? AND attendance.checked_out_at IS NOT NULL)
	`, tenant.FromContext(ctx), instanceID, studentID, schedule.AttendanceStatusPresent, checkedInAt,
		schedule.AttendanceStatusExpected, schedule.AttendanceStatusPresent).Exec(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "create unplanned slot attendance", Err: err}
	}
	return r.FindByInstanceAndStudent(ctx, instanceID, studentID)
}

func (r *InstanceStudentRepository) UpdateAttendanceCheckout(
	ctx context.Context, instanceID, studentID int64, checkedOutAt time.Time,
) error {
	q := base.GetDB(ctx, r.db).NewUpdate().
		Model((*schedule.InstanceStudent)(nil)).
		ModelTableExpr(modelTblInstanceStudent).
		Set(`checked_out_at = CASE
			WHEN "instance_student".checked_out_at IS NULL OR "instance_student".checked_out_at < ? THEN ?
			ELSE "instance_student".checked_out_at END`, checkedOutAt, checkedOutAt).
		Set(`updated_at = ?`, time.Now().UTC()).
		Where(`"instance_student".instance_id = ?`, instanceID).
		Where(`"instance_student".student_id = ?`, studentID).
		Where(`"instance_student".status = ?`, schedule.AttendanceStatusPresent).
		Where(`"instance_student".checked_in_at IS NOT NULL`).
		Where(`"instance_student".checked_in_at <= ?`, checkedOutAt)
	q = base.WithTenantFilter(ctx, q, aliasInstanceStudent)
	if _, err := q.Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "update slot attendance checkout", Err: err}
	}
	return nil
}

func (r *InstanceStudentRepository) ReconcileAttendanceInterval(
	ctx context.Context,
	instanceID, studentID int64,
	previousCheckIn time.Time,
	previousCheckOut *time.Time,
	updatedCheckIn time.Time,
	updatedCheckOut *time.Time,
) (bool, error) {
	q := base.GetDB(ctx, r.db).NewUpdate().
		Model((*schedule.InstanceStudent)(nil)).
		ModelTableExpr(modelTblInstanceStudent).
		Set(`checked_in_at = ?`, updatedCheckIn).
		Set(`checked_out_at = ?`, updatedCheckOut).
		Set(`updated_at = ?`, time.Now().UTC()).
		Where(`"instance_student".instance_id = ?`, instanceID).
		Where(`"instance_student".student_id = ?`, studentID).
		Where(`"instance_student".status = ?`, schedule.AttendanceStatusPresent).
		Where(`"instance_student".checked_in_at = ?`, previousCheckIn).
		Where(`(
			"instance_student".checked_out_at IS NOT DISTINCT FROM ?
			OR (? AND "instance_student".checked_out_at IS NULL)
		)`, previousCheckOut, previousCheckOut != nil)
	q = base.WithTenantFilter(ctx, q, aliasInstanceStudent)
	res, err := q.Exec(ctx)
	if err != nil {
		return false, &modelBase.DatabaseError{Op: "reconcile slot attendance interval", Err: err}
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (r *InstanceStudentRepository) FindCurrentCandidates(
	ctx context.Context, studentID int64, date timezone.Date, at time.Time,
) ([]*schedule.InstanceStudent, error) {
	var rows []*schedule.InstanceStudent
	clock := at.In(timezone.Berlin).Format("15:04:05")
	q := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(modelTblInstanceStudent).
		Join(`JOIN schedule.activity_instances AS "activity_instance" ON "activity_instance".id = "instance_student".instance_id AND "activity_instance".tenant_id = "instance_student".tenant_id`).
		Where(`"instance_student".student_id = ?`, studentID).
		Where(`"activity_instance".date = ?`, date).
		Where(`"activity_instance".status IN (?, ?)`, schedule.InstanceStatusPlanned, schedule.InstanceStatusActive).
		Where(`"activity_instance".start_time <= ?::time`, clock).
		Where(`"activity_instance".end_time > ?::time`, clock).
		OrderExpr(`"activity_instance".start_time ASC, "activity_instance".id ASC`)
	q = base.WithTenantFilter(ctx, q, aliasInstanceStudent)
	if err := q.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find current student slot candidates", Err: err}
	}
	return rows, nil
}

// ApplyStatusDay stamps a broad day status (sick / excused / class trip) onto
// the student's slots for that date.
//
// Rows carrying the #1747 non-booking marker are skipped: the child was not
// booked into care on that day, so there is no attendance to excuse. Writing
// 'absent' there would record a missed day of care that was never owed — the
// exact misclaim ending the block deliberately avoided.
func (r *InstanceStudentRepository) ApplyStatusDay(
	ctx context.Context, studentID int64, date timezone.Date, statusDayID int64, substatus string,
) (int, error) {
	res, err := base.GetDB(ctx, r.db).NewRaw(`
		WITH incoming AS (
			SELECT candidate.id
			FROM active.student_status_days AS candidate
			WHERE candidate.tenant_id = ?
				AND candidate.id = ?
				AND candidate.student_id = ?
				AND candidate.date = ?
				AND candidate.cleared_at IS NULL
				AND NOT EXISTS (
					SELECT 1
					FROM active.student_status_days AS newer
					WHERE newer.tenant_id = candidate.tenant_id
						AND newer.student_id = candidate.student_id
						AND newer.date = candidate.date
						AND newer.cleared_at IS NULL
						AND (newer.reported_at, newer.id) > (candidate.reported_at, candidate.id)
				)
		)
		UPDATE schedule.instance_students AS attendance
		SET status = ?, substatus = ?, student_status_day_id = ?, updated_at = ?
		FROM schedule.activity_instances AS instance, incoming
		WHERE attendance.tenant_id = ?
			AND attendance.student_id = ?
			AND NOT attendance.not_scheduled
			AND (attendance.status = ? OR attendance.student_status_day_id IS NOT NULL)
			AND instance.id = attendance.instance_id
			AND instance.tenant_id = attendance.tenant_id
			AND instance.date = ?
			AND instance.status <> ?
	`, tenant.FromContext(ctx), statusDayID, studentID, date,
		schedule.AttendanceStatusAbsent, substatus, statusDayID, time.Now().UTC(),
		tenant.FromContext(ctx), studentID, schedule.AttendanceStatusExpected, date, schedule.InstanceStatusCancelled).Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "apply student status day to slots", Err: err}
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

func (r *InstanceStudentRepository) ReleaseStatusDay(ctx context.Context, statusDayID int64) (int, error) {
	res, err := base.GetDB(ctx, r.db).NewRaw(`
		WITH released AS (
			SELECT tenant_id, student_id, date
			FROM active.student_status_days
			WHERE tenant_id = ? AND id = ?
		), replacement AS (
			SELECT released.student_id, latest.id, latest.status
			FROM released
			LEFT JOIN LATERAL (
				SELECT candidate.id, candidate.status
				FROM active.student_status_days AS candidate
				WHERE candidate.tenant_id = released.tenant_id
					AND candidate.student_id = released.student_id
					AND candidate.date = released.date
					AND candidate.cleared_at IS NULL
				ORDER BY candidate.reported_at DESC, candidate.id DESC
				LIMIT 1
			) AS latest ON TRUE
		)
		UPDATE schedule.instance_students AS attendance
		SET status = CASE
				WHEN replacement.id IS NOT NULL THEN ?
				WHEN instance.status = ? THEN ?
				ELSE ?
			END,
			substatus = CASE replacement.status
				WHEN 'sick' THEN ?
				WHEN 'excused' THEN ?
				WHEN 'class_trip' THEN ?
				ELSE NULL
			END,
			student_status_day_id = replacement.id,
			updated_at = ?
		FROM schedule.activity_instances AS instance, replacement
		WHERE attendance.tenant_id = ?
			AND attendance.student_status_day_id = ?
			AND attendance.student_id = replacement.student_id
			AND instance.id = attendance.instance_id
			AND instance.tenant_id = attendance.tenant_id
	`, tenant.FromContext(ctx), statusDayID,
		schedule.AttendanceStatusAbsent,
		schedule.InstanceStatusCompleted, schedule.AttendanceStatusAbsent, schedule.AttendanceStatusExpected,
		schedule.AttendanceSubstatusSick, schedule.AttendanceSubstatusExcused,
		schedule.AttendanceSubstatusFieldTrip,
		time.Now().UTC(), tenant.FromContext(ctx), statusDayID).Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "release student status day from slots", Err: err}
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// ApplyActiveStatusDaysForInstance restores broad day-status provenance after
// materialization or re-planning created fresh expected rows. The latest
// active report wins if corrupt legacy data contains competing statuses.
// Marked non-bookings are skipped for the same reason as in ApplyStatusDay.
func (r *InstanceStudentRepository) ApplyActiveStatusDaysForInstance(
	ctx context.Context, instanceID int64, date timezone.Date,
) (int, error) {
	res, err := base.GetDB(ctx, r.db).NewRaw(`
		WITH latest_status AS (
			SELECT DISTINCT ON (student_id) id, student_id, status
			FROM active.student_status_days
			WHERE tenant_id = ? AND date = ? AND cleared_at IS NULL
			ORDER BY student_id, reported_at DESC, id DESC
		)
		UPDATE schedule.instance_students AS attendance
		SET status = ?,
			substatus = CASE latest_status.status
				WHEN 'sick' THEN ?
				WHEN 'excused' THEN ?
				WHEN 'class_trip' THEN ?
				ELSE NULL
			END,
			student_status_day_id = latest_status.id,
			updated_at = ?
		FROM latest_status
		WHERE attendance.tenant_id = ?
			AND attendance.instance_id = ?
			AND attendance.student_id = latest_status.student_id
			AND NOT attendance.not_scheduled
			AND attendance.status = ?
	`, tenant.FromContext(ctx), date, schedule.AttendanceStatusAbsent,
		schedule.AttendanceSubstatusSick, schedule.AttendanceSubstatusExcused,
		schedule.AttendanceSubstatusFieldTrip, time.Now().UTC(),
		tenant.FromContext(ctx), instanceID, schedule.AttendanceStatusExpected).Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "apply active status days to instance", Err: err}
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// UpdateAttendanceFields writes only the fields the patch carries. Pointer-nil
// (and the *Clear bools unset) means "do not touch that column". Substatus and
// Note support explicit NULL via the *Clear companion bools, since both
// columns are nullable in the schema.
//
// This is the PATCH-endpoint write — it overwrites whatever is there. The
// handler is responsible for cross-field validation (e.g. substatus-with-
// expected-status) before calling.
func (r *InstanceStudentRepository) UpdateAttendanceFields(
	ctx context.Context, id int64, patch schedule.AttendanceFieldPatch,
) error {
	if !patch.HasChanges() {
		// Defensive: the handler should reject this at 400. A repo no-op
		// here is safer than issuing an empty UPDATE and bumping updated_at.
		return nil
	}

	q := base.GetDB(ctx, r.db).NewUpdate().
		Model((*schedule.InstanceStudent)(nil)).
		ModelTableExpr(modelTblInstanceStudent).
		Where(`"instance_student".id = ?`, id)

	q = base.WithTenantFilter(ctx, q, aliasInstanceStudent)

	clearStatusDayProvenance := false
	if patch.Status != nil {
		q = q.Set(`status = ?`, *patch.Status)
		clearStatusDayProvenance = true
		// A human decided this row's status. Record that, and drop any
		// non-booking marker the completion had stamped: staff setting an
		// unbooked slot back to 'expected' is precisely the override the marker
		// must not survive, and ending the block later must not re-stamp it
		// (MarkNotScheduled skips rows carrying manual_status_at). Without both
		// writes the decision vanishes from the completed-instance views, the
		// child's history and the exports (#1747 review).
		q = q.Set(`manual_status_at = ?`, time.Now().UTC()).
			Set(`not_scheduled = FALSE`)
	}
	switch {
	case patch.SubstatusClear:
		q = q.Set(`substatus = NULL`)
		clearStatusDayProvenance = true
	case patch.Substatus != nil:
		q = q.Set(`substatus = ?`, *patch.Substatus)
		clearStatusDayProvenance = true
	}
	if clearStatusDayProvenance {
		q = q.Set(`student_status_day_id = NULL`)
	}
	switch {
	case patch.NoteClear:
		q = q.Set(`note = NULL`)
	case patch.Note != nil:
		q = q.Set(`note = ?`, *patch.Note)
	}
	q = q.Set(`updated_at = ?`, time.Now().UTC())

	if _, err := q.Exec(ctx); err != nil {
		return &modelBase.DatabaseError{
			Op:  "update attendance fields",
			Err: err,
		}
	}
	return nil
}

// BulkUpdateStatus flips every attendance row for instanceID whose current
// status equals fromStatus to toStatus. Returns the number of rows changed.
//
// Used by instance Complete() to mark remaining expected students as absent.
// The fromStatus predicate keeps the update idempotent and free of clobber:
// rows already moved past 'expected' (present via checkin, absent via PATCH)
// stay put.
func (r *InstanceStudentRepository) BulkUpdateStatus(
	ctx context.Context, instanceID int64, fromStatus, toStatus string, excludeStudentIDs []int64,
) (int, error) {
	q := base.GetDB(ctx, r.db).NewUpdate().
		Model((*schedule.InstanceStudent)(nil)).
		ModelTableExpr(modelTblInstanceStudent).
		Set(`status = ?`, toStatus).
		Set(`updated_at = ?`, time.Now().UTC()).
		Where(`"instance_student".instance_id = ?`, instanceID).
		Where(`"instance_student".status = ?`, fromStatus)

	if len(excludeStudentIDs) > 0 {
		q = q.Where(`"instance_student".student_id NOT IN (?)`, bun.List(excludeStudentIDs))
	}

	q = base.WithTenantFilter(ctx, q, aliasInstanceStudent)

	res, err := q.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "bulk update status",
			Err: err,
		}
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// MarkNotScheduled implements schedule.InstanceStudentRepository.
//
// One statement per pair rather than a composite IN: the list holds only the
// children a single ended block spared, and bun's non-deprecated placeholder
// helpers cannot render a tuple IN.
//
// Two row shapes are marked. A row still 'expected' is the ordinary case. A row
// already flipped to 'absent' by a broad day status (sick / excused / class
// trip) is the second: those statuses land on every expected row of the day —
// reported before the block ended, or replayed onto freshly materialized rows —
// and at that moment nothing knows yet that the child was not booked into care.
// Only ending the block resolves that, so the absence is undone here, together
// with the provenance that would otherwise let ReleaseStatusDay write it back.
// A child owed no care that day cannot be absent from it, excused or not.
//
// Everything else is left alone: a manual PATCH decision and an observed
// check-in must never be relabelled as a non-booking. A hand-set status is
// excluded by manual_status_at rather than by its value — staff can set an
// unbooked slot back to 'expected', which is otherwise the exact shape this
// write claims (#1747 review). Such a row stays a genuine expectation and takes
// the ordinary expected → absent path.
//
// A finished block is out of reach entirely. Both callers read the instance
// while it is still active and write immediately after, but only Complete()
// holds the day lock — the nightly bridge and the force-start path do not, so a
// second path can stamp the instance completed (or cancelled) in between. The
// marker exists precisely so a finished day stops changing; a write that lands
// after that flip would rewrite the history it was invented to freeze (#1747
// review). Stated as "not finished" rather than "still active" on purpose: the
// invariant is about frozen days, and MarkExpectedAbsentByActiveGroupIDs
// already carries the active-only predicate for the absence half.
func (r *InstanceStudentRepository) MarkNotScheduled(ctx context.Context, refs []schedule.StudentInstanceRef) error {
	if len(refs) == 0 {
		return nil
	}

	q := base.GetDB(ctx, r.db).NewUpdate().
		Model((*schedule.InstanceStudent)(nil)).
		ModelTableExpr(modelTblInstanceStudent).
		Set(`not_scheduled = TRUE`).
		Set(`status = ?`, schedule.AttendanceStatusExpected).
		Set(`substatus = CASE WHEN "instance_student".student_status_day_id IS NOT NULL THEN NULL ELSE "instance_student".substatus END`).
		Set(`student_status_day_id = NULL`).
		Set(`updated_at = ?`, time.Now().UTC()).
		Where(`"instance_student".manual_status_at IS NULL`).
		Where(`NOT EXISTS (
			SELECT 1
			FROM schedule.activity_instances AS "instance"
			WHERE "instance".id = "instance_student".instance_id
				AND "instance".status IN (?, ?)
		)`, schedule.InstanceStatusCompleted, schedule.InstanceStatusCancelled).
		WhereGroup(" AND ", func(group *bun.UpdateQuery) *bun.UpdateQuery {
			return group.
				WhereOr(`"instance_student".status = ?`, schedule.AttendanceStatusExpected).
				WhereOr(`("instance_student".status = ? AND "instance_student".student_status_day_id IS NOT NULL)`,
					schedule.AttendanceStatusAbsent)
		}).
		WhereGroup(" AND ", func(group *bun.UpdateQuery) *bun.UpdateQuery {
			for _, ref := range refs {
				group = group.WhereOr(`("instance_student".instance_id = ? AND "instance_student".student_id = ?)`,
					ref.InstanceID, ref.StudentID)
			}
			return group
		})

	q = base.WithTenantFilter(ctx, q, aliasInstanceStudent)

	if _, err := q.Exec(ctx); err != nil {
		return &modelBase.DatabaseError{
			Op:  "mark attendance rows not scheduled",
			Err: err,
		}
	}
	return nil
}

// DeleteByInstanceID removes all attendance rows for an instance.
func (r *InstanceStudentRepository) DeleteByInstanceID(ctx context.Context, instanceID int64) error {
	query := base.GetDB(ctx, r.db).NewDelete().
		Model((*schedule.InstanceStudent)(nil)).
		ModelTableExpr(modelTblInstanceStudent).
		Where(`"instance_student".instance_id = ?`, instanceID)

	query = base.WithTenantFilter(ctx, query, aliasInstanceStudent)

	_, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "delete by instance id",
			Err: err,
		}
	}
	return nil
}

// MarkExpectedAbsentByActiveGroupIDs flips status 'expected' → 'absent' for
// students on still-active instances bridged to the given active.groups.
// Custom method (backend-conventions Rule 2): the subquery join on
// activity_instances is not expressible through the generic filter shape.
// Used by the scheduler's daily session-end bridge.
func (r *InstanceStudentRepository) MarkExpectedAbsentByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, updatedAt time.Time, exclusions []schedule.StudentInstanceRef) error {
	if len(activeGroupIDs) == 0 {
		return nil
	}

	q := base.GetDB(ctx, r.db).NewUpdate().
		Model((*schedule.InstanceStudent)(nil)).
		ModelTableExpr(modelTblInstanceStudent).
		Set(`status = ?`, schedule.AttendanceStatusAbsent).
		Set(`updated_at = ?`, updatedAt).
		Where(`"instance_student".status = ?`, schedule.AttendanceStatusExpected).
		Where(`"instance_student".instance_id IN (
			SELECT "instance".id
			FROM schedule.activity_instances AS "instance"
			WHERE "instance".status = ?
				AND "instance".active_group_id IN (?)
		)`, schedule.InstanceStatusActive, bun.List(activeGroupIDs))

	// Per-pair, not per-student: a child not booked on one closed instance's
	// date may be genuinely expected on another instance the same run closes.
	// One AND'ed NOT per pair — the list is small (children spared per nightly
	// run), and bun's non-deprecated placeholder helpers cannot render a
	// composite-tuple NOT IN.
	for _, ex := range exclusions {
		q = q.Where(`NOT ("instance_student".instance_id = ? AND "instance_student".student_id = ?)`,
			ex.InstanceID, ex.StudentID)
	}

	q = base.WithTenantFilter(ctx, q, aliasInstanceStudent)

	if _, err := q.Exec(ctx); err != nil {
		return &modelBase.DatabaseError{
			Op:  "mark expected absent by active group ids",
			Err: err,
		}
	}
	return nil
}

// CloseOpenCheckoutsByActiveGroupIDs stamps checked_out_at on every open
// present row (checked in, not yet checked out) whose instance is bridged to
// one of the given active.groups. Mirrors the daily session-end bulk visit
// close (EndVisitsByActiveGroupIDs) into slot attendance so history and
// exports never show children as still checked in after the nightly cleanup.
// Custom method (backend-conventions Rule 2): the subquery join on
// activity_instances is not expressible through the generic filter shape.
func (r *InstanceStudentRepository) CloseOpenCheckoutsByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, checkedOutAt time.Time) (int, error) {
	if len(activeGroupIDs) == 0 {
		return 0, nil
	}

	q := base.GetDB(ctx, r.db).NewUpdate().
		Model((*schedule.InstanceStudent)(nil)).
		ModelTableExpr(modelTblInstanceStudent).
		Set(`checked_out_at = ?`, checkedOutAt).
		Set(`updated_at = ?`, time.Now().UTC()).
		Where(`"instance_student".status = ?`, schedule.AttendanceStatusPresent).
		Where(`"instance_student".checked_in_at IS NOT NULL`).
		Where(`"instance_student".checked_in_at <= ?`, checkedOutAt).
		Where(`"instance_student".checked_out_at IS NULL`).
		Where(`"instance_student".instance_id IN (
			SELECT "instance".id
			FROM schedule.activity_instances AS "instance"
			WHERE "instance".active_group_id IN (?)
		)`, bun.List(activeGroupIDs))

	q = base.WithTenantFilter(ctx, q, aliasInstanceStudent)

	res, err := q.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "close open checkouts by active group ids",
			Err: err,
		}
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// ListStudentInstanceRefsBefore returns (student_id, instance_id) pairs for
// attendance rows whose instance date is before the cutoff, ordered by
// student then instance. Custom projection (backend-conventions Rule 2): the
// join on activity_instances for the date predicate is not expressible
// through the generic filter shape. Feeds the per-student audit rows of the
// timetable retention cleanup.
func (r *InstanceStudentRepository) ListStudentInstanceRefsBefore(ctx context.Context, cutoff timezone.Date) ([]schedule.StudentInstanceRef, error) {
	var rows []schedule.StudentInstanceRef

	q := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`schedule.instance_students AS i_s`).
		ColumnExpr(`i_s.student_id AS student_id`).
		ColumnExpr(`i_s.instance_id AS instance_id`).
		Join(`JOIN schedule.activity_instances AS i ON i.id = i_s.instance_id`).
		Where(`i.date < ?`, cutoff).
		Order("i_s.student_id", "i_s.instance_id")

	q = base.WithTenantFilter(ctx, q, "i")

	if err := q.Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list student instance refs before",
			Err: err,
		}
	}
	return rows, nil
}
