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
		ModelTableExpr("users.students AS student").
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
		ModelTableExpr("users.students AS student").
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

// FindByGroupIDs retrieves students by multiple group IDs
func (r *StudentRepository) FindByGroupIDs(ctx context.Context, groupIDs []int64) ([]*users.Student, error) {
	if len(groupIDs) == 0 {
		return []*users.Student{}, nil
	}

	var students []*users.Student
	err := r.db.NewSelect().
		Model(&students).
		ModelTableExpr("users.students AS student").
		Where("group_id IN (?)", bun.In(groupIDs)).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by group IDs",
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
		ModelTableExpr("users.students AS student").
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
			case "has_group":
				if boolValue, ok := value.(bool); ok && boolValue {
					filter.IsNotNull("group_id")
				} else if boolValue, ok := value.(bool); ok && !boolValue {
					filter.IsNull("group_id")
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

// CountWithOptions counts students matching the query options
func (r *StudentRepository) CountWithOptions(ctx context.Context, options *modelBase.QueryOptions) (int, error) {
	query := r.db.NewSelect().
		Model((*users.Student)(nil)).
		ModelTableExpr("users.students AS student").
		Column("student.id")

	// Apply query options with table alias
	if options != nil {
		if options.Filter != nil {
			options.Filter.WithTableAlias("student")
			query = options.Filter.ApplyToQuery(query)
		}
		// Apply sorting if needed (but not pagination for counting)
		if options.Sorting != nil {
			query = options.Sorting.ApplyToQuery(query)
		}
	}

	count, err := query.Count(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "count with options",
			Err: err,
		}
	}

	return count, nil
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
// Now queries through the guardians table and students_guardians join table
func (r *StudentRepository) FindByGuardianEmail(ctx context.Context, email string) ([]*users.Student, error) {
	var students []*users.Student
	err := r.db.NewSelect().
		Model(&students).
		ModelTableExpr(`users.students AS "student"`).
		Join(`INNER JOIN users.students_guardians AS "sg" ON "sg".student_id = "student".id`).
		Join(`INNER JOIN users.guardians AS "g" ON "g".id = "sg".guardian_id`).
		Where("LOWER(g.email) = LOWER(?)", email).
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
// Now queries through the guardians table and students_guardians join table
func (r *StudentRepository) FindByGuardianPhone(ctx context.Context, phone string) ([]*users.Student, error) {
	var students []*users.Student
	err := r.db.NewSelect().
		Model(&students).
		ModelTableExpr(`users.students AS "student"`).
		Join(`INNER JOIN users.students_guardians AS "sg" ON "sg".student_id = "student".id`).
		Join(`INNER JOIN users.guardians AS "g" ON "g".id = "sg".guardian_id`).
		Where("g.phone = ?", phone).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by guardian phone",
			Err: err,
		}
	}

	return students, nil
}

// FindByTeacherID retrieves students supervised by a teacher (through group assignments)
func (r *StudentRepository) FindByTeacherID(ctx context.Context, teacherID int64) ([]*users.Student, error) {
	// Define a result struct to handle the complex JOIN and mapping
	type studentWithPersonAndGroup struct {
		Student   *users.Student `bun:"student"`
		Person    *users.Person  `bun:"person"`
		GroupName string         `bun:"group_name"`
	}

	var results []*studentWithPersonAndGroup
	err := r.db.NewSelect().
		Model(&results).
		ModelTableExpr(`users.students AS "student"`).
		// Student columns with proper aliasing
		ColumnExpr(`"student".id AS "student__id", "student".created_at AS "student__created_at", "student".updated_at AS "student__updated_at"`).
		ColumnExpr(`"student".person_id AS "student__person_id", "student".school_class AS "student__school_class"`).
		ColumnExpr(`"student".group_id AS "student__group_id"`).
		// Person columns with proper aliasing
		ColumnExpr(`"person".id AS "person__id", "person".created_at AS "person__created_at", "person".updated_at AS "person__updated_at"`).
		ColumnExpr(`"person".first_name AS "person__first_name", "person".last_name AS "person__last_name"`).
		ColumnExpr(`"person".tag_id AS "person__tag_id", "person".account_id AS "person__account_id"`).
		// Group name for reference
		ColumnExpr(`"group".name AS "group_name"`).
		// JOINs to traverse the relationship chain
		Join(`INNER JOIN users.persons AS "person" ON "person".id = "student".person_id`).
		Join(`INNER JOIN education.groups AS "group" ON "group".id = "student".group_id`).
		Join(`INNER JOIN education.group_teacher AS "gt" ON "gt".group_id = "group".id`).
		// Filter by teacher ID and ensure student has a group assignment
		Where(`"gt".teacher_id = ? AND "student".group_id IS NOT NULL`, teacherID).
		// Use DISTINCT to avoid duplicates if a teacher supervises multiple groups with same student
		Distinct().
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by teacher ID",
			Err: err,
		}
	}

	// Extract students from results and map the person relationship
	students := make([]*users.Student, len(results))
	for i, result := range results {
		student := result.Student
		if result.Person != nil && result.Person.ID != 0 {
			student.Person = result.Person
		}
		students[i] = student
	}

	return students, nil
}

