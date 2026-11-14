package parentbilling

import (
	"context"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The debtor numbers of an import of existing care contracts (#4723). The
// Betreuungsvertrag reads the file and stores the contracts; the
// Elternabrechnung decides what happens with the number of each row and
// keeps it: per recipient of the invoices or per child, as the Träger says
// (#7007).
//
// Invariants:
//   - A number is taken over only for a child, or a payer, that has none: a
//     number moto keeps is never overwritten.
//   - Rows of one payer (or, per child, of one child) with different numbers
//     take over none of them.
//   - A row without a mail address names no family (FamilyKey) and takes
//     over no number.
//   - A number outside the format of the Träger (#6675) is not taken over.

// ContractImportNumber is the debtor number a row of the file names for the
// child of a contract and its family. ContractID is the contract the import
// stored, 0 while it only looks; Until the last day of that contract, zero
// for an open one.
type ContractImportNumber struct {
	Line           int
	FamilyEmail    string
	ContractID     int64
	ChildFirstName string
	ChildLastName  string
	ChildBirthDate *calendar.Date
	DebtorNumber   string
	Until          calendar.Date
}

// ContractImportNumberDecision is what happens with the number of a row:
// the number as it is stored and one of DebtorNumberNew, DebtorNumberSame,
// DebtorNumberDiffers, DebtorNumberSiblingsDiffer, DebtorNumberRowsDiffer,
// DebtorNumberInvalid, DebtorNumberNoFamily and DebtorNumberNoChild.
type ContractImportNumberDecision struct {
	Line         int
	DebtorNumber string
	Status       string
}

// ContractImportNumbers decides and keeps the debtor numbers of an import
// of care contracts. It runs in the Träger transaction the caller holds.
type ContractImportNumbers interface {
	DebtorNumberScopes
	// DecideContractImportNumbers answers per row what the import does with
	// its number, as the numbers stand before the call, and with save takes
	// the new ones over and records each in its trail.
	DecideContractImportNumbers(ctx context.Context, reviewer HolidayReviewer, numbers []ContractImportNumber, save bool) ([]ContractImportNumberDecision, error)
}
