package audit

import (
	"context"
	"errors"
	"time"
)

// Audited guardian payment fields (#2608). The names are part of the audit
// wire format; the payer flag is student-scoped, the bank fields are not.
const (
	GuardianPaymentFieldIBAN          = "iban"
	GuardianPaymentFieldAccountHolder = "account_holder"
	GuardianPaymentFieldIsPayer       = "is_payer"
)

// GuardianFinancialChange is an append-only audit row for one changed payment
// field of a guardian (#2608). Written in the same tenant transaction as the
// update — no change without a trace.
//
// OldValue/NewValue carry MASKED values for the bank fields: the trail must
// not become a second store of IBANs outside the guardians:financial gate.
// StudentID is set only for GuardianPaymentFieldIsPayer, which decides which
// guardian pays for ONE child; the bank fields belong to the guardian and
// leave it nil. ChangedBy is the authenticated account ID.
type GuardianFinancialChange struct {
	ID int64 `bun:"id,pk,autoincrement" json:"id"`
	TenantModel
	GuardianProfileID int64     `bun:"guardian_profile_id,notnull" json:"guardian_profile_id"`
	StudentID         *int64    `bun:"student_id" json:"student_id,omitempty"`
	ChangedBy         int64     `bun:"changed_by,notnull" json:"changed_by"`
	FieldName         string    `bun:"field_name,notnull" json:"field_name"`
	OldValue          string    `bun:"old_value,notnull" json:"old_value"`
	NewValue          string    `bun:"new_value,notnull" json:"new_value"`
	Note              string    `bun:"note,notnull" json:"note"`
	OccurredAt        time.Time `bun:"occurred_at,nullzero,notnull,default:now()" json:"occurred_at"`
}

// Validate rejects rows that would make the trail unreadable or untraceable.
func (c *GuardianFinancialChange) Validate() error {
	if c.GuardianProfileID <= 0 {
		return errors.New("guardian_profile_id is required")
	}
	if c.ChangedBy <= 0 {
		return errors.New("changed_by is required")
	}
	switch c.FieldName {
	case GuardianPaymentFieldIBAN, GuardianPaymentFieldAccountHolder:
		if c.StudentID != nil {
			return errors.New("bank fields are guardian-scoped and must not carry a student_id")
		}
	case GuardianPaymentFieldIsPayer:
		if c.StudentID == nil || *c.StudentID <= 0 {
			return errors.New("student_id is required for a payer change")
		}
	default:
		return errors.New("unknown guardian payment field")
	}
	if c.OldValue == c.NewValue {
		return errors.New("old and new value are identical")
	}
	return nil
}

// GuardianFinancialChangeCreator is append-only; there is no application-side
// update or delete on the trail.
type GuardianFinancialChangeCreator interface {
	Create(ctx context.Context, event *GuardianFinancialChange) error
}
