package ports

import (
	"errors"
	"fmt"
)

// Payment input errors — HTTP 400. Each is its own sentinel so the handler can
// render one specific German sentence: these messages are shown to a school
// office worker, not to a developer, and a wrapped "invalid payment value:
// malformed IBAN" told them nothing they could act on.
var (
	// ErrGuardianPaymentInvalid is the class marker every input error wraps,
	// so callers can classify without listing each case.
	ErrGuardianPaymentInvalid = errors.New("invalid payment value")

	// ErrGuardianIBANInvalid marks an IBAN that fails the structural check or
	// the ISO 13616 mod-97 checksum.
	ErrGuardianIBANInvalid = fmt.Errorf("%w: malformed IBAN", ErrGuardianPaymentInvalid)

	// ErrGuardianAccountHolderTooLong marks a Kontoinhaber over the column bound.
	ErrGuardianAccountHolderTooLong = fmt.Errorf("%w: account holder too long", ErrGuardianPaymentInvalid)

	// ErrGuardianStudentRequired marks a missing student id.
	ErrGuardianStudentRequired = fmt.Errorf("%w: student id is required", ErrGuardianPaymentInvalid)
)

// ErrGuardianNotLinkedToStudent is returned when a guardian is named as payer
// for a child they are not linked to — HTTP 400 rather than 404: the caller
// asked for an assignment that cannot exist.
var ErrGuardianNotLinkedToStudent = errors.New("guardian is not linked to this student")

// GuardianPaymentInput is the full bank section of a guardian; nil clears a
// field.
type GuardianPaymentInput struct {
	IBAN          *string
	AccountHolder *string
}

// GuardianPaymentMasked is the default (masked) read: the IBAN shows its last
// four characters only. The Kontoinhaber is a name, not an access credential,
// and is served in full. The unmasked IBAN requires RevealGuardianPayment.
type GuardianPaymentMasked struct {
	GuardianProfileID int64
	IBANMasked        *string
	AccountHolder     *string
}

// GuardianPaymentPlain carries the unmasked values after an audited reveal.
type GuardianPaymentPlain struct {
	GuardianProfileID int64
	IBAN              *string
	AccountHolder     *string
}

// GuardianPaymentRow is one line of the Bankverbindungen list: a child, the
// guardian charged for it, and that guardian's IBAN. GuardianProfileID is nil
// when no payer is assigned — those rows are the point of the list, not noise.
// IBAN carries the full value ONLY on the export path; the on-screen list
// fills IBANMasked and leaves it empty.
type GuardianPaymentRow struct {
	StudentID         int64
	StudentName       string
	SchoolClass       string
	GuardianProfileID *int64
	GuardianName      string
	RelationshipType  string
	// AccountHolder is the resolved Kontoinhaber: the explicit override when
	// the account runs on another name, otherwise the guardian's own name.
	AccountHolder string
	IBAN          string
	IBANMasked    string
}

// HasIBAN reports whether a usable bank account is stored for this row.
func (r GuardianPaymentRow) HasIBAN() bool {
	return r.IBAN != "" || r.IBANMasked != ""
}
