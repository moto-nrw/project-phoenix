package auditlog

import "context"

// Change types for an audited guardian change.
const (
	GuardianChangeTypeContact = "contact"
	GuardianChangeTypePickup  = "pickup"
)

// Audited guardian field names. The pickup flags are the safety-critical
// authority toggles; the contact fields are the editable profile data. Pickup
// notes and emergency priority are annotations and are deliberately not
// audited.
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

// GuardianChange is one append-only parent-portal change to a guardian. Pickup
// rows carry the before/after flag as "true"/"false"; contact rows leave
// OldValue/NewValue nil because the values are third-party PII. The actor
// name/email snapshots keep the trail readable after account deletion. The
// tenant and change instant come from the ambient tenant transaction.
type GuardianChange struct {
	StudentID          int64
	GuardianProfileID  int64
	ActorAccountID     *int64
	ActorNameSnapshot  *string
	ActorEmailSnapshot *string
	ChangeType         string
	FieldName          string
	OldValue           *string
	NewValue           *string
}

// GuardianChangeLog appends guardian changes inside the caller's tenant
// transaction, so the trail commits or rolls back with the change itself. An
// empty slice is a no-op; any other call without a tenant transaction fails.
type GuardianChangeLog interface {
	RecordGuardianChanges(context.Context, []GuardianChange) error
}
