package parentbilling

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The Mensa of the monthly run (#4686, ADR 0168). The Träger sets for each
// school how it bills the Mensa from a month on: a flat amount per child and
// month (Pauschale) or a price per meal (Spitzabrechnung). The meals come
// from the Mensa-Monatsfreigabe of the school; a month the OGS-Leitung has
// not released is not billed. A run of a month bills, on the invoice of a
// child's contract in that month, every released Mensa month up to it that
// no invoice billed yet, one position per month: the late release and the
// billing of several months at once (Hardt, MK-001) are the same rule.
//
// Invariants:
//   - A child's Mensa month is billed once (uq_care_meal_positions_child_month),
//     also with two releases at once.
//   - A Mensa price an issued position used never changes for its months.
//   - The run reads only the meals per child of a release, never the
//     registered days, the absences or the class.

// How a Mensa price bills a month.
const (
	// MealModeFlat bills a flat amount per child and month.
	MealModeFlat = "flat"
	// MealModePerMeal bills the meals of the month at a price each.
	MealModePerMeal = "per_meal"
)

// Why a released Mensa month of a child is not on an invoice of the run.
const (
	// MealUnbilledNoContract: the child has no contract at the school in the
	// month of the run.
	MealUnbilledNoContract = "no_contract"
	// MealUnbilledContractBilled: an earlier run of the month already issued
	// the contract's invoice; the next month's run bills the meals.
	MealUnbilledContractBilled = "contract_billed"
)

// MaxMealPriceCents caps a Mensa price at one million euro.
const MaxMealPriceCents = 100_000_000

var (
	// ErrCareMealTariffMonthBilled reports a change of a Mensa price an
	// issued position already used.
	ErrCareMealTariffMonthBilled = errors.New("an issued invoice already billed this Mensa price")
	// ErrCareMealTariffNotFound reports a Mensa price that does not exist.
	ErrCareMealTariffNotFound = errors.New("care meal tariff not found")
	// ErrCareMealTariffInvalid reports a Mensa price outside its bounds.
	ErrCareMealTariffInvalid = errors.New("care meal tariff is invalid")
)

// CareMealTariff is how a school's Mensa is billed from ValidFrom (the first
// day of a month) until ValidUntil, the day before the next price.
type CareMealTariff struct {
	ID          int64          `json:"id"`
	SchoolID    int64          `json:"school_id"`
	ValidFrom   calendar.Date  `json:"valid_from"`
	ValidUntil  *calendar.Date `json:"valid_until"`
	Mode        string         `json:"mode"`
	AmountCents int64          `json:"amount_cents"`
	CreatedAt   time.Time      `json:"created_at"`
}

// CareMealTariffs is the Mensa price list of the Träger. BilledUntil names
// per school the last Mensa month an issued invoice bills; a new price
// starts after it.
type CareMealTariffs struct {
	Schools     []CareSchool            `json:"schools"`
	Tariffs     []CareMealTariff        `json:"tariffs"`
	BilledUntil map[int64]calendar.Date `json:"billed_until"`
}

// CareMealPrice sets the Mensa of a school from ValidFrom on.
type CareMealPrice struct {
	SchoolID    int64
	ValidFrom   calendar.Date
	Mode        string
	AmountCents int64
}

// CareMealPosition is one Mensa month of a child on an invoice.
type CareMealPosition struct {
	SchoolID    int64         `json:"school_id"`
	StudentID   int64         `json:"student_id"`
	MealMonth   calendar.Date `json:"meal_month"`
	Mode        string        `json:"mode"`
	Quantity    int           `json:"quantity"`
	UnitCents   int64         `json:"unit_cents"`
	AmountCents int64         `json:"amount_cents"`
	// ReleaseVersion is the version of the school's release the position
	// bills (#4686); a later version shows the office it changed.
	ReleaseVersion int `json:"release_version"`
	// Key names the position for a change of the review (#6197).
	Key string `json:"key,omitempty"`
}

// CareBilledMealPosition is a child's Mensa month an issued invoice bills:
// the release version, quantity and amount it billed, the invoice and its
// child. Cancelled positions wait for the reissue of their month.
type CareBilledMealPosition struct {
	CareBilledMeal
	ReleaseVersion int
	Quantity       int
	AmountCents    int64
	InvoiceNumber  string
	ChildFirstName string
	ChildLastName  string
	Cancelled      bool
}

// CareMealChanged is a child's Mensa month an issued invoice bills with an
// amount a later release of the school no longer gives (#4686): the office
// cancels the invoice and issues it again; issued invoices never change.
type CareMealChanged struct {
	SchoolID       int64         `json:"school_id"`
	SchoolName     string        `json:"school_name"`
	StudentID      int64         `json:"student_id"`
	ChildFirstName string        `json:"child_first_name"`
	ChildLastName  string        `json:"child_last_name"`
	Month          calendar.Date `json:"month"`
	InvoiceNumber  string        `json:"invoice_number"`
	BilledQuantity int           `json:"billed_quantity"`
	BilledCents    int64         `json:"billed_cents"`
	NewQuantity    int           `json:"new_quantity"`
	NewCents       int64         `json:"new_cents"`
}

// CareMealGap is an ended month a school bills its Mensa for that its
// OGS-Leitung has not released yet ("Freigabe fehlt"). Expired: the month
// starts before the attendance the school still keeps, so nobody can
// release or bill it any more ("Nicht mehr abrechenbar").
type CareMealGap struct {
	SchoolID   int64         `json:"school_id"`
	SchoolName string        `json:"school_name"`
	Month      calendar.Date `json:"month"`
	Expired    bool          `json:"expired"`
}

// CareMealUnbilled is a released Mensa month of a child the run does not
// bill, with the reason.
type CareMealUnbilled struct {
	SchoolID       int64         `json:"school_id"`
	SchoolName     string        `json:"school_name"`
	StudentID      int64         `json:"student_id"`
	ChildFirstName string        `json:"child_first_name"`
	ChildLastName  string        `json:"child_last_name"`
	Month          calendar.Date `json:"month"`
	MealDays       int           `json:"meal_days"`
	Reason         string        `json:"reason"`
}

// CareBilledMeal is a child's Mensa month an issued invoice bills.
type CareBilledMeal struct {
	SchoolID  int64
	StudentID int64
	Month     calendar.Date
}

// CareMealChild is a child of a released Mensa month: its meals only.
type CareMealChild struct {
	StudentID int64
	FirstName string
	LastName  string
	MealDays  int
}

// CareMealRelease is a Mensa month a school of the Träger released.
type CareMealRelease struct {
	SchoolID int64
	Month    calendar.Date
	// Version is the latest release of the month; a correction raises it.
	Version  int
	Children []CareMealChild
}

// CareMeals is the Mensa-Monatsfreigabe of Meal Plan as the monthly run
// reads it, in the caller's Träger transaction. The root binds it.
//
// MealAttendanceFrom names per school the earliest day whose attendance the
// school still keeps; a month that starts before it can no longer be
// released (the rule of the Mensa-Monatsfreigabe).
type CareMeals interface {
	ReleasedMealMonths(ctx context.Context, organizationID int64, until calendar.Date) ([]CareMealRelease, error)
	MealAttendanceFrom(ctx context.Context, schoolIDs []int64) (map[int64]calendar.Date, error)
}

// CareMealBilling lets the Abrechnung of a Träger keep the Mensa prices of
// its schools. Every operation runs in the Träger transaction of its
// Träger, which the caller opens.
type CareMealBilling interface {
	CareMealTariffs(ctx context.Context, reviewer HolidayReviewer) (CareMealTariffs, error)
	SaveCareMealTariff(ctx context.Context, reviewer HolidayReviewer, price CareMealPrice) (CareMealTariffs, error)
	DeleteCareMealTariff(ctx context.Context, reviewer HolidayReviewer, schoolID int64, validFrom calendar.Date) (CareMealTariffs, error)
}
