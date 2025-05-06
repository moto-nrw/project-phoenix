package activities

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// StudentEnrollment represents a student enrollment in an activity group
type StudentEnrollment struct {
	base.Model
	StudentID        int64     `bun:"student_id,notnull" json:"student_id"`
	ActivityGroupID  int64     `bun:"activity_group_id,notnull" json:"activity_group_id"`
	EnrollmentDate   time.Time `bun:"enrollment_date,notnull" json:"enrollment_date"`
	AttendanceStatus string    `bun:"attendance_status" json:"attendance_status,omitempty"`

	// Relations
	Student       *users.Student `bun:"rel:belongs-to,join:student_id=id" json:"student,omitempty"`
	ActivityGroup *Group         `bun:"rel:belongs-to,join:activity_group_id=id" json:"activity_group,omitempty"`
}

// TableName returns the table name for the StudentEnrollment model
func (se *StudentEnrollment) TableName() string {
	return "activities.student_enrollments"
}

// GetID returns the student enrollment ID
func (se *StudentEnrollment) GetID() interface{} {
	return se.ID
}

// GetCreatedAt returns the creation timestamp
func (se *StudentEnrollment) GetCreatedAt() time.Time {
	return se.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (se *StudentEnrollment) GetUpdatedAt() time.Time {
	return se.UpdatedAt
}

// Validate validates the student enrollment fields
func (se *StudentEnrollment) Validate() error {
	if se.StudentID <= 0 {
		return errors.New("student ID is required")
	}

	if se.ActivityGroupID <= 0 {
		return errors.New("activity group ID is required")
	}

	if se.EnrollmentDate.IsZero() {
		return errors.New("enrollment date is required")
	}

	return nil
}

// BeforeAppend sets default values before saving to the database
func (se *StudentEnrollment) BeforeAppend() error {
	// Call parent's BeforeAppend to set timestamps
	if err := se.Model.BeforeAppend(); err != nil {
		return err
	}

	// Set default enrollment date if not provided
	if se.EnrollmentDate.IsZero() {
		se.EnrollmentDate = time.Now()
	}

	return nil
}

// StudentEnrollmentRepository defines operations for working with student enrollments
type StudentEnrollmentRepository interface {
	base.Repository[*StudentEnrollment]
	FindByStudent(ctx context.Context, studentID int64) ([]*StudentEnrollment, error)
	FindByActivityGroup(ctx context.Context, activityGroupID int64) ([]*StudentEnrollment, error)
	FindByStudentAndActivityGroup(ctx context.Context, studentID, activityGroupID int64) (*StudentEnrollment, error)
	FindByAttendanceStatus(ctx context.Context, status string) ([]*StudentEnrollment, error)
	UpdateAttendanceStatus(ctx context.Context, id int64, status string) error
	CountByActivityGroup(ctx context.Context, activityGroupID int64) (int, error)
}

// DefaultStudentEnrollmentRepository is the default implementation of StudentEnrollmentRepository
type DefaultStudentEnrollmentRepository struct {
	db *bun.DB
}

// NewStudentEnrollmentRepository creates a new student enrollment repository
func NewStudentEnrollmentRepository(db *bun.DB) StudentEnrollmentRepository {
	return &DefaultStudentEnrollmentRepository{db: db}
}

// Create inserts a new student enrollment into the database
func (r *DefaultStudentEnrollmentRepository) Create(ctx context.Context, enrollment *StudentEnrollment) error {
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
func (r *DefaultStudentEnrollmentRepository) FindByID(ctx context.Context, id interface{}) (*StudentEnrollment, error) {
	enrollment := new(StudentEnrollment)
	err := r.db.NewSelect().Model(enrollment).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return enrollment, nil
}

// FindByStudent retrieves all enrollments for a student
func (r *DefaultStudentEnrollmentRepository) FindByStudent(ctx context.Context, studentID int64) ([]*StudentEnrollment, error) {
	var enrollments []*StudentEnrollment
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
func (r *DefaultStudentEnrollmentRepository) FindByActivityGroup(ctx context.Context, activityGroupID int64) ([]*StudentEnrollment, error) {
	var enrollments []*StudentEnrollment
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
func (r *DefaultStudentEnrollmentRepository) FindByStudentAndActivityGroup(ctx context.Context, studentID, activityGroupID int64) (*StudentEnrollment, error) {
	enrollment := new(StudentEnrollment)
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
func (r *DefaultStudentEnrollmentRepository) FindByAttendanceStatus(ctx context.Context, status string) ([]*StudentEnrollment, error) {
	var enrollments []*StudentEnrollment
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
func (r *DefaultStudentEnrollmentRepository) UpdateAttendanceStatus(ctx context.Context, id int64, status string) error {
	_, err := r.db.NewUpdate().
		Model((*StudentEnrollment)(nil)).
		Set("attendance_status = ?", status).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &base.DatabaseError{Op: "update_attendance_status", Err: err}
	}
	return nil
}

// CountByActivityGroup counts the number of enrollments for an activity group
func (r *DefaultStudentEnrollmentRepository) CountByActivityGroup(ctx context.Context, activityGroupID int64) (int, error) {
	count, err := r.db.NewSelect().
		Model((*StudentEnrollment)(nil)).
		Where("activity_group_id = ?", activityGroupID).
		Count(ctx)

	if err != nil {
		return 0, &base.DatabaseError{Op: "count_by_activity_group", Err: err}
	}
	return count, nil
}

// Update updates an existing student enrollment
func (r *DefaultStudentEnrollmentRepository) Update(ctx context.Context, enrollment *StudentEnrollment) error {
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
func (r *DefaultStudentEnrollmentRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*StudentEnrollment)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves student enrollments matching the filters
func (r *DefaultStudentEnrollmentRepository) List(ctx context.Context, filters map[string]interface{}) ([]*StudentEnrollment, error) {
	var enrollments []*StudentEnrollment
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
