package schedule

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/careplanning"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

const (
	tableInstanceStudents   = "schedule.instance_students"
	aliasInstanceStudent    = "instance_student"
	modelTblInstanceStudent = `schedule.instance_students AS "instance_student"`
)

// InstanceStudentRepository implements schedule.InstanceStudentRepository.
type InstanceStudentRepository struct {
	*base.Repository[*schedule.InstanceStudent]
	db       *bun.DB
	students StudentDirectory
	rooms    RoomDirectory
}

// BindRoomDirectory installs the Facilities directory the roster restore
// re-validates room references through (#2665).
func (r *InstanceStudentRepository) BindRoomDirectory(rooms RoomDirectory) {
	r.rooms = rooms
}

// BindStudentDirectory installs the People Directory the partial-absence
// preview resolves the child's lifecycle status through (#2662).
func (r *InstanceStudentRepository) BindStudentDirectory(students StudentDirectory) {
	r.students = students
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
			Err: base.TranslateNotFound(err),
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
			Err: base.TranslateNotFound(err),
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
			Err: base.TranslateNotFound(err),
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
			Err: base.TranslateNotFound(err),
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
					schedule.AttendanceStatusAbsent).
				WhereOr(`("instance_student".status = ? AND "instance_student".pickup_exception_id IS NOT NULL)`,
					schedule.AttendanceStatusAbsent)
		}).
		OrderExpr(`"instance_student".instance_id ASC, "instance_student".student_id ASC`)

	query = base.WithTenantFilter(ctx, query, aliasInstanceStudent)

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find not scheduled candidates by instance ids",
			Err: base.TranslateNotFound(err),
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
			Err: base.TranslateNotFound(err),
		}
	}

	out := make(map[int64]int, len(rows))
	for _, row := range rows {
		out[row.InstanceID] = row.Cnt
	}
	return out, nil
}

// FindPresentInOtherActiveInstances returns, for the given students, rows
// where the student is recorded status='present' in another instance that is
// currently active on the given date (#2265 parallel-presence hint).
func (r *InstanceStudentRepository) FindPresentInOtherActiveInstances(ctx context.Context, excludeInstanceID int64, date timezone.Date, studentIDs []int64) ([]schedule.ParallelPresence, error) {
	if len(studentIDs) == 0 {
		return []schedule.ParallelPresence{}, nil
	}
	var rows []struct {
		StudentID  int64     `bun:"student_id"`
		InstanceID int64     `bun:"instance_id"`
		Title      string    `bun:"title"`
		StartTime  time.Time `bun:"start_time"`
		EndTime    time.Time `bun:"end_time"`
	}
	query := base.GetDB(ctx, r.db).NewSelect().
		ModelTableExpr(modelTblInstanceStudent).
		ColumnExpr(`"instance_student".student_id`).
		ColumnExpr(`"activity_instance".id AS instance_id`).
		ColumnExpr(`"activity_instance".title`).
		ColumnExpr(`"activity_instance".start_time`).
		ColumnExpr(`"activity_instance".end_time`).
		Join(`INNER JOIN schedule.activity_instances AS "activity_instance" ON "activity_instance".id = "instance_student".instance_id AND "activity_instance".tenant_id = "instance_student".tenant_id`).
		Where(`"instance_student".instance_id != ?`, excludeInstanceID).
		Where(`"instance_student".student_id IN (?)`, bun.List(studentIDs)).
		Where(`"instance_student".status = ?`, schedule.AttendanceStatusPresent).
		Where(`"instance_student".checked_out_at IS NULL`).
		Where(`"activity_instance".date = ?`, date).
		Where(`"activity_instance".status = ?`, schedule.InstanceStatusActive).
		OrderExpr(`"activity_instance".start_time DESC, "activity_instance".id DESC`)

	query = base.WithTenantFilter(ctx, query, aliasInstanceStudent)

	if err := query.Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find present in other active instances",
			Err: base.TranslateNotFound(err),
		}
	}
	out := make([]schedule.ParallelPresence, 0, len(rows))
	for _, row := range rows {
		out = append(out, schedule.ParallelPresence{
			StudentID:  row.StudentID,
			InstanceID: row.InstanceID,
			Title:      row.Title,
			StartTime:  row.StartTime,
			EndTime:    row.EndTime,
		})
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
			Err: base.TranslateNotFound(err),
		}
	}
	return rows, nil
}

