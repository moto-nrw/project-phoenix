package parentbilling

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The debtor number of a child (#7007). A Träger says with
// parent_billing.debtor_number_scope whom its debtor numbers name: the
// payer (DebtorNumberScopePayer, the default and the rule of #6692: one
// number per recipient of the invoices, FamilyKey, siblings share it) or
// each child (DebtorNumberScopeChild, as SJR Pforzheim keeps them).
//
// Invariants:
//   - A child of the Träger is its family (FamilyKey) with the child's first
//     name, last name and date of birth (ChildKey), compared without case:
//     the child of a care contract and the same child at a holiday
//     registration share one number. A new mail address makes a new child,
//     as it makes a new family.
//   - Both kinds of numbers stay stored whatever the setting says; the
//     setting decides only which one a new invoice copies. An issued
//     invoice keeps the number it copied.
//   - The format of the Träger (#6675) applies to every number the office
//     enters, as at the family.
//   - Every read and change the office makes is recorded in the trail of
//     the child's number, by its ID, without names.

// Whom a debtor number of a Träger names (parent_billing.debtor_number_scope).
const (
	// DebtorNumberScopePayer is one number per recipient of the invoices.
	DebtorNumberScopePayer = "payer"
	// DebtorNumberScopeChild is one number per child.
	DebtorNumberScopeChild = "child"
)

// DebtorNumberScopes reads whom the debtor numbers of a Träger name: its
// value of parent_billing.debtor_number_scope or the default. It joins the
// caller's Träger transaction.
type DebtorNumberScopes interface {
	DebtorNumberScope(ctx context.Context, organizationID int64) (string, error)
}

// ErrChildNotFound reports a child the Träger has no contract, document or
// number for under the family it names.
var ErrChildNotFound = errors.New("no contract or number of the Träger names this child")

// ChildKey is the key of a child within its family: the trimmed first and
// last name without case and the date of birth, empty without one. An
// empty name gives an empty key: such a child has no number of its own.
func ChildKey(firstName, lastName string, birthDate *calendar.Date) string {
	first, last := strings.ToLower(strings.TrimSpace(firstName)), strings.ToLower(strings.TrimSpace(lastName))
	if first == "" || last == "" {
		return ""
	}
	birth := ""
	if birthDate != nil && !birthDate.IsZero() {
		birth = birthDate.String()
	}
	return first + "\x1f" + last + "\x1f" + birth
}

// ChildDebtorNumberKey is the key of a child of the Träger: its family and
// its ChildKey.
func ChildDebtorNumberKey(familyEmail, childKey string) string {
	return FamilyKey(familyEmail) + "\x1e" + childKey
}

// ChildDebtorNumber is the number of one child of a family of the Träger.
// LastUsedOn is the last day a save, a registration or a contract needs it;
// the retention counts from it.
type ChildDebtorNumber struct {
	ID             int64          `json:"id"`
	FamilyEmail    string         `json:"family_email"`
	ChildFirstName string         `json:"child_first_name"`
	ChildLastName  string         `json:"child_last_name"`
	ChildBirthDate *calendar.Date `json:"child_birth_date"`
	DebtorNumber   string         `json:"debtor_number"`
	LastUsedOn     calendar.Date  `json:"-"`
	UpdatedAt      time.Time      `json:"-"`
}

// Key is the key of the child within the Träger.
func (n ChildDebtorNumber) Key() string {
	return ChildDebtorNumberKey(n.FamilyEmail, ChildKey(n.ChildFirstName, n.ChildLastName, n.ChildBirthDate))
}

// ChildDebtorNumberInput sets the number of a child of a family from the
// list of families; an empty number removes it.
type ChildDebtorNumberInput struct {
	FamilyEmail    string
	ChildFirstName string
	ChildLastName  string
	ChildBirthDate *calendar.Date
	DebtorNumber   string
}

// FamilyChildNumbers is a family with its children as the list of families
// names them when the Träger keeps a number per child: the recipient of its
// invoices, empty for a family known only by a number.
type FamilyChildNumbers struct {
	FamilyEmail        string        `json:"family_email"`
	RecipientFirstName string        `json:"recipient_first_name"`
	RecipientLastName  string        `json:"recipient_last_name"`
	Children           []FamilyChild `json:"children"`
}

// FamilyChild is a child of a family as the list of families shows it when the
// Träger keeps a number per child: a child of a care contract of the
// family or a child that has a number, with that number, empty for none.
type FamilyChild struct {
	FirstName    string         `json:"first_name"`
	LastName     string         `json:"last_name"`
	BirthDate    *calendar.Date `json:"birth_date"`
	DebtorNumber string         `json:"debtor_number"`
}
