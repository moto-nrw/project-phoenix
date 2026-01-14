package users

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

// StudentService defines the operations available in the student service layer
type StudentService interface {
	base.TransactionalService

	// Get retrieves a student by their ID
	Get(ctx context.Context, id int64) (*userModels.Student, error)

	// Create creates a new student
	Create(ctx context.Context, student *userModels.Student) error

	// Update updates an existing student
	Update(ctx context.Context, student *userModels.Student) error

	// Delete removes a student
	Delete(ctx context.Context, id int64) error

	// FindByGroupID retrieves students by their group ID
	FindByGroupID(ctx context.Context, groupID int64) ([]*userModels.Student, error)

	// FindByGroupIDs retrieves students by multiple group IDs
	FindByGroupIDs(ctx context.Context, groupIDs []int64) ([]*userModels.Student, error)

	// ListWithOptions retrieves students with query options
	ListWithOptions(ctx context.Context, options *base.QueryOptions) ([]*userModels.Student, error)

	// CountWithOptions counts students matching the query options
	CountWithOptions(ctx context.Context, options *base.QueryOptions) (int, error)

	// Privacy consent operations
	GetPrivacyConsent(ctx context.Context, studentID int64) ([]*userModels.PrivacyConsent, error)
	CreatePrivacyConsent(ctx context.Context, consent *userModels.PrivacyConsent) error
	UpdatePrivacyConsent(ctx context.Context, consent *userModels.PrivacyConsent) error
}
