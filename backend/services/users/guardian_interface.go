package users

import (
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
	// InvitationPending is true when an open (not accepted, not expired, not
	// rejected) invitation exists: profile-wide for guardians without a portal
	// account, and anchored to this child for guardians WITH an account (a
	// pending-approval role-upgrade request, #2172). Lets the staff UI show
	// "Einladung offen" instead of a misleading active/no-access state.
	InvitationPending bool
}