// FindByStudentIDsAndDate is the multi-student form of
// FindByStudentAndDateRange for a single day: one query for the whole batch,
// so the roomless checkout mirror can resolve every student's slot rows
// without a per-student round trip (review #2372).
func (r *InstanceStudentRepository) FindByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date timezone.Date) ([]*schedule.InstanceStudent, error) {
	if len(studentIDs) == 0 {
		return []*schedule.InstanceStudent{}, nil
	}
	var rows []*schedule.InstanceStudent
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(modelTblInstanceStudent).
		Join(`INNER JOIN schedule.activity_instances AS "activity_instance" ON "activity_instance".id = "instance_student".instance_id`).
		Where(`"instance_student".student_id IN (?)`, bun.List(studentIDs)).
		Where(`"activity_instance".date = ?`, date).
		OrderExpr(`"instance_student".student_id ASC, "activity_instance".start_time ASC`)

	query = base.WithTenantFilter(ctx, query, aliasInstanceStudent)

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by student ids and date",
			Err: base.TranslateNotFound(err),
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
			Err: base.TranslateNotFound(err),
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
		Set(`substatus = CASE WHEN "instance_student".student_status_day_id IS NOT NULL OR "instance_student".pickup_exception_id IS NOT NULL THEN NULL ELSE "instance_student".substatus END`).
		Set(`student_status_day_id = NULL`).
		Set(`pickup_exception_id = NULL`).
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
				OR "instance_student".pickup_exception_id IS NOT NULL
			OR ("instance_student".status = ? AND "instance_student".checked_out_at IS NOT NULL)
		)`, schedule.AttendanceStatusExpected, schedule.AttendanceStatusPresent)

	q = base.WithTenantFilter(ctx, q, aliasInstanceStudent)

	res, err := q.Exec(ctx)
	if err != nil {
		return false, &modelBase.DatabaseError{
			Op:  "update attendance from checkin",
			Err: base.TranslateNotFound(err),
		}
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// UpdateAttendanceFromCheckinBatch is the multi-row form of
// UpdateAttendanceFromCheckin for one shared check-in instant: identical SET
// and guard predicates, restricted to the given (instance_id, student_id)
// pairs in ONE statement instead of a per-student UPDATE (review #2372).
// Rows whose guards no longer match are silently skipped, exactly like the
// single-row zero-rows race path.
func (r *InstanceStudentRepository) UpdateAttendanceFromCheckinBatch(
	ctx context.Context, keys []schedule.InstanceStudentKey, checkedInAt time.Time,
) error {
	if len(keys) == 0 {
		return nil
	}
	q := base.GetDB(ctx, r.db).NewUpdate().
		Model((*schedule.InstanceStudent)(nil)).
		ModelTableExpr(modelTblInstanceStudent).
		Set(`status = ?`, schedule.AttendanceStatusPresent).
		Set(`substatus = CASE WHEN "instance_student".student_status_day_id IS NOT NULL OR "instance_student".pickup_exception_id IS NOT NULL THEN NULL ELSE "instance_student".substatus END`).
		Set(`student_status_day_id = NULL`).
		Set(`pickup_exception_id = NULL`).
		Set(`checked_in_at = CASE
			WHEN "instance_student".checked_out_at IS NOT NULL THEN ?
			ELSE COALESCE("instance_student".checked_in_at, ?) END`, checkedInAt, checkedInAt).
		Set(`checked_out_at = NULL`).
		Set(`updated_at = ?`, time.Now().UTC()).
		Where(`("instance_student".instance_id, "instance_student".student_id) IN (?)`, bun.List(instanceStudentKeyTuples(keys))).
		Where(`(
				"instance_student".status = ?
				OR "instance_student".student_status_day_id IS NOT NULL
				OR "instance_student".pickup_exception_id IS NOT NULL
			OR ("instance_student".status = ? AND "instance_student".checked_out_at IS NOT NULL)
		)`, schedule.AttendanceStatusExpected, schedule.AttendanceStatusPresent)

	q = base.WithTenantFilter(ctx, q, aliasInstanceStudent)

	if _, err := q.Exec(ctx); err != nil {
		return &modelBase.DatabaseError{
			Op:  "update attendance from checkin batch",
			Err: base.TranslateNotFound(err),
		}
	}
	return nil
}

// UpdateAttendanceCheckoutBatch is the multi-row form of
// UpdateAttendanceCheckout for one shared checkout instant: identical SET and
// guard predicates over the given (instance_id, student_id) pairs in ONE
// statement (review #2372). Guarded rows that no longer match are skipped,
// like the single-row form's silent zero-rows outcome.
func (r *InstanceStudentRepository) UpdateAttendanceCheckoutBatch(
	ctx context.Context, keys []schedule.InstanceStudentKey, checkedOutAt time.Time,
) error {
	if len(keys) == 0 {
		return nil
	}
	q := base.GetDB(ctx, r.db).NewUpdate().
		Model((*schedule.InstanceStudent)(nil)).
		ModelTableExpr(modelTblInstanceStudent).
		Set(`checked_out_at = CASE
			WHEN "instance_student".checked_out_at IS NULL OR "instance_student".checked_out_at < ? THEN ?
			ELSE "instance_student".checked_out_at END`, checkedOutAt, checkedOutAt).
		Set(`updated_at = ?`, time.Now().UTC()).
		Where(`("instance_student".instance_id, "instance_student".student_id) IN (?)`, bun.List(instanceStudentKeyTuples(keys))).
		Where(`"instance_student".status = ?`, schedule.AttendanceStatusPresent).
		Where(`"instance_student".checked_in_at IS NOT NULL`).
		Where(`"instance_student".checked_in_at <= ?`, checkedOutAt)
	q = base.WithTenantFilter(ctx, q, aliasInstanceStudent)
	if _, err := q.Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "update slot attendance checkout batch", Err: base.TranslateNotFound(err)}
	}
	return nil
}

// instanceStudentKeyTuples renders keys as a bun.List of bun.Tuple values so
// the composite IN predicate emits ((instance_id, student_id), …).
func instanceStudentKeyTuples(keys []schedule.InstanceStudentKey) []any {
	tuples := make([]any, 0, len(keys))
	for _, key := range keys {
		tuples = append(tuples, bun.Tuple([]int64{key.InstanceID, key.StudentID}))
	}
	return tuples
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
					WHEN attendance.student_status_day_id IS NOT NULL OR attendance.pickup_exception_id IS NOT NULL THEN NULL
				ELSE attendance.substatus
			END,
				student_status_day_id = NULL,
				pickup_exception_id = NULL,
			checked_in_at = CASE
				WHEN attendance.checked_out_at IS NOT NULL THEN EXCLUDED.checked_in_at
				ELSE COALESCE(attendance.checked_in_at, EXCLUDED.checked_in_at)
			END,
			checked_out_at = NULL,
			updated_at = EXCLUDED.updated_at
			WHERE attendance.status = ?
				OR attendance.student_status_day_id IS NOT NULL
				OR attendance.pickup_exception_id IS NOT NULL
			OR (attendance.status = ? AND attendance.checked_out_at IS NOT NULL)
	`, tenant.FromContext(ctx), instanceID, studentID, schedule.AttendanceStatusPresent, checkedInAt,
		schedule.AttendanceStatusExpected, schedule.AttendanceStatusPresent).Exec(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "create unplanned slot attendance", Err: base.TranslateNotFound(err)}
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
		return &modelBase.DatabaseError{Op: "update slot attendance checkout", Err: base.TranslateNotFound(err)}
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
		return false, &modelBase.DatabaseError{Op: "reconcile slot attendance interval", Err: base.TranslateNotFound(err)}
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
		return nil, &modelBase.DatabaseError{Op: "find current student slot candidates", Err: base.TranslateNotFound(err)}
	}
	return rows, nil
}

