// backend/database/repositories/active/attendance_repository.go
package active

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// AttendanceRepository implements active.AttendanceRepository interface
type AttendanceRepository struct {
	*base.Repository[*active.Attendance]
	db *bun.DB
}

// NewAttendanceRepository creates a new AttendanceRepository
func NewAttendanceRepository(db *bun.DB) active.AttendanceRepository {
	repo := base.NewRepository[*active.Attendance](db, "active.attendance", "Attendance")
	repo.TenantScoped = true
	return &AttendanceRepository{
		Repository: repo,
		db:         db,
	}
}

// FindByStudentAndDate finds all attendance records for a student on a specific date
func (r *AttendanceRepository) FindByStudentAndDate(ctx context.Context, studentID int64, date timezone.Date) ([]*active.Attendance, error) {
	var attendance []*active.Attendance

	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&attendance).
		ModelTableExpr(`active.attendance AS "attendance"`).
		Where(`"attendance".student_id = ? AND "attendance".date = ?`, studentID, date).
		Order(`check_in_time ASC`)

	query = base.WithTenantFilter(ctx, query, "attendance")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by student and date",
			Err: err,
		}
	}

	return attendance, nil
}

// FindByStudentAndDateRange finds all attendance records for a student between two
// dates (inclusive), ordered by date descending then check_in_time descending.
func (r *AttendanceRepository) FindByStudentAndDateRange(ctx context.Context, studentID int64, startDate, endDate timezone.Date) ([]*active.Attendance, error) {
	var attendance []*active.Attendance

	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&attendance).
		ModelTableExpr(`active.attendance AS "attendance"`).
		Where(`"attendance".student_id = ?`, studentID).
		Where(`"attendance".date >= ?`, startDate).
		Where(`"attendance".date <= ?`, endDate).
		OrderExpr(`"attendance".date DESC`).
		OrderExpr(`"attendance".check_in_time DESC`)

	query = base.WithTenantFilter(ctx, query, "attendance")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by student and date range",
			Err: err,
		}
	}
	return attendance, nil
}

// FindLatestByStudent finds the most recent attendance record for a student
func (r *AttendanceRepository) FindLatestByStudent(ctx context.Context, studentID int64) (*active.Attendance, error) {
	attendance := new(active.Attendance)

	query := base.GetDB(ctx, r.db).NewSelect().
		Model(attendance).
		ModelTableExpr(`active.attendance AS "attendance"`).
		Where(`"attendance".student_id = ?`, studentID).
		OrderExpr(`"attendance".date DESC`).
		OrderExpr(`"attendance".check_in_time DESC`).
		Limit(1)

	query = base.WithTenantFilter(ctx, query, "attendance")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find latest by student",
			Err: err,
		}
	}

	return attendance, nil
}

// GetStudentCurrentStatus gets the current check-in status for a student (today's latest record)
func (r *AttendanceRepository) GetStudentCurrentStatus(ctx context.Context, studentID int64) (*active.Attendance, error) {
	attendance := new(active.Attendance)

	today := timezone.TodayDate()
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(attendance).
		ModelTableExpr(`active.attendance AS "attendance"`).
		Where(`"attendance".student_id = ? AND "attendance".date = ?`, studentID, today).
		Order(`check_in_time DESC`).
		Limit(1)

	query = base.WithTenantFilter(ctx, query, "attendance")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get student current status",
			Err: err,
		}
	}

	return attendance, nil
}

// CreateIfNoOpenForToday inserts the attendance row, deferring to the partial
// unique index uniq_attendance_open_per_student_day on
// (student_id, date) WHERE check_out_time IS NULL when a concurrent caller
// already opened today's attendance. Returns inserted=true when the row was
// actually written; false signals the conflict path swallowed the insert and
// the caller should re-fetch the existing open row.
func (r *AttendanceRepository) CreateIfNoOpenForToday(ctx context.Context, attendance *active.Attendance) (bool, error) {
	if attendance == nil {
		return false, fmt.Errorf("attendance cannot be nil")
	}

	// Auto-set tenant_id from context if not yet populated, matching base.Create.
	if attendance.GetTenantID() == 0 {
		if tid := tenant.FromContext(ctx); tid != 0 {
			attendance.SetTenantID(tid)
		}
	}

	res, err := base.GetDB(ctx, r.db).NewInsert().
		Model(attendance).
		ModelTableExpr("active.attendance").
		On("CONFLICT (student_id, date) WHERE check_out_time IS NULL DO NOTHING").
		Exec(ctx)
	if err != nil {
		return false, &modelBase.DatabaseError{Op: "create_if_no_open_for_today", Err: err}
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return false, &modelBase.DatabaseError{Op: "create_if_no_open_for_today_rows_affected", Err: err}
	}
	return affected > 0, nil
}

