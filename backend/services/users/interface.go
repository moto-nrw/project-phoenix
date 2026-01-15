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

// PersonCRUD handles basic person CRUD operations
type PersonCRUD interface {
	Get(ctx context.Context, id interface{}) (*userModels.Person, error)
	GetByIDs(ctx context.Context, ids []int64) (map[int64]*userModels.Person, error)
	Create(ctx context.Context, person *userModels.Person) error
	Update(ctx context.Context, person *userModels.Person) error
	Delete(ctx context.Context, id interface{}) error
	List(ctx context.Context, options *base.QueryOptions) ([]*userModels.Person, error)
}

// PersonFinder handles person lookup operations
type PersonFinder interface {
	FindByTagID(ctx context.Context, tagID string) (*userModels.Person, error)
	FindByAccountID(ctx context.Context, accountID int64) (*userModels.Person, error)
	FindByName(ctx context.Context, firstName, lastName string) ([]*userModels.Person, error)
	FindByGuardianID(ctx context.Context, guardianAccountID int64) ([]*userModels.Person, error)
	GetFullProfile(ctx context.Context, personID int64) (*userModels.Person, error)
}

// PersonLinker handles linking persons to accounts and RFID cards
type PersonLinker interface {
	LinkToAccount(ctx context.Context, personID int64, accountID int64) error
	UnlinkFromAccount(ctx context.Context, personID int64) error
	LinkToRFIDCard(ctx context.Context, personID int64, tagID string) error
	UnlinkFromRFIDCard(ctx context.Context, personID int64) error
}

// StaffOperations handles staff-related operations
type StaffOperations interface {
	GetStaffByID(ctx context.Context, staffID int64) (*userModels.Staff, error)
	GetStaffByPersonID(ctx context.Context, personID int64) (*userModels.Staff, error)
	GetStaffWithPerson(ctx context.Context, staffID int64) (*userModels.Staff, error)
	ListStaff(ctx context.Context, options *base.QueryOptions) ([]*userModels.Staff, error)
	CreateStaff(ctx context.Context, staff *userModels.Staff) error
	UpdateStaff(ctx context.Context, staff *userModels.Staff) error
	DeleteStaff(ctx context.Context, staffID int64) error
	ValidateStaffPIN(ctx context.Context, pin string) (*userModels.Staff, error)
	ValidateStaffPINForSpecificStaff(ctx context.Context, staffID int64, pin string) (*userModels.Staff, error)
}

// TeacherOperations handles teacher-related operations
type TeacherOperations interface {
	GetTeacherByStaffID(ctx context.Context, staffID int64) (*userModels.Teacher, error)
	GetTeacherWithDetails(ctx context.Context, teacherID int64) (*userModels.Teacher, error)
	GetTeachersBySpecialization(ctx context.Context, specialization string) ([]*userModels.Teacher, error)
	CreateTeacher(ctx context.Context, teacher *userModels.Teacher) error
	UpdateTeacher(ctx context.Context, teacher *userModels.Teacher) error
	DeleteTeacher(ctx context.Context, teacherID int64) error
	GetStudentsByTeacher(ctx context.Context, teacherID int64) ([]*userModels.Student, error)
	GetStudentsWithGroupsByTeacher(ctx context.Context, teacherID int64) ([]StudentWithGroup, error)
}

// StudentOperations handles student-related operations
type StudentOperations interface {
	GetStudentByID(ctx context.Context, studentID int64) (*userModels.Student, error)
	GetStudentByPersonID(ctx context.Context, personID int64) (*userModels.Student, error)
}

// RFIDCardOperations handles RFID card queries
type RFIDCardOperations interface {
	ListAvailableRFIDCards(ctx context.Context) ([]*userModels.RFIDCard, error)
}

// PersonService composes all person-related operations
// Existing callers can continue using this full interface.
// New code can depend on smaller sub-interfaces for better decoupling.
type PersonService interface {
	base.TransactionalService
	PersonCRUD
	PersonFinder
	PersonLinker
	StaffOperations
	TeacherOperations
	StudentOperations
	RFIDCardOperations
}