// FindCurrentCandidatesByStudentIDs is the multi-student form of
// FindCurrentCandidates: one query resolves every student's currently-running
// booked slots so the batch check-in mirror needs no per-student round trip
// (review #2372). Callers group the rows per student and apply the same
// exactly-one-candidate rule as the single-student path.
func (r *InstanceStudentRepository) FindCurrentCandidatesByStudentIDs(
	ctx context.Context, studentIDs []int64, date timezone.Date, at time.Time,
) ([]*schedule.InstanceStudent, error) {
	if len(studentIDs) == 0 {
		return []*schedule.InstanceStudent{}, nil
	}
	var rows []*schedule.InstanceStudent
	clock := at.In(timezone.Berlin).Format("15:04:05")
	q := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(modelTblInstanceStudent).
		Join(`JOIN schedule.activity_instances AS "activity_instance" ON "activity_instance".id = "instance_student".instance_id AND "activity_instance".tenant_id = "instance_student".tenant_id`).
		Where(`"instance_student".student_id IN (?)`, bun.List(studentIDs)).
		Where(`"activity_instance".date = ?`, date).
		Where(`"activity_instance".status IN (?, ?)`, schedule.InstanceStatusPlanned, schedule.InstanceStatusActive).
		Where(`"activity_instance".start_time <= ?::time`, clock).
		Where(`"activity_instance".end_time > ?::time`, clock).
		OrderExpr(`"instance_student".student_id ASC, "activity_instance".start_time ASC, "activity_instance".id ASC`)
	q = base.WithTenantFilter(ctx, q, aliasInstanceStudent)
	if err := q.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find current student slot candidates batch", Err: base.TranslateNotFound(err)}
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
		return 0, &modelBase.DatabaseError{Op: "apply student status day to slots", Err: base.TranslateNotFound(err)}
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
		return 0, &modelBase.DatabaseError{Op: "release student status day from slots", Err: base.TranslateNotFound(err)}
	}
	n, _ := res.RowsAffected()

	// Replay any partial excusal for the released day (#2360): a broad day
	// status may coexist with an AUTO-derived partial absence (a pulled-forward
	// pickup time). The release above restores the status day's rows to
	// 'expected'; without this replay the blocks after the pickup cutoff would
	// stay expected even though the child is still picked up early. Mirrors
	// ApplyPartialAbsence keyed by (student, date) instead of exception id; a
	// no-op when no timed excusal exists for the day. Completed instances are
	// additionally excluded: the release above deliberately preserves them as
	// historical absent, and the replay must not rewrite that record to excused
	// with fresh pickup provenance (#2360 review).
	_, err = base.GetDB(ctx, r.db).NewRaw(`
		WITH released AS (
			SELECT tenant_id, student_id, date
			FROM active.student_status_days
			WHERE tenant_id = ? AND id = ?
		)
		UPDATE schedule.instance_students AS attendance
		SET status = ?,
			substatus = ?,
			student_status_day_id = NULL,
			pickup_exception_id = exc.id,
			updated_at = ?
		FROM schedule.activity_instances AS instance,
			released,
			schedule.student_pickup_exceptions AS exc
		WHERE exc.tenant_id = released.tenant_id
			AND exc.student_id = released.student_id
			AND exc.exception_date = released.date
			AND exc.excused_from IS NOT NULL
			AND attendance.tenant_id = exc.tenant_id
			AND attendance.student_id = exc.student_id
			AND attendance.manual_status_at IS NULL
			AND NOT attendance.not_scheduled
			AND (
				attendance.status = ?
				OR (
					attendance.status = ?
					AND attendance.pickup_exception_id IS NULL
					AND attendance.student_status_day_id IS NULL
				)
			)
			AND instance.id = attendance.instance_id
			AND instance.tenant_id = attendance.tenant_id
			AND instance.date = exc.exception_date
			AND instance.start_time >= exc.excused_from
			AND instance.status NOT IN (?, ?)
	`, tenant.FromContext(ctx), statusDayID,
		schedule.AttendanceStatusAbsent, schedule.AttendanceSubstatusExcused, time.Now().UTC(),
		schedule.AttendanceStatusExpected, schedule.AttendanceStatusAbsent,
		schedule.InstanceStatusCancelled, schedule.InstanceStatusCompleted).Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "replay partial absence after status day release", Err: base.TranslateNotFound(err)}
	}
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
		return 0, &modelBase.DatabaseError{Op: "apply active status days to instance", Err: base.TranslateNotFound(err)}
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// ApplyPartialAbsence marks only slots that start at or after the excused-from
// time. A slot already carrying actual or manual attendance is outside this
// write's ownership and remains untouched.
//
// The predicate also claims bare absences (status=absent, no provenance): the
// session-end bridge flips expected → absent without the shared care-day lock,
// so a concurrent partial write can otherwise persist the exception with no
// owned rows and leave ReleasePartialAbsence unable to reconcile them. The
// bridge only touches still-active instances; completed instances are a
// closed historical record and excluded here — the auto-excusal sync runs
// this projection on same-day pickup and weekly-baseline changes and must
// not rewrite what already happened (#2360 review).
func (r *InstanceStudentRepository) ApplyPartialAbsence(ctx context.Context, pickupExceptionID int64) (int, error) {
	res, err := base.GetDB(ctx, r.db).NewRaw(`
		WITH partial_absence AS (
			SELECT tenant_id, id, student_id, exception_date, excused_from
			FROM schedule.student_pickup_exceptions
			WHERE tenant_id = ? AND id = ? AND excused_from IS NOT NULL
		)
		UPDATE schedule.instance_students AS attendance
		SET status = ?,
			substatus = ?,
			student_status_day_id = NULL,
			pickup_exception_id = partial_absence.id,
			updated_at = ?
		FROM schedule.activity_instances AS instance, partial_absence
		WHERE attendance.tenant_id = partial_absence.tenant_id
			AND attendance.student_id = partial_absence.student_id
			AND attendance.manual_status_at IS NULL
			AND NOT attendance.not_scheduled
			AND (
				attendance.status = ?
				OR (
					attendance.status = ?
					AND attendance.pickup_exception_id IS NULL
					AND attendance.student_status_day_id IS NULL
				)
			)
			AND instance.id = attendance.instance_id
			AND instance.tenant_id = attendance.tenant_id
			AND instance.date = partial_absence.exception_date
			AND instance.start_time >= partial_absence.excused_from
			AND instance.status NOT IN (?, ?)
	`, tenant.FromContext(ctx), pickupExceptionID,
		schedule.AttendanceStatusAbsent, schedule.AttendanceSubstatusExcused, time.Now().UTC(),
		schedule.AttendanceStatusExpected, schedule.AttendanceStatusAbsent,
		schedule.InstanceStatusCancelled, schedule.InstanceStatusCompleted).Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "apply partial absence to slots", Err: base.TranslateNotFound(err)}
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// FindPartialAbsenceBlocks needs a custom query because the generic repository
// cannot join activity_instances or project its title and wall-clock window.
// It previews the actionable blocks that a partial absence would excuse.
// Template-backed instances can precede their instance_students rows; mirror
// materialization's enrollment predicate so those future rows are visible too.
// That predicate excludes graduates; the child's lifecycle status belongs to
// the People Directory (#2662) and is resolved before the query runs.
func (r *InstanceStudentRepository) FindPartialAbsenceBlocks(
	ctx context.Context, studentID int64, date timezone.Date, from time.Time,
) ([]schedule.PartialAbsenceBlock, error) {
	if r.students == nil {
		return nil, errStudentDirectoryRequired
	}
	students, err := r.students.ListStudentsByID(ctx, []int64{studentID})
	if err != nil {
		return nil, err
	}
	enrolled := false
	for _, student := range students {
		if student.ID == studentID && !student.Alumnus {
			enrolled = true
		}
	}
	rows := make([]schedule.PartialAbsenceBlock, 0)
	err = base.GetDB(ctx, r.db).NewRaw(`
		SELECT instance.id, instance.title, instance.start_time, instance.end_time
		FROM schedule.activity_instances AS instance
		WHERE instance.tenant_id = ?
			AND instance.date = ?
			AND instance.start_time >= ?
			AND instance.status NOT IN (?, ?)
			AND (
				EXISTS (
					SELECT 1
					FROM schedule.instance_students AS attendance
					WHERE attendance.tenant_id = instance.tenant_id
						AND attendance.instance_id = instance.id
						AND attendance.student_id = ?
						AND attendance.manual_status_at IS NULL
						AND NOT attendance.not_scheduled
						AND attendance.status IN (?, ?)
						AND attendance.student_status_day_id IS NULL
						AND (
							attendance.pickup_exception_id IS NULL
							OR EXISTS (
								SELECT 1
								FROM schedule.student_pickup_exceptions AS pickup_exception
								WHERE pickup_exception.tenant_id = attendance.tenant_id
									AND pickup_exception.id = attendance.pickup_exception_id
									AND pickup_exception.excused_auto
							)
						)
				)
				OR (
					NOT EXISTS (
						SELECT 1
						FROM schedule.instance_students AS attendance
						WHERE attendance.tenant_id = instance.tenant_id
							AND attendance.instance_id = instance.id
							AND attendance.student_id = ?
					)
					AND ?::boolean
					AND EXISTS (
						SELECT 1
						FROM activities.student_enrollments AS enrollment
						WHERE enrollment.tenant_id = instance.tenant_id
							AND enrollment.student_id = ?
							AND enrollment.activity_group_id = instance.activity_group_id
							AND enrollment.valid_from <= instance.date
							AND (enrollment.valid_until IS NULL OR enrollment.valid_until > instance.date)
							AND (enrollment.calendar_period_id IS NULL OR enrollment.calendar_period_id = instance.calendar_period_id)
							AND (enrollment.weekday IS NULL OR enrollment.weekday = EXTRACT(ISODOW FROM instance.date))
							AND (COALESCE(jsonb_array_length(enrollment.selected_weekdays), 0) = 0
								OR enrollment.selected_weekdays @> to_jsonb(ARRAY[EXTRACT(ISODOW FROM instance.date)::integer]))
					)
				)
			)
		ORDER BY instance.start_time ASC, instance.id ASC
	`, tenant.FromContext(ctx), date, timezone.NormalizeWallClock(from),
		schedule.InstanceStatusCancelled, schedule.InstanceStatusCompleted,
		studentID, schedule.AttendanceStatusExpected, schedule.AttendanceStatusAbsent,
		studentID, enrolled, studentID).Scan(ctx, &rows)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find partial absence blocks", Err: base.TranslateNotFound(err)}
	}
	return rows, nil
}

