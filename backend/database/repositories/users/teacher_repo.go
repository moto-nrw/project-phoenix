package users

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// TeacherRepository implements users.TeacherRepository
type TeacherRepository struct {
	db *bun.DB
}

// NewTeacherRepository creates a new teacher repository
func NewTeacherRepository(db *bun.DB) users.TeacherRepository {
	return &TeacherRepository{db: db}
}

// Create inserts a new teacher into the database
func (r *TeacherRepository) Create(ctx context.Context, teacher *users.Teacher) error {
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
func (r *TeacherRepository) FindByID(ctx context.Context, id interface{}) (*users.Teacher, error) {
	teacher := new(users.Teacher)
	err := r.db.NewSelect().Model(teacher).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return teacher, nil
}

// FindByPersonID retrieves a teacher by their person ID
func (r *TeacherRepository) FindByPersonID(ctx context.Context, personID int64) (*users.Teacher, error) {
	teacher := new(users.Teacher)
	err := r.db.NewSelect().Model(teacher).Where("person_id = ?", personID).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_person_id", Err: err}
	}
	return teacher, nil
}

// FindBySpecialization retrieves teachers by their specialization
func (r *TeacherRepository) FindBySpecialization(ctx context.Context, specialization string) ([]*users.Teacher, error) {
	var teachers []*users.Teacher
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
func (r *TeacherRepository) FindByRole(ctx context.Context, role string) ([]*users.Teacher, error) {
	var teachers []*users.Teacher
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
func (r *TeacherRepository) FindWithPerson(ctx context.Context, id int64) (*users.Teacher, error) {
	teacher := new(users.Teacher)
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
func (r *TeacherRepository) Update(ctx context.Context, teacher *users.Teacher) error {
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
func (r *TeacherRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*users.Teacher)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves teachers matching the filters
func (r *TeacherRepository) List(ctx context.Context, filters map[string]interface{}) ([]*users.Teacher, error) {
	var teachers []*users.Teacher
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