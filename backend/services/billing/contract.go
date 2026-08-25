// Package billing exposes the school's moto contract as one read surface and
// the operator's maintenance of it as a small CRUD surface (#1459 demo).
//
// Scope on purpose: there is no payment provider, no dunning, no enforcement.
// A school that exceeds its booked contingent is NOT blocked — the overview
// only makes the gap visible. Anything that locks a school out of its own
// attendance data is a product decision, not a demo.
package billing

import (
	"errors"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/models/platform"
)

// Errors surfaced to HTTP. German where the operator reads them verbatim.
var (
	// ErrInvoiceNotFound is returned for an unknown invoice id, or for one that
	// belongs to another school — the two are indistinguishable on purpose.
	ErrInvoiceNotFound = errors.New("Rechnung wurde nicht gefunden.")
	// ErrInvoiceNumberTaken maps the (tenant_id, invoice_number) unique index
	// to something an operator can act on.
	ErrInvoiceNumberTaken = errors.New("Diese Rechnungsnummer gibt es für diese Schule schon.")
	// ErrNoTenantContext guards the tenant read path against being called
	// outside a tenant transaction, where it would silently see nothing.
	ErrNoTenantContext = errors.New("billing: no tenant in context")
)

// contractSettingKeys is the batch every overview read prefetches. Keeping it
// in one place means the handler and the service cannot drift apart on which
// keys are loaded in a single round trip.
var contractSettingKeys = []string{
	configModel.KeyContractTier,
	configModel.KeyContractBookedChildren,
	configModel.KeyContractPricePerChildCents,
	configModel.KeyContractBillingCycle,
	configModel.KeyContractTermStart,
	configModel.KeyContractTermEnd,
	configModel.KeyContractInvoiceRecipient,
	configModel.KeyContractCustomerNumber,
	configModel.KeyContractSupportEmail,
	configModel.KeyContractNote,
}

// ContractSettingKeys returns the settings the overview reads, for callers
// that want to prefetch them (api/common.PrefetchSettings).
func ContractSettingKeys() []string {
	return append([]string(nil), contractSettingKeys...)
}

// InvoiceView is one invoice as the school and the operator see it: the stored
// row plus the one derived flag that only makes sense against a reference day.
type InvoiceView struct {
	ID            int64          `json:"id"`
	PeriodLabel   string         `json:"period_label"`
	InvoiceNumber string         `json:"invoice_number"`
	AmountCents   int64          `json:"amount_cents"`
	DueDate       timezone.Date  `json:"due_date"`
	Status        string         `json:"status"`
	Overdue       bool           `json:"overdue"`
	PaidOn        *timezone.Date `json:"paid_on,omitempty"`
	Note          string         `json:"note"`
}

// ContractOverview is the complete /vertrag payload: the contract facts from
// the settings registry, the live child count, and the payment schedule.
type ContractOverview struct {
	Tier               string         `json:"tier"`
	TierLabel          string         `json:"tier_label"`
	BookedChildren     int            `json:"booked_children"`
	ActiveChildren     int            `json:"active_children"`
	PricePerChildCents int            `json:"price_per_child_cents"`
	BillingCycle       string         `json:"billing_cycle"`
	BillingCycleLabel  string         `json:"billing_cycle_label"`
	TermStart          *timezone.Date `json:"term_start,omitempty"`
	TermEnd            *timezone.Date `json:"term_end,omitempty"`
	InvoiceRecipient   string         `json:"invoice_recipient"`
	CustomerNumber     string         `json:"customer_number"`
	SupportEmail       string         `json:"support_email"`
	Note               string         `json:"note"`
	// Configured reports whether the moto team has filled the contract in at
	// all. The frontend needs one flag, not ten emptiness checks, to decide
	// between "kein Vertrag hinterlegt" and the detail view.
	Configured bool `json:"configured"`
	// ReferenceDate is the day Overdue and the child count were computed for.
	// Shipping it makes the page's "Stand heute" honest across timezones.
	ReferenceDate timezone.Date `json:"reference_date"`
	Invoices      []InvoiceView `json:"invoices"`
	// OpenAmountCents sums every invoice that is neither paid nor cancelled.
	OpenAmountCents int64 `json:"open_amount_cents"`
	// NextDue is the earliest unpaid invoice on or after ReferenceDate, or the
	// oldest overdue one when something is already late. Nil when nothing is
	// open. The school's single most important number, so it is computed once
	// here instead of three times in the UI.
	NextDue *InvoiceView `json:"next_due,omitempty"`
}

// InvoiceInput is the operator's write payload. Distinct from the model so a
// request can never set tenant_id, id, or the timestamps.
type InvoiceInput struct {
	PeriodLabel   string         `json:"period_label"`
	InvoiceNumber string         `json:"invoice_number"`
	AmountCents   int64          `json:"amount_cents"`
	DueDate       timezone.Date  `json:"due_date"`
	Status        string         `json:"status"`
	PaidOn        *timezone.Date `json:"paid_on"`
	Note          string         `json:"note"`
}

