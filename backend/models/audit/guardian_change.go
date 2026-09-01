package audit

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// Change types for an audited guardian change.
const (
	GuardianChangeTypeContact = "contact"
	GuardianChangeTypePickup  = "pickup"
)

// Audited field names. The pickup flags are the safety-critical authority
// toggles; the contact fields are the editable profile data. Pickup notes and
// emergency priority are annotations and are deliberately not audited.
const (
	GuardianFieldCanPickup        = "can_pickup"
	GuardianFieldEmergencyContact = "is_emergency_contact"

	GuardianFieldFirstName         = "first_name"
	GuardianFieldLastName          = "last_name"
	GuardianFieldEmail             = "email"
	GuardianFieldAddressStreet     = "address_street"
	GuardianFieldAddressCity       = "address_city"
	GuardianFieldAddressPostalCode = "address_postal_code"
	GuardianFieldPhones            = "phones"
)

// GuardianChange records a single parent-portal change to a guardian: either a
// per-child pickup/emergency flag (ChangeType "pickup") or a profile contact
// field (ChangeType "contact"). Rows are append-only. For "pickup" rows
// OldValue/NewValue carry the before/after flag as "true"/"false". For
// "contact" rows OldValue/NewValue are always NULL: the values are third-party
// PII (name/email/phone/address) and are deliberately not retained — the row
// records only which field changed (the live profile holds the current value).
// The actor name/email are snapshotted so the trail survives account deletion.
type GuardianChange struct {
	ID int64 `bun:"id,pk,autoincrement" json:"id"`
	base.TenantModel
	StudentID         int64 `bun:"student_id,notnull" json:"student_id"`
	GuardianProfileID int64 `bun:"guardian_profile_id,notnull" json:"guardian_profile_id"`
	// ActorAccountID is nulled if the acting account is later deleted; the
	// name/email snapshot preserves who made the change.
	ActorAccountID     *int64    `bun:"actor_account_id" json:"actor_account_id,omitempty"`
	ActorNameSnapshot  *string   `bun:"actor_name_snapshot" json:"actor_name_snapshot,omitempty"`
	ActorEmailSnapshot *string   `bun:"actor_email_snapshot" json:"actor_email_snapshot,omitempty"`
	ChangeType         string    `bun:"change_type,notnull" json:"change_type"`
	FieldName          string    `bun:"field_name,notnull" json:"field_name"`
	OldValue           *string   `bun:"old_value" json:"old_value,omitempty"`
	NewValue           *string   `bun:"new_value" json:"new_value,omitempty"`
	ChangedAt          time.Time `bun:"changed_at,notnull,default:now()" json:"changed_at"`
}

type GuardianChangeRepository interface {
	Create(ctx context.Context, entry *GuardianChange) error
	ListByStudentID(ctx context.Context, studentID int64) ([]*GuardianChange, error)
}
