package users

import (
	"context"

	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// GuardianCreateRequest represents data for creating a new guardian
// Note: Phone numbers are managed separately via PhoneNumberCreateRequest
type GuardianCreateRequest struct {
	FirstName              string
	LastName               string
	Email                  *string
	AddressStreet          *string
	AddressCity            *string
	AddressPostalCode      *string
	PreferredContactMethod string
	LanguagePreference     string
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
	GuardianRole       string
	IsPrimary          bool
	IsEmergencyContact bool
	CanPickup          bool
	PickupNotes        *string
	EmergencyPriority  int
}

// StudentGuardianUpdateRequest represents data for updating a student-guardian relationship
type StudentGuardianUpdateRequest struct {
	RelationshipType   *string
	GuardianRole       *string
	IsPrimary          *bool
	IsEmergencyContact *bool
	CanPickup          *bool
	PickupNotes        *string
	EmergencyPriority  *int
}

// PhoneNumberCreateRequest represents data for creating a new phone number
type PhoneNumberCreateRequest struct {
	PhoneNumber string
	PhoneType   string  // mobile, home, work, other
	Label       *string // Optional label like "Dienstlich"
	IsPrimary   bool
}

// PhoneNumberUpdateRequest represents data for updating a phone number
type PhoneNumberUpdateRequest struct {
	PhoneNumber *string
	PhoneType   *string
	Label       *string
	IsPrimary   *bool
	Priority    *int
}

// StudentGuardianRelationship holds the relationship flags for linking a
// guardian to a student, without the student/guardian IDs which are set
// internally by AddGuardiansToStudent.
type StudentGuardianRelationship struct {
	RelationshipType   string // parent, guardian, relative, other
	GuardianRole       string
	IsPrimary          bool
	IsEmergencyContact bool
	CanPickup          bool
	PickupNotes        *string
	EmergencyPriority  int
}

// NewStudentGuardian bundles a guardian profile, its relationship to a
// student, and its phone numbers so a guardian can be created and linked
// atomically alongside a new student.
type NewStudentGuardian struct {
	Profile      GuardianCreateRequest
	Relationship StudentGuardianRelationship
	PhoneNumbers []PhoneNumberCreateRequest

	// ExistingProfileID, when non-nil, links an already-existing guardian
	// profile to the student instead of creating a new one (sibling case,
	// issue #1513). In that case Profile and PhoneNumbers are ignored and the
	// existing profile is never mutated — only the Relationship flags apply to
	// the new link.
	ExistingProfileID *int64
}

// GuardianWithStudents represents a guardian with their associated students
type GuardianWithStudents struct {
	Profile  *users.GuardianProfile
	Students []*StudentWithRelationship
}

// GuardianPickerMatch is one result from the guardian picker search: a guardian
// profile plus the children currently linked to it. Backs GET /guardians/search
// (sibling case, #1513). The handler projects only id/name/email + a COUNT of
// children onto the wire — address, notes, language, and contact method never
// leave the server, and child names are never exposed (only the count).
type GuardianPickerMatch struct {
	Profile  *users.GuardianProfile
	Children []*users.GuardianLinkedChild
}

// GuardianDeleteImpact is the exact current blast radius of a full guardian
// delete. LinkIDs is the concurrency token the frontend sends back on confirm;
// StudentNames is display-only warning text.
type GuardianDeleteImpact struct {
	LinkIDs      []int64
	StudentNames []string
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

// GuardianService defines operations for managing guardians
type GuardianService interface {
	// CreateGuardian creates a new guardian profile (without account)
	CreateGuardian(ctx context.Context, req GuardianCreateRequest) (*users.GuardianProfile, error)

	// CreateGuardianWithInvitation creates a guardian and sends an invitation email
	CreateGuardianWithInvitation(ctx context.Context, req GuardianCreateRequest, createdBy int64) (*users.GuardianProfile, *authModels.GuardianInvitation, error)

	// GetGuardianByID retrieves a guardian profile by ID
	GetGuardianByID(ctx context.Context, id int64) (*users.GuardianProfile, error)

	// GetGuardianByEmail retrieves a guardian profile by email
	GetGuardianByEmail(ctx context.Context, email string) (*users.GuardianProfile, error)

	// UpdateGuardian updates a guardian profile
	UpdateGuardian(ctx context.Context, id int64, req GuardianCreateRequest) error

	// DeleteGuardian removes a guardian profile without touching its student
	// links. Fails with a FK violation (→ 409 at the handler) when links remain,
	// because the FK is ON DELETE RESTRICT since migration 1.15.127.
	DeleteGuardian(ctx context.Context, id int64) error

	// DeleteGuardianWithLinks deletes a guardian together with all of its
	// student↔guardian links (the deliberate "Komplett löschen" path, #819).
	// Must run inside a tenant transaction; the handler gates it to admins.
	// expectedLinkIDs must be the exact link set returned by the delete preview.
	DeleteGuardianWithLinks(ctx context.Context, id int64, expectedLinkIDs []int64) error

	// GetLinkedStudentNames returns the full names of every student linked to
	// the guardian. Empty means a plain delete is safe; non-empty drives the
	// 409 warning listing the affected children.
	GetLinkedStudentNames(ctx context.Context, guardianProfileID int64) ([]string, error)

	// GetGuardianDeleteImpact returns the exact current affected link IDs and
	// student names for the admin full-delete confirmation.
	GetGuardianDeleteImpact(ctx context.Context, guardianProfileID int64) (*GuardianDeleteImpact, error)

	// SendInvitation sends an invitation to a guardian
	SendInvitation(ctx context.Context, req GuardianInvitationRequest) (*authModels.GuardianInvitation, error)

	// ValidateInvitation validates an invitation token
	ValidateInvitation(ctx context.Context, token string) (*GuardianInvitationValidationResult, error)

	// AcceptInvitation accepts an invitation and creates a guardian account
	AcceptInvitation(ctx context.Context, req GuardianInvitationAcceptRequest) (*authModels.AccountParent, error)

	// GetStudentGuardians retrieves all guardians for a student
	GetStudentGuardians(ctx context.Context, studentID int64) ([]*GuardianWithRelationship, error)

	// GetGuardianStudents retrieves all students for a guardian
	GetGuardianStudents(ctx context.Context, guardianProfileID int64) ([]*StudentWithRelationship, error)

	// LinkGuardianToStudent creates a relationship between guardian and student
	LinkGuardianToStudent(ctx context.Context, req StudentGuardianCreateRequest) (*users.StudentGuardian, error)

	// ValidateNewGuardians checks guardian input (profile, relationship type,
	// emergency priority, phone numbers, and duplicate email) WITHOUT writing
	// anything. Callers that persist a student and its guardians in one
	// transaction MUST call this before the first write so a ValidationError
	// rolls back an empty transaction instead of committing an orphaned
	// student (TenantTxMiddleware only rolls back on 5xx). Returns a
	// *ValidationError for bad client input.
	ValidateNewGuardians(ctx context.Context, guardians []NewStudentGuardian) error

	// AddGuardiansToStudent creates each guardian profile, links it to the
	// student, and adds its phone numbers. Designed to run inside an ambient
	// tenant transaction so the whole set is atomic with student creation;
	// any failure aborts the surrounding transaction. Re-runs
	// ValidateNewGuardians internally as defense-in-depth.
	AddGuardiansToStudent(ctx context.Context, studentID int64, guardians []NewStudentGuardian) error

	// GetStudentGuardianRelationship retrieves a student-guardian relationship by ID
	GetStudentGuardianRelationship(ctx context.Context, relationshipID int64) (*users.StudentGuardian, error)

	// UpdateStudentGuardianRelationship updates a student-guardian relationship
	UpdateStudentGuardianRelationship(ctx context.Context, relationshipID int64, req StudentGuardianUpdateRequest) error

	// RemoveGuardianFromStudent removes a guardian from a student
	RemoveGuardianFromStudent(ctx context.Context, studentID, guardianProfileID int64) error

	// ListGuardians retrieves guardians with pagination and filters
	ListGuardians(ctx context.Context, options *base.QueryOptions) ([]*users.GuardianProfile, error)

	// SearchGuardiansForPicker retrieves guardians whose name or email matches
	// the search text (case-insensitive), each enriched with its linked children
	// in a single batch query (no N+1). Backs the guardian picker used to link an
	// existing guardian to a student (sibling case). Tenant-scoped via RLS.
	SearchGuardiansForPicker(ctx context.Context, searchText string, limit int) ([]*GuardianPickerMatch, error)

	// GetGuardiansWithoutAccount retrieves guardians who don't have portal accounts
	GetGuardiansWithoutAccount(ctx context.Context) ([]*users.GuardianProfile, error)

	// GetInvitableGuardians retrieves guardians who can be invited (has email, no account)
	GetInvitableGuardians(ctx context.Context) ([]*users.GuardianProfile, error)

	// GetPendingInvitations retrieves all pending guardian invitations
	GetPendingInvitations(ctx context.Context) ([]*authModels.GuardianInvitation, error)

	// CleanupExpiredInvitations deletes expired invitations
	CleanupExpiredInvitations(ctx context.Context) (int, error)

	// Phone Number Management

	// AddPhoneNumber adds a new phone number to a guardian
	// If this is the first number, it will automatically be set as primary
	AddPhoneNumber(ctx context.Context, guardianID int64, req PhoneNumberCreateRequest) (*users.GuardianPhoneNumber, error)

	// UpdatePhoneNumber updates an existing phone number
	UpdatePhoneNumber(ctx context.Context, phoneID int64, req PhoneNumberUpdateRequest) error

	// DeletePhoneNumber removes a phone number
	// If the deleted number was primary, the next highest-priority number becomes primary
	DeletePhoneNumber(ctx context.Context, phoneID int64) error

	// SetPrimaryPhone sets a phone number as the primary contact
	SetPrimaryPhone(ctx context.Context, phoneID int64) error

	// GetGuardianPhoneNumbers retrieves all phone numbers for a guardian, sorted by priority
	GetGuardianPhoneNumbers(ctx context.Context, guardianID int64) ([]*users.GuardianPhoneNumber, error)

	// GetPhoneNumberByID retrieves a phone number by ID
	GetPhoneNumberByID(ctx context.Context, phoneID int64) (*users.GuardianPhoneNumber, error)
}
