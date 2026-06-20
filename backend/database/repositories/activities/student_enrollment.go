// backend/database/repositories/activities/student_enrollment.go
package activities

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
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
	repo := base.NewRepository[*activities.StudentEnrollment](db, tableActivitiesStudentEnrollments, "StudentEnrollment")
	repo.TenantScoped = true
	return &StudentEnrollmentRepository{
		Repository: repo,
		db:         db,
	}
}

// CapActiveByGroup ends every still-active enrollment (valid_until IS NULL)
// of the given group at validUntil (exclusive). Returns the number of rows
// changed. Custom method (backend-conventions Rule 2): multi-row bulk update
// for the template split (WP-B3).
func (r *StudentEnrollmentRepository) CapActiveByGroup(ctx context.Context, groupID int64, validUntil timezone.Date) (int64, error) {
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*activities.StudentEnrollment)(nil)).
		ModelTableExpr(tableExprActivitiesEnrollmentsAsEnrollment).
		Set("valid_until = ?", validUntil).
		Where(`"student_enrollment".activity_group_id = ?`, groupID).
		Where(`"student_enrollment".valid_until IS NULL`)

	if where, val, ok := base.TenantWhere(ctx, "student_enrollment"); ok {
		query = query.Where(where, val)
	}

	res, err := query.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "cap active enrollments by group",
			Err: err,
		}
	}
	rows, _ := res.RowsAffected() // nil-driver-safe: fall through with 0
	return rows, nil
}

// FindByStudentID finds all enrollments for a specific student
func (r *StudentEnrollmentRepository) FindByStudentID(ctx context.Context, studentID int64) ([]*activities.StudentEnrollment, error) {
	enrollments := make([]*activities.StudentEnrollment, 0)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&enrollments).
		ModelTableExpr(tableExprActivitiesEnrollmentsAsEnrollment).
		// Note: Relation() doesn't work with multi-schema tables
		// The caller should load ActivityGroup and ActivityGroup.Category separately if needed
		Where("student_id = ?", studentID)

	if where, val, ok := base.TenantWhere(ctx, "student_enrollment"); ok {
		query = query.Where(where, val)
	}

	err := query.
		Order("valid_from DESC").
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
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&results).
		ModelTableExpr(tableExprActivitiesEnrollmentsAsEnrollment).
		// Explicit column mapping for each table
		ColumnExpr(`"student_enrollment".id AS "student_enrollment__id"`).
		ColumnExpr(`"student_enrollment".created_at AS "student_enrollment__created_at"`).
		ColumnExpr(`"student_enrollment".updated_at AS "student_enrollment__updated_at"`).
		ColumnExpr(`"student_enrollment".tenant_id AS "student_enrollment__tenant_id"`).
		ColumnExpr(`"student_enrollment".student_id AS "student_enrollment__student_id"`).
		ColumnExpr(`"student_enrollment".activity_group_id AS "student_enrollment__activity_group_id"`).
		ColumnExpr(`"student_enrollment".valid_from AS "student_enrollment__valid_from"`).
		ColumnExpr(`"student_enrollment".valid_until AS "student_enrollment__valid_until"`).
		ColumnExpr(`"student_enrollment".calendar_period_id AS "student_enrollment__calendar_period_id"`).
		ColumnExpr(`"student_enrollment".selected_weekdays AS "student_enrollment__selected_weekdays"`).
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
		Where(`"student_enrollment".activity_group_id = ?`, groupID)

	if where, val, ok := base.TenantWhere(ctx, "student_enrollment"); ok {
		query = query.Where(where, val)
	}

	err := query.
		Order("student_enrollment.valid_from DESC").
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
	query := base.GetDB(ctx, r.db).NewSelect().
		Model((*activities.StudentEnrollment)(nil)).
		ModelTableExpr(tableExprActivitiesEnrollmentsAsEnrollment).
		Where("activity_group_id = ?", groupID)

	if where, val, ok := base.TenantWhere(ctx, "student_enrollment"); ok {
		query = query.Where(where, val)
	}

	count, err := query.Count(ctx)

	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "count by group ID",
			Err: err,
		}
	}

	return count, nil
}

// FindByValidFromRange finds enrollments within a valid_from date range
func (r *StudentEnrollmentRepository) FindByValidFromRange(ctx context.Context, start, end timezone.Date) ([]*activities.StudentEnrollment, error) {
	enrollments := make([]*activities.StudentEnrollment, 0)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&enrollments).
		ModelTableExpr(tableExprActivitiesEnrollmentsAsEnrollment).
		Where("valid_from >= ? AND valid_from <= ?", start, end)

	if where, val, ok := base.TenantWhere(ctx, "student_enrollment"); ok {
		query = query.Where(where, val)
	}

	err := query.
		Order("valid_from DESC").
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by valid_from date range",
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

	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*activities.StudentEnrollment)(nil)).
		ModelTableExpr(tableExprActivitiesEnrollmentsAsEnrollment).
		Set("attendance_status = ?", status).
		Where(whereIDEquals, id)

	if where, val, ok := base.TenantWhere(ctx, "student_enrollment"); ok {
		query = query.Where(where, val)
	}

	result, err := query.Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update attendance status",
			Err: err,
		}
	}

	return base.AssertRowsAffected(result, 1, "update attendance status")
}

