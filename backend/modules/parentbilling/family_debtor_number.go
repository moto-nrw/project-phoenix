package parentbilling

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The debtor number of a family (#6692): the number of the family in the
// finance system of the Träger, kept at the family of the Träger (its row in
// FamilyBilling, keyed by FamilyKey), not only at a holiday registration. A
// confirmation of a holiday registration and a change of its number write
// it there, so the next registration of the family is recognised after the
// first one is deleted. A family the holiday care knows only has a row
// without a contract, which keeps the number for its own period (see
// FamilyBilling.LastUsedOn).

// FamilyDebtorNumberRetentionYears is the period of a family without a care
// contract (#6692): its row goes on 1 January of the fourth year after the
// last day a holiday registration needed the number, unless a document of
// the family is still kept. Three years after the end of the year are the
// regular limitation period of the contribution the number identifies
// (§§ 195, 199 BGB); they also outlast every retention a holiday
// registration can name (at most 1095 days after its period, #6691), and the
// deletion log of a school keeps three years too.
const FamilyDebtorNumberRetentionYears = 3

// MaxDebtorNumberLength bounds a debtor number like the holiday one (#5621).
const MaxDebtorNumberLength = 35

// NormalizeDebtorNumber is the one check of a debtor number the Träger
// enters, at a family, at a confirmation and at a change of a holiday
// registration: trimmed, at most MaxDebtorNumberLength characters, one
// line. An empty result means none. The format check of the finance
// system (#6675) extends this function.
func NormalizeDebtorNumber(raw string) (string, error) {
	number := strings.TrimSpace(raw)
	switch {
	case strings.ContainsAny(number, "\r\n\v\f\u0085  "):
		return "", fmt.Errorf("must be one line")
	case utf8.RuneCountInString(number) > MaxDebtorNumberLength:
		return "", fmt.Errorf("must be at most %d characters", MaxDebtorNumberLength)
	}
	return number, nil
}

// Which number applies when the number entered at a holiday registration
// differs from the one the family has (#6692). Without a choice the
// confirmation or change is a FamilyDebtorNumberConflict and nothing is
// written.
const (
	// FamilyDebtorNumberKeep keeps the number of the family; only the
	// registration gets the new one.
	FamilyDebtorNumberKeep = "keep"
	// FamilyDebtorNumberReplace makes the new number the family's.
	FamilyDebtorNumberReplace = "replace"
)

// FamilyDebtorNumberConflict refuses a number that differs from the number
// the family has, as long as the Träger-Büro has not said which applies. It
// names both, so the office can choose.
type FamilyDebtorNumberConflict struct {
	FamilyDebtorNumber string
	DebtorNumber       string
}

func (c *FamilyDebtorNumberConflict) Error() string {
	return "the debtor number differs from the number the family has"
}

// HolidayFamilyDebtorNumber is the number the family of a holiday
// registration has, as the confirmation proposes it; empty without one.
type HolidayFamilyDebtorNumber struct {
	DebtorNumber string `json:"debtor_number"`
}

// FamilyMemory is what a holiday registration tells the family of its
// Träger (#6692): the parent who registered, the latest day a registration
// of the family needs the number and, when not nil, the number itself.
type FamilyMemory struct {
	FamilyEmail  string
	FirstName    string
	LastName     string
	UsedOn       calendar.Date
	DebtorNumber *string
}

// FamilyDebtorNumberInput sets the number of a family of the Träger from
// the list of families: the family by its key, the number empty for none.
type FamilyDebtorNumberInput struct {
	FamilyEmail  string
	DebtorNumber string
}
