// backend/database/repositories/activities/student_enrollment.go
package activities

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	enrollmentprovenance "github.com/moto-nrw/project-phoenix/modules/timetableenrollmentprovenance"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Table and query constants (S1192 - avoid duplicate string literals)
const (
	tableActivitiesStudentEnrollments          = "activities.student_enrollments"
	tableExprActivitiesEnrollmentsAsEnrollment = `activities.student_enrollments AS "student_enrollment"`
	setStudentEnrollmentValidUntil             = "valid_until = ?"
)

// StudentEnrollmentRepository implements activities.StudentEnrollmentRepository interface
type StudentEnrollmentRepository struct {
	*base.Repository[*activities.StudentEnrollment]
	db       *bun.DB
	students StudentDirectory
}

// BindStudentDirectory installs the People Directory the group roster
// resolves Student rows through (#2662).
func (r *StudentEnrollmentRepository) BindStudentDirectory(students StudentDirectory) {
	r.students = students
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

// CapActiveByGroup truncates open enrollments. Future-only open rows are
// deleted instead of creating valid_until < valid_from; begun open rows are
// capped. Bounded care-offer windows remain untouched.
func (r *StudentEnrollmentRepository) CapActiveByGroup(ctx context.Context, groupID int64, validUntil timezone.Date) (int64, error) {
	deleteQuery := base.GetDB(ctx, r.db).NewDelete().
		Model((*activities.StudentEnrollment)(nil)).
		ModelTableExpr(tableExprActivitiesEnrollmentsAsEnrollment).
		Where(`"student_enrollment".activity_group_id = ?`, groupID).
		Where(`"student_enrollment".valid_from >= ?`, validUntil).
		Where(`"student_enrollment".valid_until IS NULL`).
		Where(`"student_enrollment".enrollment_request_child_id IS NULL`).
		Where(`COALESCE(jsonb_array_length("student_enrollment".selected_weekdays), 0) = 0`)
	deleteQuery = base.WithTenantFilter(ctx, deleteQuery, "student_enrollment")
	deletedResult, err := deleteQuery.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "delete future enrollments by group", Err: base.TranslateNotFound(err)}
	}
	deleted, _ := deletedResult.RowsAffected()

	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*activities.StudentEnrollment)(nil)).
		ModelTableExpr(tableExprActivitiesEnrollmentsAsEnrollment).
		Set(setStudentEnrollmentValidUntil, validUntil).
		Where(`"student_enrollment".activity_group_id = ?`, groupID).
		Where(`"student_enrollment".valid_from < ?`, validUntil).
		Where(`"student_enrollment".valid_until IS NULL`).
		Where(`"student_enrollment".enrollment_request_child_id IS NULL`).
		Where(`COALESCE(jsonb_array_length("student_enrollment".selected_weekdays), 0) = 0`)

	query = base.WithTenantFilter(ctx, query, "student_enrollment")

	res, err := query.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "cap active enrollments by group",
			Err: base.TranslateNotFound(err),
		}
	}
	rows, _ := res.RowsAffected() // nil-driver-safe: fall through with 0
	return deleted + rows, nil
}

// SetValidUntilByID closes one enrollment using a narrow update. FindByGroupID
// intentionally projects only fields needed by roster reads, so feeding that
// model through Update would clear omitted provenance such as
// enrollment_request_child_id.
func (r *StudentEnrollmentRepository) SetValidUntilByID(ctx context.Context, id int64, validUntil timezone.Date) error {
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*activities.StudentEnrollment)(nil)).
		ModelTableExpr(tableExprActivitiesEnrollmentsAsEnrollment).
		Set(setStudentEnrollmentValidUntil, validUntil).
		Where(`"student_enrollment".id = ?`, id)
	query = base.WithTenantFilter(ctx, query, "student_enrollment")

	result, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "set enrollment valid_until", Err: base.TranslateNotFound(err)}
	}
	return base.AssertRowsAffected(result, 1, "set enrollment valid_until")
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

	query = base.WithTenantFilter(ctx, query, "student_enrollment")

	err := query.
		Order("valid_from DESC").
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by student ID",
			Err: base.TranslateNotFound(err),
		}
	}

	return enrollments, nil
}