// ReleasePartialAbsence restores only rows still owned by this pickup
// exception. A broad active day status takes ownership; otherwise actionable
// blocks return to expected. Completed instances are excluded entirely — they
// are a closed historical record, and the apply side treats them as immutable
// for the same reason (#2360 review): the row keeps its excused substatus and
// pickup provenance instead of being rewritten by a later pickup-time change.
func (r *InstanceStudentRepository) ReleasePartialAbsence(ctx context.Context, pickupExceptionID int64) (int, error) {
	res, err := base.GetDB(ctx, r.db).NewRaw(`
		WITH released AS (
			SELECT tenant_id, student_id, exception_date
			FROM schedule.student_pickup_exceptions
			WHERE tenant_id = ? AND id = ?
		), replacement AS (
			SELECT released.student_id, latest.id, latest.status
			FROM released
			LEFT JOIN LATERAL (
				SELECT candidate.id, candidate.status
				FROM active.student_status_days AS candidate
				WHERE candidate.tenant_id = released.tenant_id
					AND candidate.student_id = released.student_id
					AND candidate.date = released.exception_date
					AND candidate.cleared_at IS NULL
				ORDER BY candidate.reported_at DESC, candidate.id DESC
				LIMIT 1
			) AS latest ON TRUE
		)
		UPDATE schedule.instance_students AS attendance
		SET status = CASE
				WHEN replacement.id IS NOT NULL THEN ?
				ELSE ?
			END,
			substatus = CASE replacement.status
				WHEN 'sick' THEN ?
				WHEN 'excused' THEN ?
				WHEN 'class_trip' THEN ?
				ELSE NULL
			END,
			student_status_day_id = replacement.id,
			pickup_exception_id = NULL,
			updated_at = ?
		FROM schedule.activity_instances AS instance, replacement
		WHERE attendance.tenant_id = ?
			AND attendance.pickup_exception_id = ?
			AND attendance.student_id = replacement.student_id
			AND instance.id = attendance.instance_id
			AND instance.tenant_id = attendance.tenant_id
			AND instance.status <> ?
	`, tenant.FromContext(ctx), pickupExceptionID,
		schedule.AttendanceStatusAbsent, schedule.AttendanceStatusExpected,
		schedule.AttendanceSubstatusSick, schedule.AttendanceSubstatusExcused,
		schedule.AttendanceSubstatusFieldTrip,
		time.Now().UTC(), tenant.FromContext(ctx), pickupExceptionID,
		schedule.InstanceStatusCompleted).Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "release partial absence from slots", Err: base.TranslateNotFound(err)}
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// ApplyActivePartialAbsencesForInstance mirrors the active status-day replay
// for attendance rows created by materialization or re-planning.
//
// Before projecting, every child with a time-specific excusal on this date is
// serialized with LockExceptionDay so concurrent create/update/delete of a
// partial absence cannot leave a freshly materialised row with stale
// provenance.
func (r *InstanceStudentRepository) ApplyActivePartialAbsencesForInstance(
	ctx context.Context, instanceID int64, date timezone.Date,
) (int, error) {
	var studentIDs []int64
	if err := base.GetDB(ctx, r.db).NewRaw(`
		SELECT DISTINCT partial_absence.student_id
		FROM schedule.student_pickup_exceptions AS partial_absence
		WHERE partial_absence.tenant_id = ?
			AND partial_absence.exception_date = ?
			AND partial_absence.excused_from IS NOT NULL
		ORDER BY partial_absence.student_id
	`, tenant.FromContext(ctx), date).Scan(ctx, &studentIDs); err != nil {
		return 0, &modelBase.DatabaseError{Op: "list active partial absences for lock", Err: base.TranslateNotFound(err)}
	}
	for _, studentID := range studentIDs {
		if err := careplanning.LockExceptionDay(ctx, r.db, studentID, date.String()); err != nil {
			return 0, err
		}
	}

	res, err := base.GetDB(ctx, r.db).NewRaw(`
		UPDATE schedule.instance_students AS attendance
		SET status = ?,
			substatus = ?,
			student_status_day_id = NULL,
			pickup_exception_id = partial_absence.id,
			updated_at = ?
		FROM schedule.activity_instances AS instance
		JOIN schedule.student_pickup_exceptions AS partial_absence
			ON partial_absence.tenant_id = instance.tenant_id
			AND partial_absence.exception_date = instance.date
			AND partial_absence.excused_from IS NOT NULL
		WHERE attendance.tenant_id = ?
			AND attendance.instance_id = ?
			AND attendance.instance_id = instance.id
			AND attendance.student_id = partial_absence.student_id
			AND attendance.manual_status_at IS NULL
			AND NOT attendance.not_scheduled
			AND (
				attendance.status = ?
				OR (
					attendance.status = ?
					AND attendance.pickup_exception_id IS NULL
					AND attendance.student_status_day_id IS NULL
				)
			)
			AND instance.date = ?
			AND instance.start_time >= partial_absence.excused_from
			AND instance.status <> ?
	`, schedule.AttendanceStatusAbsent, schedule.AttendanceSubstatusExcused, time.Now().UTC(),
		tenant.FromContext(ctx), instanceID,
		schedule.AttendanceStatusExpected, schedule.AttendanceStatusAbsent, date,
		schedule.InstanceStatusCancelled).Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "apply active partial absences to instance", Err: base.TranslateNotFound(err)}
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

	clearPlanProvenance := false
	recordManualDecision := false
	if patch.Status != nil {
		q = q.Set(`status = ?`, *patch.Status)
		clearPlanProvenance = true
		recordManualDecision = true
		// A human decided this row's status. Drop any non-booking marker the
		// completion had stamped: staff setting an unbooked slot back to
		// 'expected' is precisely the override the marker must not survive,
		// and ending the block later must not re-stamp it (MarkNotScheduled
		// skips rows carrying manual_status_at). Without both writes the
		// decision vanishes from the completed-instance views, the child's
		// history and the exports (#1747 review).
		q = q.Set(`not_scheduled = FALSE`)
	}
	switch {
	case patch.SubstatusClear:
		q = q.Set(`substatus = NULL`)
		clearPlanProvenance = true
		// Substatus-only staff edits also clear plan provenance; without a
		// manual stamp a later partial projection can reclaim the row and
		// overwrite the staff decision.
		recordManualDecision = true
	case patch.Substatus != nil:
		q = q.Set(`substatus = ?`, *patch.Substatus)
		clearPlanProvenance = true
		recordManualDecision = true
	}
	if clearPlanProvenance {
		q = q.Set(`student_status_day_id = NULL`)
		q = q.Set(`pickup_exception_id = NULL`)
	}
	if recordManualDecision {
		q = q.Set(`manual_status_at = ?`, time.Now().UTC())
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
			Err: base.TranslateNotFound(err),
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
			Err: base.TranslateNotFound(err),
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
		Set(`substatus = CASE WHEN "instance_student".student_status_day_id IS NOT NULL OR "instance_student".pickup_exception_id IS NOT NULL THEN NULL ELSE "instance_student".substatus END`).
		Set(`student_status_day_id = NULL`).
		Set(`pickup_exception_id = NULL`).
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
					schedule.AttendanceStatusAbsent).
				WhereOr(`("instance_student".status = ? AND "instance_student".pickup_exception_id IS NOT NULL)`,
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
			Err: base.TranslateNotFound(err),
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
			Err: base.TranslateNotFound(err),
		}
	}
	return nil
}

