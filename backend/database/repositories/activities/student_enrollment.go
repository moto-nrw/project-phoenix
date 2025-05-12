// backend/database/repositories/activities/student_enrollment.go
package activities

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// StudentEnrollmentRepository implements activities.StudentEnrollmentRepository interface
type StudentEnrollmentRepository struct {
	*base.Repository[*activities.StudentEnrollment]
	db *bun.DB
}

// NewStudentEnrollmentRepository creates a new StudentEnrollmentRepository
func NewStudentEnrollmentRepository(db *bun.DB) activities.StudentEnrollmentRepository {
	return &StudentEnrollmentRepository{
		Repository: base.NewRepository[*activities.StudentEnrollment](db, "activities.student_enrollments", "StudentEnrollment"),
		db:         db,
	}
}

// FindByStudentID finds all enrollments for a specific student
func (r *StudentEnrollmentRepository) FindByStudentID(ctx context.Context, studentID int64) ([]*activities.StudentEnrollment, error) {
	var enrollments []*activities.StudentEnrollment
	err := r.db.NewSelect().
		Model(&enrollments).
		Relation("ActivityGroup").
		Relation("ActivityGroup.Category").
		Where("student_id = ?", studentID).
		Order("enrollment_date DESC").
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by student ID",
			Err: err,
		}
	}

	return enrollments, nil
}

// FindByGroupID finds all enrollments for a specific group
func (r *StudentEnrollmentRepository) FindByGroupID(ctx context.Context, groupID int64) ([]*activities.StudentEnrollment, error) {
	var enrollments []*activities.StudentEnrollment
	err := r.db.NewSelect().
		Model(&enrollments).
		Relation("Student").
		Relation("Student.Person").
		Where("activity_group_id = ?", groupID).
		Order("enrollment_date DESC").
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by group ID",
			Err: err,
		}
	}

	return enrollments, nil
}

// CountByGroupID counts the number of students enrolled in a specific group
func (r *StudentEnrollmentRepository) CountByGroupID(ctx context.Context, groupID int64) (int, error) {
	count, err := r.db.NewSelect().
		Model((*activities.StudentEnrollment)(nil)).
		Where("activity_group_id = ?", groupID).
		Count(ctx)

	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "count by group ID",
			Err: err,
		}
	}

	return count, nil
}

// FindByEnrollmentDateRange finds enrollments within a date range
func (r *StudentEnrollmentRepository) FindByEnrollmentDateRange(ctx context.Context, start, end time.Time) ([]*activities.StudentEnrollment, error) {
	var enrollments []*activities.StudentEnrollment
	err := r.db.NewSelect().
		Model(&enrollments).
		Relation("Student").
		Relation("Student.Person").
		Relation("ActivityGroup").
		Where("enrollment_date >= ? AND enrollment_date <= ?", start, end).
		Order("enrollment_date DESC").
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by enrollment date range",
			Err: err,
		}
	}

	return enrollments, nil
}

// UpdateAttendanceStatus updates the attendance status for a specific enrollment
func (r *StudentEnrollmentRepository) UpdateAttendanceStatus(ctx context.Context, id int64, status *string) error {
	// Validate status if provided
	if status != nil && !activities.IsValidAttendanceStatus(*status) {
		return fmt.Errorf("invalid attendance status: %s", *status)
	}

	_, err := r.db.NewUpdate().
		Model((*activities.StudentEnrollment)(nil)).
		Set("attendance_status = ?", status).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update attendance status",
			Err: err,
		}
	}

	return nil
}

// Create overrides the base Create method to handle validation
func (r *StudentEnrollmentRepository) Create(ctx context.Context, enrollment *activities.StudentEnrollment) error {
	if enrollment == nil {
		return fmt.Errorf("student enrollment cannot be nil")
	}

	// Validate enrollment
	if err := enrollment.Validate(); err != nil {
		return err
	}

	// Use the base Create method which now uses ModelTableExpr
	return r.Repository.Create(ctx, enrollment)
}

// Update overrides the base Update method to handle validation
func (r *StudentEnrollmentRepository) Update(ctx context.Context, enrollment *activities.StudentEnrollment) error {
	if enrollment == nil {
		return fmt.Errorf("student enrollment cannot be nil")
	}

	// Validate enrollment
	if err := enrollment.Validate(); err != nil {
		return err
	}

	// Get the query builder - detect if we're in a transaction
	query := r.db.NewUpdate().
		Model(enrollment).
		Where("id = ?", enrollment.ID).
		ModelTableExpr("activities.student_enrollments")

	// Extract transaction from context if it exists
	if tx, ok := ctx.Value("tx").(*bun.Tx); ok && tx != nil {
		// Use the transaction if available
		query = tx.NewUpdate().
			Model(enrollment).
			Where("id = ?", enrollment.ID).
			ModelTableExpr("activities.student_enrollments")
	}

	// Execute the query
	_, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update",
			Err: err,
		}
	}

	return nil
}

// List overrides the base List method to accept the new QueryOptions type
func (r *StudentEnrollmentRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*activities.StudentEnrollment, error) {
	var enrollments []*activities.StudentEnrollment
	query := r.db.NewSelect().Model(&enrollments)

	// Apply query options
	if options != nil {
		query = options.ApplyToQuery(query)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list",
			Err: err,
		}
	}

	return enrollments, nil
}