// FindActiveByStudentIDs finds active enrollments for a batch of students on
// the given date. valid_until is exclusive, matching timetable materialization.
func (r *StudentEnrollmentRepository) FindActiveByStudentIDs(ctx context.Context, studentIDs []int64, onDate timezone.Date) ([]*activities.StudentEnrollment, error) {
	if len(studentIDs) == 0 {
		return []*activities.StudentEnrollment{}, nil
	}

	type enrollmentResult struct {
		Enrollment    *activities.StudentEnrollment `bun:"student_enrollment"`
		ActivityGroup *activities.Group             `bun:"activity_group"`
	}

	var results []enrollmentResult
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&results).
		ModelTableExpr(tableExprActivitiesEnrollmentsAsEnrollment).
		ColumnExpr(`"student_enrollment".id AS "student_enrollment__id"`).
		ColumnExpr(`"student_enrollment".created_at AS "student_enrollment__created_at"`).
		ColumnExpr(`"student_enrollment".updated_at AS "student_enrollment__updated_at"`).
		ColumnExpr(`"student_enrollment".tenant_id AS "student_enrollment__tenant_id"`).
		ColumnExpr(`"student_enrollment".student_id AS "student_enrollment__student_id"`).
		ColumnExpr(`"student_enrollment".activity_group_id AS "student_enrollment__activity_group_id"`).
		ColumnExpr(`"student_enrollment".valid_from AS "student_enrollment__valid_from"`).
		ColumnExpr(`"student_enrollment".valid_until AS "student_enrollment__valid_until"`).
		ColumnExpr(`"student_enrollment".calendar_period_id AS "student_enrollment__calendar_period_id"`).
		ColumnExpr(`"student_enrollment".enrollment_request_child_id AS "student_enrollment__enrollment_request_child_id"`).
		ColumnExpr(`"student_enrollment".selected_weekdays AS "student_enrollment__selected_weekdays"`).
		ColumnExpr(`"student_enrollment".attendance_status AS "student_enrollment__attendance_status"`).
		ColumnExpr(`"student_enrollment".weekday AS "student_enrollment__weekday"`).
		ColumnExpr(`"activity_group".id AS "activity_group__id"`).
		ColumnExpr(`"activity_group".created_at AS "activity_group__created_at"`).
		ColumnExpr(`"activity_group".updated_at AS "activity_group__updated_at"`).
		ColumnExpr(`"activity_group".tenant_id AS "activity_group__tenant_id"`).
		ColumnExpr(`"activity_group".name AS "activity_group__name"`).
		ColumnExpr(`"activity_group".max_participants AS "activity_group__max_participants"`).
		ColumnExpr(`"activity_group".is_open AS "activity_group__is_open"`).
		ColumnExpr(`"activity_group".category_id AS "activity_group__category_id"`).
		ColumnExpr(`"activity_group".planned_room_id AS "activity_group__planned_room_id"`).
		ColumnExpr(`"activity_group".created_by AS "activity_group__created_by"`).
		ColumnExpr(`"activity_group".type AS "activity_group__type"`).
		ColumnExpr(`"activity_group".education_group_id AS "activity_group__education_group_id"`).
		ColumnExpr(`"activity_group".is_template AS "activity_group__is_template"`).
		ColumnExpr(`"activity_group".archived_at AS "activity_group__archived_at"`).
		Join(`LEFT JOIN activities.groups AS "activity_group" ON "activity_group".tenant_id = "student_enrollment".tenant_id AND "activity_group".id = "student_enrollment".activity_group_id`).
		Where(`"student_enrollment".student_id IN (?)`, bun.List(studentIDs)).
		Where(`"student_enrollment".valid_from <= ?`, onDate).
		Where(`("student_enrollment".valid_until IS NULL OR "student_enrollment".valid_until > ?)`, onDate).
		Where(`(
				"student_enrollment".weekday IS NULL
				OR "student_enrollment".weekday = EXTRACT(ISODOW FROM CAST(? AS DATE))::INT
			)`, onDate)

	query = base.WithTenantFilter(ctx, query, "student_enrollment")

	if err := query.
		OrderExpr(`"student_enrollment".student_id ASC`).
		OrderExpr(`"activity_group".name ASC`).
		Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find active enrollments by student IDs",
			Err: base.TranslateNotFound(err),
		}
	}

	enrollments := make([]*activities.StudentEnrollment, 0, len(results))
	for _, result := range results {
		if result.Enrollment == nil {
			continue
		}
		result.Enrollment.ActivityGroup = result.ActivityGroup
		enrollments = append(enrollments, result.Enrollment)
	}
	return enrollments, nil
}

// BackfillEnrollmentRequestChildSource stamps legacy rows materialized by a
// request child approval before enrollment_request_child_id existed. The
// tight approval-time guard keeps later manual rosters for the same
// student/window from being claimed.
func (r *StudentEnrollmentRepository) BackfillEnrollmentRequestChildSource(ctx context.Context, studentID, requestChildID int64, groupIDs []int64) (int64, error) {
	if studentID <= 0 {
		return 0, fmt.Errorf("student_id is required")
	}
	if requestChildID <= 0 {
		return 0, fmt.Errorf("enrollment_request_child_id is required")
	}
	if len(groupIDs) == 0 {
		return 0, nil
	}
	db, tenantID := base.GetDB(ctx, r.db), tenant.FromContext(ctx)
	ids, err := enrollmentprovenance.EligibleEnrollmentIDs(ctx, db, tenantID, studentID, requestChildID, groupIDs)
	if err != nil || len(ids) == 0 {
		return 0, err
	}
	result, err := db.NewUpdate().Table("activities.student_enrollments").
		Set("enrollment_request_child_id = ?", requestChildID).
		Set("updated_at = NOW()").
		Where("tenant_id = ?", tenantID).
		Where("id IN (?)", bun.List(ids)).Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "backfill enrollment request child source", Err: base.TranslateNotFound(err)}
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
	query = base.WithTenantFilter(ctx, query, "student_enrollment")
	result, err := query.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "delete by enrollment request child", Err: base.TranslateNotFound(err)}
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
			Err: base.TranslateNotFound(err),
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
			Err: base.TranslateNotFound(err),
		}
	}

	return base.AssertRowsAffected(result, 1, "update student_enrollment")
}

// List overrides the base List method to accept the new QueryOptions type
func (r *StudentEnrollmentRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*activities.StudentEnrollment, error) {
	enrollments, err := r.ListWithOptions(ctx, options)
	if err != nil {
		return nil, err
	}
	if enrollments == nil {
		enrollments = make([]*activities.StudentEnrollment, 0)
	}
	return enrollments, nil
}

// CloseOpenByGroupAndPeriod closes the open enrollments of a group for the
// given calendar period (issue #584: moved verbatim from api/timetable).
func (r *StudentEnrollmentRepository) CloseOpenByGroupAndPeriod(ctx context.Context, groupID int64, calendarPeriodID *int64, validFrom timezone.Date) error {
	tenantID := tenant.FromContext(ctx)
	update := base.GetDB(ctx, r.db).NewUpdate().
		Table("activities.student_enrollments").
		Set(setStudentEnrollmentValidUntil, validFrom).
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
