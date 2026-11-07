package parentbilling

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The monthly contribution of the care contract (#4684, spec #4611 user
// stories 59 and 63, ADR 0119 addendum 2026-11-07). The Abrechnung keeps a
// monthly price for every booked weekday of a module at a school (price
// form E3 A of the SJR alignment): the contribution of a child is the sum
// of the prices of the weekdays its contract books. A monthly run bills a
// month by the contract version in force, never by the attendance, and may
// run within the month it bills.
//
// Invariants:
//   - One version per contract and day: the run refuses a month for which
//     the contract owner answers two versions of one contract on one day.
//   - One invoice per contract and month (uq_care_invoices_contract_month),
//     also with two releases at once: the release holds the run and the
//     Träger's monthly billing until it commits.
//   - An issued invoice keeps its own copy of every price and never
//     changes; a price never starts in a month an invoice of its school
//     already bills.

// ServiceCare is the service of the monthly care contribution: its own
// number range next to the holiday care.
const ServiceCare = "care"

// How a run bills a month in which the contract version changes, starts or
// ends (setting parent_billing.care_month_change). The customer has not
// decided yet (#4684 "Offen beim Kunden"); the default bills the full month.
const (
	// MonthRuleVersionOnFirst bills the full month by the version in force
	// on its first day, or by the first version of the month when the
	// contract starts within it.
	MonthRuleVersionOnFirst = "version_on_first"
	// MonthRuleProRata bills every version for its calendar days of the
	// month.
	MonthRuleProRata = "pro_rata"
)

// Limits of the monthly billing.
const (
	// MaxMonthlyPriceCents caps a monthly price at one million euro.
	MaxMonthlyPriceCents = 100_000_000
)

var (
	// ErrCareRunNotFound reports a monthly run that does not exist for the
	// Träger of the session.
	ErrCareRunNotFound = errors.New("monthly billing run not found")
	// ErrCareRunOpen reports a second calculated run of a month.
	ErrCareRunOpen = errors.New("the month already has a billing run that is not released")
	// ErrCareVersionsOverlap reports two versions of one contract on one day;
	// the run bills nothing of such a month.
	ErrCareVersionsOverlap = errors.New("a contract has two versions in force on one day")
	// ErrCareTariffMonthBilled reports a price that would start in a month an
	// issued invoice of the school already bills.
	ErrCareTariffMonthBilled = errors.New("an invoice of the school already bills this month")
	// ErrCareTariffNotFound reports a price list that does not exist.
	ErrCareTariffNotFound = errors.New("care tariff not found")
)

// CareModule is a module of a school as the price list shows it: a care
// offering the contracts of the Träger book, with the weekdays they book
// it on and how many contracts book it now.
type CareModule struct {
	SchoolID       int64    `json:"school_id"`
	SchoolName     string   `json:"school_name"`
	CareOfferingID int64    `json:"care_offering_id"`
	ModuleName     string   `json:"module_name"`
	EndsAt         string   `json:"ends_at"`
	BookedWeekdays []string `json:"booked_weekdays"`
	ContractCount  int      `json:"contract_count"`
}

// CareTariff is the monthly price of one weekday of a module from a month
// on. ValidUntil is the last day before the next price of the same module
// and weekday, nil while none follows.
type CareTariff struct {
	ID             int64          `json:"id"`
	SchoolID       int64          `json:"school_id"`
	CareOfferingID int64          `json:"care_offering_id"`
	ModuleName     string         `json:"module_name"`
	Weekday        string         `json:"weekday"`
	ValidFrom      calendar.Date  `json:"valid_from"`
	ValidUntil     *calendar.Date `json:"valid_until"`
	MonthlyCents   int64          `json:"monthly_cents"`
	CreatedAt      time.Time      `json:"created_at"`
}

// CareTariffs is the price list of a Träger: the modules its contracts
// book and every price. BilledUntil names per school the last month an
// issued invoice bills; a new price starts after it.
type CareTariffs struct {
	Modules     []CareModule            `json:"modules"`
	Tariffs     []CareTariff            `json:"tariffs"`
	BilledUntil map[int64]calendar.Date `json:"billed_until"`
}

// CarePrice is one price of a price list a school gets from a month on.
type CarePrice struct {
	CareOfferingID int64
	ModuleName     string
	Weekday        string
	MonthlyCents   int64
}

// CarePriceList sets the prices of a school from ValidFrom (the first day
// of a month) on. It replaces the prices of the school that start on the
// same day; a weekday without a price keeps the price before.
type CarePriceList struct {
	SchoolID  int64
	ValidFrom calendar.Date
	Prices    []CarePrice
}

// CareRun is a monthly run as the list shows it. DocumentCount, TotalCents
// and MonthRule are set once it is released.
type CareRun struct {
	ID            int64          `json:"id"`
	BillingMonth  calendar.Date  `json:"billing_month"`
	Status        string         `json:"status"`
	CreatedAt     time.Time      `json:"created_at"`
	ReleasedAt    *time.Time     `json:"released_at"`
	IssuedOn      *calendar.Date `json:"issued_on"`
	DocumentCount *int           `json:"document_count"`
	TotalCents    *int64         `json:"total_cents"`
	MonthRule     string         `json:"month_rule"`
}

// CareRunList is the list of the monthly runs, the latest month first,
// with the month a new run would bill: the earliest month from the current
// one on without a released run.
type CareRunList struct {
	Runs           []CareRun     `json:"runs"`
	SuggestedMonth calendar.Date `json:"suggested_month"`
}

// CareWeekdayPrice is a booked weekday of a position with its monthly
// price and the part of it the position bills.
type CareWeekdayPrice struct {
	Weekday      string `json:"weekday"`
	MonthlyCents int64  `json:"monthly_cents"`
	AmountCents  int64  `json:"amount_cents"`
}

// CarePosition is one booked module of an invoice: the weekdays the
// version books it on, the days of the month it bills and the sum.
type CarePosition struct {
	CareOfferingID int64              `json:"care_offering_id"`
	ModuleName     string             `json:"module_name"`
	VersionNumber  int                `json:"version_number"`
	ServiceFrom    calendar.Date      `json:"service_from"`
	ServiceUntil   calendar.Date      `json:"service_until"`
	Weekdays       []CareWeekdayPrice `json:"weekdays"`
	AmountCents    int64              `json:"amount_cents"`
}

// CareRecipient is the contract party an invoice goes to.
type CareRecipient struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
}

