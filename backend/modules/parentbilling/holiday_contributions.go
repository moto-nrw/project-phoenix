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
// exemption for families receiving benefits or the cost takeover by a cost
// bearer (#5627), never both. Each rule rounds half up to
// whole cent and writes its own line; a contribution is the sum of its
// lines and never negative. The rules valid on the first booked day of the
// registration apply.

// Kinds of a contribution line.
const (
	LineBase              = "base"
	LineSibling           = RuleSibling
	LineBenefitsExemption = RuleBenefitsExemption
	// LineCostTakeover (cost_bearers.go) is what a cost bearer pays.
)

// How a Träger ranks siblings and when a cost bearer takes over (#5664). The
// values are those of the settings parent_billing.sibling_rank and
// parent_billing.cost_takeover_order.
const (
	// SiblingRankHighestContribution makes the child with the highest base
	// price the first child: the default.
	SiblingRankHighestContribution = "highest_contribution"
	// SiblingRankOldestChild makes the oldest child the first child.
	SiblingRankOldestChild = "oldest_child"
	// CostTakeoverAfterDiscount lets a cost bearer pay what is left after the
	// sibling discount: the default.
	CostTakeoverAfterDiscount = "after_discount"
	// CostTakeoverBeforeDiscount lets a cost bearer pay the full base price;
	// the discount reaches only the other children.
	CostTakeoverBeforeDiscount = "before_discount"
)

// HolidayChainSettings are the two choices of a Träger in the chain. A frozen
// contribution keeps those it was computed with (#5664).
type HolidayChainSettings struct {
	SiblingRank       string
	CostTakeoverOrder string
}

// States of a registration as the Holiday registrations port reports them.
const (
	HolidayRegistrationConfirmed = "confirmed"
	HolidayRegistrationCancelled = "cancelled"
)

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
	// ErrHolidayRegistrationFrozen reports a cancellation of a registration
	// after the registration deadline; the Träger-Büro reopens it first.
	ErrHolidayRegistrationFrozen = errors.New("the holiday registration is frozen")
	// ErrDebtorNumberLocked reports a change of the debtor number of a
	// registration an issued invoice already bills: the invoice keeps its
	// copy (#5621, ADR 0119).
	ErrDebtorNumberLocked = errors.New("an issued invoice keeps the debtor number of the registration")
	// ErrInvalidHolidayPlaces reports bookings of a rebooking or a capacity
	// that Enrollment refuses (#4699); it wraps the refused fields.
	ErrInvalidHolidayPlaces = errors.New("invalid holiday bookings or capacity")
	// ErrHolidayWeekNotFound reports a week that is not a week of the period.
	ErrHolidayWeekNotFound = errors.New("holiday week not found")
)

// HolidayContributionLine is one position of a contribution. A base line
// names its week and time model; a sibling line the rank of the child and
// its discount; an exemption line its note; a cost takeover the cost bearer
// by its ID and a copy of its name (#5627). Discounts and takeovers are
// negative.
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
	CostBearerID        *int64         `json:"cost_bearer_id"`
	CostBearerName      string         `json:"cost_bearer_name"`
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
// the exemption applies, CostBearerID and CostBearerName the cost bearer
// that pays (#5627). A frozen contribution carries the time and the
// account of its confirmation; Invoiced reports that an issued invoice
// bills the registration, which then keeps its debtor number (#5621).
type HolidayContribution struct {
	Children           []HolidayChildContribution `json:"children"`
	TotalCents         int64                      `json:"total_cents"`
	ExemptionAvailable bool                       `json:"exemption_available"`
	BenefitsExemption  bool                       `json:"benefits_exemption"`
	CostBearerID       *int64                     `json:"cost_bearer_id"`
	CostBearerName     string                     `json:"cost_bearer_name"`
	Frozen             bool                       `json:"frozen"`
	ConfirmedAt        *time.Time                 `json:"confirmed_at"`
	Invoiced           bool                       `json:"invoiced"`
	// Siblings are the other registrations of the sibling group of a
	// registration (#5628); none outside a group and in a quote.
	Siblings []HolidaySibling `json:"siblings,omitempty"`
	// SiblingDebtorNumber is the debtor number the group already has, which
	// a confirmation without a number takes (#5628).
	SiblingDebtorNumber string `json:"sibling_debtor_number,omitempty"`
	// SiblingsInvoiced reports that an issued invoice bills a registration
	// of the group, which then neither merges nor splits (#5628).
	SiblingsInvoiced bool `json:"siblings_invoiced,omitempty"`
	// Chain names how the contribution was computed (#5664). A frozen one
	// keeps it, so a later change of the settings does not reach it.
	Chain HolidayChainSettings `json:"-"`
	// CorrectionNeeded reports that the issued invoice billed the family or
	// the cost bearer another amount than the frozen contribution has now,
	// for example after a
	// sibling was cancelled (#5662). The invoice keeps its amount; the
	// Träger-Büro corrects it with a cancellation or credit note (#4690).
	CorrectionNeeded bool `json:"correction_needed,omitempty"`
}

