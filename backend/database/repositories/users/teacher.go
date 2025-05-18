// backend/database/repositories/users/teacher.go
package users

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// TeacherRepository implements users.TeacherRepository interface
type TeacherRepository struct {
	*base.Repository[*users.Teacher]
	db *bun.DB
}

// NewTeacherRepository creates a new TeacherRepository
func NewTeacherRepository(db *bun.DB) users.TeacherRepository {
	return &TeacherRepository{
		Repository: base.NewRepository[*users.Teacher](db, "users.teachers", "Teacher"),
		db:         db,
	}
}

// FindByStaffID retrieves a teacher by their staff ID
func (r *TeacherRepository) FindByStaffID(ctx context.Context, staffID int64) (*users.Teacher, error) {
	teacher := new(users.Teacher)
	err := r.db.NewSelect().
		Model(teacher).
		ModelTableExpr("users.teachers AS teacher").
		Where("staff_id = ?", staffID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by staff ID",
			Err: err,
		}
	}

	return teacher, nil
}

// FindBySpecialization retrieves teachers by their specialization
func (r *TeacherRepository) FindBySpecialization(ctx context.Context, specialization string) ([]*users.Teacher, error) {
	var teachers []*users.Teacher
	err := r.db.NewSelect().
		Model(&teachers).
		ModelTableExpr("users.teachers AS teacher").
		Where("LOWER(specialization) = LOWER(?)", specialization).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by specialization",
			Err: err,
		}
	}

	return teachers, nil
}

// FindByGroupID retrieves teachers assigned to a group
func (r *TeacherRepository) FindByGroupID(ctx context.Context, groupID int64) ([]*users.Teacher, error) {
	var teachers []*users.Teacher
	err := r.db.NewSelect().
		Model(&teachers).
		ModelTableExpr("users.teachers AS teacher").
		Join("JOIN education.group_teacher gt ON gt.teacher_id = teacher.id").
		Where("gt.group_id = ?", groupID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by group ID",
			Err: err,
		}
	}

	return teachers, nil
}

// UpdateQualifications updates a teacher's qualifications
func (r *TeacherRepository) UpdateQualifications(ctx context.Context, id int64, qualifications string) error {
	_, err := r.db.NewUpdate().
		Model((*users.Teacher)(nil)).
		ModelTableExpr("users.teachers AS teacher").
		Set("qualifications = ?", qualifications).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update qualifications",
			Err: err,
		}
	}

	return nil
}

// Create overrides the base Create method to handle validation
func (r *TeacherRepository) Create(ctx context.Context, teacher *users.Teacher) error {
	if teacher == nil {
		return fmt.Errorf("teacher cannot be nil")
	}

	// Validate teacher
	if err := teacher.Validate(); err != nil {
		return err
	}

	// Use the base Create method
	return r.Repository.Create(ctx, teacher)
}

// Update overrides the base Update method to handle validation
func (r *TeacherRepository) Update(ctx context.Context, teacher *users.Teacher) error {
	if teacher == nil {
		return fmt.Errorf("teacher cannot be nil")
	}

	// Validate teacher
	if err := teacher.Validate(); err != nil {
		return err
	}

	// Use the base Update method
	return r.Repository.Update(ctx, teacher)
}

// Legacy method to maintain compatibility with old interface
func (r *TeacherRepository) List(ctx context.Context, filters map[string]interface{}) ([]*users.Teacher, error) {
	// Convert old filter format to new QueryOptions
	options := modelBase.NewQueryOptions()
	filter := modelBase.NewFilter()

	for field, value := range filters {
		if value != nil {
			switch field {
			case "specialization_like":
				if strValue, ok := value.(string); ok {
					filter.ILike("specialization", "%"+strValue+"%")
				}
			case "role_like":
				if strValue, ok := value.(string); ok {
					filter.ILike("role", "%"+strValue+"%")
				}
			case "has_qualifications":
				if boolValue, ok := value.(bool); ok && boolValue {
					filter.IsNotNull("qualifications")
				} else if boolValue, ok := value.(bool); ok && !boolValue {
					filter.IsNull("qualifications")
				}
			default:
				// Default to exact match for other fields
				filter.Equal(field, value)
			}
		}
	}

	options.Filter = filter

	return r.ListWithOptions(ctx, options)
}

// ListWithOptions provides a type-safe way to list teachers with query options
func (r *TeacherRepository) ListWithOptions(ctx context.Context, options *modelBase.QueryOptions) ([]*users.Teacher, error) {
	var teachers []*users.Teacher
	query := r.db.NewSelect().
		Model(&teachers).
		ModelTableExpr("users.teachers AS teacher")

	// Apply query options
	if options != nil {
		query = options.ApplyToQuery(query)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list with options",
			Err: err,
		}
	}

	return teachers, nil
}

// FindWithStaff retrieves a teacher with their associated staff data
func (r *TeacherRepository) FindWithStaff(ctx context.Context, id int64) (*users.Teacher, error) {
	teacher := new(users.Teacher)
	err := r.db.NewSelect().
		Model(teacher).
		ModelTableExpr("users.teachers AS teacher").
		Relation("Staff").
		Where("id = ?", id).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with staff",
			Err: err,
		}
	}

	return teacher, nil
}

// FindWithStaffAndPerson retrieves a teacher with their associated staff and person data
func (r *TeacherRepository) FindWithStaffAndPerson(ctx context.Context, id int64) (*users.Teacher, error) {
	teacher := new(users.Teacher)
	err := r.db.NewSelect().
		Model(teacher).
		ModelTableExpr("users.teachers AS teacher").
		Relation("Staff").
		Relation("Staff.Person").
		Where("id = ?", id).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with staff and person",
			Err: err,
		}
	}

	return teacher, nil
}