// CareInvoice is the invoice of one contract and month: a draft without a
// number in a calculated run, the issued invoice in a released one.
type CareInvoice struct {
	ID             int64          `json:"id"`
	Number         string         `json:"number"`
	IssuedOn       *calendar.Date `json:"issued_on"`
	ContractID     int64          `json:"contract_id"`
	SchoolID       int64          `json:"school_id"`
	SchoolName     string         `json:"school_name"`
	ChildFirstName string         `json:"child_first_name"`
	ChildLastName  string         `json:"child_last_name"`
	Recipient      CareRecipient  `json:"recipient"`
	Positions      []CarePosition `json:"positions"`
	TotalCents     int64          `json:"total_cents"`
}

// CareUnpriced is a booked weekday of a module the price list has no price
// for in the month. The run leaves the contract out until it has one.
type CareUnpriced struct {
	ContractID     int64  `json:"contract_id"`
	SchoolID       int64  `json:"school_id"`
	SchoolName     string `json:"school_name"`
	ChildFirstName string `json:"child_first_name"`
	ChildLastName  string `json:"child_last_name"`
	CareOfferingID int64  `json:"care_offering_id"`
	ModuleName     string `json:"module_name"`
	Weekday        string `json:"weekday"`
}

// CareRunDetail is a run with its invoices. A calculated run shows the
// preview of today with the rule of the Träger now; Unpriced lists the
// bookings without a price, Billed counts the contracts an earlier run of
// the month already billed.
type CareRunDetail struct {
	CareRun
	Invoices       []CareInvoice  `json:"invoices"`
	Unpriced       []CareUnpriced `json:"unpriced"`
	Billed         int            `json:"billed"`
	PreviewCount   int            `json:"preview_count"`
	PreviewCents   int64          `json:"preview_cents"`
	NumberPattern  string         `json:"number_pattern"`
	NumberRangeSet bool           `json:"number_range_set"`
}

// CareContractService is a booked module of a contract version on a
// weekday. LastDay is the last booked day of a module that ends before the
// version does.
type CareContractService struct {
	Weekday        string
	CareOfferingID int64
	ModuleName     string
	EndsAt         string
	LastDay        *calendar.Date
}

// CareContractVersion is a version of a contract in force between From and
// Until within the asked span.
type CareContractVersion struct {
	ContractID     int64
	SchoolID       int64
	SchoolName     string
	Number         int
	ChildFirstName string
	ChildLastName  string
	From           calendar.Date
	Until          calendar.Date
	Services       []CareContractService
}

// CareContracts is the Betreuungsvertrag as the monthly run reads it. The
// root binds it. Both run in the caller's Träger transaction.
type CareContracts interface {
	// VersionsInForce answers each version of an active or ended contract
	// of the Träger in force between from and to, clipped to that span.
	VersionsInForce(ctx context.Context, organizationID int64, from, to calendar.Date) ([]CareContractVersion, error)
	// Recipients names per contract the party its invoice goes to: the
	// first party with an e-mail address, else the first party.
	Recipients(ctx context.Context, organizationID int64, contractIDs []int64) (map[int64]CareRecipient, error)
}

// CareBillingSettings is the Träger's choice for a month in which a
// version changes: its value of parent_billing.care_month_change or the
// default.
type CareBillingSettings interface {
	CareMonthRule(ctx context.Context, organizationID int64) (string, error)
}

// CareBilling lets the Abrechnung of a Träger keep the monthly prices of
// its modules and start, check and release the monthly runs. Every
// operation names the person of the session and runs in the Träger
// transaction of its Träger, which the caller opens. Every read of
// personal data and every change of a run is recorded in the run's trail.
type CareBilling interface {
	CareTariffs(ctx context.Context, reviewer HolidayReviewer) (CareTariffs, error)
	SaveCarePriceList(ctx context.Context, reviewer HolidayReviewer, list CarePriceList) (CareTariffs, error)
	DeleteCarePriceList(ctx context.Context, reviewer HolidayReviewer, schoolID int64, validFrom calendar.Date) (CareTariffs, error)
	ListCareRuns(ctx context.Context, organizationID int64) (CareRunList, error)
	StartCareRun(ctx context.Context, reviewer HolidayReviewer, month calendar.Date) (CareRun, error)
	CareRun(ctx context.Context, reviewer HolidayReviewer, runID int64) (CareRunDetail, error)
	ReleaseCareRun(ctx context.Context, reviewer HolidayReviewer, runID int64, release BillingRelease) (CareRunDetail, error)
}