// IssuedContribution is what the issued invoice of a registration billed,
// without the corrections of its run (#5662): TotalCents the family pays and
// CostTakeoverCents the cost bearer pays.
type IssuedContribution struct {
	TotalCents        int64
	CostTakeoverCents int64
}

// HolidayCancellation is a cancelled registration (#5662).
type HolidayCancellation struct {
	RegistrationID int64  `json:"id"`
	Status         string `json:"status"`
	// MovedUp names the registrations whose bookings got the freed places
	// (#4699).
	MovedUp []int64 `json:"moved_up"`
}

// HolidayConfirmation is a confirmed registration with the contribution its
// confirmation froze.
type HolidayConfirmation struct {
	RegistrationID int64               `json:"id"`
	Status         string              `json:"status"`
	Contribution   HolidayContribution `json:"contribution"`
}

// HolidayBookingReplacement is a week whose frozen charge a rebooking retains.
type HolidayBookingReplacement struct {
	WeekStartsOn calendar.Date
	WeekEndsOn   calendar.Date
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
	// Waitlisted marks a booking that waits for a place (#4699). It costs
	// nothing and gives the child no sibling rank until it moves up.
	Waitlisted bool
	// Promoted distinguishes waiting-list admissions from direct rebookings.
	Promoted bool
	// ReplacedWeeks preserves the lineage of replacements admitted later.
	ReplacedWeeks []HolidayBookingReplacement
}

// HolidayChild is a child of a registration with its bookings. Position is
// its place in the form; the names show only in the Träger-Büro (#5628).
// BirthDate ranks the children of a registration when the Träger ranks by
// age; a quote knows no date and carries AgeRank instead (#5664).
type HolidayChild struct {
	ID        int64
	Position  int
	FirstName string
	LastName  string
	BirthDate calendar.Date
	AgeRank   int
	Bookings  []HolidayBooking
}

// HolidayRegistration is a holiday registration as the contribution needs
// it: its state, when the parents sent it, which ranks siblings of equal
// contribution (#5628), the parents' statement on benefits, the debtor
// number of the family (#5621) and the children.
type HolidayRegistration struct {
	ID               int64
	Status           string
	SubmittedAt      time.Time
	BenefitsDeclared bool
	DebtorNumber     string
	Children         []HolidayChild
	// MovedUp names the other registrations whose bookings moved up from
	// the waiting list because of the change that answers with this
	// registration: a cancellation or a rebooking (#4699).
	MovedUp []int64
	// View is the registration as Enrollment shows it to the Träger-Büro;
	// the rebooking route answers with it unchanged (#4699).
	View any
}

// HolidayBookingChoice is one week and time model a rebooking gives a
// child (#4699).
type HolidayBookingChoice struct {
	WeekID      int64 `json:"week_id"`
	TimeModelID int64 `json:"time_model_id"`
}

