package parentbilling

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The billing run for a holiday period (spec #4611 user stories 70, 71, 75
// and 78, #4668, ADR 0119). The Abrechnung starts a run for a period, checks
// the invoice of every family in its preview, corrects single amounts with a
// reason and releases the run. Holidays are billed in advance: a run bills
// the contribution each confirmation froze (ADR 0115), not the attendance,
// and may run before the first week.
//
// The release issues one invoice per confirmed registration not yet billed,
// numbered from the Träger's number range of the service without a gap. An
// issued invoice keeps its own copy of the recipient, the children and every
// position and never changes.

// States of a run.
const (
	RunCalculated = "calculated"
	RunReleased   = "released"
)

// LineCorrection is the kind of a position a correction of the run adds.
const LineCorrection = "correction"

// Limits of a run.
const (
	// MaxCorrectionReasonLength is the longest reason of a correction.
	MaxCorrectionReasonLength = 500
	// MaxNumberPatternLength is the longest pattern of a number range.
	MaxNumberPatternLength = 40
)

var (
	// ErrBillingRunNotFound reports a run that does not exist for the
	// Träger of the session.
	ErrBillingRunNotFound = errors.New("billing run not found")
	// ErrBillingRunOpen reports a second calculated run for a period.
	ErrBillingRunOpen = errors.New("the holiday period already has a billing run that is not released")
	// ErrBillingRunReleased reports a change of a released run.
	ErrBillingRunReleased = errors.New("the billing run is released and does not change")
	// ErrBillingRunChanged reports a release whose preview no longer
	// matches the preview the Abrechnung checked.
	ErrBillingRunChanged = errors.New("the billing run changed since it was checked")
	// ErrBillingRunEmpty reports a release without any invoice.
	ErrBillingRunEmpty = errors.New("the billing run has no invoice to issue")
	// ErrBillingFamilyNotFound reports a correction of a child the preview
	// of the run does not bill.
	ErrBillingFamilyNotFound = errors.New("the child is not billed in this run")
	// ErrNumberRangeMissing reports a release without a number range for
	// the service.
	ErrNumberRangeMissing = errors.New("the Träger has no number range for this service")
	// ErrInvoiceNumberTaken reports an invoice number another invoice of the
	// Träger already carries, after the pattern was changed.
	ErrInvoiceNumberTaken = errors.New("an invoice of the Träger already carries this number")
)

// BillingRun is a run as the list shows it. DocumentCount and TotalCents are
// set once the run is released.
type BillingRun struct {
	ID              int64          `json:"id"`
	Service         string         `json:"service"`
	HolidayPeriodID int64          `json:"holiday_period_id"`
	PeriodName      string         `json:"period_name"`
	ServiceFrom     calendar.Date  `json:"service_from"`
	ServiceUntil    calendar.Date  `json:"service_until"`
	Status          string         `json:"status"`
	CreatedAt       time.Time      `json:"created_at"`
	ReleasedAt      *time.Time     `json:"released_at"`
	IssuedOn        *calendar.Date `json:"issued_on"`
	DocumentCount   *int           `json:"document_count"`
	TotalCents      *int64         `json:"total_cents"`
}

// BillingCorrection is one entry of the history of a child's amount.
type BillingCorrection struct {
	ID                 int64     `json:"id"`
	ChildID            int64     `json:"child_id"`
	AmountCents        int64     `json:"amount_cents"`
	TotalBeforeCents   int64     `json:"total_before_cents"`
	TotalAfterCents    int64     `json:"total_after_cents"`
	Reason             string    `json:"reason"`
	CreatedByAccountID *int64    `json:"created_by_account_id"`
	CreatedAt          time.Time `json:"created_at"`
}

// BillingPosition is one position of an invoice: a line of the frozen
// contribution or a correction of the run, whose Note is its reason.
type BillingPosition struct {
	Kind                string         `json:"kind"`
	AmountCents         int64          `json:"amount_cents"`
	WeekStartsOn        *calendar.Date `json:"week_starts_on"`
	WeekEndsOn          *calendar.Date `json:"week_ends_on"`
	TimeModelName       string         `json:"time_model_name"`
	ChildRank           *int           `json:"child_rank"`
	DiscountBasisPoints *int           `json:"discount_basis_points"`
	DiscountCents       *int64         `json:"discount_cents"`
	Note                string         `json:"note"`
}

// BillingChild is a child of an invoice with its positions and the history
// of its corrections. ChildID is zero on an issued invoice once the
// registration is gone.
type BillingChild struct {
	ChildID     int64               `json:"child_id"`
	FirstName   string              `json:"first_name"`
	LastName    string              `json:"last_name"`
	Positions   []BillingPosition   `json:"positions"`
	Corrections []BillingCorrection `json:"corrections"`
	TotalCents  int64               `json:"total_cents"`
}

// BillingDocument is the invoice of one family: in a calculated run the
// draft without a number, in a released run the issued invoice.
type BillingDocument struct {
	ID                 int64          `json:"id"`
	Number             string         `json:"number"`
	IssuedOn           *calendar.Date `json:"issued_on"`
	RegistrationID     int64          `json:"registration_id"`
	RecipientFirstName string         `json:"recipient_first_name"`
	RecipientLastName  string         `json:"recipient_last_name"`
	RecipientEmail     string         `json:"recipient_email"`
	ServiceFrom        calendar.Date  `json:"service_from"`
	ServiceUntil       calendar.Date  `json:"service_until"`
	Children           []BillingChild `json:"children"`
	TotalCents         int64          `json:"total_cents"`
}

// BillingRunDetail is a run with its invoices. A calculated run shows the
// preview of today: Unbilled counts the confirmed registrations without a
// frozen contribution, which the run leaves out.
type BillingRunDetail struct {
	BillingRun
	Documents       []BillingDocument `json:"documents"`
	PreviewCount    int               `json:"preview_count"`
	PreviewCents    int64             `json:"preview_cents"`
	Unbilled        int               `json:"unbilled"`
	NumberPattern   string            `json:"number_pattern"`
	NumberRangeSet  bool              `json:"number_range_set"`
	BilledInAdvance bool              `json:"billed_in_advance"`
}

// BillingCorrectionInput changes the amount of one child of a family in a
// calculated run to TotalCents, with its reason.
type BillingCorrectionInput struct {
	RegistrationID int64
	ChildID        int64
	TotalCents     int64
	Reason         string
}

// BillingRelease names the preview the Abrechnung checked: the release
// refuses when the run would issue a different number of invoices or a
// different sum.
type BillingRelease struct {
	DocumentCount int
	TotalCents    int64
}

// NumberRange is the pattern of the invoice numbers of a service. Example
// is the number the next invoice would get today.
type NumberRange struct {
	Service   string     `json:"service"`
	Pattern   string     `json:"pattern"`
	Example   string     `json:"example"`
	UpdatedAt *time.Time `json:"updated_at"`
}

// HolidayBillingPeriod is a holiday period as a run needs it: its name and
// the days of its first and last week. Published names a period parents can
// register for.
type HolidayBillingPeriod struct {
	ID        int64         `json:"id"`
	Name      string        `json:"name"`
	StartsOn  calendar.Date `json:"starts_on"`
	EndsOn    calendar.Date `json:"ends_on"`
	Published bool          `json:"-"`
}

// HolidayBillingChild is a child of a confirmed registration by its name.
type HolidayBillingChild struct {
	ID        int64
	FirstName string
	LastName  string
}

// HolidayBillingRegistration is a confirmed registration with the parent
// who registered, the recipient of its invoice.
type HolidayBillingRegistration struct {
	ID                int64
	GuardianFirstName string
	GuardianLastName  string
	GuardianEmail     string
	Children          []HolidayBillingChild
}

// HolidayBillingRegistrations is Enrollment as the billing run reads a
// period and its confirmed registrations. The root binds it. Both run in
// the Träger transaction of the reviewer's Träger and refuse a period of
// another Träger; ConfirmedHolidayRegistrations records the read in the
// period's audit trail (ADR 0114).
type HolidayBillingRegistrations interface {
	HolidayBillingPeriods(ctx context.Context, reviewer HolidayReviewer) ([]HolidayBillingPeriod, error)
	HolidayBillingPeriod(ctx context.Context, reviewer HolidayReviewer, periodID int64) (HolidayBillingPeriod, error)
	ConfirmedHolidayRegistrations(ctx context.Context, reviewer HolidayReviewer, periodID int64) ([]HolidayBillingRegistration, error)
}

// BillingRuns lets the Abrechnung of a Träger start, check, correct and
// release its billing runs and keep its number ranges. Every operation
// names the person of the session and runs in the Träger transaction of its
// Träger, which the caller opens. Every read of personal data and every
// change of a run is recorded in the run's trail.
type BillingRuns interface {
	ListBillingRuns(ctx context.Context, organizationID int64) ([]BillingRun, error)
	// ListBillablePeriods lists the published holiday periods of the Träger
	// with weeks, newest first, so the Abrechnung can start a run without
	// the right "Anmeldung verwalten". It carries no personal data.
	ListBillablePeriods(ctx context.Context, reviewer HolidayReviewer) ([]HolidayBillingPeriod, error)
	StartHolidayBillingRun(ctx context.Context, reviewer HolidayReviewer, periodID int64) (BillingRun, error)
	BillingRun(ctx context.Context, reviewer HolidayReviewer, runID int64) (BillingRunDetail, error)
	CorrectBillingRun(ctx context.Context, reviewer HolidayReviewer, runID int64, input BillingCorrectionInput) (BillingRunDetail, error)
	ReleaseBillingRun(ctx context.Context, reviewer HolidayReviewer, runID int64, release BillingRelease) (BillingRunDetail, error)
	// ExportBillingRun renders the overview of a released run for the
	// Kämmerei as a PDF, an Excel or a CSV file (#4672, ADR 0122).
	ExportBillingRun(ctx context.Context, reviewer HolidayReviewer, runID int64, format string) (ExportFile, error)
	ListNumberRanges(ctx context.Context, organizationID int64) ([]NumberRange, error)
	SetNumberRange(ctx context.Context, organizationID int64, service, pattern string) (NumberRange, error)
}
