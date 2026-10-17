package ports

import (
	"context"
	"time"
)

// GuardianPaymentField names the bank-data field a financial audit row
// records. The root maps each value to the audit trail's field name.
type GuardianPaymentField int

const (
	GuardianPaymentFieldIBAN GuardianPaymentField = iota + 1
	GuardianPaymentFieldAccountHolder
	GuardianPaymentFieldIsPayer
)

// GuardianPaymentAccess names the kind of bank-data read a data-access log
// row records. The root maps each value to the audit trail's resource type.
type GuardianPaymentAccess int

const (
	// GuardianPaymentAccessView is the masked read of one guardian.
	GuardianPaymentAccessView GuardianPaymentAccess = iota + 1
	// GuardianPaymentAccessReveal is the unmasked read of one guardian.
	GuardianPaymentAccessReveal
	// GuardianPaymentAccessOverview is the school-wide masked list.
	GuardianPaymentAccessOverview
	// GuardianPaymentAccessExport is the school-wide unmasked export.
	GuardianPaymentAccessExport
)

// GuardianFinancialChange is one append-only audit row of a guardian's bank
// data. Old and new values are masked; StudentID is set only for the payer
// mark, which belongs to one child.
type GuardianFinancialChange struct {
	GuardianProfileID int64
	StudentID         *int64
	ChangedBy         int64
	Field             GuardianPaymentField
	OldValue          string
	NewValue          string
	Note              string
}

// GuardianFinancialAudit writes the change trail of the guardian bank data.
// Every write runs in the caller's tenant transaction, so a failed audit row
// rolls the change back.
type GuardianFinancialAudit interface {
	RecordGuardianFinancialChange(ctx context.Context, change GuardianFinancialChange) error
}

// GuardianDataAccess is one data-access log row of a bank-data read.
type GuardianDataAccess struct {
	ActorAccountID int64
	ActorRole      string
	Access         GuardianPaymentAccess
	AccessedAt     time.Time
	Metadata       map[string]interface{}
}

// GuardianDataAccessLog writes the read trail of the guardian bank data. No
// bank value is served when the write fails.
type GuardianDataAccessLog interface {
	RecordGuardianDataAccess(ctx context.Context, access GuardianDataAccess) error
}

// GuardianRolePresets is the Security Runtime's role-preset policy of a
// student-guardian relationship.
type GuardianRolePresets interface {
	// DefaultRole is the preset a new relationship takes when the caller
	// chose none.
	DefaultRole(relationshipType string, isPrimary, isEmergencyContact, canPickup bool) string
	// Preset returns the stored role of a preset and the parent-portal
	// permissions it grants.
	Preset(role string) (string, []string)
}