// CloseOpenForToday closes the open attendance row for the given student
// on the caller-supplied calendar day via a state-checked UPDATE — the
// WHERE check_out_time IS NULL guard turns concurrent "out" calls (and
// "in/out" races where the in lost) into no-ops at the database layer
// instead of corrupting the row. The day is a parameter, not re-derived
// here, so a batch checkout crossing Berlin midnight stays on its snapshot
// day (review #2372) — see the interface doc.
//
// Why this exists: the school-checkin handler used to route writes through
// ToggleStudentAttendance, which read-then-flipped the state. Two concurrent
// "in" requests for an absent student could end up calling check-out, because
// the second request's internal re-read saw the first request's commit and
// flipped the action. Action-explicit handlers now call this method directly
// and skip the toggle entirely.
func (r *AttendanceRepository) CloseOpenForToday(ctx context.Context, studentID int64, now time.Time, today timezone.Date, staffID int64) (*active.Attendance, error) {
	// UPDATE … RETURNING populates the row scan target. Bun bubbles up
	// sql.ErrNoRows when zero rows match, so we treat that as the
	// idempotent "no open row" path rather than a database error.
	//
	// Tenant scoping note: the request is already inside a tenant tx with
	// RLS, which makes student_id de facto unique (one tenant only sees
	// its own students), so we don't need an explicit tenant_id predicate
	// here — RLS rejects rows that aren't ours before the WHERE runs.
	row := &active.Attendance{}
	q := base.GetDB(ctx, r.db).NewUpdate().
		Model(row).
		ModelTableExpr("active.attendance").
		Set("check_out_time = ?", now).
		Set("yard_since = NULL").
		Where("student_id = ?", studentID).
		Where("date = ?", today).
		Where("check_out_time IS NULL").
		Returning("*")
	if staffID > 0 {
		q = q.Set("checked_out_by = ?", staffID)
	}

	if _, err := q.Exec(ctx, row); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// No open row — student wasn't checked in, or another concurrent
			// caller already closed it. Both are idempotent successes.
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{Op: "close_open_for_today", Err: err}
	}
	return row, nil
}

// CreateIfNoOpenForTodayBatch is the multi-row form of CreateIfNoOpenForToday:
// one INSERT … ON CONFLICT DO NOTHING for the whole batch, deferring to the
// same partial unique index. Returns the student ids whose row was actually
// inserted; conflicting students (already open attendance on that date) are
// silently absorbed exactly like the single-row method. Rows must arrive in
// ascending student-id order — the caller already holds the students' row
// locks in that order, which is what makes the multi-row write deadlock-safe.
func (r *AttendanceRepository) CreateIfNoOpenForTodayBatch(ctx context.Context, rows []*active.Attendance) ([]int64, error) {
	if len(rows) == 0 {
		return []int64{}, nil
	}
	tid := tenant.FromContext(ctx)
	for _, row := range rows {
		if row == nil {
			return nil, fmt.Errorf("attendance cannot be nil")
		}
		if row.GetTenantID() == 0 && tid != 0 {
			row.SetTenantID(tid)
		}
	}

	var insertedStudentIDs []int64
	err := base.GetDB(ctx, r.db).NewInsert().
		Model(&rows).
		ModelTableExpr("active.attendance").
		On("CONFLICT (student_id, date) WHERE check_out_time IS NULL DO NOTHING").
		Returning("student_id").
		Scan(ctx, &insertedStudentIDs)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Every row conflicted — the whole batch was absorbed.
			return []int64{}, nil
		}
		return nil, &modelBase.DatabaseError{Op: "create_if_no_open_for_today_batch", Err: err}
	}
	return insertedStudentIDs, nil
}

