package users

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres/base"
	modelBase "github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/users"
	userPort "github.com/moto-nrw/project-phoenix/internal/core/port/users"
	"github.com/uptrace/bun"
)

// TeacherRepository implements users.TeacherRepository interface
type TeacherRepository struct {
	*base.Repository[*users.Teacher]
	db *bun.DB
}

// NewTeacherRepository creates a new TeacherRepository
func NewTeacherRepository(db *bun.DB) userPort.TeacherRepository {
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
		ModelTableExpr(`users.teachers AS "teacher"`).
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

// FindByStaffIDs retrieves teachers by multiple staff IDs in a single query
// Returns a map of staff_id -> Teacher for efficient lookup
func (r *TeacherRepository) FindByStaffIDs(ctx context.Context, staffIDs []int64) (map[int64]*users.Teacher, error) {
	if len(staffIDs) == 0 {
		return make(map[int64]*users.Teacher), nil
	}

	var teachers []*users.Teacher
	err := r.db.NewSelect().
		Model(&teachers).
		ModelTableExpr(`users.teachers AS "teacher"`).
		Where(`"teacher".staff_id IN (?)`, bun.In(staffIDs)).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by staff IDs",
			Err: err,
		}
	}

	// Build map keyed by staff_id for O(1) lookups
	result := make(map[int64]*users.Teacher, len(teachers))
	for _, t := range teachers {
		result[t.StaffID] = t
	}

	return result, nil
}

// FindBySpecialization retrieves teachers by their specialization
func (r *TeacherRepository) FindBySpecialization(ctx context.Context, specialization string) ([]*users.Teacher, error) {
	var teachers []*users.Teacher
	err := r.db.NewSelect().
		Model(&teachers).
		ModelTableExpr(`users.teachers AS "teacher"`).
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
		ModelTableExpr(`users.teachers AS "teacher"`).
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
	options := modelBase.NewQueryOptions()
	filter := modelBase.NewFilter()

	for field, value := range filters {
		if value != nil {
			applyTeacherFilter(filter, field, value)
		}
	}

	options.Filter = filter
	return r.ListWithOptions(ctx, options)
}

// applyTeacherFilter applies a single filter based on field name
func applyTeacherFilter(filter *modelBase.Filter, field string, value interface{}) {
	switch field {
	case "specialization_like":
		applyTeacherStringLikeFilter(filter, "specialization", value)
	case "role_like":
		applyTeacherStringLikeFilter(filter, "role", value)
	case "has_qualifications":
		applyNullableFieldFilter(filter, "qualifications", value)
	default:
		filter.Equal(field, value)
	}
}

// applyTeacherStringLikeFilter applies LIKE filter for string fields
func applyTeacherStringLikeFilter(filter *modelBase.Filter, column string, value interface{}) {
	if strValue, ok := value.(string); ok {
		filter.ILike(column, "%"+strValue+"%")
	}
}

