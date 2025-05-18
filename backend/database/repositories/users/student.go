// backend/database/repositories/users/student.go
package users

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// StudentRepository implements users.StudentRepository interface
type StudentRepository struct {
	*base.Repository[*users.Student]
	db *bun.DB
}

// NewStudentRepository creates a new StudentRepository
func NewStudentRepository(db *bun.DB) users.StudentRepository {
	return &StudentRepository{
		Repository: base.NewRepository[*users.Student](db, "users.students", "Student"),
		db:         db,
	}
}

// FindByPersonID retrieves a student by their person ID
func (r *StudentRepository) FindByPersonID(ctx context.Context, personID int64) (*users.Student, error) {
	student := new(users.Student)
	err := r.db.NewSelect().
		Model(student).
		Where("person_id = ?", personID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by person ID",
			Err: err,
		}
	}

	return student, nil
}

// FindByGroupID retrieves students by their group ID
func (r *StudentRepository) FindByGroupID(ctx context.Context, groupID int64) ([]*users.Student, error) {
	var students []*users.Student
	err := r.db.NewSelect().
		Model(&students).
		Where("group_id = ?", groupID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by group ID",
			Err: err,
		}
	}

	return students, nil
}

// FindBySchoolClass retrieves students by their school class
func (r *StudentRepository) FindBySchoolClass(ctx context.Context, schoolClass string) ([]*users.Student, error) {
	var students []*users.Student
	err := r.db.NewSelect().
		Model(&students).
		Where("LOWER(school_class) = LOWER(?)", schoolClass).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by school class",
			Err: err,
		}
	}

	return students, nil
}

// UpdateLocation updates a student's location status
func (r *StudentRepository) UpdateLocation(ctx context.Context, id int64, location string) error {
	// First, get the student
	student, err := r.FindByID(ctx, id)
	if err != nil {
		return err
	}

	// Update location
	if err := student.SetLocation(location); err != nil {
		return err
	}

	// Save changes
	_, err = r.db.NewUpdate().
		Model(student).
		Column("bus", "in_house", "wc", "school_yard").
		WherePK().
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update location",
			Err: err,
		}
	}

	return nil
}

// AssignToGroup assigns a student to a group
func (r *StudentRepository) AssignToGroup(ctx context.Context, studentID int64, groupID int64) error {
	_, err := r.db.NewUpdate().
		Model((*users.Student)(nil)).
		Set("group_id = ?", groupID).
		Where("id = ?", studentID).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "assign to group",
			Err: err,
		}
	}

	return nil
}

// RemoveFromGroup removes a student from their group
func (r *StudentRepository) RemoveFromGroup(ctx context.Context, studentID int64) error {
	_, err := r.db.NewUpdate().
		Model((*users.Student)(nil)).
		Set("group_id = NULL").
		Where("id = ?", studentID).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "remove from group",
			Err: err,
		}
	}

	return nil
}

// Create overrides the base Create method to handle validation
func (r *StudentRepository) Create(ctx context.Context, student *users.Student) error {
	if student == nil {
		return fmt.Errorf("student cannot be nil")
	}

	// Validate student
	if err := student.Validate(); err != nil {
		return err
	}

	// Use the base Create method
	return r.Repository.Create(ctx, student)
}

// Update overrides the base Update method to handle validation
func (r *StudentRepository) Update(ctx context.Context, student *users.Student) error {
	if student == nil {
		return fmt.Errorf("student cannot be nil")
	}

	// Validate student
	if err := student.Validate(); err != nil {
		return err
	}

	// Use the base Update method
	return r.Repository.Update(ctx, student)
}

// Legacy method to maintain compatibility with old interface
func (r *StudentRepository) List(ctx context.Context, filters map[string]interface{}) ([]*users.Student, error) {
	// Convert old filter format to new QueryOptions
	options := modelBase.NewQueryOptions()
	filter := modelBase.NewFilter()

	for field, value := range filters {
		if value != nil {
			switch field {
			case "school_class_like":
				if strValue, ok := value.(string); ok {
					filter.ILike("school_class", "%"+strValue+"%")
				}
			case "guardian_name_like":
				if strValue, ok := value.(string); ok {
					filter.ILike("guardian_name", "%"+strValue+"%")
				}
			case "has_group":
				if boolValue, ok := value.(bool); ok && boolValue {
					filter.IsNotNull("group_id")
				} else if boolValue, ok := value.(bool); ok && !boolValue {
					filter.IsNull("group_id")
				}
			case "location":
				if strValue, ok := value.(string); ok {
					switch strValue {
					case "bus":
						filter.Equal("bus", true)
					case "in_house", "house":
						filter.Equal("in_house", true)
					case "wc", "bathroom":
						filter.Equal("wc", true)
					case "school_yard", "yard":
						filter.Equal("school_yard", true)
					case "unknown", "none", "":
						filter.Equal("bus", false).
							Equal("in_house", false).
							Equal("wc", false).
							Equal("school_yard", false)
					}
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

// ListWithOptions provides a type-safe way to list students with query options
func (r *StudentRepository) ListWithOptions(ctx context.Context, options *modelBase.QueryOptions) ([]*users.Student, error) {
	var students []*users.Student
	query := r.db.NewSelect().
		Model(&students).
		ModelTableExpr("users.students AS student")

	// Apply query options with table alias
	if options != nil {
		if options.Filter != nil {
			options.Filter.WithTableAlias("student")
		}
		query = options.ApplyToQuery(query)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list with options",
			Err: err,
		}
	}

	return students, nil
}

// FindWithPerson retrieves a student with their associated person data
func (r *StudentRepository) FindWithPerson(ctx context.Context, id int64) (*users.Student, error) {
	student := new(users.Student)
	err := r.db.NewSelect().
		Model(student).
		Relation("Person").
		Where("users.students.id = ?", id).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with person",
			Err: err,
		}
	}

	return student, nil
}

// FindByGuardianEmail finds students with a specific guardian email
func (r *StudentRepository) FindByGuardianEmail(ctx context.Context, email string) ([]*users.Student, error) {
	var students []*users.Student
	err := r.db.NewSelect().
		Model(&students).
		Where("LOWER(guardian_email) = LOWER(?)", email).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by guardian email",
			Err: err,
		}
	}

	return students, nil
}

// FindByGuardianPhone finds students with a specific guardian phone
func (r *StudentRepository) FindByGuardianPhone(ctx context.Context, phone string) ([]*users.Student, error) {
	var students []*users.Student
	err := r.db.NewSelect().
		Model(&students).
		Where("guardian_phone = ?", phone).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by guardian phone",
			Err: err,
		}
	}

	return students, nil
}
