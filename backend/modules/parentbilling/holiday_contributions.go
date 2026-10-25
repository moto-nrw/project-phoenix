package parentbilling

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The holiday contribution (spec #4611 user stories 29, 61, 63 and 69, #4664,
// ADR 0115). Parents see it with its positions in the summary of the public
// registration before they send it; the Träger-Büro sees it in the
// registration and freezes it when it confirms the registration. A frozen
// contribution never changes: a later price, time model or rule applies only
// to registrations confirmed after it.
//
// The chain runs per registration in a fixed order: the base price of every
// booked week from the price of its time model, the sibling rule, then the
// exemption for families receiving benefits. Each rule rounds half up to
// whole cent and writes its own line; a contribution is the sum of its
// lines and never negative. The rules valid on the first booked day of the
// registration apply.

// Kinds of a contribution line.
const (
	LineBase              = "base"
	LineSibling           = RuleSibling
	LineBenefitsExemption = RuleBenefitsExemption
)

// HolidayRegistrationConfirmed is the state of a confirmed registration as
// the Holiday registrations port reports it.
const HolidayRegistrationConfirmed = "confirmed"

var (
	// ErrHolidayLinkNotFound reports a public link without a published
	// period of an active Träger.
	ErrHolidayLinkNotFound = errors.New("holiday registration link not found")
	// ErrHolidayPeriodNotFound reports a period that does not exist for the
	// Träger of the session.
	ErrHolidayPeriodNotFound = errors.New("holiday period not found")
	// ErrHolidayRegistrationNotFound reports a registration that does not
	// exist in the period of the session's Träger.
	ErrHolidayRegistrationNotFound = errors.New("holiday registration not found")
	// ErrHolidayRegistrationStateConflict reports a confirmation the state of
	// the registration does not allow.
	ErrHolidayRegistrationStateConflict = errors.New("the holiday registration cannot be confirmed in its current state")
	// ErrBenefitsExemptionUnavailable reports an exemption the Träger has no
	// valid exemption rule for.
	ErrBenefitsExemptionUnavailable = errors.New("the Träger has no exemption rule for families receiving benefits at this time")
	// ErrInvalidHolidayQuote reports bookings of a quote that the period does
	// not offer.
	ErrInvalidHolidayQuote = errors.New("invalid holiday contribution quote")
)

// HolidayContributionLine is one position of a contribution. A base line
// names its week and time model; a sibling line the rank of the child and
// its discount; an exemption line its note. Discounts are negative.
type HolidayContributionLine struct {
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

// HolidayChildContribution is the contribution of one child. ChildID names
// the child of a registration; a quote names the child by its Position in
// the form (1 to 3).
type HolidayChildContribution struct {
	ChildID    int64                     `json:"child_id"`
	Position   int                       `json:"position"`
	Lines      []HolidayContributionLine `json:"lines"`
	TotalCents int64                     `json:"total_cents"`
}

// HolidayContribution is the contribution of a registration or a quote.
// ExemptionAvailable reports a valid exemption rule, BenefitsExemption that
// the exemption applies. A frozen contribution carries the time and the
// account of its confirmation.
type HolidayContribution struct {
	Children           []HolidayChildContribution `json:"children"`
	TotalCents         int64                      `json:"total_cents"`
	ExemptionAvailable bool                       `json:"exemption_available"`
	BenefitsExemption  bool                       `json:"benefits_exemption"`
	Frozen             bool                       `json:"frozen"`
	ConfirmedAt        *time.Time                 `json:"confirmed_at"`
}

// HolidayConfirmation is a confirmed registration with the contribution its
// confirmation froze.
type HolidayConfirmation struct {
	RegistrationID int64               `json:"id"`
	Status         string              `json:"status"`
	Contribution   HolidayContribution `json:"contribution"`
}

// HolidayBooking is the time model a child books in one week, with the
// price the time model has now.
type HolidayBooking struct {
	WeekID        int64
	WeekStartsOn  calendar.Date
	WeekEndsOn    calendar.Date
	TimeModelID   int64
	TimeModelName string
	PriceCents    int64
}

// HolidayChild is a child of a registration with its bookings. Position is
// its place in the form.
type HolidayChild struct {
	ID       int64
	Position int
	Bookings []HolidayBooking
}

// HolidayRegistration is a holiday registration as the contribution needs
// it: its state, the parents' statement on benefits and the children.
type HolidayRegistration struct {
	ID               int64
	Status           string
	BenefitsDeclared bool
	Children         []HolidayChild
}

// HolidayTimeModel is a bookable time model of a week with its price.
type HolidayTimeModel struct {
	ID         int64
	Name       string
	PriceCents int64
}

// HolidayWeek is a bookable week of a period.
type HolidayWeek struct {
	ID         int64
	StartsOn   calendar.Date
	EndsOn     calendar.Date
	TimeModels []HolidayTimeModel
}

// HolidayLink is the Träger and the period a public link names.
type HolidayLink struct {
	OrganizationID int64
	PeriodID       int64
}

// HolidayReviewer is the person of the Träger-Büro: the Träger and the
// account of the session.
type HolidayReviewer struct {
	OrganizationID int64
	AccountID      int64
}

// HolidayRegistrations is Enrollment as the holiday contribution reads and
// confirms a registration. The root binds it. ResolveHolidayLink runs in
// the caller's administrative transaction, everything else in the Träger
// transaction of the Träger; HolidayRegistration and
// ConfirmHolidayRegistration record themselves in the registration's audit
// trail and refuse a period or registration of another Träger.
type HolidayRegistrations interface {
	ResolveHolidayLink(ctx context.Context, token string) (HolidayLink, error)
	HolidayWeeks(ctx context.Context, link HolidayLink) ([]HolidayWeek, error)
	HolidayRegistration(ctx context.Context, reviewer HolidayReviewer, periodID, registrationID int64) (HolidayRegistration, error)
	ConfirmHolidayRegistration(ctx context.Context, reviewer HolidayReviewer, periodID, registrationID int64) (HolidayRegistration, error)
}

// HolidayQuoteBooking is a booking of the public form.
type HolidayQuoteBooking struct {
	WeekID      int64
	TimeModelID int64
}

// HolidayQuoteChild is a child of the public form by its bookings.
type HolidayQuoteChild struct {
	Bookings []HolidayQuoteBooking
}

// HolidayContributions computes, shows and freezes the holiday contribution.
// ResolveHolidayLink runs in the administrative transaction, every other
// operation in the Träger transaction of the link's or the session's
// Träger, which the caller opens.
type HolidayContributions interface {
	ResolveHolidayLink(ctx context.Context, token string) (HolidayLink, error)
	// QuoteHolidayContribution computes the contribution of the public form
	// before it is sent, with the current prices and rules. It stores
	// nothing.
	QuoteHolidayContribution(ctx context.Context, link HolidayLink, children []HolidayQuoteChild, benefitsDeclared bool) (HolidayContribution, error)
	// HolidayContribution shows the frozen contribution of a confirmed
	// registration, or else what a confirmation would freeze now.
	HolidayContribution(ctx context.Context, reviewer HolidayReviewer, periodID, registrationID int64) (HolidayContribution, error)
	// ConfirmHolidayRegistration confirms the registration and freezes its
	// contribution in the same transaction. benefitsExemption applies the
	// exemption rule.
	ConfirmHolidayRegistration(ctx context.Context, reviewer HolidayReviewer, periodID, registrationID int64, benefitsExemption bool) (HolidayConfirmation, error)
}