// FindByTeacherIDWithGroups retrieves students with group names supervised by a teacher
func (r *StudentRepository) FindByTeacherIDWithGroups(ctx context.Context, teacherID int64) ([]*users.StudentWithGroupInfo, error) {
	// Define a result struct to handle the complex JOIN and mapping
	type studentWithPersonAndGroup struct {
		Student   *users.Student `bun:"student"`
		Person    *users.Person  `bun:"person"`
		GroupName string         `bun:"group_name"`
	}

	var results []*studentWithPersonAndGroup
	err := r.db.NewSelect().
		Model(&results).
		ModelTableExpr(`users.students AS "student"`).
		// Student columns with proper aliasing
		ColumnExpr(`"student".id AS "student__id", "student".created_at AS "student__created_at", "student".updated_at AS "student__updated_at"`).
		ColumnExpr(`"student".person_id AS "student__person_id", "student".school_class AS "student__school_class"`).
		ColumnExpr(`"student".group_id AS "student__group_id"`).
		// Person columns with proper aliasing
		ColumnExpr(`"person".id AS "person__id", "person".created_at AS "person__created_at", "person".updated_at AS "person__updated_at"`).
		ColumnExpr(`"person".first_name AS "person__first_name", "person".last_name AS "person__last_name"`).
		ColumnExpr(`"person".tag_id AS "person__tag_id", "person".account_id AS "person__account_id"`).
		// Group name for reference
		ColumnExpr(`"group".name AS "group_name"`).
		// JOINs to traverse the relationship chain
		Join(`INNER JOIN users.persons AS "person" ON "person".id = "student".person_id`).
		Join(`INNER JOIN education.groups AS "group" ON "group".id = "student".group_id`).
		Join(`INNER JOIN education.group_teacher AS "gt" ON "gt".group_id = "group".id`).
		// Filter by teacher ID and ensure student has a group assignment
		Where(`"gt".teacher_id = ? AND "student".group_id IS NOT NULL`, teacherID).
		// Use DISTINCT to avoid duplicates if a teacher supervises multiple groups with same student
		Distinct().
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by teacher ID with groups",
			Err: err,
		}
	}

	// Extract students from results and map the person relationship with group info
	studentsWithGroups := make([]*users.StudentWithGroupInfo, len(results))
	for i, result := range results {
		student := result.Student
		if result.Person != nil && result.Person.ID != 0 {
			student.Person = result.Person
		}

		studentsWithGroups[i] = &users.StudentWithGroupInfo{
			Student:   student,
			GroupName: result.GroupName,
		}
	}

	return studentsWithGroups, nil
}