// ArchivePlannedByStudentIDsFrom removes the given students' still-planned
// attendance rows on non-cancelled instances dated on or after `from`, and
// snapshots every removed row into schedule.grade_transition_roster_removals
// under `transitionID` so the transition's revert can put them back verbatim.
//
// "Planned" is deliberately wider than status = 'expected': a future status day
// (planned sickness, excusal, class trip) rewrites the row to 'absent' with a
// student_status_day_id, and a graduated child left on such a row stays visible
// and counted in slot-list Plan/Abgleich reads and future exports, which load
// every instance row regardless of status. So the predicate excludes every row
// that records something that actually HAPPENED — a stamped check-in/checkout,
// or an attendance marker a human put there ('present', or any status finalized
// BY HAND via manual_status_at) on an occurrence that has already started or
// become history — plus every row on an instance that ran to completion (#405
// review).
//
// 'present' is NOT an unconditional exemption. The status alone does not prove
// an observation: every real check-in either stamps checked_in_at (the visit
// mirror, the unplanned-slot insert) or runs against an already-started
// instance (markPlannedStudentPresent stamps manual_status_at while the block
// is 'active'), and both are excluded by the clauses above. What a blanket
// `status <> 'present'` additionally kept was a presence somebody pre-marked on
// a block that has not started — a plan, treated below exactly like any other
// hand-set status (#405 review).
//
// The exclusions are not cosmetic. The revert deliberately refuses to replay
// archived rows into completed or past instances (that attendance is frozen
// history), while consuming their ledger entries — so anything this statement
// deletes there is gone for good. A hand-finalized absence on today's completed
// block is exactly such a row: recorded attendance, not a plan. Deleting it
// would erase a supervisor's observation permanently.
//
// A hand-set status on a block that has NOT started yet is the opposite case:
// nothing was observed, it is still a plan, and `at` is where the line is drawn.
// Exempting those rows too would leave the departed child on the roster of every
// future occurrence a supervisor happened to touch — visible in the timetable
// list and in slot-list/export reads, and counted for staffing on an 'expected'
// row, since none of those readers filter alumni. They are archived like any
// other plan, and the replay restores the hand-set status verbatim (#405
// review).
//
// The date bound is INCLUSIVE of `from` (today, for a graduation): slot-list
// eligibleOn decides visibility from the enrollment interval, not from alumnus
// status, so a still-planned row on a later block of the current day would keep
// a departed child in today's Plan/Abgleich lists and staffing counts. Rows that
// already recorded an event today are excluded by the predicate above and stay
// as the historical record (#405 review).
//
// Cross-table predicate (join on activity_instances for date/status) plus the
// archive write, so it is expressed as one data-modifying CTE in the repository
// rather than the generic builder. Tenant-scoped; a nil/empty student set is a
// no-op.
func (r *InstanceStudentRepository) ArchivePlannedByStudentIDsFrom(
	ctx context.Context, transitionID int64, studentIDs []int64, from timezone.Date, at time.Time,
) (int, error) {
	if len(studentIDs) == 0 {
		return 0, nil
	}

	today := timezone.DateFromTime(at)
	clock := at.In(timezone.Berlin).Format("15:04:05")

	const rawSQL = `
		WITH removed AS (
			DELETE FROM schedule.instance_students AS s
			USING schedule.activity_instances AS ai
			WHERE s.instance_id = ai.id
			  AND s.student_id IN (?)
			  AND s.checked_in_at IS NULL
			  AND s.checked_out_at IS NULL
			  AND (
			        (s.manual_status_at IS NULL AND s.status <> ?)
			        OR (
			              ai.status = ?
			              AND (ai.date > ? OR (ai.date = ? AND ai.start_time > ?::time))
			        )
			  )
			  AND ai.date >= ?
			  AND ai.status NOT IN (?, ?)
			  AND s.tenant_id = ?
			RETURNING s.*
		)
		INSERT INTO schedule.grade_transition_roster_removals (
			tenant_id, transition_id, instance_id, student_id, room_id, status,
			substatus, note, is_unplanned, not_scheduled, manual_status_at,
			student_status_day_id
		)
		SELECT removed.tenant_id, ?, removed.instance_id, removed.student_id,
		       removed.room_id, removed.status, removed.substatus, removed.note,
		       removed.is_unplanned, removed.not_scheduled, removed.manual_status_at,
		       removed.student_status_day_id
		FROM removed
		ON CONFLICT (transition_id, instance_id, student_id) DO UPDATE SET
			room_id               = EXCLUDED.room_id,
			status                = EXCLUDED.status,
			substatus             = EXCLUDED.substatus,
			note                  = EXCLUDED.note,
			is_unplanned          = EXCLUDED.is_unplanned,
			not_scheduled         = EXCLUDED.not_scheduled,
			manual_status_at      = EXCLUDED.manual_status_at,
			student_status_day_id = EXCLUDED.student_status_day_id,
			created_at            = NOW()`

	result, err := base.GetDB(ctx, r.db).ExecContext(ctx, rawSQL,
		bun.List(studentIDs),
		schedule.AttendanceStatusPresent,
		schedule.InstanceStatusPlanned,
		today,
		today,
		clock,
		from,
		schedule.InstanceStatusCompleted,
		schedule.InstanceStatusCancelled,
		tenant.FromContext(ctx),
		transitionID,
	)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "archive planned by student ids from",
			Err: base.TranslateNotFound(err),
		}
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "get rows affected",
			Err: base.TranslateNotFound(err),
		}
	}
	return int(affected), nil
}

