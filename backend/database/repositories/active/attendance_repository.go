// backend/database/repositories/active/attendance_repository.go
package active

import (
	"context"
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
func (r *AttendanceRepository) FindByStudentAndDate(ctx context.Context, studentID int64, date time.Time) ([]*active.Attendance, error) {
	var attendance []*active.Attendance

	// Use DateOfUTC: the PG session runs in UTC, so Berlin-midnight timestamptz
	// gets cast to the previous UTC day. DateOfUTC yields UTC midnight of the
	// Berlin calendar date, which round-trips correctly through DATE columns.
	dateOnly := timezone.DateOfUTC(date)

	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&attendance).
		ModelTableExpr(`active.attendance AS "attendance"`).
		Where(`"attendance".student_id = ? AND "attendance".date = ?`, studentID, dateOnly).
		Order(`check_in_time ASC`)

	if where, val, ok := base.TenantWhere(ctx, "attendance"); ok {
		query = query.Where(where, val)
	}

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
//
// Uses DateOfUTC (not DateOf) when binding to the PostgreSQL DATE column: the
// PG session runs in UTC, so a Berlin-midnight timestamptz gets cast to the
// previous UTC day, silently excluding today's row. DateOfUTC yields UTC
// midnight of the Berlin calendar date, which round-trips correctly.
func (r *AttendanceRepository) FindByStudentAndDateRange(ctx context.Context, studentID int64, startDate, endDate time.Time) ([]*active.Attendance, error) {
	var attendance []*active.Attendance

	startOnly := timezone.DateOfUTC(startDate)
	endOnly := timezone.DateOfUTC(endDate)

	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&attendance).
		ModelTableExpr(`active.attendance AS "attendance"`).
		Where(`"attendance".student_id = ?`, studentID).
		Where(`"attendance".date >= ?`, startOnly).
		Where(`"attendance".date <= ?`, endOnly).
		OrderExpr(`"attendance".date DESC`).
		OrderExpr(`"attendance".check_in_time DESC`)

	if where, val, ok := base.TenantWhere(ctx, "attendance"); ok {
		query = query.Where(where, val)
	}

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

	if where, val, ok := base.TenantWhere(ctx, "attendance"); ok {
		query = query.Where(where, val)
	}

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

	// Use TodayUTC: the PG session runs in UTC, so Berlin-midnight gets cast to
	// the previous UTC day. TodayUTC yields UTC midnight of the Berlin calendar
	// date, which round-trips correctly through DATE columns.
	today := timezone.TodayUTC()
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(attendance).
		ModelTableExpr(`active.attendance AS "attendance"`).
		Where(`"attendance".student_id = ? AND "attendance".date = ?`, studentID, today).
		Order(`check_in_time DESC`).
		Limit(1)

	if where, val, ok := base.TenantWhere(ctx, "attendance"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get student current status",
			Err: err,
		}
	}

	return attendance, nil
}

// Create overrides base Create to handle validation
func (r *AttendanceRepository) Create(ctx context.Context, attendance *active.Attendance) error {
	if attendance == nil {
		return fmt.Errorf("attendance cannot be nil")
	}

	// Use the base Create method
	return r.Repository.Create(ctx, attendance)
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

// Update overrides base Update to handle validation
func (r *AttendanceRepository) Update(ctx context.Context, attendance *active.Attendance) error {
	if attendance == nil {
		return fmt.Errorf("attendance cannot be nil")
	}

	// Use the base Update method
	return r.Repository.Update(ctx, attendance)
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

	// Use TodayUTC: PG session runs in UTC, so Berlin-midnight gets cast to the
	// previous UTC day. TodayUTC round-trips correctly through DATE columns.
	today := timezone.TodayUTC()
	var attendances []*active.Attendance
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&attendances).
		ModelTableExpr(`active.attendance AS "attendance"`).
		Where(`"attendance".student_id IN (?)`, bun.List(uniqueIDs)).
		Where(`"attendance".date = ?`, today).
		OrderExpr(`"attendance".student_id ASC`).
		OrderExpr(`"attendance".check_in_time DESC`)

	if where, val, ok := base.TenantWhere(ctx, "attendance"); ok {
		query = query.Where(where, val)
	}

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
func (r *AttendanceRepository) FindForDate(ctx context.Context, date time.Time) ([]*active.Attendance, error) {
	var attendance []*active.Attendance

	// Use DateOfUTC: the PG session runs in UTC, so Berlin-midnight timestamptz
	// gets cast to the previous UTC day. DateOfUTC yields UTC midnight of the
	// Berlin calendar date, which round-trips correctly through DATE columns.
	dateOnly := timezone.DateOfUTC(date)

	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&attendance).
		ModelTableExpr(`active.attendance AS "attendance"`).
		Where(`"attendance".date = ?`, dateOnly).
		// Use OrderExpr to avoid Bun re-quoting the alias and direction together
		OrderExpr(`"attendance".student_id ASC`).
		OrderExpr(`"attendance".check_in_time ASC`)

	if where, val, ok := base.TenantWhere(ctx, "attendance"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find for date",
			Err: err,
		}
	}

	return attendance, nil
}

// CountByStaffID counts attendance records where the staff member checked in or checked out students.
func (r *AttendanceRepository) CountByStaffID(ctx context.Context, staffID int64) (int, error) {
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`active.attendance AS "attendance"`).
		Where(`"attendance".checked_in_by = ? OR "attendance".checked_out_by = ?`, staffID, staffID)

	if where, val, ok := base.TenantWhere(ctx, "attendance"); ok {
		query = query.Where(where, val)
	}

	count, err := query.Count(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "count by staff ID",
			Err: err,
		}
	}

	return count, nil
}
