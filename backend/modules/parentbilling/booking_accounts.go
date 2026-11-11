package parentbilling

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Where the Elternabrechnung books (#4821): the revenue account of each
// position and the cost centers KOST1 (school or holiday care) and KOST2
// (Angebot). The Träger keeps a revenue account per Leistung and per
// Angebot of the monthly contributions; without one the standard of the
// DATEV settings applies (parent_billing.datev_revenue_account). A cost
// center holds from its day until the next one of its scope.
//
// Invariants:
//  1. The release resolves the accounts of every position and copies them
//     onto it (PositionAccounts.Frozen). An issued position never changes
//     (trigger refuse_issued_change); a later change of an account or cost
//     center never reaches it, nor a file of it.
//  2. A cost center applies only from its valid_from until the next row of
//     its scope; the day of a position is its first service day.
//  3. Every booking a file writes has a valid revenue account; a file
//     without one is refused and names the documents.
//  4. The values saved here pass the same rules the DATEV file checks
//     (one case list, backend/testdata/finance-booking-accounts-cases.json).

// Scopes of a revenue account: a Leistung, or an Angebot of the monthly
// contributions (its care offering).
const (
	BookingScopeCare     = "care"
	BookingScopeMeals    = "meals"
	BookingScopeHoliday  = "holiday"
	BookingScopeOffering = "offering"
	// BookingScopeSchool is the scope of the KOST1 of a school.
	BookingScopeSchool = "school"
)

// Limits of the values (DATEV: a Sachkonto has at most 8 digits, a cost
// center at most 36 characters of [A-Za-z0-9_ ]).
const (
	MaxRevenueAccountLength = 8
	MaxCostCenterLength     = 36
)

var (
	// ErrBookingScopeNotFound reports a school or Angebot that is not the
	// Träger's, or an unknown scope.
	ErrBookingScopeNotFound = errors.New("no school or Angebot of the Träger")
	// ErrCostCenterNotFound reports a cost center of another Träger or none.
	ErrCostCenterNotFound = errors.New("no cost center of the Träger")
	// ErrCostCenterDateTaken reports a second cost center of a scope on the
	// same day.
	ErrCostCenterDateTaken = errors.New("the scope has a cost center from this day")
)

// PositionAccounts is where one position books. Frozen says the release
// copied the values; a position issued before #4821 has none, and the
// export resolves it when it runs.
type PositionAccounts struct {
	RevenueAccount string `json:"revenue_account"`
	CostCenter     string `json:"cost_center"`
	CostCenter2    string `json:"cost_center_2"`
	Frozen         bool   `json:"-"`
}

// RevenueAccount is the revenue account of a Leistung (ScopeID 0) or of an
// Angebot.
type RevenueAccount struct {
	Scope     string    `json:"scope"`
	ScopeID   int64     `json:"scope_id"`
	Account   string    `json:"account"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CostCenter is a cost center from ValidFrom on: KOST1 of a school or of
// the holiday care (scope holiday, ScopeID 0), KOST2 of an Angebot. An
// empty Code ends the cost center of the scope from that day on.
type CostCenter struct {
	ID        int64         `json:"id"`
	Scope     string        `json:"scope"`
	ScopeID   int64         `json:"scope_id"`
	ValidFrom calendar.Date `json:"valid_from"`
	Code      string        `json:"code"`
	CreatedAt time.Time     `json:"created_at"`
}

// BookingOffering is an Angebot of a school that has a price: the scope of
// a revenue account and of a KOST2, with the modules of its prices.
type BookingOffering struct {
	SchoolID       int64    `json:"school_id"`
	CareOfferingID int64    `json:"care_offering_id"`
	ModuleNames    []string `json:"module_names"`
}

// BookingAccounts is what the page of the accounts shows: the standard of
// the DATEV settings, the schools and Angebote of the Träger and every
// revenue account and cost center. CostCenterRequired is the DATEV setting
// that a booking needs a KOST1.
type BookingAccounts struct {
	StandardAccount    string            `json:"standard_account"`
	CostCenterRequired bool              `json:"cost_center_required"`
	Schools            []CareSchool      `json:"schools"`
	Offerings          []BookingOffering `json:"offerings"`
	RevenueAccounts    []RevenueAccount  `json:"revenue_accounts"`
	CostCenters        []CostCenter      `json:"cost_centers"`
}

// BookingAssignments are the revenue accounts and cost centers of a Träger
// as the release and the export resolve them.
type BookingAssignments struct {
	RevenueAccounts []RevenueAccount
	CostCenters     []CostCenter
}

// BookingAccountService lets a person with the Träger right
// "Finanz-Export" keep the revenue accounts and cost centers. Every
// operation runs in the Träger transaction of the reviewer's Träger, which
// the caller opens and rolls back when an operation fails.
type BookingAccountService interface {
	BookingAccounts(ctx context.Context, reviewer HolidayReviewer) (BookingAccounts, error)
	// SetRevenueAccount sets the account of a scope; an empty account
	// removes it, so the next scope up applies.
	SetRevenueAccount(ctx context.Context, reviewer HolidayReviewer, scope string, scopeID int64, account string) (BookingAccounts, error)
	AddCostCenter(ctx context.Context, reviewer HolidayReviewer, center CostCenter) (BookingAccounts, error)
	DeleteCostCenter(ctx context.Context, reviewer HolidayReviewer, id int64) (BookingAccounts, error)
}