// RestoreArchivedByTransition replays the rows ArchivePlannedByStudentIDsFrom
// removed for `transitionID` and consumes the archive entries, so the revert is
// the exact inverse of the apply: an occurrence a supervisor had customized by
// hand comes back exactly as it was, and a row the apply did NOT remove is never
// invented (reconstructing rosters from enrollments does both wrong — #405
// review).
//
// "Verbatim" covers the STRUCTURAL fields — room, note, unplanned / non-booking
// markers — but NOT the attendance state. status / substatus /
// student_status_day_id are re-derived from the day statuses that are active
// NOW, because the archived pair only ever describes a PLAN (the archive
// predicate excludes everything that recorded an event), and plans expire: an
// alumnus window of several weeks is ample time for a sickness to be reported or
// cleared. Replaying the snapshot would resurrect the pre-graduation state —
// expected for a child who has since been reported sick, absent for one whose
// status day was cleared. The reconciler's active-status pass cannot repair
// either: it only touches rows it just materialized, never these replayed ones
// (#405 review).
//
// Rows marked not_scheduled keep their own status: a non-booking is not an
// attendance plan, and ApplyActiveStatusDaysForInstance skips them for the same
// reason. So do rows a supervisor had set by hand to something other than
// 'expected' (manual_status_at with a non-expected status): the archive only
// takes those off occurrences that had not started yet, and re-deriving would
// silently drop the decision the archive is supposed to hand back. Their
// status-day provenance stays NULL — the PATCH that set the status cleared it
// (#405 review).
//
// room_id is re-validated against its current row instead of being trusted: a
// room deleted during the alumnus window restores as NULL rather than failing
// the whole revert on a foreign key. ON CONFLICT DO NOTHING covers a row that
// already exists again (a re-run, or a manual re-add during the alumnus
// window) — the existing row wins.
//
// Only STILL-ACTIONABLE instances are replayed: dated on or after `from`
// (today at revert time) and neither completed nor cancelled. The alumnus
// window can span weeks, so archived rows routinely describe occurrences that
// have since become history. Replaying an 'expected' or status-day 'absent' row
// into a past or completed instance would retroactively insert a child into
// frozen attendance — after every chance to record what actually happened has
// passed — corrupting reports and exports. Their ledger entries are still
// consumed by the same DELETE (they are obsolete, and leaving them would make a
// re-apply's upsert collide with a stale snapshot); only the INSERT is filtered
// (#405 review).
//
// (The archiving direction upserts instead, so a repeated archive of the same
// pair refreshes the snapshot rather than silently keeping a stale one.)
func (r *InstanceStudentRepository) RestoreArchivedByTransition(
	ctx context.Context, transitionID int64, studentIDs []int64, from timezone.Date,
) (int, error) {
	if len(studentIDs) == 0 {
		return 0, nil
	}
	if _, ok := tenant.TransactionFromContext(ctx); !ok {
		var restored int
		err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
			var restoreErr error
			restored, restoreErr = r.restoreArchivedByTransition(txCtx, transitionID, studentIDs, from)
			return restoreErr
		})
		return restored, err
	}
	return r.restoreArchivedByTransition(ctx, transitionID, studentIDs, from)
}

