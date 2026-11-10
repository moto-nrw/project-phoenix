package parentbilling

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Who gets an invoice, how it is paid and by when (#4689, spec #4611 user
// stories 74, 76, 77, ADR 0167).
//
// A Träger sets per service a payment method, a payment term in days and
// whether its positions travel on the invoice of another service (the
// bundling). A run sets per payment method the date its invoices are due.
// A family may deviate: another recipient (both parents or another payer),
// its own payment method or the day of the month its invoices are due.
//
// One function decides the payment of an invoice, ResolvePayment, and one
// its recipient, ResolveRecipient. The preview, the release, the letter and
// the export use them, so they cannot disagree. The release copies both onto
// the invoice; a later change reaches the next release, never an issued
// invoice (ADR 0119 point 7).
//
// The family is the mail address of the registration: the party of the care
// contract or the guardian of a holiday registration, lower case. A payer
// who gets the invoice does not change the family, so the sibling rules
// (rankFamilies) count the same children whoever pays.

// ServiceMeal is the lunch of the care (#4686): its positions travel on the
// invoice of the care unless the Träger bills them on their own.
const ServiceMeal = "meal"

// Payment methods.
const (
	// PaymentTransfer is paid by the family by bank transfer.
	PaymentTransfer = "transfer"
	// PaymentDirectDebit is collected by the Träger from the family's
	// account with a SEPA mandate; moto collects no money.
	PaymentDirectDebit = "direct_debit"
)

// PaymentMethods are the payment methods in the order the forms show them.
var PaymentMethods = []string{PaymentDirectDebit, PaymentTransfer}

// Recipient kinds of a family.
const (
	// RecipientApplicant is the person of the registration; the default.
	RecipientApplicant = "applicant"
	// RecipientBoth names the person of the registration and a second
	// parent.
	RecipientBoth = "both"
	// RecipientPayer is another payer with an address of its own.
	RecipientPayer = "payer"
)

// Where the due date of an invoice comes from.
const (
	// DueFromFamily is the due day of the family's deviation.
	DueFromFamily = "family"
	// DueFromRun is the date of the run for the payment method.
	DueFromRun = "run"
	// DueFromTerm is the issue date plus the payment term of the service.
	DueFromTerm = "term"
)

// Limits of the terms.
const (
	// MaxDueDays is the longest payment term.
	MaxDueDays = 120
	// DefaultDueDays is the payment term without a choice of the Träger.
	DefaultDueDays = 14
	// MaxDueDay is the latest day of a month a family's invoices may be due
	// on, so the day exists in every month.
	MaxDueDay = 28
	// MaxBillingNameLength is the longest name of a second parent or payer.
	MaxBillingNameLength = 100
)

var (
	// ErrServiceUnknown reports a service the terms do not know.
	ErrServiceUnknown = errors.New("unknown billing service")
	// ErrFamilyBillingNotFound reports a deviation that does not exist for
	// the Träger of the session.
	ErrFamilyBillingNotFound = errors.New("family billing not found")
	// ErrFamilyUnknown reports a contract of the Träger whose party has no
	// mail address, or no such contract: there is no family to deviate.
	ErrFamilyUnknown = errors.New("the contract names no family with a mail address")
)

// BillingService is a service the terms know, with the services its
// positions may travel with. The holiday care has its own run and number
// range and travels with none.
type BillingService struct {
	Service    string   `json:"service"`
	BundleWith []string `json:"bundle_with"`
}

// BillingServices are the services in the order the forms show them. A new
// service is a new entry, not a new branch in the code.
var BillingServices = []BillingService{
	{Service: ServiceCare, BundleWith: []string{}},
	{Service: ServiceMeal, BundleWith: []string{ServiceCare}},
	{Service: ServiceHoliday, BundleWith: []string{}},
}

// ServiceTerms are the payment method, the payment term and the bundling
// of one service. BundleWith names the service whose invoice carries this
// service's positions; empty is an invoice of its own. Custom is false for
// the defaults of the code.
type ServiceTerms struct {
	Service       string     `json:"service"`
	PaymentMethod string     `json:"payment_method"`
	DueDays       int        `json:"due_days"`
	BundleWith    string     `json:"bundle_with"`
	Custom        bool       `json:"custom"`
	UpdatedAt     *time.Time `json:"updated_at"`
}

// DefaultServiceTerms are the terms of a service the Träger did not set:
// transfer within DefaultDueDays, the lunch on the invoice of the care.
func DefaultServiceTerms(service string) ServiceTerms {
	terms := ServiceTerms{Service: service, PaymentMethod: PaymentTransfer, DueDays: DefaultDueDays}
	if service == ServiceMeal {
		terms.BundleWith = ServiceCare
	}
	return terms
}

// TermsSet are the terms of every service of a Träger.
type TermsSet map[string]ServiceTerms

// Of answers the terms of a service, the defaults without a choice.
func (s TermsSet) Of(service string) ServiceTerms {
	if terms, ok := s[service]; ok {
		return terms
	}
	return DefaultServiceTerms(service)
}

// InvoiceService is the service whose invoice carries the positions of
// service (ADR 0167 point 1): the service it is bundled with, when
// BillingServices lets it travel with that one, else the service itself on
// an invoice of its own. The Mensa (#4686) asks it in every run.
func (s TermsSet) InvoiceService(service string) string {
	bundle := s.Of(service).BundleWith
	for _, known := range BillingServices {
		if known.Service == service && slices.Contains(known.BundleWith, bundle) {
			return bundle
		}
	}
	return service
}

// FamilyBilling is the deviation of one family. FamilyEmail is the key, the
// mail address of the registration. PaymentMethod empty and DueDay nil keep
// the terms of the service and the date of the run.
type FamilyBilling struct {
	ID              int64          `json:"id"`
	FamilyEmail     string         `json:"family_email"`
	ContractID      int64          `json:"contract_id"`
	RecipientKind   string         `json:"recipient_kind"`
	SecondFirstName string         `json:"second_first_name"`
	SecondLastName  string         `json:"second_last_name"`
	PayerFirstName  string         `json:"payer_first_name"`
	PayerLastName   string         `json:"payer_last_name"`
	PayerEmail      string         `json:"payer_email"`
	PayerAddress    *PostalAddress `json:"payer_address"`
	PaymentMethod   string         `json:"payment_method"`
	DueDay          *int           `json:"due_day"`
	// DebtorNumber is the number of the family in the finance system of
	// the Träger (#4693); the release copies it onto the monthly invoice.
	DebtorNumber string    `json:"debtor_number"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// FamilyBillingInput is a deviation as the Abrechnung writes it: for the
// family of a contract of the Träger.
type FamilyBillingInput struct {
	ContractID      int64
	RecipientKind   string
	SecondFirstName string
	SecondLastName  string
	PayerFirstName  string
	PayerLastName   string
	PayerEmail      string
	PayerAddress    *PostalAddress
	PaymentMethod   string
	DueDay          *int
	DebtorNumber    string
}

// RunDueDates are the due dates of a run per payment method.
type RunDueDates map[string]calendar.Date

// RunDueDate is one due date of a run as the wire shows it.
type RunDueDate struct {
	PaymentMethod string        `json:"payment_method"`
	DueOn         calendar.Date `json:"due_on"`
}

// InPaymentOrder answers the dates in the order of PaymentMethods.
func (d RunDueDates) InPaymentOrder() []RunDueDate {
	list := []RunDueDate{}
	for _, method := range PaymentMethods {
		if due, ok := d[method]; ok {
			list = append(list, RunDueDate{PaymentMethod: method, DueOn: due})
		}
	}
	return list
}

// InvoicePayment is how an invoice is paid and by when, with where the
// date came from. A direct debit carries the mandate it is collected with
// (ADR 0171); DebitGap names why a family that pays by direct debit is not
// collected (ResolveDebit).
type InvoicePayment struct {
	Method    string          `json:"method"`
	DueOn     calendar.Date   `json:"due_on"`
	DueSource string          `json:"due_source"`
	Mandate   *InvoiceMandate `json:"mandate,omitempty"`
	DebitGap  string          `json:"debit_gap,omitempty"`
}

// ResolvePayment is the one rule for the payment of an invoice: the
// payment method of the family's deviation, else of the service's terms;
// the due date of the family's deviation (the next DueDay on or after the
// issue date), else the run's date for that payment method, else the issue
// date plus the payment term. family may be nil, run empty.
func ResolvePayment(terms ServiceTerms, run RunDueDates, family *FamilyBilling, issuedOn calendar.Date) InvoicePayment {
	method := terms.PaymentMethod
	if method == "" {
		method = PaymentTransfer
	}
	if family != nil && family.PaymentMethod != "" {
		method = family.PaymentMethod
	}
	if family != nil && family.DueDay != nil {
		return InvoicePayment{Method: method, DueOn: nextDay(issuedOn, *family.DueDay), DueSource: DueFromFamily}
	}
	if due, ok := run[method]; ok && !due.IsZero() {
		return InvoicePayment{Method: method, DueOn: due, DueSource: DueFromRun}
	}
	return InvoicePayment{Method: method, DueOn: issuedOn.AddDays(terms.DueDays), DueSource: DueFromTerm}
}

// nextDay is the first date on or after from whose day of the month is day.
func nextDay(from calendar.Date, day int) calendar.Date {
	candidate := calendar.NewDate(from.Year(), from.Month(), day)
	if candidate.Before(from) {
		next := calendar.NewDate(from.Year(), from.Month(), 1).AddDays(32)
		candidate = calendar.NewDate(next.Year(), next.Month(), day)
	}
	return candidate
}

// BillingRecipient is who an invoice goes to: the person of the
// registration, both parents or another payer, with the address an invoice
// copies when the payer has one. SecondName names the second parent.
type BillingRecipient struct {
	Kind       string         `json:"kind"`
	FirstName  string         `json:"first_name"`
	LastName   string         `json:"last_name"`
	Email      string         `json:"email"`
	SecondName string         `json:"second_name"`
	Address    *PostalAddress `json:"address"`
}

// ResolveRecipient is the one rule for the recipient of an invoice: the
// person of the registration (applicant) unless the family deviates.
// applicant keeps its address for both parents; another payer brings its
// own. The family key is never the payer's address.
func ResolveRecipient(applicant BillingRecipient, family *FamilyBilling) BillingRecipient {
	applicant.Kind = RecipientApplicant
	if family == nil {
		return applicant
	}
	switch family.RecipientKind {
	case RecipientBoth:
		applicant.Kind = RecipientBoth
		applicant.SecondName = strings.TrimSpace(family.SecondFirstName + " " + family.SecondLastName)
	case RecipientPayer:
		return BillingRecipient{Kind: RecipientPayer, FirstName: family.PayerFirstName, LastName: family.PayerLastName,
			Email: family.PayerEmail, Address: family.PayerAddress}
	}
	return applicant
}

// FamilyKey is the key of a family: the mail address of the registration,
// trimmed and lower case.
func FamilyKey(email string) string { return strings.ToLower(strings.TrimSpace(email)) }

// BillingTerms lets the Abrechnung of a Träger keep the terms of its
// services, the deviations of its families and the due dates of its runs.
// Every operation names the person of the session and runs in the Träger
// transaction of its Träger, which the caller opens. Every read and change
// of a deviation is recorded in its trail.
type BillingTerms interface {
	// ServiceTerms answers the terms of every service of BillingServices.
	ServiceTerms(ctx context.Context, organizationID int64) ([]ServiceTerms, error)
	// SaveServiceTerms sets the terms of one service.
	SaveServiceTerms(ctx context.Context, reviewer HolidayReviewer, terms ServiceTerms) ([]ServiceTerms, error)
	// ListFamilyBilling lists the deviations of the families, by family.
	ListFamilyBilling(ctx context.Context, reviewer HolidayReviewer) ([]FamilyBilling, error)
	// SaveFamilyBilling sets the deviation of the family of a contract; an
	// applicant without payment deviation removes it.
	SaveFamilyBilling(ctx context.Context, reviewer HolidayReviewer, input FamilyBillingInput) (*FamilyBilling, error)
	// DeleteFamilyBilling removes a deviation.
	DeleteFamilyBilling(ctx context.Context, reviewer HolidayReviewer, id int64) error
	// SetCareRunDueDates and SetHolidayRunDueDates replace the due dates of a
	// calculated run; a released run is ErrBillingRunReleased.
	SetCareRunDueDates(ctx context.Context, reviewer HolidayReviewer, runID int64, dates RunDueDates) (RunDueDates, error)
	SetHolidayRunDueDates(ctx context.Context, reviewer HolidayReviewer, runID int64, dates RunDueDates) (RunDueDates, error)
}