// HolidayPlaceChange is what a rebooking or a new capacity changed in the
// places (#4699): the registrations whose bookings moved up from the waiting
// list and got their contribution computed again.
type HolidayPlaceChange struct {
	// Registration is the rebooked registration as Enrollment shows it.
	Registration   any     `json:"-"`
	RegistrationID int64   `json:"registration_id,omitempty"`
	WeekID         int64   `json:"week_id,omitempty"`
	Capacity       *int    `json:"capacity"`
	MovedUp        []int64 `json:"moved_up"`
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
// transaction of the Träger; HolidayRegistration,
// ConfirmHolidayRegistration and SetHolidayDebtorNumber record themselves in
// the registration's audit trail and refuse a period or registration of
// another Träger. Enrollment trims the debtor number and refuses one that
// is too long or has a line break as a validation error on debtor_number
// (#5621). RecordHolidaySiblings writes a merge or split of siblings into
// the registration's audit trail (#5628).
type HolidayRegistrations interface {
	ResolveHolidayLink(ctx context.Context, token string) (HolidayLink, error)
	HolidayWeeks(ctx context.Context, link HolidayLink) ([]HolidayWeek, error)
	HolidayRegistration(ctx context.Context, reviewer HolidayReviewer, periodID, registrationID int64) (HolidayRegistration, error)
	// ConfirmHolidayRegistration confirms the registration with the debtor
	// number; a blank one confirms without.
	ConfirmHolidayRegistration(ctx context.Context, reviewer HolidayReviewer, periodID, registrationID int64, debtorNumber string) (HolidayRegistration, error)
	// SetHolidayDebtorNumber changes the debtor number of a confirmed
	// registration; a blank one removes it.
	SetHolidayDebtorNumber(ctx context.Context, reviewer HolidayReviewer, periodID, registrationID int64, debtorNumber string) (HolidayRegistration, error)
	// CancelHolidayRegistration cancels the registration (#5662); a
	// registration after the deadline is ErrHolidayRegistrationFrozen.
	CancelHolidayRegistration(ctx context.Context, reviewer HolidayReviewer, periodID, registrationID int64) (HolidayRegistration, error)
	// RebookHolidayChild replaces the bookings of one child (#4699); a week
	// it leaves gives its place to the waiting list. MovedUp names the
	// registrations that moved up.
	RebookHolidayChild(ctx context.Context, reviewer HolidayReviewer, periodID, registrationID, childID int64, bookings []HolidayBookingChoice) (HolidayRegistration, error)
	// SetHolidayWeekCapacity sets the places of a week, nil for unlimited,
	// and moves bookings up into the places it frees; it returns the
	// registrations that moved up.
	SetHolidayWeekCapacity(ctx context.Context, reviewer HolidayReviewer, periodID, weekID int64, capacity *int) ([]int64, error)
	// HoldHolidayPeriod locks the period until the transaction ends, so no
	// registration of it changes while a sibling group is read and decided
	// on (#5628). A period of another Träger is ErrHolidayPeriodNotFound.
	HoldHolidayPeriod(ctx context.Context, reviewer HolidayReviewer, periodID int64) error
	// RecordHolidaySiblings records action, HolidaySiblingsMerged,
	// HolidaySiblingsSplit or HolidaySiblingsCancelled, with the other
	// registrations it names.
	RecordHolidaySiblings(ctx context.Context, reviewer HolidayReviewer, periodID, registrationID int64, action string, siblingIDs []int64) error
}

// HolidayBillingSettings is the Träger level of the settings system as the
// Elternabrechnung reads it (#5621, #5622). The root binds it; it reads in the
// caller's Träger transaction.
type HolidayBillingSettings interface {
	// AsksDebtorNumber reports whether the Träger enters the debtor number
	// of the family when it confirms a holiday registration: the Träger's
	// value of parent_billing.debtor_number_on_confirm or its default.
	AsksDebtorNumber(ctx context.Context, organizationID int64) (bool, error)
	// PaymentText is the Träger's own payment sentence of the confirmation
	// letter with its placeholders, empty for the built-in sentence: its
	// value of parent_billing.payment_text or the default (#5622).
	PaymentText(ctx context.Context, organizationID int64) (string, error)
	// HolidayChain is how the Träger ranks siblings and when a cost bearer
	// takes over: its values of parent_billing.sibling_rank and
	// parent_billing.cost_takeover_order or the defaults (#5664).
	HolidayChain(ctx context.Context, organizationID int64) (HolidayChainSettings, error)
}

// HolidayConfirmationSettings is what the Träger-Büro needs to know before
// it confirms a registration (#5621).
type HolidayConfirmationSettings struct {
	// DebtorNumberRequired reports that a confirmation needs the debtor
	// number of the family.
	DebtorNumberRequired bool `json:"debtor_number_required"`
	// CostBearers are the active cost bearers the Träger-Büro may choose
	// (#5627), by name; the Träger-Büro needs no right "Abrechnung" for
	// them.
	CostBearers []CostBearerChoice `json:"cost_bearers"`
}

// CostBearerChoice is a cost bearer as the confirmation offers it (#5627).
type CostBearerChoice struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// HolidayDebtorNumber is the debtor number of a registration after a
// change; empty means none.
type HolidayDebtorNumber struct {
	RegistrationID int64  `json:"id"`
	DebtorNumber   string `json:"debtor_number"`
}

// HolidayQuoteBooking is a booking of the public form.
type HolidayQuoteBooking struct {
	WeekID      int64
	TimeModelID int64
}

// HolidayQuoteChild is a child of the public form by its bookings and its
// place among the children by age: 1 for the oldest, the same number for the
// same date of birth, 0 when the form knows no date yet (#5664). The quote
// needs no date of birth to rank them.
type HolidayQuoteChild struct {
	Bookings []HolidayQuoteBooking
	AgeRank  int
}

// HolidayContributions computes, shows and freezes the holiday contribution
// and keeps the sibling groups it ranks across (#5628). ResolveHolidayLink
// runs in the administrative transaction, every other operation in the
// Träger transaction of the link's or the session's Träger, which the
// caller opens.
type HolidayContributions interface {
	HolidaySiblingGroups
	ResolveHolidayLink(ctx context.Context, token string) (HolidayLink, error)
	// QuoteHolidayContribution computes the contribution of the public form
	// before it is sent, with the current prices and rules. It stores
	// nothing.
	QuoteHolidayContribution(ctx context.Context, link HolidayLink, children []HolidayQuoteChild, benefitsDeclared bool) (HolidayContribution, error)
	// HolidayContribution shows the frozen contribution of a confirmed
	// registration, or else what a confirmation would freeze now: with the
	// exemption benefitsExemption names, or the parents' statement when it
	// is nil, or with the cost takeover by the active cost bearer
	// costBearerID names when it is not zero (#5627).
	HolidayContribution(ctx context.Context, reviewer HolidayReviewer, periodID, registrationID int64, benefitsExemption *bool, costBearerID int64) (HolidayContribution, error)
	// ConfirmHolidayRegistration confirms the registration and freezes its
	// contribution in the same transaction. benefitsExemption applies the
	// exemption rule; a costBearerID other than zero lets that active cost
	// bearer of the Träger pay instead (#5627): both together are
	// ErrCostTakeoverWithExemption, an unknown cost bearer
	// ErrCostBearerNotFound, a switched-off one ErrCostBearerInactive, each
	// before anything is written. debtorNumber is the number of the family; when the
	// Träger asks for it, a blank one is a validation error on
	// debtor_number before anything is written (#5621).
	ConfirmHolidayRegistration(ctx context.Context, reviewer HolidayReviewer, periodID, registrationID int64, benefitsExemption bool, costBearerID int64, debtorNumber string) (HolidayConfirmation, error)
	// CancelHolidayRegistration cancels the registration through Enrollment
	// in the caller's Träger transaction (#5662). In a sibling group it
	// takes the registration out of the group and computes the frozen
	// contribution of every other confirmed registration of the group again,
	// under the locks of a merge, also of one an issued invoice bills, which
	// keeps its own positions. A cancelled registration is
	// ErrHolidayRegistrationStateConflict, one after the deadline
	// ErrHolidayRegistrationFrozen.
	CancelHolidayRegistration(ctx context.Context, reviewer HolidayReviewer, periodID, registrationID int64) (HolidayCancellation, error)
	// RebookHolidayChild replaces the bookings of a child through Enrollment
	// (#4699). A week the child leaves gives its place to the waiting list;
	// every confirmed registration that moves up gets its contribution
	// computed again, with its sibling group, in the same transaction.
	RebookHolidayChild(ctx context.Context, reviewer HolidayReviewer, periodID, registrationID, childID int64, bookings []HolidayBookingChoice) (HolidayPlaceChange, error)
	// SetHolidayWeekCapacity sets the places of a week through Enrollment,
	// nil for unlimited, and computes the contribution of every confirmed
	// registration that moves up again (#4699).
	SetHolidayWeekCapacity(ctx context.Context, reviewer HolidayReviewer, periodID, weekID int64, capacity *int) (HolidayPlaceChange, error)
	// HolidayConfirmationSettings reports what a confirmation of the
	// session's Träger needs (#5621).
	HolidayConfirmationSettings(ctx context.Context, reviewer HolidayReviewer) (HolidayConfirmationSettings, error)
	// ChangeHolidayDebtorNumber changes the debtor number of a confirmed
	// registration until an issued invoice bills it; then it is
	// ErrDebtorNumberLocked (#5621). When the Träger asks for the number, a
	// blank one is a validation error on debtor_number.
	ChangeHolidayDebtorNumber(ctx context.Context, reviewer HolidayReviewer, periodID, registrationID int64, debtorNumber string) (HolidayDebtorNumber, error)
}