func (r *InstanceStudentRepository) restoreArchivedByTransition(
	ctx context.Context, transitionID int64, studentIDs []int64, from timezone.Date,
) (int, error) {

	// Serialize with partial-absence create/update/delete on the same
	// child/day before reading pe.excused_from. Without the care-day lock a
	// concurrent delete can drop the exception after this query stamps
	// pickup_exception_id, leaving an absent row with cleared provenance.
	if err := r.lockRestoreCareExceptionDays(ctx, transitionID, studentIDs, from); err != nil {
		return 0, err
	}

	// Re-validate the snapshot's room references through the room owner
	// before the replay, instead of joining facilities.rooms here (#2665).
	tenantID := tenant.FromContext(ctx)
	var archivedRoomIDs []int64
	if err := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`schedule.grade_transition_roster_removals AS "rm"`).
		ColumnExpr("DISTINCT rm.room_id").
		Where("rm.transition_id = ?", transitionID).
		Where("rm.student_id IN (?)", bun.List(studentIDs)).
		Where("rm.tenant_id = ?", tenantID).
		Where("rm.room_id IS NOT NULL").
		Scan(ctx, &archivedRoomIDs); err != nil {
		return 0, &modelBase.DatabaseError{Op: "restore archived rows by transition", Err: base.TranslateNotFound(err)}
	}
	roomIDs, err := validRoomIDs(ctx, r.rooms, tenantID, archivedRoomIDs)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "restore archived rows by transition", Err: err}
	}

	const rawSQL = `
		WITH restored AS (
			DELETE FROM schedule.grade_transition_roster_removals AS rm
			WHERE rm.transition_id = ?
			  AND rm.student_id IN (?)
			  AND rm.tenant_id = ?
			RETURNING rm.*
		)
		INSERT INTO schedule.instance_students (
			tenant_id, instance_id, student_id, room_id, status, substatus, note,
			is_unplanned, not_scheduled, manual_status_at, student_status_day_id, pickup_exception_id,
			created_at, updated_at
		)
		SELECT restored.tenant_id, restored.instance_id, restored.student_id,
		       CASE WHEN restored.room_id = ANY(?) THEN restored.room_id END,
		       CASE
		           WHEN restored.not_scheduled          THEN restored.status
		           WHEN hand_set.kept                   THEN restored.status
		           WHEN active_day.id IS NOT NULL       THEN ?
		           WHEN partial.id IS NOT NULL          THEN ?
		           ELSE ?
		       END,
		       CASE
		           WHEN restored.not_scheduled          THEN restored.substatus
		           WHEN hand_set.kept                   THEN restored.substatus
		           WHEN active_day.status = 'sick'       THEN ?
		           WHEN active_day.status = 'excused'    THEN ?
		           WHEN active_day.status = 'class_trip' THEN ?
		           WHEN partial.id IS NOT NULL           THEN ?
		           ELSE NULL
		       END,
		       restored.note,
		       restored.is_unplanned, restored.not_scheduled, restored.manual_status_at,
		       CASE WHEN restored.not_scheduled OR hand_set.kept THEN NULL ELSE active_day.id END,
		       CASE WHEN restored.not_scheduled OR hand_set.kept OR active_day.id IS NOT NULL THEN NULL ELSE partial.id END,
		       NOW(), NOW()
		FROM restored
		JOIN schedule.activity_instances AS ai
		       ON ai.id = restored.instance_id
		      AND ai.tenant_id = restored.tenant_id
		CROSS JOIN LATERAL (
		       SELECT restored.manual_status_at IS NOT NULL
		              AND restored.status <> ? AS kept
		) AS hand_set
		LEFT JOIN LATERAL (
		       SELECT sd.id, sd.status
		       FROM active.student_status_days AS sd
		       WHERE sd.student_id = restored.student_id
		         AND sd.tenant_id  = restored.tenant_id
		         AND sd.date       = ai.date
		         AND sd.cleared_at IS NULL
		       ORDER BY sd.reported_at DESC, sd.id DESC
		       LIMIT 1
		) AS active_day ON TRUE
		LEFT JOIN LATERAL (
		       SELECT pe.id
		       FROM schedule.student_pickup_exceptions AS pe
		       WHERE pe.student_id = restored.student_id
		         AND pe.tenant_id = restored.tenant_id
		         AND pe.exception_date = ai.date
		         AND pe.excused_from IS NOT NULL
		         AND ai.start_time >= pe.excused_from
		       LIMIT 1
		) AS partial ON TRUE
		WHERE ai.date >= ?
		  AND ai.status NOT IN (?, ?)
		ON CONFLICT (instance_id, student_id) DO NOTHING`

	result, err := base.GetDB(ctx, r.db).ExecContext(ctx, rawSQL,
		transitionID,
		bun.List(studentIDs),
		tenantID,
		pgdialect.Array(roomIDs),
		schedule.AttendanceStatusAbsent,
		schedule.AttendanceStatusAbsent,
		schedule.AttendanceStatusExpected,
		schedule.AttendanceSubstatusSick,
		schedule.AttendanceSubstatusExcused,
		schedule.AttendanceSubstatusFieldTrip,
		schedule.AttendanceSubstatusExcused,
		schedule.AttendanceStatusExpected,
		from,
		schedule.InstanceStatusCompleted,
		schedule.InstanceStatusCancelled,
	)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "restore archived rows by transition",
			Err: base.TranslateNotFound(err),
		}
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "get rows affected",
			Err: base.TranslateNotFound(err),
		}
	}
	return int(affected), nil
}

