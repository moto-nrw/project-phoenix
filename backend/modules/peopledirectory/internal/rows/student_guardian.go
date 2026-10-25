package users

import (
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/contact"
)

// ErrStudentGuardianNotFound is returned when no student-guardian relationship joins a
// given student and guardian profile. Mirrors ErrGuardianProfileNotFound so
// callers can map a missing relationship to a stable sentinel instead of
// matching sql.ErrNoRows.
var ErrStudentGuardianNotFound = contact.ErrStudentGuardianNotFound

// GuardianLinkedChild is a minimal projection of a child currently linked to a
// guardian, used by the guardian picker search to show which children a guardian
// belongs to (sibling case, #1513). It is NOT a persisted entity — it is scanned
// from a join across guardian relationships → students → persons.
type GuardianLinkedChild struct {
	GuardianProfileID int64  `bun:"guardian_profile_id"`
	StudentID         int64  `bun:"student_id"`
	FirstName         string `bun:"first_name"`
	LastName          string `bun:"last_name"`
}

// StudentGuardian represents the relationship between a student and their
// guardian as one row: People Directory's relationship with Care Plan's
// pickup permission and Identity & Access's parents-portal permissions
// (#2756). It is read through the guardian-link projection and written to the
// owner of each column; it names no table of its own.
type StudentGuardian struct {
	Model `bun:"alias:student_guardian"`
	TenantModel
	StudentID          int64   `bun:"student_id,notnull" json:"student_id"`
	GuardianProfileID  int64   `bun:"guardian_profile_id,notnull" json:"guardian_profile_id"`
	RelationshipType   string  `bun:"relationship_type,notnull" json:"relationship_type"`
	GuardianRole       string  `bun:"guardian_role,notnull,default:'custom'" json:"guardian_role"`
	IsPrimary          bool    `bun:"is_primary,notnull" json:"is_primary"`
	IsEmergencyContact bool    `bun:"is_emergency_contact,notnull" json:"is_emergency_contact"`
	CanPickup          bool    `bun:"can_pickup,notnull" json:"can_pickup"`
	PickupNotes        *string `bun:"pickup_notes" json:"pickup_notes,omitempty"`
	// PickupWeekdays are the plan weekdays Care Plan's pickup permission
	// holds on (1 = Monday … 5 = Friday); nil means every day (#4780).
	PickupWeekdays    []int `bun:"pickup_weekdays,array" json:"pickup_weekdays,omitempty"`
	EmergencyPriority int   `bun:"emergency_priority,default:1" json:"emergency_priority"`
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
	value := contact.GuardianRelationshipValues{StudentID: sg.StudentID, GuardianProfileID: sg.GuardianProfileID, RelationshipType: sg.RelationshipType, GuardianRole: sg.GuardianRole, PermissionsPresent: sg.Permissions != nil}
	err := contact.NormalizeGuardianRelationship(&value)
	sg.RelationshipType, sg.GuardianRole = value.RelationshipType, value.GuardianRole
	if value.PermissionsPresent && sg.Permissions == nil {
		sg.Permissions = make(map[string]interface{})
	}
	return err
}

// GetPermissions returns permissions map
func (sg *StudentGuardian) GetPermissions() map[string]interface{} {
	if sg.Permissions == nil {
		sg.Permissions = make(map[string]interface{})
	}
	return sg.Permissions
}

// Note: the permission check (formerly StudentGuardian.HasPermission) moved out
// of the model in issue #586 (Rule 12) to:
//   - auth/authorize/guardian_permission.go (StudentGuardianHasPermission)
