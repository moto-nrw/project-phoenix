package activities

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// StudentEnrollmentRepository implements activities.StudentEnrollmentRepository
type StudentEnrollmentRepository struct {
	db *bun.DB
}

// NewStudentEnrollmentRepository creates a new student enrollment repository
func NewStudentEnrollmentRepository(db *bun.DB) activities.StudentEnrollmentRepository {
	return &StudentEnrollmentRepository{db: db}
}

// Create inserts a new student enrollment into the database
func (r *StudentEnrollmentRepository) Create(ctx context.Context, enrollment *activities.StudentEnrollment) error {
	if err := enrollment.Validate(); err != nil {
		return err
	}

	// Check if the student is already enrolled in this activity group
	existing, err := r.FindByStudentAndActivityGroup(ctx, enrollment.StudentID, enrollment.ActivityGroupID)
	if err == nil && existing != nil {
		return errors.New("student is already enrolled in this activity group")
	}

	// Check if the activity group has available spots
	groupRepo := NewGroupRepository(r.db)
	group, err := groupRepo.FindByID(ctx, enrollment.ActivityGroupID)
	if err != nil {
		return &base.DatabaseError{Op: "get_group", Err: err}
	}

	count, err := r.CountByActivityGroup(ctx, enrollment.ActivityGroupID)
	if err != nil {
		return &base.DatabaseError{Op: "count_enrollments", Err: err}
	}

	if count >= group.MaxParticipants {
		return errors.New("activity group is at maximum capacity")
	}

	// Insert the enrollment
	_, err = r.db.NewInsert().Model(enrollment).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "create", Err: err}
	}

	return nil
}

// FindByID retrieves a student enrollment by its ID
func (r *StudentEnrollmentRepository) FindByID(ctx context.Context, id interface{}) (*activities.StudentEnrollment, error) {
	enrollment := new(activities.StudentEnrollment)
	err := r.db.NewSelect().Model(enrollment).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return enrollment, nil
}

// FindByStudent retrieves all enrollments for a student
func (r *StudentEnrollmentRepository) FindByStudent(ctx context.Context, studentID int64) ([]*activities.StudentEnrollment, error) {
	var enrollments []*activities.StudentEnrollment
	err := r.db.NewSelect().
		Model(&enrollments).
		Where("student_id = ?", studentID).
		Order("enrollment_date DESC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_student", Err: err}
	}
	return enrollments, nil
}

// FindByActivityGroup retrieves all enrollments for an activity group
func (r *StudentEnrollmentRepository) FindByActivityGroup(ctx context.Context, activityGroupID int64) ([]*activities.StudentEnrollment, error) {
	var enrollments []*activities.StudentEnrollment
	err := r.db.NewSelect().
		Model(&enrollments).
		Where("activity_group_id = ?", activityGroupID).
		Order("enrollment_date ASC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_activity_group", Err: err}
	}
	return enrollments, nil
}

// FindByStudentAndActivityGroup retrieves an enrollment by student and activity group
func (r *StudentEnrollmentRepository) FindByStudentAndActivityGroup(ctx context.Context, studentID, activityGroupID int64) (*activities.StudentEnrollment, error) {
	enrollment := new(activities.StudentEnrollment)
	err := r.db.NewSelect().
		Model(enrollment).
		Where("student_id = ?", studentID).
		Where("activity_group_id = ?", activityGroupID).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_student_and_activity_group", Err: err}
	}
	return enrollment, nil
}

// FindByAttendanceStatus retrieves enrollments by attendance status
func (r *StudentEnrollmentRepository) FindByAttendanceStatus(ctx context.Context, status string) ([]*activities.StudentEnrollment, error) {
	var enrollments []*activities.StudentEnrollment
	err := r.db.NewSelect().
		Model(&enrollments).
		Where("attendance_status = ?", status).
		Order("enrollment_date DESC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_attendance_status", Err: err}
	}
	return enrollments, nil
}

// UpdateAttendanceStatus updates the attendance status of an enrollment
func (r *StudentEnrollmentRepository) UpdateAttendanceStatus(ctx context.Context, id int64, status string) error {
	_, err := r.db.NewUpdate().
		Model((*activities.StudentEnrollment)(nil)).
		Set("attendance_status = ?", status).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &base.DatabaseError{Op: "update_attendance_status", Err: err}
	}
	return nil
}

// CountByActivityGroup counts the number of enrollments for an activity group
func (r *StudentEnrollmentRepository) CountByActivityGroup(ctx context.Context, activityGroupID int64) (int, error) {
	count, err := r.db.NewSelect().
		Model((*activities.StudentEnrollment)(nil)).
		Where("activity_group_id = ?", activityGroupID).
		Count(ctx)

	if err != nil {
		return 0, &base.DatabaseError{Op: "count_by_activity_group", Err: err}
	}
	return count, nil
}

// Update updates an existing student enrollment
func (r *StudentEnrollmentRepository) Update(ctx context.Context, enrollment *activities.StudentEnrollment) error {
	if err := enrollment.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewUpdate().Model(enrollment).WherePK().Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "update", Err: err}
	}
	return nil
}

// Delete removes a student enrollment
func (r *StudentEnrollmentRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*activities.StudentEnrollment)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves student enrollments matching the filters
func (r *StudentEnrollmentRepository) List(ctx context.Context, filters map[string]interface{}) ([]*activities.StudentEnrollment, error) {
	var enrollments []*activities.StudentEnrollment
	query := r.db.NewSelect().Model(&enrollments)

	// Apply filters
	for key, value := range filters {
		query = query.Where("? = ?", bun.Ident(key), value)
	}

	// Execute the query
	err := query.Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "list", Err: err}
	}

	return enrollments, nil
}