// CloseOpenForDateByStudentIDs is the multi-student form of CloseOpenForToday:
// one state-checked UPDATE … WHERE check_out_time IS NULL over the whole
// batch, returning only the rows that were actually closed. Students without
// an open row on the date are idempotent no-ops and simply missing from the
// result. The caller holds the students' row locks (ascending), which
// serializes overlapping batches before this UPDATE runs.
func (r *AttendanceRepository) CloseOpenForDateByStudentIDs(ctx context.Context, studentIDs []int64, now time.Time, day timezone.Date, staffID int64) ([]*active.Attendance, error) {
	if len(studentIDs) == 0 {
		return []*active.Attendance{}, nil
	}
	var closed []*active.Attendance
	q := base.GetDB(ctx, r.db).NewUpdate().
		Model((*active.Attendance)(nil)).
		ModelTableExpr("active.attendance").
		Set("check_out_time = ?", now).
		Set("yard_since = NULL").
		Where("student_id IN (?)", bun.List(studentIDs)).
		Where("date = ?", day).
		Where("check_out_time IS NULL").
		Returning("*")
	if staffID > 0 {
		q = q.Set("checked_out_by = ?", staffID)
	}

	if err := q.Scan(ctx, &closed); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []*active.Attendance{}, nil
		}
		return nil, &modelBase.DatabaseError{Op: "close_open_for_date_by_student_ids", Err: err}
	}
	return closed, nil
}

// FindByID overrides base FindByID to match the interface signature
func (r *AttendanceRepository) FindByID(ctx context.Context, id int64) (*active.Attendance, error) {
	// Use the base FindByID method with interface{} conversion
	return r.Repository.FindByID(ctx, id)
}

// Delete overrides base Delete to match the interface signature
func (r *AttendanceRepository) Delete(ctx context.Context, id int64) error {
	// Use the base Delete method with interface{} conversion
	return r.Repository.Delete(ctx, id)
}

// GetTodayByStudentID gets today's attendance record for a student
func (r *AttendanceRepository) GetTodayByStudentID(ctx context.Context, studentID int64) (*active.Attendance, error) {
	// This is the same as GetStudentCurrentStatus - just call that method
	return r.GetStudentCurrentStatus(ctx, studentID)
}

// GetTodayByStudentIDs gets today's attendance records for multiple students in a single query
func (r *AttendanceRepository) GetTodayByStudentIDs(ctx context.Context, studentIDs []int64) (map[int64]*active.Attendance, error) {
	return r.getTodayByStudentIDs(ctx, studentIDs, false, false)
}

// GetOpenTodayByStudentIDsForUpdate gets today's open attendance records for
// multiple students and locks the matching rows for the current transaction.
func (r *AttendanceRepository) GetOpenTodayByStudentIDsForUpdate(ctx context.Context, studentIDs []int64) (map[int64]*active.Attendance, error) {
	return r.getTodayByStudentIDs(ctx, studentIDs, true, true)
}

func (r *AttendanceRepository) getTodayByStudentIDs(ctx context.Context, studentIDs []int64, openOnly bool, lockRows bool) (map[int64]*active.Attendance, error) {
	result := make(map[int64]*active.Attendance, len(studentIDs))

	if len(studentIDs) == 0 {
		return result, nil
	}

	// Ensure we only query unique student IDs
	uniqueIDs := make([]int64, 0, len(studentIDs))
	seen := make(map[int64]struct{}, len(studentIDs))
	for _, id := range studentIDs {
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		uniqueIDs = append(uniqueIDs, id)
	}

	today := timezone.TodayDate()
	var attendances []*active.Attendance
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&attendances).
		ModelTableExpr(`active.attendance AS "attendance"`).
		Where(`"attendance".student_id IN (?)`, bun.List(uniqueIDs)).
		Where(`"attendance".date = ?`, today).
		OrderExpr(`"attendance".student_id ASC`).
		OrderExpr(`"attendance".check_in_time DESC`)
	if openOnly {
		query = query.Where(`"attendance".check_out_time IS NULL`)
	}
	if lockRows {
		query = query.For("UPDATE")
	}

	query = base.WithTenantFilter(ctx, query, "attendance")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get today by student IDs",
			Err: err,
		}
	}

	for _, attendance := range attendances {
		if _, exists := result[attendance.StudentID]; !exists {
			result[attendance.StudentID] = attendance
		}
	}

	return result, nil
}

