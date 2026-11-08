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
//     changes; a price an issued invoice billed never changes for its
//     month, while a booked weekday without a price may still get one.

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
	// MaxModuleNameLength is the longest name of a module a price copies.
	MaxModuleNameLength = 200
)

var (
	// ErrCareRunNotFound reports a monthly run that does not exist for the
	// Träger of the session.
	ErrCareRunNotFound = errors.New("monthly billing run not found")
	// ErrCareRunOpen reports a second calculated run of a month for the same
	// schools.
	ErrCareRunOpen = errors.New("the month already has a billing run that is not released")
	// ErrCareVersionsOverlap reports two versions of one contract on one day;
	// the run bills nothing of such a month.
	ErrCareVersionsOverlap = errors.New("a contract has two versions in force on one day")
	// ErrCareTariffMonthBilled reports a change of a price an issued invoice
	// already billed.
	ErrCareTariffMonthBilled = errors.New("an issued invoice already billed this price")
	// ErrCareTariffNotFound reports a price list that does not exist.
	ErrCareTariffNotFound = errors.New("care tariff not found")
)

// CareModule is a module of a school as the price list shows it: a care
// offering the contracts of the Träger book in the year before or after the
// current month, with the weekdays they book it on and how many contracts.
type CareModule struct {
	SchoolID       int64    `json:"school_id"`
	SchoolName     string   `json:"school_name"`
	CareOfferingID int64    `json:"care_offering_id"`
	ModuleName     string   `json:"module_name"`
	EndsAt         string   `json:"ends_at"`
	BookedWeekdays []string `json:"booked_weekdays"`
	// BookedDaysPerWeek lists actual booking sizes, never the weekday union.
	BookedDaysPerWeek []int `json:"booked_days_per_week"`
	ContractCount     int   `json:"contract_count"`
}

// Price forms of a module (#6196). A module of a price list has one form:
// a price per booked weekday, the sum of the booked weekdays, or a scale by
// the number of booked days per week.
const (
	PriceFormWeekday     = "weekday"
	PriceFormDaysPerWeek = "days_per_week"
)

// MaxDaysPerWeek is the highest step of a scale.
const MaxDaysPerWeek = 7

// CareTariff is the monthly price of one weekday of a module from a month
// on, or with DaysPerWeek set the monthly price of a module booked on that
// many days per week (#6196). SiblingCents is the price for every further
// child, which a sibling rule in the price-list mode takes; nil keeps the
// price of the first child. MealExtra marks a module whose price excludes
// the meal. ValidUntil is the last day before the next price of the same
// module and key, nil while none follows.
type CareTariff struct {
	ID             int64          `json:"id"`
	SchoolID       int64          `json:"school_id"`
	CareOfferingID int64          `json:"care_offering_id"`
	ModuleName     string         `json:"module_name"`
	Weekday        string         `json:"weekday"`
	DaysPerWeek    *int           `json:"days_per_week"`
	ValidFrom      calendar.Date  `json:"valid_from"`
	ValidUntil     *calendar.Date `json:"valid_until"`
	MonthlyCents   int64          `json:"monthly_cents"`
	SiblingCents   *int64         `json:"sibling_cents"`
	MealExtra      bool           `json:"meal_extra"`
	CreatedAt      time.Time      `json:"created_at"`
}

// PriceForm is the form of the price: per weekday or a scale step.
func (t CareTariff) PriceForm() string {
	if t.DaysPerWeek != nil {
		return PriceFormDaysPerWeek
	}
	return PriceFormWeekday
}

// CarePriceListHead is the head of the price list of a school from a month
// on: the months of the year the monthly run bills nothing for, e.g.
// August (#6196). It holds until the next price list of the school.
type CarePriceListHead struct {
	SchoolID   int64         `json:"school_id"`
	ValidFrom  calendar.Date `json:"valid_from"`
	FreeMonths []int         `json:"free_months"`
}

// CareSchool is a school of the Träger by its name.
type CareSchool struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// CareTariffs is the price list of a Träger: its schools, the modules its
// contracts book and every price. BilledUntil names per school the last
// month an issued invoice bills; a new price starts after it.
type CareTariffs struct {
	Schools     []CareSchool            `json:"schools"`
	Modules     []CareModule            `json:"modules"`
	Tariffs     []CareTariff            `json:"tariffs"`
	Lists       []CarePriceListHead     `json:"lists"`
	BilledUntil map[int64]calendar.Date `json:"billed_until"`
}

// CareBilledPrice is a weekday of a module an issued invoice of a school
// billed in a month, or with DaysPerWeek the scale step it billed; its
// price there never changes.
type CareBilledPrice struct {
	CareOfferingID int64
	Weekday        string
	DaysPerWeek    *int
	Month          calendar.Date
}

// CarePrice is one price of a price list a school gets from a month on:
// for a weekday or, with DaysPerWeek, for a scale step (#6196).
type CarePrice struct {
	CareOfferingID int64
	ModuleName     string
	Weekday        string
	DaysPerWeek    *int
	MonthlyCents   int64
	SiblingCents   *int64
	MealExtra      bool
}

