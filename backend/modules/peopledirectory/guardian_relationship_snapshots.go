package peopledirectory

import (
	"context"
	"encoding/json"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/contact"
)

// These are the original canonical causes retained model consumers expose.
var ErrGuardianProfileNotFound = contact.ErrGuardianProfileNotFound
var ErrStudentGuardianNotFound = contact.ErrStudentGuardianNotFound

type GuardianRelationshipSnapshot struct {
	ID                 int64           `json:"id"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
	TenantID           int64           `json:"tenant_id"`
	StudentID          int64           `json:"student_id"`
	GuardianProfileID  int64           `json:"guardian_profile_id"`
	RelationshipType   string          `json:"relationship_type"`
	GuardianRole       string          `json:"guardian_role"`
	IsPrimary          bool            `json:"is_primary"`
	IsEmergencyContact bool            `json:"is_emergency_contact"`
	CanPickup          bool            `json:"can_pickup"`
	PickupNotes        *string         `json:"pickup_notes,omitempty"`
	EmergencyPriority  int             `json:"emergency_priority"`
	IsPayer            bool            `json:"is_payer"`
	Permissions        json.RawMessage `json:"permissions,omitempty"`
	Student            *StudentState   `json:"student,omitempty"`
}
type GuardianLinkedChildSnapshot struct {
	GuardianProfileID int64
	StudentID         int64
	FirstName         string
	LastName          string
}

type GuardianRelationshipReads interface {
	GuardianRelationshipsForStudent(context.Context, int64) ([]*GuardianRelationshipSnapshot, error)
	GuardianRelationshipsForProfile(context.Context, int64) ([]*GuardianRelationshipSnapshot, error)
	GuardianRelationshipForUpdate(context.Context, int64, int64) (*GuardianRelationshipSnapshot, error)
	GuardianLinkedChildren(context.Context, []int64) ([]*GuardianLinkedChildSnapshot, error)
	AccountHasStudentPermission(context.Context, int64, int64, int64, string) (bool, error)
}

func GuardianRelationshipTypeValid(value string) bool { return contact.IsValidRelationshipType(value) }

// MapGuardianRelationshipType converts a German or English relationship label
// to its stored type; an unknown label is "other".
func MapGuardianRelationshipType(label string) string { return contact.MapRelationshipType(label) }
func NormalizeGuardianRelationship(value *GuardianRelationshipSnapshot) error {
	state := contact.GuardianRelationshipValues{StudentID: value.StudentID, GuardianProfileID: value.GuardianProfileID, RelationshipType: value.RelationshipType, GuardianRole: value.GuardianRole, PermissionsPresent: value.Permissions != nil}
	err := contact.NormalizeGuardianRelationship(&state)
	value.RelationshipType, value.GuardianRole = state.RelationshipType, state.GuardianRole
	if state.PermissionsPresent && value.Permissions == nil {
		value.Permissions = json.RawMessage(`{}`)
	}
	return err
}
