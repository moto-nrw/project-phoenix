package users

import (
	"context"
	"errors"
	"strings"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// StudentRepository implements users.StudentRepository
type StudentRepository struct {
	db *bun.DB
}

// NewStudentRepository creates a new student repository
func NewStudentRepository(db *bun.DB) users.StudentRepository {
	return &StudentRepository{db: db}
}

// Create inserts a new student into the database
func (r *StudentRepository) Create(ctx context.Context, student *users.Student) error {
	if err := student.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewInsert().Model(student).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "create", Err: err}
	}

	return nil
}

// FindByID retrieves a student by their ID
func (r *StudentRepository) FindByID(ctx context.Context, id interface{}) (*users.Student, error) {
	student := new(users.Student)
	err := r.db.NewSelect().Model(student).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return student, nil
}

// FindByPersonID retrieves a student by their person ID
func (r *StudentRepository) FindByPersonID(ctx context.Context, personID int64) (*users.Student, error) {
	student := new(users.Student)
	err := r.db.NewSelect().Model(student).Where("person_id = ?", personID).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_person_id", Err: err}
	}
	return student, nil
}

// FindBySchoolClass retrieves students by school class
func (r *StudentRepository) FindBySchoolClass(ctx context.Context, schoolClass string) ([]*users.Student, error) {
	var students []*users.Student
	err := r.db.NewSelect().
		Model(&students).
		Where("school_class = ?", schoolClass).
		Order("id ASC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_school_class", Err: err}
	}
	return students, nil
}

// FindByGroup retrieves students by group
func (r *StudentRepository) FindByGroup(ctx context.Context, groupID int64) ([]*users.Student, error) {
	var students []*users.Student
	err := r.db.NewSelect().
		Model(&students).
		Where("group_id = ?", groupID).
		Order("id ASC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_group", Err: err}
	}
	return students, nil
}

// FindByGuardianName retrieves students by guardian name (partial match)
func (r *StudentRepository) FindByGuardianName(ctx context.Context, guardianName string) ([]*users.Student, error) {
	var students []*users.Student
	err := r.db.NewSelect().
		Model(&students).
		Where("guardian_name ILIKE ?", "%"+guardianName+"%").
		Order("id ASC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_guardian_name", Err: err}
	}
	return students, nil
}

// FindInBus retrieves all students currently in the bus
func (r *StudentRepository) FindInBus(ctx context.Context) ([]*users.Student, error) {
	var students []*users.Student
	err := r.db.NewSelect().
		Model(&students).
		Where("bus = ?", true).
		Order("id ASC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_in_bus", Err: err}
	}
	return students, nil
}

// FindInHouse retrieves all students currently in the house
func (r *StudentRepository) FindInHouse(ctx context.Context) ([]*users.Student, error) {
	var students []*users.Student
	err := r.db.NewSelect().
		Model(&students).
		Where("in_house = ?", true).
		Order("id ASC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_in_house", Err: err}
	}
	return students, nil
}

// FindInWC retrieves all students currently in the WC
func (r *StudentRepository) FindInWC(ctx context.Context) ([]*users.Student, error) {
	var students []*users.Student
	err := r.db.NewSelect().
		Model(&students).
		Where("wc = ?", true).
		Order("id ASC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_in_wc", Err: err}
	}
	return students, nil
}

// FindInSchoolYard retrieves all students currently in the school yard
func (r *StudentRepository) FindInSchoolYard(ctx context.Context) ([]*users.Student, error) {
	var students []*users.Student
	err := r.db.NewSelect().
		Model(&students).
		Where("school_yard = ?", true).
		Order("id ASC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_in_school_yard", Err: err}
	}
	return students, nil
}

// UpdateLocation updates a student's location
func (r *StudentRepository) UpdateLocation(ctx context.Context, id int64, location string) error {
	// Create a map to hold the updates
	updates := map[string]bool{
		"bus":         false,
		"in_house":    false,
		"wc":          false,
		"school_yard": false,
	}

	// Set the correct location to true based on the input
	switch strings.ToLower(location) {
	case "bus":
		updates["bus"] = true
	case "house", "in_house":
		updates["in_house"] = true
	case "wc", "bathroom":
		updates["wc"] = true
	case "yard", "school_yard":
		updates["school_yard"] = true
	case "":
		// No changes needed, all locations remain false
	default:
		return errors.New("invalid location: must be bus, house, wc, or yard")
	}

	// Apply the updates
	_, err := r.db.NewUpdate().
		Model((*users.Student)(nil)).
		Set("bus = ?", updates["bus"]).
		Set("in_house = ?", updates["in_house"]).
		Set("wc = ?", updates["wc"]).
		Set("school_yard = ?", updates["school_yard"]).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &base.DatabaseError{Op: "update_location", Err: err}
	}
	return nil
}

// FindWithPerson retrieves a student with their associated person data
func (r *StudentRepository) FindWithPerson(ctx context.Context, id int64) (*users.Student, error) {
	student := new(users.Student)
	err := r.db.NewSelect().
		Model(student).
		Relation("Person").
		Where("student.id = ?", id).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_with_person", Err: err}
	}
	return student, nil
}

// Update updates an existing student
func (r *StudentRepository) Update(ctx context.Context, student *users.Student) error {
	if err := student.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewUpdate().Model(student).WherePK().Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "update", Err: err}
	}
	return nil
}

// Delete removes a student
func (r *StudentRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*users.Student)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves students matching the filters
func (r *StudentRepository) List(ctx context.Context, filters map[string]interface{}) ([]*users.Student, error) {
	var students []*users.Student
	query := r.db.NewSelect().Model(&students)

	// Apply filters
	for key, value := range filters {
		query = query.Where("? = ?", bun.Ident(key), value)
	}

	// Execute the query
	err := query.Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "list", Err: err}
	}

	return students, nil
}