// CarePriceList sets the prices of a school from ValidFrom (the first day
// of a month) on. It replaces the prices of the school that start on the
// same day; a weekday or scale step without a price keeps the price
// before. FreeMonths are the months of the year it bills nothing for.
type CarePriceList struct {
	SchoolID   int64
	ValidFrom  calendar.Date
	FreeMonths []int
	Prices     []CarePrice
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
	// FreeOfCharge is the free-month count saved by the release.
	FreeOfCharge int `json:"free_of_charge"`
	// CareRunScope names the schools and the invoice date (#5916).
	CareRunScope
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
	// DaysPerWeek and TierMonthlyCents are the scale step a module priced
	// by days per week billed (#6196); nil for a price per weekday.
	DaysPerWeek      *int   `json:"days_per_week,omitempty"`
	TierMonthlyCents *int64 `json:"tier_monthly_cents,omitempty"`
	// MealExtra says the price excludes the meal, which the Mensa bills.
	MealExtra   bool  `json:"meal_extra,omitempty"`
	AmountCents int64 `json:"amount_cents"`
	// PayableCents preserves the rule chain allocation for later runs of the month.
	PayableCents int64 `json:"payable_cents"`
	// Key names the position for a change of the review (#6197).
	Key string `json:"key,omitempty"`
	// SiblingCents is what the position costs at the sibling prices of the
	// price list; a sibling rule in the price-list mode charges it (#6196).
	SiblingCents int64 `json:"-"`
}

// CareRecipient is who an invoice goes to: the contract party, which the
// registration named, unless the family deviates (#4689). Kind is
// RecipientApplicant, RecipientBoth or RecipientPayer; SecondName the
// second parent; Address the payer's address. An invoice issued before
// #4689 has no kind.
type CareRecipient struct {
	FirstName  string         `json:"first_name"`
	LastName   string         `json:"last_name"`
	Email      string         `json:"email"`
	Kind       string         `json:"kind"`
	SecondName string         `json:"second_name"`
	Address    *PostalAddress `json:"address"`
}

// CareInvoice is the invoice of one contract and month: a draft without a
// number in a calculated run, the issued invoice in a released one.
type CareInvoice struct {
	ID         int64          `json:"id"`
	Number     string         `json:"number"`
	IssuedOn   *calendar.Date `json:"issued_on"`
	ContractID int64          `json:"contract_id"`
	// Service is what the invoice bills (#4686, ADR 0167): ServiceCare with
	// every service bundled with it, or ServiceMeal on an invoice of its own
	// when the Träger bills the Mensa apart (TermsSet.InvoiceService).
	Service        string `json:"service"`
	SchoolID       int64  `json:"school_id"`
	SchoolName     string `json:"school_name"`
	ChildFirstName string `json:"child_first_name"`
	ChildLastName  string `json:"child_last_name"`
	// StudentID is the child record the contract names, nil without one; the
	// parents inbox shows the letter only for it (ADR 0165).
	StudentID *int64 `json:"-"`
	// Exclusion is set on an invoice of a preview the release leaves out
	// (#5916).
	Exclusion *CareExclusion `json:"exclusion,omitempty"`
	Recipient CareRecipient  `json:"recipient"`
	// DocumentKind is the kind of document the release froze (#4688); a
	// draft carries the kind its release would freeze now, so the
	// fingerprint of the review covers it.
	DocumentKind string `json:"document_kind,omitempty"`
	// FamilyEmail is the family of the invoice: the mail address of the
	// contract party, whoever pays (#4689). The sibling rules count by it.
	FamilyEmail string `json:"family_email"`
	// Payment is how and by when the invoice is paid (#4689); an invoice
	// issued before has none.
	Payment   *InvoicePayment `json:"payment"`
	Positions []CarePosition  `json:"positions"`
	// Reductions are the rules of the chain in their order (#4685); the
	// total is the positions less the reductions.
	Reductions []CareReduction `json:"reductions"`
	// MealPositions are the Mensa months of the child the invoice bills
	// (#4686). No rule of the chain reduces them; TotalCents adds them in
	// full.
	MealPositions []CareMealPosition `json:"meal_positions"`
	// CostShares are the parts cost bearers pay of what the family would
	// pay after the reductions (#4687, ADR 0169); TotalCents is the rest the
	// family pays.
	CostShares []CareInvoiceCostShare `json:"cost_shares"`
	// Amendments are the changes of the invoice in the review (#6197): on a
	// preview its history with revoked and conflicting entries, on an issued
	// invoice the corrections its release stored. TotalCents adds the
	// correction lines of the applied ones.
	Amendments []CareAmendment `json:"amendments,omitempty"`
	TotalCents int64           `json:"total_cents"`
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
	// DaysPerWeek names the missing scale step of a module priced by days
	// per week (#6196); Weekday is empty then.
	DaysPerWeek *int `json:"days_per_week,omitempty"`
}

