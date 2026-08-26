package platform

import (
	"context"
	"errors"
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
)

// Invoice status values. They are stored verbatim (German) because they are
// also the wire contract towards the frontend, which renders them as labels
// without a second mapping table. The DB CHECK constraint pins the same set.
const (
	// InvoiceStatusOpen: billed, not yet paid. Whether it is late follows from
	// the due date and today — it is never a fourth stored status, because a
	// stored "überfällig" would need a nightly job to stay true.
	InvoiceStatusOpen = "offen"
	// InvoiceStatusPaid: payment received. Requires PaidOn.
	InvoiceStatusPaid = "bezahlt"
	// InvoiceStatusCancelled: withdrawn by the moto team, nothing to pay.
	InvoiceStatusCancelled = "storniert"
)

// Validation errors. German because they surface in the operator UI verbatim.
var (
	ErrInvoicePeriodLabelRequired = errors.New("zeitraum darf nicht leer sein")
	ErrInvoiceAmountNegative      = errors.New("betrag darf nicht negativ sein")
	ErrInvoiceDueDateRequired     = errors.New("fälligkeitsdatum wird benötigt")
	ErrInvoiceUnknownStatus       = errors.New("unbekannter Zahlungsstatus")
	ErrInvoicePaidOnRequired      = errors.New("bei „bezahlt“ wird ein Zahlungsdatum benötigt")
	ErrInvoicePaidOnNotAllowed    = errors.New("ein Zahlungsdatum ist nur bei „bezahlt“ erlaubt")
	ErrInvoiceNoteTooLong         = errors.New("hinweis ist zu lang")
)

// MaxInvoiceNoteLen caps the free-text note so a paste accident cannot fill a
// column the school reads on a phone.
const MaxInvoiceNoteLen = 500

// SchoolInvoice is one billing period of a school's moto contract: what is
// owed, when it is due, and whether the moto team has seen the money.
//
// The whole row is operator-maintained. There is no payment provider behind
// it — status is a human statement, which is exactly the demo scope (#1459).
type SchoolInvoice struct {
	base.Model `bun:"schema:platform,table:school_invoices"`
	base.TenantModel

	// PeriodLabel names the billed span in the school's own words, e.g.
	// "Januar 2026" or "1. Quartal 2026". Free text on purpose: the demo must
	// carry monthly, quarterly and yearly rhythms without a period model.
	PeriodLabel string `bun:"period_label,notnull" json:"period_label"`
	// InvoiceNumber is the number printed on the PDF the school received.
	// Empty means "not issued yet"; unique per tenant when set.
	InvoiceNumber string `bun:"invoice_number,notnull" json:"invoice_number"`
	// AmountCents is the gross amount in cents. Integer, never a float.
	AmountCents int64 `bun:"amount_cents,notnull" json:"amount_cents"`
	// DueDate is a calendar day, not an instant (see calendar-dates rule).
	DueDate timezone.Date `bun:"due_date,type:date,notnull" json:"due_date"`
	Status  string        `bun:"status,notnull" json:"status"`
	// PaidOn is set exactly when Status is InvoiceStatusPaid.
	PaidOn *timezone.Date `bun:"paid_on,type:date" json:"paid_on,omitempty"`
	Note   string         `bun:"note,notnull" json:"note"`
}

// ValidInvoiceStatuses reports whether s is one of the three stored statuses.
func ValidInvoiceStatus(s string) bool {
	switch s {
	case InvoiceStatusOpen, InvoiceStatusPaid, InvoiceStatusCancelled:
		return true
	default:
		return false
	}
}

// Validate implements base.Validator so base.Repository.Create rejects a row
// the DB CHECK constraints would reject anyway — with a German message instead
// of a driver error.
func (i *SchoolInvoice) Validate() error {
	if strings.TrimSpace(i.PeriodLabel) == "" {
		return ErrInvoicePeriodLabelRequired
	}
	if i.AmountCents < 0 {
		return ErrInvoiceAmountNegative
	}
	if i.DueDate.IsZero() {
		return ErrInvoiceDueDateRequired
	}
	if !ValidInvoiceStatus(i.Status) {
		return ErrInvoiceUnknownStatus
	}
	if i.Status == InvoiceStatusPaid && i.PaidOn == nil {
		return ErrInvoicePaidOnRequired
	}
	if i.Status != InvoiceStatusPaid && i.PaidOn != nil {
		return ErrInvoicePaidOnNotAllowed
	}
	if len([]rune(i.Note)) > MaxInvoiceNoteLen {
		return ErrInvoiceNoteTooLong
	}
	return nil
}

// IsOverdueOn is a pure derivation from stored fields: an open invoice whose
// due date has passed relative to the caller's reference day.
//
// The reference day is a parameter, not time.Now(), so this stays a fact about
// the row and the policy ("what counts as today") stays in the service —
// same shape as CareWithdrawalCompletion.UrgencyOn.
func (i SchoolInvoice) IsOverdueOn(reference timezone.Date) bool {
	return i.Status == InvoiceStatusOpen && i.DueDate.Before(reference)
}

// SchoolInvoiceRepository is the data-access contract for the payment
// schedule. It deliberately adds nothing to the generic CRUD block beyond an
// ordered listing: every access is "this tenant's invoices, newest due date
// first", which the plain equality-filter List cannot express.
type SchoolInvoiceRepository interface {
	base.CRUDRepository[*SchoolInvoice]

	// ListForTenant returns the tenant's invoices ordered by due date
	// descending (id descending as a stable tiebreaker). Tenant scoping comes
	// from the ambient tenant transaction, like every other tenant-scoped repo.
	ListForTenant(ctx context.Context) ([]*SchoolInvoice, error)

	// FindByIDOrNil returns (nil, nil) for a row that does not exist or belongs
	// to another tenant, and a real error only for a real failure. Promoted
	// from the embedded base repository so callers can tell "not found" from
	// "database is down" — which the plain FindByID collapses into one error.
	FindByIDOrNil(ctx context.Context, id any) (*SchoolInvoice, error)
}
