package users

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/uptrace/bun"
)

// Student represents a student entity in the system
type Student struct {
	base.Model
	PersonID        int64  `bun:"person_id,notnull,unique" json:"person_id"`
	SchoolClass     string `bun:"school_class,notnull" json:"school_class"`
	Bus             bool   `bun:"bus,notnull,default:false" json:"bus"`
	InHouse         bool   `bun:"in_house,notnull,default:false" json:"in_house"`
	WC              bool   `bun:"wc,notnull,default:false" json:"wc"`
	SchoolYard      bool   `bun:"school_yard,notnull,default:false" json:"school_yard"`
	GuardianName    string `bun:"guardian_name,notnull" json:"guardian_name"`
	GuardianContact string `bun:"guardian_contact,notnull" json:"guardian_contact"`
	GuardianEmail   string `bun:"guardian_email" json:"guardian_email,omitempty"`
	GuardianPhone   string `bun:"guardian_phone" json:"guardian_phone,omitempty"`
	GroupID         *int64 `bun:"group_id" json:"group_id,omitempty"`

	// Relations
	Person *Person          `bun:"rel:belongs-to,join:person_id=id" json:"person,omitempty"`
	Group  *education.Group `bun:"rel:belongs-to,join:group_id=id" json:"group,omitempty"`
}

// TableName returns the table name for the Student model
func (s *Student) TableName() string {
	return "users.students"
}

// GetID returns the student ID
func (s *Student) GetID() interface{} {
	return s.ID
}

