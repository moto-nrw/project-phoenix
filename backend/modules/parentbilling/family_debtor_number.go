package parentbilling

import (
	"context"
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
// line, and in the format of the Träger's finance system (#6675). An empty
// result means none; the format applies only to a number. A number in
// another format is a DebtorNumberFormatError. Stored numbers are never
// checked again: only what the office enters passes here.
func NormalizeDebtorNumber(raw string, format DebtorNumberFormat) (string, error) {
	number := strings.TrimSpace(raw)
	switch {
	case strings.ContainsAny(number, "\r\n\v\f\u0085\u2028\u2029"):
		return "", fmt.Errorf("must be one line")
	case utf8.RuneCountInString(number) > MaxDebtorNumberLength:
		return "", fmt.Errorf("must be at most %d characters", MaxDebtorNumberLength)
	case number != "" && !format.Admits(number):
		return "", &DebtorNumberFormatError{Format: format}
	}
	return number, nil
}

// DebtorNumberFormat is the format of a debtor number in the finance system
// of a Träger (#6675): with DigitsOnly, only the digits 0 to 9 and between
// MinDigits and MaxDigits of them. The zero value admits every number.
type DebtorNumberFormat struct {
	DigitsOnly bool `json:"digits_only"`
	MinDigits  int  `json:"min_digits"`
	MaxDigits  int  `json:"max_digits"`
}

// NewDebtorNumberFormat builds the format from the Träger's three settings.
// The two lengths are saved one after the other, so they count in either
// order; each is held within 1 and MaxDebtorNumberLength. Off has no
// lengths.
func NewDebtorNumberFormat(digitsOnly bool, first, second int) DebtorNumberFormat {
	if !digitsOnly {
		return DebtorNumberFormat{}
	}
	low, high := min(first, second), max(first, second)
	return DebtorNumberFormat{DigitsOnly: true, MinDigits: max(1, min(low, MaxDebtorNumberLength)),
		MaxDigits: max(1, min(high, MaxDebtorNumberLength))}
}

// Admits reports whether number has the format.
func (f DebtorNumberFormat) Admits(number string) bool {
	if !f.DigitsOnly {
		return true
	}
	if len(number) < f.MinDigits || len(number) > f.MaxDigits {
		return false
	}
	for _, digit := range number {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}

// exampleDigits are the digits an example number takes from the front.
const exampleDigits = "20815471136925814703692581470369258"

// Example is a number in the format, as the office reads it in a hint or a
// refusal: five digits where the lengths allow it, else the nearest length.
// Without a format it is empty.
func (f DebtorNumberFormat) Example() string {
	if !f.DigitsOnly {
		return ""
	}
	return exampleDigits[:max(f.MinDigits, min(5, f.MaxDigits))]
}

// DebtorNumberFormatError refuses a number in another format than the one
// of the Träger, naming the format so the refusal can say what it expects.
type DebtorNumberFormatError struct {
	Format DebtorNumberFormat
}

func (e *DebtorNumberFormatError) Error() string {
	return fmt.Sprintf("must have %d to %d digits and nothing else", e.Format.MinDigits, e.Format.MaxDigits)
}

// DebtorNumberFormats reads the format of a Träger's debtor numbers: its
// values of the parent_billing.debtor_number_* settings or their defaults.
// It joins the caller's Träger transaction.
type DebtorNumberFormats interface {
	DebtorNumberFormat(ctx context.Context, organizationID int64) (DebtorNumberFormat, error)
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
	// FamilyEmail is the family whose number differs: the family of the
	// registration or, in a sibling group, the family of a sibling.
	FamilyEmail        string
	FamilyDebtorNumber string
	DebtorNumber       string
	// Families names every conflict in a sibling group, including the first.
	Families []FamilyDebtorNumber
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
// of the family needs the number, the registration the number comes from
// and, when not nil, the number itself. Without a number it tells the
// family nothing and is not written.
type FamilyMemory struct {
	FamilyEmail    string
	FirstName      string
	LastName       string
	UsedOn         calendar.Date
	RegistrationID int64
	DebtorNumber   *string
}

// FamilyDebtorNumberInput sets the number of a family of the Träger from
// the list of families: the family by its key, the number empty for none.
type FamilyDebtorNumberInput struct {
	FamilyEmail  string
	DebtorNumber string
}