// FindForDate finds all attendance records for a specific date
func (r *AttendanceRepository) FindForDate(ctx context.Context, date timezone.Date) ([]*active.Attendance, error) {
	return r.findForDate(ctx, date, nil)
}

// FindForDateByStudentIDs finds attendance records for a specific date and
// student set. Restricting the query at the repository boundary prevents
// scoped consumers from loading other students' attendance into memory.
func (r *AttendanceRepository) FindForDateByStudentIDs(ctx context.Context, date timezone.Date, studentIDs []int64) ([]*active.Attendance, error) {
	if len(studentIDs) == 0 {
		return []*active.Attendance{}, nil
	}
	return r.findForDate(ctx, date, studentIDs)
}

func (r *AttendanceRepository) findForDate(ctx context.Context, date timezone.Date, studentIDs []int64) ([]*active.Attendance, error) {
	var attendance []*active.Attendance

	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&attendance).
		ModelTableExpr(`active.attendance AS "attendance"`).
		Where(`"attendance".date = ?`, date).
		// Use OrderExpr to avoid Bun re-quoting the alias and direction together
		OrderExpr(`"attendance".student_id ASC`).
		OrderExpr(`"attendance".check_in_time ASC`)
	if studentIDs != nil {
		query = query.Where(`"attendance".student_id IN (?)`, bun.List(studentIDs))
	}

	query = base.WithTenantFilter(ctx, query, "attendance")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find for date",
			Err: err,
		}
	}

	return attendance, nil
}

// ListOpenStudentIDsForDate returns unique student IDs with open attendance
// rows on the given date.
func (r *AttendanceRepository) ListOpenStudentIDsForDate(ctx context.Context, date timezone.Date) ([]int64, error) {
	var ids []int64

	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`active.attendance AS "attendance"`).
		ColumnExpr(`DISTINCT "attendance".student_id`).
		Where(`"attendance".date = ?`, date).
		Where(`"attendance".check_out_time IS NULL`).
		OrderExpr(`"attendance".student_id ASC`)

	query = base.WithTenantFilter(ctx, query, "attendance")

	if err := query.Scan(ctx, &ids); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list open student IDs for date",
			Err: err,
		}
	}

	return ids, nil
}

// CountByStaffID counts attendance records where the staff member checked in or checked out students.
func (r *AttendanceRepository) CountByStaffID(ctx context.Context, staffID int64) (int, error) {
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`active.attendance AS "attendance"`).
		Where(`"attendance".checked_in_by = ? OR "attendance".checked_out_by = ?`, staffID, staffID)

	query = base.WithTenantFilter(ctx, query, "attendance")

	count, err := query.Count(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "count by staff ID",
			Err: err,
		}
	}

	return count, nil
}

// FindStaleOpen returns attendance rows dated before the given day that still
// lack a check-out time. Feeds the nightly stale-attendance cleanup and its
// preview (services/active/cleanup_service.go).
func (r *AttendanceRepository) FindStaleOpen(ctx context.Context, before timezone.Date) ([]*active.Attendance, error) {
	var records []*active.Attendance

	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&records).
		ModelTableExpr(`active.attendance AS "attendance"`).
		Where(`"attendance".date < ?`, before).
		Where(`"attendance".check_out_time IS NULL`)

	query = base.WithTenantFilter(ctx, query, "attendance")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find stale open attendance",
			Err: err,
		}
	}

	return records, nil
}

// HasOpenAttendanceOn reports whether any attendance row on the given
// calendar date is still open (check_out_time IS NULL). The date binds as a
// DATE literal (timezone.Date) so the Berlin calendar day is matched exactly,
// independent of the UTC session date (see the operator presence-mode guard).
//
// Requires a tenant tx in ctx; RLS is the only tenant scope — there is no
// tenant_id WHERE clause, so calling without a tenant tx falls back to the
// bare *bun.DB and would read attendance across all tenants.
func (r *AttendanceRepository) HasOpenAttendanceOn(ctx context.Context, date timezone.Date) (bool, error) {
	var exists bool
	err := base.GetDB(ctx, r.db).NewRaw(`
		SELECT EXISTS(
			SELECT 1 FROM active.attendance
			WHERE date = ?
			  AND check_out_time IS NULL
		)
	`, date).Scan(ctx, &exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}
