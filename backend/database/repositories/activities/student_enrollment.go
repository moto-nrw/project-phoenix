// backend/database/repositories/activities/student_enrollment.go
package activities

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// Table and query constants (S1192 - avoid duplicate string literals)
const (
	tableActivitiesStudentEnrollments          = "activities.student_enrollments"
	tableExprActivitiesEnrollmentsAsEnrollment = `activities.student_enrollments AS "student_enrollment"`
)

// StudentEnrollmentRepository implements activities.StudentEnrollmentRepository interface
type StudentEnrollmentRepository struct {
	*base.Repository[*activities.StudentEnrollment]
	db *bun.DB
}

// NewStudentEnrollmentRepository creates a new StudentEnrollmentRepository
func NewStudentEnrollmentRepository(db *bun.DB) activities.StudentEnrollmentRepository {
	return &StudentEnrollmentRepository{
		Repository: base.NewRepository[*activities.StudentEnrollment](db, tableActivitiesStudentEnrollments, "StudentEnrollment"),
		db:         db,
	}
}

// FindByStudentID finds all enrollments for a specific student
func (r *StudentEnrollmentRepository) FindByStudentID(ctx context.Context, studentID int64) ([]*activities.StudentEnrollment, error) {
	enrollments := make([]*activities.StudentEnrollment, 0)
	err := r.db.NewSelect().
		Model(&enrollments).
		ModelTableExpr(tableExprActivitiesEnrollmentsAsEnrollment).
		// Note: Relation() doesn't work with multi-schema tables
		// The caller should load ActivityGroup and ActivityGroup.Category separately if needed
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
	type enrollmentResult struct {
		Enrollment *activities.StudentEnrollment `bun:"student_enrollment"`
		Student    *users.Student                `bun:"student"`
		Person     *users.Person                 `bun:"person"`
	}

	var results []enrollmentResult

	// Use explicit joins with schema qualification
	err := r.db.NewSelect().
		Model(&results).
		ModelTableExpr(tableExprActivitiesEnrollmentsAsEnrollment).
		// Explicit column mapping for each table
		ColumnExpr(`"student_enrollment".id AS "student_enrollment__id"`).
		ColumnExpr(`"student_enrollment".created_at AS "student_enrollment__created_at"`).
		ColumnExpr(`"student_enrollment".updated_at AS "student_enrollment__updated_at"`).
		ColumnExpr(`"student_enrollment".student_id AS "student_enrollment__student_id"`).
		ColumnExpr(`"student_enrollment".activity_group_id AS "student_enrollment__activity_group_id"`).
		ColumnExpr(`"student_enrollment".enrollment_date AS "student_enrollment__enrollment_date"`).
		ColumnExpr(`"student_enrollment".attendance_status AS "student_enrollment__attendance_status"`).
		ColumnExpr(`"student".id AS "student__id"`).
		ColumnExpr(`"student".created_at AS "student__created_at"`).
		ColumnExpr(`"student".updated_at AS "student__updated_at"`).
		ColumnExpr(`"student".person_id AS "student__person_id"`).
		ColumnExpr(`"student".school_class AS "student__school_class"`).
		ColumnExpr(`"student".guardian_name AS "student__guardian_name"`).
		ColumnExpr(`"student".guardian_contact AS "student__guardian_contact"`).
		ColumnExpr(`"student".guardian_email AS "student__guardian_email"`).
		ColumnExpr(`"student".guardian_phone AS "student__guardian_phone"`).
		ColumnExpr(`"student".group_id AS "student__group_id"`).
		ColumnExpr(`"person".id AS "person__id"`).
		ColumnExpr(`"person".created_at AS "person__created_at"`).
		ColumnExpr(`"person".updated_at AS "person__updated_at"`).
		ColumnExpr(`"person".first_name AS "person__first_name"`).
		ColumnExpr(`"person".last_name AS "person__last_name"`).
		ColumnExpr(`"person".tag_id AS "person__tag_id"`).
		ColumnExpr(`"person".account_id AS "person__account_id"`).
		// Properly schema-qualified joins
		Join(`LEFT JOIN users.students AS "student" ON "student".id = "student_enrollment".student_id`).
		Join(`LEFT JOIN users.persons AS "person" ON "person".id = "student".person_id`).
		// Filter by group ID
		Where(`"student_enrollment".activity_group_id = ?`, groupID).
		Order("student_enrollment.enrollment_date DESC").
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by group ID",
			Err: err,
		}
	}

	// Convert results to StudentEnrollment objects
	enrollments := make([]*activities.StudentEnrollment, len(results))
	for i, result := range results {
		enrollments[i] = result.Enrollment
		enrollments[i].Student = result.Student
		if result.Student != nil {
			result.Student.Person = result.Person
		}
	}

	return enrollments, nil
}

// CountByGroupID counts the number of students enrolled in a specific group
func (r *StudentEnrollmentRepository) CountByGroupID(ctx context.Context, groupID int64) (int, error) {
	count, err := r.db.NewSelect().
		Model((*activities.StudentEnrollment)(nil)).
		ModelTableExpr(tableExprActivitiesEnrollmentsAsEnrollment).
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
	enrollments := make([]*activities.StudentEnrollment, 0)
	err := r.db.NewSelect().
		Model(&enrollments).
		ModelTableExpr(tableExprActivitiesEnrollmentsAsEnrollment).
		// Note: Relation() doesn't work with multi-schema tables
		// The caller should load Student, Student.Person, and ActivityGroup separately if needed
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
		ModelTableExpr(tableExprActivitiesEnrollmentsAsEnrollment).
		Set("attendance_status = ?", status).
		Where(whereIDEquals, id).
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
		Where(whereIDEquals, enrollment.ID).
		ModelTableExpr(tableActivitiesStudentEnrollments)

	// Extract transaction from context if it exists
	if tx, ok := modelBase.TxFromContext(ctx); ok && tx != nil {
		// Use the transaction if available
		query = tx.NewUpdate().
			Model(enrollment).
			Where(whereIDEquals, enrollment.ID).
			ModelTableExpr(tableActivitiesStudentEnrollments)
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
	enrollments := make([]*activities.StudentEnrollment, 0)
	query := r.db.NewSelect().Model(&enrollments).ModelTableExpr(tableExprActivitiesEnrollmentsAsEnrollment)

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
