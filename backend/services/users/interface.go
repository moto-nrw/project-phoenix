package users

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

// StudentWithGroup represents a student with their group information
type StudentWithGroup struct {
	Student   *userModels.Student `json:"student"`
	GroupName string              `json:"group_name"`
}

// PersonService defines the operations available in the person service layer
type PersonService interface {
	base.TransactionalService
	// Get retrieves a person by their ID
	Get(ctx context.Context, id interface{}) (*userModels.Person, error)

	// GetByIDs retrieves multiple persons by their IDs in a single query
	GetByIDs(ctx context.Context, ids []int64) (map[int64]*userModels.Person, error)

	// Create creates a new person
	Create(ctx context.Context, person *userModels.Person) error

	// Update updates an existing person
	Update(ctx context.Context, person *userModels.Person) error

	// Delete removes a person
	Delete(ctx context.Context, id interface{}) error

	// List retrieves persons matching the provided query options
	List(ctx context.Context, options *base.QueryOptions) ([]*userModels.Person, error)

	// FindByTagID finds a person by their RFID tag ID
	FindByTagID(ctx context.Context, tagID string) (*userModels.Person, error)

	// FindByAccountID finds a person by their account ID
	FindByAccountID(ctx context.Context, accountID int64) (*userModels.Person, error)

	// FindByName finds persons matching the provided name (first name, last name, or both)
	FindByName(ctx context.Context, firstName, lastName string) ([]*userModels.Person, error)

	// LinkToAccount associates a person with an account
	LinkToAccount(ctx context.Context, personID int64, accountID int64) error

	// UnlinkFromAccount removes account association from a person
	UnlinkFromAccount(ctx context.Context, personID int64) error

	// LinkToRFIDCard associates a person with an RFID card
	LinkToRFIDCard(ctx context.Context, personID int64, tagID string) error

	// UnlinkFromRFIDCard removes RFID card association from a person
	UnlinkFromRFIDCard(ctx context.Context, personID int64) error

	// GetFullProfile retrieves a person with all related entities
	GetFullProfile(ctx context.Context, personID int64) (*userModels.Person, error)

	// FindByGuardianID finds all persons with a guardian relationship to the specified account
	FindByGuardianID(ctx context.Context, guardianAccountID int64) ([]*userModels.Person, error)

	// GetStaffByID retrieves a staff record by its ID
	GetStaffByID(ctx context.Context, staffID int64) (*userModels.Staff, error)

	// GetStaffByPersonID retrieves a staff record by person ID
	GetStaffByPersonID(ctx context.Context, personID int64) (*userModels.Staff, error)

	// GetStudentByPersonID retrieves a student record by person ID
	GetStudentByPersonID(ctx context.Context, personID int64) (*userModels.Student, error)

	// GetStudentByID retrieves a student by their ID
	GetStudentByID(ctx context.Context, studentID int64) (*userModels.Student, error)

	// GetTeacherByStaffID retrieves a teacher by their staff ID
	GetTeacherByStaffID(ctx context.Context, staffID int64) (*userModels.Teacher, error)

	// ListStaff retrieves all staff members
	ListStaff(ctx context.Context, options *base.QueryOptions) ([]*userModels.Staff, error)

	// GetStaffWithPerson retrieves a staff member with their person details
	GetStaffWithPerson(ctx context.Context, staffID int64) (*userModels.Staff, error)

	// StaffRepository returns the staff repository
	// Deprecated: Use GetStaffByPersonID, ListStaff, GetStaffWithPerson instead
	StaffRepository() userModels.StaffRepository

	// TeacherRepository returns the teacher repository
	// Deprecated: Use GetTeacherByStaffID, GetTeachersBySpecialization, GetTeacherWithDetails instead
	TeacherRepository() userModels.TeacherRepository

	// GetTeachersBySpecialization retrieves teachers by their specialization
	GetTeachersBySpecialization(ctx context.Context, specialization string) ([]*userModels.Teacher, error)

	// GetTeacherWithDetails retrieves a teacher with their associated staff and person data
	GetTeacherWithDetails(ctx context.Context, teacherID int64) (*userModels.Teacher, error)

	// ListAvailableRFIDCards returns RFID cards that are not assigned to any person
	ListAvailableRFIDCards(ctx context.Context) ([]*userModels.RFIDCard, error)

	// Authentication operations
	ValidateStaffPIN(ctx context.Context, pin string) (*userModels.Staff, error)
	ValidateStaffPINForSpecificStaff(ctx context.Context, staffID int64, pin string) (*userModels.Staff, error)

	// GetStudentsByTeacher retrieves students supervised by a teacher (through group assignments)
	GetStudentsByTeacher(ctx context.Context, teacherID int64) ([]*userModels.Student, error)

	// GetStudentsWithGroupsByTeacher retrieves students with group info supervised by a teacher
	GetStudentsWithGroupsByTeacher(ctx context.Context, teacherID int64) ([]StudentWithGroup, error)
}