// lockRestoreCareExceptionDays takes the shared care-day lock for every
// (student, date) pair the restore INSERT will touch, ordered by student then
// date so concurrent multi-day writers do not deadlock each other.
func (r *InstanceStudentRepository) lockRestoreCareExceptionDays(
	ctx context.Context, transitionID int64, studentIDs []int64, from timezone.Date,
) error {
	type careDay struct {
		StudentID int64         `bun:"student_id"`
		Date      timezone.Date `bun:"date"`
	}
	var days []careDay
	if err := base.GetDB(ctx, r.db).NewRaw(`
		SELECT DISTINCT rm.student_id, ai.date
		FROM schedule.grade_transition_roster_removals AS rm
		JOIN schedule.activity_instances AS ai
			ON ai.id = rm.instance_id
			AND ai.tenant_id = rm.tenant_id
		WHERE rm.transition_id = ?
			AND rm.student_id IN (?)
			AND rm.tenant_id = ?
			AND ai.date >= ?
			AND ai.status NOT IN (?, ?)
		ORDER BY rm.student_id, ai.date
	`, transitionID, bun.List(studentIDs), tenant.FromContext(ctx), from,
		schedule.InstanceStatusCompleted, schedule.InstanceStatusCancelled,
	).Scan(ctx, &days); err != nil {
		return &modelBase.DatabaseError{Op: "list restore care-exception days for lock", Err: base.TranslateNotFound(err)}
	}
	// Student row then care-day for every pair (student FOR UPDATE is
	// re-entrant within the same transaction when the same child appears on
	// multiple dates). Matches partial-absence and excused-request writers.
	// The row belongs to the People Directory (#2662), so the lock comes from
	// the bound directory; the care-day advisory lock stays here.
	if r.students == nil {
		return errStudentDirectoryRequired
	}
	for _, day := range days {
		if err := r.students.LockStudent(ctx, day.StudentID); err != nil {
			if errors.Is(err, ErrStudentNotFound) {
				return sql.ErrNoRows
			}
			return fmt.Errorf("lock student for care exception day: %w", err)
		}
		if err := careplanning.LockExceptionDay(ctx, r.db, day.StudentID, day.Date.String()); err != nil {
			return err
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
			Err: base.TranslateNotFound(err),
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
			Err: base.TranslateNotFound(err),
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
			Err: base.TranslateNotFound(err),
		}
	}
	return rows, nil
}
