package users

import (
	"errors"
	"strings"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// ErrStudentGuardianNotFound is returned when no students_guardians row joins a
// given student and guardian profile. Mirrors ErrGuardianProfileNotFound so
// callers can map a missing relationship to a stable sentinel instead of
// matching sql.ErrNoRows.
var ErrStudentGuardianNotFound = errors.New("student guardian relationship not found")

// GuardianLinkedChild is a minimal projection of a child currently linked to a
// guardian, used by the guardian picker search to show which children a guardian
// belongs to (sibling case, #1513). It is NOT a persisted entity — it is scanned
// from a join across students_guardians → students → persons.
type GuardianLinkedChild struct {
	GuardianProfileID int64  `bun:"guardian_profile_id"`
	StudentID         int64  `bun:"student_id"`
	FirstName         string `bun:"first_name"`
	LastName          string `bun:"last_name"`
}

// StudentGuardian represents the relationship between a student and their guardian
type StudentGuardian struct {
	base.Model `bun:"schema:users,table:students_guardians"`
	base.TenantModel
	StudentID          int64   `bun:"student_id,notnull" json:"student_id"`
	GuardianProfileID  int64   `bun:"guardian_profile_id,notnull" json:"guardian_profile_id"`
	RelationshipType   string  `bun:"relationship_type,notnull" json:"relationship_type"`
	GuardianRole       string  `bun:"guardian_role,notnull,default:'custom'" json:"guardian_role"`
	IsPrimary          bool    `bun:"is_primary,notnull" json:"is_primary"`
	IsEmergencyContact bool    `bun:"is_emergency_contact,notnull" json:"is_emergency_contact"`
	CanPickup          bool    `bun:"can_pickup,notnull" json:"can_pickup"`
	PickupNotes        *string `bun:"pickup_notes" json:"pickup_notes,omitempty"`
	EmergencyPriority  int     `bun:"emergency_priority,default:1" json:"emergency_priority"`
	// IsPayer marks this guardian's bank account as the one charged for this
	// child (#2608). At most one relationship per child carries it, enforced
	// by a partial unique index. Deliberately separate from IsPrimary: the
	// primary contact and the person who pays are frequently not the same,
	// and after a separation they are routinely not.
	IsPayer     bool                   `bun:"is_payer,notnull" json:"is_payer"`
	Permissions map[string]interface{} `bun:"permissions,type:jsonb,nullzero" json:"permissions,omitempty"`

	// Relations not stored in the database
	Student *Student `bun:"rel:belongs-to,join:student_id=id" json:"student,omitempty"`
}

// GuardianAuthorizationData exposes relationship facts used by guardian
// authorization without moving policy onto the persistence model.
func (sg *StudentGuardian) GuardianAuthorizationData() (relationshipType, role string, primary, emergency, pickup bool, permissions map[string]interface{}) {
	if sg == nil {
		return "", "", false, false, false, nil
	}
	return sg.RelationshipType, sg.GuardianRole, sg.IsPrimary, sg.IsEmergencyContact, sg.CanPickup, sg.GetPermissions()
}

// SetGuardianAuthorizationData stores a role and its derived permissions.
func (sg *StudentGuardian) SetGuardianAuthorizationData(role string, permissions map[string]interface{}) {
	if sg == nil {
		return
	}
	sg.GuardianRole = role
	sg.Permissions = permissions
}

// Validate ensures student guardian data is valid
func (sg *StudentGuardian) Validate() error {
	if sg.StudentID <= 0 {
		return errors.New("student ID is required")
	}

	if sg.GuardianProfileID <= 0 {
		return errors.New("guardian profile ID is required")
	}

	// Validate relationship type
	if sg.RelationshipType == "" {
		return errors.New("relationship type is required")
	}

	// Convert relationship type to lowercase for consistency
	sg.RelationshipType = strings.ToLower(sg.RelationshipType)

	// Validate against the shared allowed set (single source of truth)
	if !IsValidRelationshipType(sg.RelationshipType) {
		return errors.New("invalid relationship type")
	}

	sg.GuardianRole = strings.ToLower(strings.TrimSpace(sg.GuardianRole))
	if sg.GuardianRole == "" {
		sg.GuardianRole = "custom"
	}

	// Initialize permissions with empty object if nil
	if sg.Permissions == nil {
		sg.Permissions = make(map[string]interface{})
	}

	return nil
}

// SetStudent links this relationship to a student
func (sg *StudentGuardian) SetStudent(student *Student) {
	sg.Student = student
	if student != nil {
		sg.StudentID = student.ID
	}
}

// GetPermissions returns permissions map
func (sg *StudentGuardian) GetPermissions() map[string]interface{} {
	if sg.Permissions == nil {
		sg.Permissions = make(map[string]interface{})
	}
	return sg.Permissions
}

// UpdatePermissions updates the permissions map
func (sg *StudentGuardian) UpdatePermissions(permissions map[string]interface{}) error {
	sg.Permissions = permissions
	return nil
}

// Note: the permission check (formerly StudentGuardian.HasPermission) moved out
// of the model in issue #586 (Rule 12) to:
//   - auth/authorize/guardian_permission.go (StudentGuardianHasPermission)

// GetRelationshipName returns a formatted name for the relationship type
func (sg *StudentGuardian) GetRelationshipName() string {
	switch sg.RelationshipType {
	case "parent":
		return "Parent"
	case "guardian":
		return "Guardian"
	case "relative":
		return "Relative"
	case "other":
		return "Other"
	default:
		return "Unknown"
	}
}