// tierLabels / cycleLabels render a stored value for the UI. They live next to
// the select options in defaults/contract.go and are asserted to stay in sync
// by TestContractLabelsCoverRegistryOptions.
var tierLabels = map[string]string{
	configModel.ContractTierUnset:   "Noch nicht hinterlegt",
	configModel.ContractTierTest:    "Testphase",
	configModel.ContractTierBasis:   "Basis",
	configModel.ContractTierPlus:    "Plus",
	configModel.ContractTierPremium: "Premium",
}

var cycleLabels = map[string]string{
	configModel.ContractCycleUnset:     "Noch nicht hinterlegt",
	configModel.ContractCycleMonthly:   "Monatlich",
	configModel.ContractCycleQuarterly: "Quartalsweise",
	configModel.ContractCycleYearly:    "Jährlich",
}

// TierLabel maps a stored tier to its German label, falling back to the raw
// value so an unknown stored value is visible rather than silently blank.
func TierLabel(tier string) string {
	if label, ok := tierLabels[tier]; ok {
		return label
	}
	return tier
}

// BillingCycleLabel is TierLabel's counterpart for the payment rhythm.
func BillingCycleLabel(cycle string) string {
	if label, ok := cycleLabels[cycle]; ok {
		return label
	}
	return cycle
}

// normalizeInput trims free text and enforces the paid/paid_on pairing that
// the DB CHECK constraint also carries.
//
// Two deliberate conveniences, both business rules and therefore here rather
// than in the model: marking an invoice paid without naming a day means "paid
// today", and moving an invoice away from paid drops the payment date instead
// of rejecting the change.
func normalizeInput(in InvoiceInput, today timezone.Date) InvoiceInput {
	in.PeriodLabel = strings.TrimSpace(in.PeriodLabel)
	in.InvoiceNumber = strings.TrimSpace(in.InvoiceNumber)
	in.Note = strings.TrimSpace(in.Note)
	if in.Status == "" {
		in.Status = platform.InvoiceStatusOpen
	}

	switch in.Status {
	case platform.InvoiceStatusPaid:
		if in.PaidOn == nil {
			paid := today
			in.PaidOn = &paid
		}
	default:
		in.PaidOn = nil
	}

	return in
}

// applyInput copies a normalized input onto an invoice row.
func applyInput(invoice *platform.SchoolInvoice, in InvoiceInput) {
	invoice.PeriodLabel = in.PeriodLabel
	invoice.InvoiceNumber = in.InvoiceNumber
	invoice.AmountCents = in.AmountCents
	invoice.DueDate = in.DueDate
	invoice.Status = in.Status
	invoice.PaidOn = in.PaidOn
	invoice.Note = in.Note
}

// toView projects a stored row for the wire, deriving Overdue against today.
func toView(invoice *platform.SchoolInvoice, today timezone.Date) InvoiceView {
	return InvoiceView{
		ID:            invoice.ID,
		PeriodLabel:   invoice.PeriodLabel,
		InvoiceNumber: invoice.InvoiceNumber,
		AmountCents:   invoice.AmountCents,
		DueDate:       invoice.DueDate,
		Status:        invoice.Status,
		Overdue:       invoice.IsOverdueOn(today),
		PaidOn:        invoice.PaidOn,
		Note:          invoice.Note,
	}
}

// ToViews projects a list of stored rows, preserving order.
func ToViews(invoices []*platform.SchoolInvoice, today timezone.Date) []InvoiceView {
	views := make([]InvoiceView, 0, len(invoices))
	for _, invoice := range invoices {
		if invoice == nil {
			continue
		}
		views = append(views, toView(invoice, today))
	}
	return views
}

// summarizeInvoices computes the two aggregates the overview carries.
//
// "Next due" prefers the oldest overdue invoice over the nearest future one:
// a school that is already late needs to see that date, not the next one it
// has not missed yet. Input order is due date DESC (the repository contract),
// so the last matching row in each pass is the earliest one.
func summarizeInvoices(views []InvoiceView) (openCents int64, next *InvoiceView) {
	var earliestOpen *InvoiceView
	var earliestOverdue *InvoiceView

	for i := range views {
		view := &views[i]
		if view.Status != platform.InvoiceStatusOpen {
			continue
		}
		openCents += view.AmountCents
		earliestOpen = view
		if view.Overdue {
			earliestOverdue = view
		}
	}

	if earliestOverdue != nil {
		return openCents, earliestOverdue
	}
	return openCents, earliestOpen
}

// wrapWriteError maps the storage-level unique-index violation to a message an
// operator can act on. Everything else is passed through unchanged.
func wrapWriteError(op string, err error) error {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "uq_school_invoices_number") {
		return ErrInvoiceNumberTaken
	}
	return fmt.Errorf("billing: %s: %w", op, err)
}
