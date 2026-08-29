package users

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// GuardianFinancialData is the 1:1 bank record of a guardian (#2608).
// Deliberately its own table: nothing outside the guardians:financial code
// path may ever join or list these columns. Generic guardian JSON must not
// expose them, so every field is json:"-" and the handlers use dedicated
// masked/reveal responses.
//
// The IBAN hangs off the guardian rather than off the child because a child
// has no bank account, and because guardians are already shared across
// siblings — maintaining it once covers every sibling. Which of a child's
// guardians is actually charged is a property of the relationship
// (StudentGuardian.IsPayer), not of the bank record.
type GuardianFinancialData struct {
	base.Model `bun:"schema:users,table:guardian_financial_data"`
	base.TenantModel
	GuardianProfileID int64   `bun:"guardian_profile_id,notnull" json:"guardian_profile_id"`
	IBAN              *string `bun:"iban" json:"-"`
	// AccountHolder names the Kontoinhaber when the account does not run on
	// the guardian's own name (a spouse, a maiden name). nil means the
	// guardian is the holder; readers fall back to the guardian's name.
	AccountHolder *string `bun:"account_holder" json:"-"`
}

// Validate ensures the financial row is storable. IBAN syntax is validated in
// the service layer (the rule carries business meaning, not a storage rule).
func (f *GuardianFinancialData) Validate() error {
	if f.GuardianProfileID <= 0 {
		return errors.New("guardian_profile_id is required")
	}
	return nil
}

// HasData reports whether the row carries anything worth keeping. An edit that
// clears both fields leaves an empty shell, which readers must treat as "no
// bank details stored".
func (f *GuardianFinancialData) HasData() bool {
	return (f.IBAN != nil && *f.IBAN != "") || (f.AccountHolder != nil && *f.AccountHolder != "")
}

// GuardianFinancialDataRepository persists the 1:1 guardian bank rows.
type GuardianFinancialDataRepository interface {
	Create(ctx context.Context, data *GuardianFinancialData) error
	Update(ctx context.Context, data *GuardianFinancialData) error
	FindByGuardianProfileID(ctx context.Context, guardianProfileID int64) (*GuardianFinancialData, error)
	// ListByGuardianProfileIDs loads the bank rows of several guardians at
	// once, keyed by guardian profile ID. The Bankverbindungen overview needs
	// one row per child and would otherwise issue a query per sibling.
	ListByGuardianProfileIDs(ctx context.Context, guardianProfileIDs []int64) (map[int64]*GuardianFinancialData, error)
}

// GuardianPaymentAssignment is a projection of one child and the guardian
// charged for it, scanned from a join across students_guardians → students →
// persons → guardian_profiles. Not a persisted entity: it is the row shape of
// the Bankverbindungen list and its export.
type GuardianPaymentAssignment struct {
	StudentID           int64  `bun:"student_id"`
	StudentFirstName    string `bun:"student_first_name"`
	StudentLastName     string `bun:"student_last_name"`
	SchoolClass         string `bun:"school_class"`
	GuardianProfileID   *int64 `bun:"guardian_profile_id"`
	GuardianFirstName   string `bun:"guardian_first_name"`
	GuardianLastName    string `bun:"guardian_last_name"`
	RelationshipTypeRaw string `bun:"relationship_type"`
}