// ListWithOptions provides a type-safe way to list teachers with query options
func (r *TeacherRepository) ListWithOptions(ctx context.Context, options *modelBase.QueryOptions) ([]*users.Teacher, error) {
	var teachers []*users.Teacher
	query := r.db.NewSelect().
		Model(&teachers).
		ModelTableExpr(`users.teachers AS "teacher"`)

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
		ModelTableExpr(`users.teachers AS "teacher"`).
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
	// Use explicit JOINs similar to the group repository's FindWithRoom approach
	type teacherResult struct {
		Teacher *users.Teacher `bun:"teacher"`
		Staff   *users.Staff   `bun:"staff"`
		Person  *users.Person  `bun:"person"`
	}

	result := &teacherResult{
		Teacher: new(users.Teacher),
		Staff:   new(users.Staff),
		Person:  new(users.Person),
	}

	err := r.db.NewSelect().
		Model(result).
		ModelTableExpr(`users.teachers AS "teacher"`).
		// Teacher columns with proper aliasing
		ColumnExpr(`"teacher".id AS "teacher__id", "teacher".created_at AS "teacher__created_at", "teacher".updated_at AS "teacher__updated_at"`).
		ColumnExpr(`"teacher".staff_id AS "teacher__staff_id", "teacher".specialization AS "teacher__specialization"`).
		ColumnExpr(`"teacher".role AS "teacher__role", "teacher".qualifications AS "teacher__qualifications"`).
		// Staff columns
		ColumnExpr(`"staff".id AS "staff__id", "staff".created_at AS "staff__created_at", "staff".updated_at AS "staff__updated_at"`).
		ColumnExpr(`"staff".person_id AS "staff__person_id", "staff".staff_notes AS "staff__staff_notes"`).
		// Person columns
		ColumnExpr(`"person".id AS "person__id", "person".created_at AS "person__created_at", "person".updated_at AS "person__updated_at"`).
		ColumnExpr(`"person".first_name AS "person__first_name", "person".last_name AS "person__last_name"`).
		ColumnExpr(`"person".tag_id AS "person__tag_id", "person".account_id AS "person__account_id"`).
		// JOINs
		Join(`INNER JOIN users.staff AS "staff" ON "staff".id = "teacher".staff_id`).
		Join(`INNER JOIN users.persons AS "person" ON "person".id = "staff".person_id`).
		Where(`"teacher".id = ?`, id).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with staff and person",
			Err: err,
		}
	}

	// Map result to teacher
	teacher := result.Teacher
	if result.Staff != nil && result.Staff.ID != 0 {
		teacher.Staff = result.Staff
		if result.Person != nil && result.Person.ID != 0 {
			teacher.Staff.Person = result.Person
		}
	}

	return teacher, nil
}

// ListAllWithStaffAndPerson retrieves all teachers with their staff and person data in a single query
func (r *TeacherRepository) ListAllWithStaffAndPerson(ctx context.Context) ([]*users.Teacher, error) {
	type teacherResult struct {
		Teacher *users.Teacher `bun:"teacher"`
		Staff   *users.Staff   `bun:"staff"`
		Person  *users.Person  `bun:"person"`
	}

	var results []teacherResult

	err := r.db.NewSelect().
		Model(&results).
		ModelTableExpr(`users.teachers AS "teacher"`).
		// Teacher columns with proper aliasing
		ColumnExpr(`"teacher".id AS "teacher__id", "teacher".created_at AS "teacher__created_at", "teacher".updated_at AS "teacher__updated_at"`).
		ColumnExpr(`"teacher".staff_id AS "teacher__staff_id", "teacher".specialization AS "teacher__specialization"`).
		ColumnExpr(`"teacher".role AS "teacher__role", "teacher".qualifications AS "teacher__qualifications"`).
		// Staff columns
		ColumnExpr(`"staff".id AS "staff__id", "staff".created_at AS "staff__created_at", "staff".updated_at AS "staff__updated_at"`).
		ColumnExpr(`"staff".person_id AS "staff__person_id", "staff".staff_notes AS "staff__staff_notes"`).
		// Person columns
		ColumnExpr(`"person".id AS "person__id", "person".created_at AS "person__created_at", "person".updated_at AS "person__updated_at"`).
		ColumnExpr(`"person".first_name AS "person__first_name", "person".last_name AS "person__last_name"`).
		ColumnExpr(`"person".tag_id AS "person__tag_id", "person".account_id AS "person__account_id"`).
		// JOINs
		Join(`INNER JOIN users.staff AS "staff" ON "staff".id = "teacher".staff_id`).
		Join(`INNER JOIN users.persons AS "person" ON "person".id = "staff".person_id`).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list all with staff and person",
			Err: err,
		}
	}

	// Convert results to Teacher objects with Staff and Person attached
	teachers := make([]*users.Teacher, len(results))
	for i, result := range results {
		teachers[i] = result.Teacher
		if result.Staff != nil && result.Staff.ID != 0 {
			result.Teacher.Staff = result.Staff
			if result.Person != nil && result.Person.ID != 0 {
				result.Staff.Person = result.Person
			}
		}
	}

	return teachers, nil
}
