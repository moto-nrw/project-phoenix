package users

import (
	"context"

	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// GuardianCreateRequest represents data for creating a new guardian
type GuardianCreateRequest struct {
	FirstName              string
	LastName               string
	Email                  *string
	Phone                  *string
	MobilePhone            *string
	AddressStreet          *string
	AddressCity            *string
	AddressPostalCode      *string
	PreferredContactMethod string
	LanguagePreference     string
	Occupation             *string
	Employer               *string
	Notes                  *string
}

// GuardianInvitationRequest represents data for inviting a guardian
type GuardianInvitationRequest struct {
	GuardianProfileID int64
	CreatedBy         int64 // Staff/admin sending the invitation
}

// GuardianInvitationAcceptRequest represents data for accepting an invitation
type GuardianInvitationAcceptRequest struct {
	Token           string
	Password        string
	ConfirmPassword string
}

// GuardianInvitationValidationResult contains public-safe invitation details
type GuardianInvitationValidationResult struct {
	GuardianFirstName string   `json:"guardian_first_name"`
	GuardianLastName  string   `json:"guardian_last_name"`
	Email             string   `json:"email"`
	StudentNames      []string `json:"student_names"`
	ExpiresAt         string   `json:"expires_at"`
}

// StudentGuardianCreateRequest represents data for linking a guardian to a student
type StudentGuardianCreateRequest struct {
	StudentID          int64
	GuardianProfileID  int64
	RelationshipType   string // parent, guardian, relative, other
	IsPrimary          bool
	IsEmergencyContact bool
	CanPickup          bool
	PickupNotes        *string
	EmergencyPriority  int
}

// StudentGuardianUpdateRequest represents data for updating a student-guardian relationship
type StudentGuardianUpdateRequest struct {
	RelationshipType   *string
	IsPrimary          *bool
	IsEmergencyContact *bool
	CanPickup          *bool
	PickupNotes        *string
	EmergencyPriority  *int
}

// GuardianWithStudents represents a guardian with their associated students
type GuardianWithStudents struct {
	Profile  *users.GuardianProfile
	Students []*StudentWithRelationship
}

// StudentWithRelationship represents a student with guardian relationship details
type StudentWithRelationship struct {
	Student      *users.Student
	Relationship *users.StudentGuardian
}

// GuardianWithRelationship represents a guardian with student relationship details
type GuardianWithRelationship struct {
	Profile      *users.GuardianProfile
	Relationship *users.StudentGuardian
}

// GuardianCRUD handles basic guardian profile CRUD operations
type GuardianCRUD interface {
	// CreateGuardian creates a new guardian profile (without account)
	CreateGuardian(ctx context.Context, req GuardianCreateRequest) (*users.GuardianProfile, error)

	// GetGuardianByID retrieves a guardian profile by ID
	GetGuardianByID(ctx context.Context, id int64) (*users.GuardianProfile, error)

	// GetGuardianByEmail retrieves a guardian profile by email
	GetGuardianByEmail(ctx context.Context, email string) (*users.GuardianProfile, error)

	// UpdateGuardian updates a guardian profile
	UpdateGuardian(ctx context.Context, id int64, req GuardianCreateRequest) error

	// DeleteGuardian removes a guardian profile (and all relationships)
	DeleteGuardian(ctx context.Context, id int64) error

	// ListGuardians retrieves guardians with pagination and filters
	ListGuardians(ctx context.Context, options *base.QueryOptions) ([]*users.GuardianProfile, error)
}

// GuardianRelationshipOperations handles student-guardian relationship operations
type GuardianRelationshipOperations interface {
	// GetStudentGuardians retrieves all guardians for a student
	GetStudentGuardians(ctx context.Context, studentID int64) ([]*GuardianWithRelationship, error)

	// GetGuardianStudents retrieves all students for a guardian
	GetGuardianStudents(ctx context.Context, guardianProfileID int64) ([]*StudentWithRelationship, error)

	// LinkGuardianToStudent creates a relationship between guardian and student
	LinkGuardianToStudent(ctx context.Context, req StudentGuardianCreateRequest) (*users.StudentGuardian, error)

	// GetStudentGuardianRelationship retrieves a student-guardian relationship by ID
	GetStudentGuardianRelationship(ctx context.Context, relationshipID int64) (*users.StudentGuardian, error)

	// UpdateStudentGuardianRelationship updates a student-guardian relationship
	UpdateStudentGuardianRelationship(ctx context.Context, relationshipID int64, req StudentGuardianUpdateRequest) error

	// RemoveGuardianFromStudent removes a guardian from a student
	RemoveGuardianFromStudent(ctx context.Context, studentID, guardianProfileID int64) error
}

// GuardianInvitationOperations handles guardian invitation workflow
type GuardianInvitationOperations interface {
	// CreateGuardianWithInvitation creates a guardian and sends an invitation email
	CreateGuardianWithInvitation(ctx context.Context, req GuardianCreateRequest, createdBy int64) (*users.GuardianProfile, *authModels.GuardianInvitation, error)

	// SendInvitation sends an invitation to a guardian
	SendInvitation(ctx context.Context, req GuardianInvitationRequest) (*authModels.GuardianInvitation, error)

	// ValidateInvitation validates an invitation token
	ValidateInvitation(ctx context.Context, token string) (*GuardianInvitationValidationResult, error)

	// AcceptInvitation accepts an invitation and creates a guardian account
	AcceptInvitation(ctx context.Context, req GuardianInvitationAcceptRequest) (*authModels.AccountParent, error)

	// GetPendingInvitations retrieves all pending guardian invitations
	GetPendingInvitations(ctx context.Context) ([]*authModels.GuardianInvitation, error)

	// CleanupExpiredInvitations deletes expired invitations
	CleanupExpiredInvitations(ctx context.Context) (int, error)
}

// GuardianQueryOperations handles special query operations
type GuardianQueryOperations interface {
	// GetGuardiansWithoutAccount retrieves guardians who don't have portal accounts
	GetGuardiansWithoutAccount(ctx context.Context) ([]*users.GuardianProfile, error)

	// GetInvitableGuardians retrieves guardians who can be invited (has email, no account)
	GetInvitableGuardians(ctx context.Context) ([]*users.GuardianProfile, error)
}

// GuardianService composes all guardian-related operations.
// Existing callers can continue using this full interface.
// New code can depend on smaller sub-interfaces for better decoupling.
type GuardianService interface {
	base.TransactionalService
	GuardianCRUD
	GuardianRelationshipOperations
	GuardianInvitationOperations
	GuardianQueryOperations
}
