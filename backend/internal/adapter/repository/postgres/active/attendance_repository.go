// backend/database/repositories/active/attendance_repository.go
package active

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres/base"
	"github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// AttendanceRepository implements active.AttendanceRepository interface
type AttendanceRepository struct {
	*base.Repository[*active.Attendance]
	db *bun.DB
}

// NewAttendanceRepository creates a new AttendanceRepository
func NewAttendanceRepository(db *bun.DB) active.AttendanceRepository {
	return &AttendanceRepository{
		Repository: base.NewRepository[*active.Attendance](db, "active.attendance", "Attendance"),
		db:         db,
	}
}

// FindByStudentAndDate finds all attendance records for a student on a specific date
func (r *AttendanceRepository) FindByStudentAndDate(ctx context.Context, studentID int64, date time.Time) ([]*active.Attendance, error) {
	var attendance []*active.Attendance

	// Extract date only (ignore time component) - use UTC to match other methods
	dateOnly := date.Truncate(24 * time.Hour)

	err := r.db.NewSelect().
		Model(&attendance).
		ModelTableExpr(`active.attendance AS "attendance"`).
		Where(`"attendance".student_id = ? AND "attendance".date = ?`, studentID, dateOnly).
		Order(`check_in_time ASC`).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by student and date",
			Err: err,
		}
	}

	return attendance, nil
}

// FindLatestByStudent finds the most recent attendance record for a student
func (r *AttendanceRepository) FindLatestByStudent(ctx context.Context, studentID int64) (*active.Attendance, error) {
	attendance := new(active.Attendance)

	err := r.db.NewSelect().
		Model(attendance).
		ModelTableExpr(`active.attendance AS "attendance"`).
		Where(`"attendance".student_id = ?`, studentID).
		OrderExpr(`"attendance".date DESC`).
		OrderExpr(`"attendance".check_in_time DESC`).
		Limit(1).
		Scan(ctx)

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

	// Get today's date in local time (school operates in local timezone)
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	err := r.db.NewSelect().
		Model(attendance).
		ModelTableExpr(`active.attendance AS "attendance"`).
		Where(`"attendance".student_id = ? AND "attendance".date = ?`, studentID, today).
		Order(`check_in_time DESC`).
		Limit(1).
		Scan(ctx)

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

	// Get today's date in local time (school operates in local timezone)
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	var attendances []*active.Attendance
	err := r.db.NewSelect().
		Model(&attendances).
		ModelTableExpr(`active.attendance AS "attendance"`).
		Where(`"attendance".student_id IN (?)`, bun.In(uniqueIDs)).
		Where(`"attendance".date = ?`, today).
		OrderExpr(`"attendance".student_id ASC`).
		OrderExpr(`"attendance".check_in_time DESC`).
		Scan(ctx)
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

	// Extract date only (ignore time component) - use UTC to match other methods
	dateOnly := date.Truncate(24 * time.Hour)

	err := r.db.NewSelect().
		Model(&attendance).
		ModelTableExpr(`active.attendance AS "attendance"`).
		Where(`"attendance".date = ?`, dateOnly).
		// Use OrderExpr to avoid Bun re-quoting the alias and direction together
		OrderExpr(`"attendance".student_id ASC`).
		OrderExpr(`"attendance".check_in_time ASC`).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find for date",
			Err: err,
		}
	}

	return attendance, nil
}

// FindStaleRecords finds attendance records from before the given date that have no check-out time
func (r *AttendanceRepository) FindStaleRecords(ctx context.Context, beforeDate time.Time) ([]active.StaleAttendanceRecord, error) {
	var records []active.StaleAttendanceRecord

	err := r.db.NewSelect().
		Table("active.attendance").
		Column("id", "student_id", "date", "check_in_time").
		Where("date < ?", beforeDate).
		Where("check_out_time IS NULL").
		Scan(ctx, &records)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find stale records",
			Err: err,
		}
	}

	return records, nil
}

// CloseStaleRecord sets the check-out time for a stale attendance record
func (r *AttendanceRepository) CloseStaleRecord(ctx context.Context, id int64, checkOutTime time.Time) error {
	_, err := r.db.NewUpdate().
		Table("active.attendance").
		Set("check_out_time = ?", checkOutTime).
		Set("updated_at = ?", time.Now().UTC()).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "close stale record",
			Err: err,
		}
	}

	return nil
}
