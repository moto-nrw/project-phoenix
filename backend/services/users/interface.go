package users

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

// PersonService defines the operations available in the person service layer
type PersonService interface {
	base.TransactionalService
	// Get retrieves a person by their ID
	Get(ctx context.Context, id interface{}) (*userModels.Person, error)

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

	// StudentRepository returns the student repository
	StudentRepository() userModels.StudentRepository

	// StaffRepository returns the staff repository
	StaffRepository() userModels.StaffRepository

	// TeacherRepository returns the teacher repository
	TeacherRepository() userModels.TeacherRepository
}