// GetCreatedAt returns the creation timestamp
func (s *Student) GetCreatedAt() time.Time {
	return s.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (s *Student) GetUpdatedAt() time.Time {
	return s.UpdatedAt
}

// Validate validates the student fields
func (s *Student) Validate() error {
	if s.PersonID <= 0 {
		return errors.New("person ID is required")
	}

	if strings.TrimSpace(s.SchoolClass) == "" {
		return errors.New("school class is required")
	}

	if strings.TrimSpace(s.GuardianName) == "" {
		return errors.New("guardian name is required")
	}

	if strings.TrimSpace(s.GuardianContact) == "" {
		return errors.New("guardian contact is required")
	}

	// Validate guardian email if provided
	if s.GuardianEmail != "" {
		emailRegex := regexp.MustCompile(`^[A-Za-z0-9._%-]+@[A-Za-z0-9.-]+[.][A-Za-z]+$`)
		if !emailRegex.MatchString(s.GuardianEmail) {
			return errors.New("invalid guardian email format")
		}
	}

	// Validate guardian phone if provided
	if s.GuardianPhone != "" {
		phoneRegex := regexp.MustCompile(`^(\+[0-9]{1,3}\s?)?[0-9\s-]{7,15}$`)
		if !phoneRegex.MatchString(s.GuardianPhone) {
			return errors.New("invalid guardian phone format")
		}
	}

	// Validate location constraint
	locationCount := 0
	if s.Bus {
		locationCount++
	}
	if s.InHouse {
		locationCount++
	}
	if s.WC {
		locationCount++
	}
	if s.SchoolYard {
		locationCount++
	}

	if locationCount > 1 {
		return errors.New("student can only be in one location at a time")
	}

	return nil
}

// BeforeAppend sets default values before saving to the database
func (s *Student) BeforeAppend() error {
	// Call parent's BeforeAppend to set timestamps
	if err := s.Model.BeforeAppend(); err != nil {
		return err
	}

	// Trim whitespace
	s.SchoolClass = strings.TrimSpace(s.SchoolClass)
	s.GuardianName = strings.TrimSpace(s.GuardianName)
	s.GuardianContact = strings.TrimSpace(s.GuardianContact)
	s.GuardianEmail = strings.TrimSpace(s.GuardianEmail)
	s.GuardianPhone = strings.TrimSpace(s.GuardianPhone)

	return nil
}

// IsInBus returns whether the student is currently in the bus
func (s *Student) IsInBus() bool {
	return s.Bus
}

// IsInHouse returns whether the student is currently in the house
func (s *Student) IsInHouse() bool {
	return s.InHouse
}

// IsInWC returns whether the student is currently in the WC
func (s *Student) IsInWC() bool {
	return s.WC
}

// IsInSchoolYard returns whether the student is currently in the school yard
func (s *Student) IsInSchoolYard() bool {
	return s.SchoolYard
}

// SetLocation sets the student's location
// Only one location can be active at a time
func (s *Student) SetLocation(location string) error {
	// Reset all locations first
	s.Bus = false
	s.InHouse = false
	s.WC = false
	s.SchoolYard = false

	// Set the specified location
	switch strings.ToLower(location) {
	case "bus":
		s.Bus = true
	case "house", "in_house":
		s.InHouse = true
	case "wc", "bathroom":
		s.WC = true
	case "yard", "school_yard":
		s.SchoolYard = true
	case "":
		// No location specified (all remain false)
	default:
		return errors.New("invalid location: must be bus, house, wc, or yard")
	}

	return nil
}

// GetCurrentLocation returns the student's current location as a string
func (s *Student) GetCurrentLocation() string {
	if s.Bus {
		return "bus"
	}
	if s.InHouse {
		return "house"
	}
	if s.WC {
		return "wc"
	}
	if s.SchoolYard {
		return "yard"
	}
	return "unknown"
}

// StudentRepository defines operations for working with students
type StudentRepository interface {
	base.Repository[*Student]
	FindByPersonID(ctx context.Context, personID int64) (*Student, error)
	FindBySchoolClass(ctx context.Context, schoolClass string) ([]*Student, error)
	FindByGroup(ctx context.Context, groupID int64) ([]*Student, error)
	FindByGuardianName(ctx context.Context, guardianName string) ([]*Student, error)
	FindInBus(ctx context.Context) ([]*Student, error)
	FindInHouse(ctx context.Context) ([]*Student, error)
	FindInWC(ctx context.Context) ([]*Student, error)
	FindInSchoolYard(ctx context.Context) ([]*Student, error)
	UpdateLocation(ctx context.Context, id int64, location string) error
	FindWithPerson(ctx context.Context, id int64) (*Student, error)
}

// DefaultStudentRepository is the default implementation of StudentRepository
type DefaultStudentRepository struct {
	db *bun.DB
}

// NewStudentRepository creates a new student repository
func NewStudentRepository(db *bun.DB) StudentRepository {
	return &DefaultStudentRepository{db: db}
}

// Create inserts a new student into the database
func (r *DefaultStudentRepository) Create(ctx context.Context, student *Student) error {
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
func (r *DefaultStudentRepository) FindByID(ctx context.Context, id interface{}) (*Student, error) {
	student := new(Student)
	err := r.db.NewSelect().Model(student).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return student, nil
}

// FindByPersonID retrieves a student by their person ID
func (r *DefaultStudentRepository) FindByPersonID(ctx context.Context, personID int64) (*Student, error) {
	student := new(Student)
	err := r.db.NewSelect().Model(student).Where("person_id = ?", personID).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_person_id", Err: err}
	}
	return student, nil
}

// FindBySchoolClass retrieves students by school class
func (r *DefaultStudentRepository) FindBySchoolClass(ctx context.Context, schoolClass string) ([]*Student, error) {
	var students []*Student
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
func (r *DefaultStudentRepository) FindByGroup(ctx context.Context, groupID int64) ([]*Student, error) {
	var students []*Student
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
func (r *DefaultStudentRepository) FindByGuardianName(ctx context.Context, guardianName string) ([]*Student, error) {
	var students []*Student
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
func (r *DefaultStudentRepository) FindInBus(ctx context.Context) ([]*Student, error) {
	var students []*Student
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
func (r *DefaultStudentRepository) FindInHouse(ctx context.Context) ([]*Student, error) {
	var students []*Student
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
func (r *DefaultStudentRepository) FindInWC(ctx context.Context) ([]*Student, error) {
	var students []*Student
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
func (r *DefaultStudentRepository) FindInSchoolYard(ctx context.Context) ([]*Student, error) {
	var students []*Student
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
func (r *DefaultStudentRepository) UpdateLocation(ctx context.Context, id int64, location string) error {
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
		Model((*Student)(nil)).
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
func (r *DefaultStudentRepository) FindWithPerson(ctx context.Context, id int64) (*Student, error) {
	student := new(Student)
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
func (r *DefaultStudentRepository) Update(ctx context.Context, student *Student) error {
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
func (r *DefaultStudentRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*Student)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves students matching the filters
func (r *DefaultStudentRepository) List(ctx context.Context, filters map[string]interface{}) ([]*Student, error) {
	var students []*Student
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