// CareDuplicateContract is one of the contracts of a duplicate.
type CareDuplicateContract struct {
	ContractID int64  `json:"contract_id"`
	SchoolID   int64  `json:"school_id"`
	SchoolName string `json:"school_name"`
}

// CareDuplicate is a child the run holds back (ADR 0158, Nachtrag
// Kind-Monat): in a preview one entry per module and weekday two of its
// contracts bill on the same day, naming every contract of the child; in a
// released run one entry per child of the stored held contracts, without a
// module. The run bills none of the child's contracts this month.
type CareDuplicate struct {
	ChildFirstName string                  `json:"child_first_name"`
	ChildLastName  string                  `json:"child_last_name"`
	ModuleName     string                  `json:"module_name"`
	Weekday        string                  `json:"weekday"`
	Contracts      []CareDuplicateContract `json:"contracts"`
}

// HoldReasonDuplicateContract is why a release held a child back: two of
// its contracts bill the same module on the same day (ADR 0158 E3).
const HoldReasonDuplicateContract = "duplicate_contract"

// CareHeldSlot is a module and weekday two contracts of a held child bill
// on the same day.
type CareHeldSlot struct {
	ModuleName string `json:"module_name"`
	Weekday    string `json:"weekday"`
}

// CareRunHold is a child a release held back, as it was at the release
// (ADR 0158 E5): its name, every contract with its school, the reason and
// the modules and weekdays billed twice. It never changes.
type CareRunHold struct {
	Position       int
	Reason         string
	ChildFirstName string
	ChildLastName  string
	Contracts      []CareDuplicateContract
	Slots          []CareHeldSlot
}

// CareRunDetail is a run with its invoices. A calculated run shows the
// preview of today with the rule of the Träger now; Unpriced lists the
// bookings without a price, Duplicates the children with two contracts for
// one module and weekday, Billed counts the contracts an earlier run of the
// month already billed.
type CareRunDetail struct {
	CareRun
	Invoices   []CareInvoice   `json:"invoices"`
	Unpriced   []CareUnpriced  `json:"unpriced"`
	Duplicates []CareDuplicate `json:"duplicates"`
	// DuplicateChildren counts the children of Duplicates: the run, and its
	// release, bills none of them.
	DuplicateChildren int `json:"duplicate_children"`
	// UncountedFamilies counts the families a sibling rule that counts the
	// family ranked without a confirmed number for the month (#5918): the
	// rule counts their children with a contract alone.
	UncountedFamilies int    `json:"uncounted_families"`
	Billed            int    `json:"billed"`
	PreviewCount      int    `json:"preview_count"`
	PreviewCents      int64  `json:"preview_cents"`
	NumberPattern     string `json:"number_pattern"`
	NumberRangeSet    bool   `json:"number_range_set"`
	// MealGaps and MealUnbilled are the Mensa of a calculated run (#4686):
	// the ended months a school has not released and the released months
	// of children the run does not bill.
	MealGaps     []CareMealGap      `json:"meal_gaps"`
	MealUnbilled []CareMealUnbilled `json:"meal_unbilled"`
	// MealChanged are Mensa months issued invoices bill with an amount a
	// later release of the school changed (#4686).
	MealChanged []CareMealChanged `json:"meal_changed"`
	// DueDates are the due dates of the run per payment method (#4689).
	DueDates []RunDueDate `json:"due_dates"`
	// CostBearers are what each cost bearer pays of the invoices (#4687).
	CostBearers []CareCostBearerTotal `json:"cost_bearers"`
	// CareRunReviewState is the review of a calculated run (#5916).
	CareRunReviewState
	// DocumentKindState is the kind the run issues (#4688).
	DocumentKindState
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
	// StudentID names the child's record when the contract has one;
	// ChildBirthDate tells two children of one name apart without it.
	StudentID      *int64
	ChildBirthDate *calendar.Date
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

// CareSchools lists the schools of the Träger that are not deleted, by
// name. The root binds it to Organisation & Tenancy.
type CareSchools interface {
	TraegerSchools(ctx context.Context, organizationID int64) ([]CareSchool, error)
}

// CareBillingSettings is the Träger's choice for a month in which a
// version changes: its value of parent_billing.care_month_change or the
// default.
type CareBillingSettings interface {
	CareMonthRule(ctx context.Context, organizationID int64) (string, error)
	// CareAmendmentSettings says which changes of an invoice the review
	// allows (#6197).
	CareAmendmentSettings
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
	StartCareRun(ctx context.Context, reviewer HolidayReviewer, start CareRunStart) (CareRun, error)
	CareRun(ctx context.Context, reviewer HolidayReviewer, runID int64) (CareRunDetail, error)
	ReleaseCareRun(ctx context.Context, reviewer HolidayReviewer, runID int64, release CareRelease) (CareRunDetail, error)
	CareRunReview
}