// DeleteByStudentGroupsAndWindow removes rows for one student in the given
// linked groups and phase window. It deliberately requires explicit group ids
// and dates so callers do not wipe unrelated rosters.
func (r *StudentEnrollmentRepository) DeleteByStudentGroupsAndWindow(ctx context.Context, studentID int64, groupIDs []int64, validFrom timezone.Date, validUntil *timezone.Date) (int64, error) {
	if studentID <= 0 {
		return 0, fmt.Errorf("student_id is required")
	}
	if len(groupIDs) == 0 {
		return 0, nil
	}
	query := base.GetDB(ctx, r.db).NewDelete().
		Model((*activities.StudentEnrollment)(nil)).
		ModelTableExpr(tableExprActivitiesEnrollmentsAsEnrollment).
		Where(`"student_enrollment".student_id = ?`, studentID).
		Where(`"student_enrollment".activity_group_id IN (?)`, bun.List(groupIDs)).
		Where(`"student_enrollment".valid_from = ?`, validFrom)
	if validUntil == nil {
		query = query.Where(`"student_enrollment".valid_until IS NULL`)
	} else {
		query = query.Where(`"student_enrollment".valid_until = ?`, *validUntil)
	}
	if where, val, ok := base.TenantWhere(ctx, "student_enrollment"); ok {
		query = query.Where(where, val)
	}
	result, err := query.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "delete by student groups and window", Err: err}
	}
	rows, _ := result.RowsAffected()
	return rows, nil
}

// DeleteByEnrollmentRequestChild removes rows materialized from one approved
// enrollment request child for a specific student.
func (r *StudentEnrollmentRepository) DeleteByEnrollmentRequestChild(ctx context.Context, studentID, requestChildID int64) (int64, error) {
	if studentID <= 0 {
		return 0, fmt.Errorf("student_id is required")
	}
	if requestChildID <= 0 {
		return 0, fmt.Errorf("enrollment_request_child_id is required")
	}
	query := base.GetDB(ctx, r.db).NewDelete().
		Model((*activities.StudentEnrollment)(nil)).
		ModelTableExpr(tableExprActivitiesEnrollmentsAsEnrollment).
		Where(`"student_enrollment".student_id = ?`, studentID).
		Where(`"student_enrollment".enrollment_request_child_id = ?`, requestChildID)
	if where, val, ok := base.TenantWhere(ctx, "student_enrollment"); ok {
		query = query.Where(where, val)
	}
	result, err := query.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "delete by enrollment request child", Err: err}
	}
	rows, _ := result.RowsAffected()
	return rows, nil
}

// Create overrides the base Create method to handle validation. Template
// rosters usually apply on every template weekday, so they leave
// selected_weekdays empty; omit the column in that case.
func (r *StudentEnrollmentRepository) Create(ctx context.Context, enrollment *activities.StudentEnrollment) error {
	if enrollment == nil {
		return fmt.Errorf("student enrollment cannot be nil")
	}

	if err := enrollment.Validate(); err != nil {
		return err
	}
	if enrollment.GetTenantID() == 0 {
		if tid := tenant.FromContext(ctx); tid != 0 {
			enrollment.SetTenantID(tid)
		}
	}

	query := base.GetDB(ctx, r.db).NewInsert().
		Model(enrollment).
		ModelTableExpr(tableActivitiesStudentEnrollments)
	if len(enrollment.SelectedWeekdays) == 0 {
		query = query.ExcludeColumn("selected_weekdays")
	}
	if _, err := query.Exec(ctx); err != nil {
		return &modelBase.DatabaseError{
			Op:  "create",
			Err: err,
		}
	}
	return nil
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

	// Get the query builder - GetDB handles transaction extraction from context
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model(enrollment).
		Where(whereIDEquals, enrollment.ID).
		ModelTableExpr(tableActivitiesStudentEnrollments)

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}

	// Execute the query
	result, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update",
			Err: err,
		}
	}

	return base.AssertRowsAffected(result, 1, "update student_enrollment")
}

// List overrides the base List method to accept the new QueryOptions type
func (r *StudentEnrollmentRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*activities.StudentEnrollment, error) {
	enrollments := make([]*activities.StudentEnrollment, 0)
	query := base.GetDB(ctx, r.db).NewSelect().Model(&enrollments).ModelTableExpr(tableExprActivitiesEnrollmentsAsEnrollment)

	if where, val, ok := base.TenantWhere(ctx, "student_enrollment"); ok {
		query = query.Where(where, val)
	}

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

// CloseOpenByGroupAndPeriod closes the open enrollments of a group for the
// given calendar period (issue #584: moved verbatim from api/timetable).
func (r *StudentEnrollmentRepository) CloseOpenByGroupAndPeriod(ctx context.Context, groupID int64, calendarPeriodID *int64, validFrom timezone.Date) error {
	tenantID := tenant.FromContext(ctx)
	update := base.GetDB(ctx, r.db).NewUpdate().
		Table("activities.student_enrollments").
		Set("valid_until = ?", validFrom).
		Where("tenant_id = ?", tenantID).
		Where("activity_group_id = ?", groupID).
		Where("valid_until IS NULL")
	if calendarPeriodID == nil {
		update = update.Where("calendar_period_id IS NULL")
	} else {
		update = update.Where("calendar_period_id = ?", *calendarPeriodID)
	}
	_, err := update.Exec(ctx)
	return err
}
