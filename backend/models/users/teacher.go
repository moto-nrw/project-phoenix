package users

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// Teacher represents a teacher entity in the system
type Teacher struct {
	base.Model
	PersonID       int64  `bun:"person_id,notnull" json:"person_id"`
	Specialization string `bun:"specialization,notnull" json:"specialization"`
	Role           string `bun:"role" json:"role,omitempty"`
	IsPasswordOTP  bool   `bun:"is_password_otp,default:false" json:"is_password_otp"`
	Qualifications string `bun:"qualifications" json:"qualifications,omitempty"`

	// Relations
	Person *Person `bun:"rel:belongs-to,join:person_id=id" json:"person,omitempty"`
}

// TableName returns the table name for the Teacher model
func (t *Teacher) TableName() string {
	return "users.teachers"
}

// GetID returns the teacher ID
func (t *Teacher) GetID() interface{} {
	return t.ID
}

// GetCreatedAt returns the creation timestamp
func (t *Teacher) GetCreatedAt() time.Time {
	return t.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (t *Teacher) GetUpdatedAt() time.Time {
	return t.UpdatedAt
}

// Validate validates the teacher fields
func (t *Teacher) Validate() error {
	if t.PersonID <= 0 {
		return errors.New("person ID is required")
	}

	if strings.TrimSpace(t.Specialization) == "" {
		return errors.New("specialization is required")
	}

	return nil
}

// BeforeAppend sets default values before saving to the database
func (t *Teacher) BeforeAppend() error {
	// Call parent's BeforeAppend to set timestamps
	if err := t.Model.BeforeAppend(); err != nil {
		return err
	}

	// Trim whitespace
	t.Specialization = strings.TrimSpace(t.Specialization)
	t.Role = strings.TrimSpace(t.Role)
	t.Qualifications = strings.TrimSpace(t.Qualifications)

	return nil
}

// TeacherRepository defines operations for working with teachers
type TeacherRepository interface {
	base.Repository[*Teacher]
	FindByPersonID(ctx context.Context, personID int64) (*Teacher, error)
	FindBySpecialization(ctx context.Context, specialization string) ([]*Teacher, error)
	FindByRole(ctx context.Context, role string) ([]*Teacher, error)
	FindWithPerson(ctx context.Context, id int64) (*Teacher, error)
}

// DefaultTeacherRepository is the default implementation of TeacherRepository
type DefaultTeacherRepository struct {
	db *bun.DB
}

// NewTeacherRepository creates a new teacher repository
func NewTeacherRepository(db *bun.DB) TeacherRepository {
	return &DefaultTeacherRepository{db: db}
}

// Create inserts a new teacher into the database
func (r *DefaultTeacherRepository) Create(ctx context.Context, teacher *Teacher) error {
	if err := teacher.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewInsert().Model(teacher).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "create", Err: err}
	}

	return nil
}

// FindByID retrieves a teacher by their ID
func (r *DefaultTeacherRepository) FindByID(ctx context.Context, id interface{}) (*Teacher, error) {
	teacher := new(Teacher)
	err := r.db.NewSelect().Model(teacher).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return teacher, nil
}

// FindByPersonID retrieves a teacher by their person ID
func (r *DefaultTeacherRepository) FindByPersonID(ctx context.Context, personID int64) (*Teacher, error) {
	teacher := new(Teacher)
	err := r.db.NewSelect().Model(teacher).Where("person_id = ?", personID).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_person_id", Err: err}
	}
	return teacher, nil
}

// FindBySpecialization retrieves teachers by their specialization
func (r *DefaultTeacherRepository) FindBySpecialization(ctx context.Context, specialization string) ([]*Teacher, error) {
	var teachers []*Teacher
	err := r.db.NewSelect().
		Model(&teachers).
		Where("specialization ILIKE ?", "%"+specialization+"%").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_specialization", Err: err}
	}
	return teachers, nil
}

// FindByRole retrieves teachers by their role
func (r *DefaultTeacherRepository) FindByRole(ctx context.Context, role string) ([]*Teacher, error) {
	var teachers []*Teacher
	err := r.db.NewSelect().
		Model(&teachers).
		Where("role ILIKE ?", "%"+role+"%").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_role", Err: err}
	}
	return teachers, nil
}

// FindWithPerson retrieves a teacher with their associated person data
func (r *DefaultTeacherRepository) FindWithPerson(ctx context.Context, id int64) (*Teacher, error) {
	teacher := new(Teacher)
	err := r.db.NewSelect().
		Model(teacher).
		Relation("Person").
		Where("teacher.id = ?", id).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_with_person", Err: err}
	}
	return teacher, nil
}

// Update updates an existing teacher
func (r *DefaultTeacherRepository) Update(ctx context.Context, teacher *Teacher) error {
	if err := teacher.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewUpdate().Model(teacher).WherePK().Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "update", Err: err}
	}
	return nil
}

// Delete removes a teacher
func (r *DefaultTeacherRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*Teacher)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves teachers matching the filters
func (r *DefaultTeacherRepository) List(ctx context.Context, filters map[string]interface{}) ([]*Teacher, error) {
	var teachers []*Teacher
	query := r.db.NewSelect().Model(&teachers)

	// Apply filters
	for key, value := range filters {
		query = query.Where("? = ?", bun.Ident(key), value)
	}

	// Execute the query
	err := query.Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "list", Err: err}
	}

	return teachers, nil
}
