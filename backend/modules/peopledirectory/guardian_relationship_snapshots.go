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
	ID                 int64
	CreatedAt          time.Time
	UpdatedAt          time.Time
	TenantID           int64
	StudentID          int64
	GuardianProfileID  int64
	RelationshipType   string
	GuardianRole       string
	IsPrimary          bool
	IsEmergencyContact bool
	CanPickup          bool
	PickupNotes        *string
	EmergencyPriority  int
	IsPayer            bool
	Permissions        json.RawMessage
	Student            *StudentSnapshot
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
func NormalizeGuardianRelationship(value *GuardianRelationshipSnapshot) error {
	state := contact.GuardianRelationshipValues{StudentID: value.StudentID, GuardianProfileID: value.GuardianProfileID, RelationshipType: value.RelationshipType, GuardianRole: value.GuardianRole, PermissionsPresent: value.Permissions != nil}
	err := contact.NormalizeGuardianRelationship(&state)
	value.RelationshipType, value.GuardianRole = state.RelationshipType, state.GuardianRole
	if state.PermissionsPresent && value.Permissions == nil {
		value.Permissions = json.RawMessage(`{}`)
	}
	return err
}
