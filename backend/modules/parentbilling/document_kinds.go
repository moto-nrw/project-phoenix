package parentbilling

import (
	"context"
	"errors"
	"slices"
	"time"
)

// The kind of document a service issues (#4688, spec #4611 user story 73,
// ADR 0119 addendum 2026-11-08). The Abrechnung chooses per service: an
// invoice, a fee notice with the legal remedy of the Träger (Hardt, MK-007)
// or, for the holiday care, the confirmation letter as invoice. The release
// freezes the kind on every document it issues; a later choice changes only
// the next release. A Storno and a Gutschrift keep the kind of their invoice.

// Kinds of document.
const (
	// DocumentKindConfirmation is the confirmation letter as invoice of a
	// holiday registration (ADR 0121); the holiday care issues it unless the
	// Träger chooses otherwise.
	DocumentKindConfirmation = "confirmation"
	// DocumentKindInvoice is an invoice; the monthly care issues it unless
	// the Träger chooses otherwise.
	DocumentKindInvoice = "invoice"
	// DocumentKindFeeNotice is a fee notice, an administrative act with the
	// legal remedy the Träger writes into its template.
	DocumentKindFeeNotice = "fee_notice"
)

var (
	// ErrDocumentKindInvalid reports a kind of document a service does not
	// offer or an unknown service.
	ErrDocumentKindInvalid = errors.New("the service does not issue this kind of document")
	// ErrLegalRemedyMissing reports the release of fee notices while the
	// template of the fee notice has no legal remedy: without it the time
	// limit for an appeal grows to one year (§ 58 Abs. 2 VwGO).
	ErrLegalRemedyMissing = errors.New("the template of the fee notice has no legal remedy")
)

// DocumentKindChoices lists the kinds a service may issue, the default
// first. A confirmation letter belongs to a holiday registration; the
// monthly care has none.
func DocumentKindChoices(service string) []string {
	switch service {
	case ServiceHoliday:
		return []string{DocumentKindConfirmation, DocumentKindInvoice, DocumentKindFeeNotice}
	case ServiceCare:
		return []string{DocumentKindInvoice, DocumentKindFeeNotice}
	}
	return nil
}

// DefaultDocumentKind is the kind a service issues without a choice: what
// it issued before #4688.
func DefaultDocumentKind(service string) string {
	if choices := DocumentKindChoices(service); len(choices) > 0 {
		return choices[0]
	}
	return ""
}

// ValidDocumentKind reports whether service issues kind.
func ValidDocumentKind(service, kind string) bool {
	return slices.Contains(DocumentKindChoices(service), kind)
}

// ServiceDocumentKind is the kind a service issues now, with its choices.
// UpdatedAt is nil while the service keeps its default.
type ServiceDocumentKind struct {
	Service      string     `json:"service"`
	DocumentKind string     `json:"document_kind"`
	Choices      []string   `json:"choices"`
	UpdatedAt    *time.Time `json:"updated_at"`
}

// DocumentKindOverview is the kind of every service and whether the
// template of the fee notice has its legal remedy, so the Abrechnung sees
// before a release what is missing.
type DocumentKindOverview struct {
	Services []ServiceDocumentKind `json:"services"`
	// LegalRemedySet reports a legal remedy in the current template of the
	// fee notice; FeeNoticeTemplateVersion is that template's version, 0
	// for the standard template, which has none.
	LegalRemedySet           bool `json:"legal_remedy_set"`
	FeeNoticeTemplateVersion int  `json:"fee_notice_template_version"`
}

// DocumentKindState is the kind of document a run issues: for a
// calculated run the kind its release would freeze now, with whether the
// release would refuse it for a missing legal remedy; for a released run
// the kind it froze.
type DocumentKindState struct {
	DocumentKind       string `json:"document_kind"`
	LegalRemedyMissing bool   `json:"legal_remedy_missing"`
}

// BillingDocumentTemplates is the Träger's current template of a kind of
// document as the billing reads it. The root binds it to the document
// templates of Organisation & Tenancy (ADR 0152); it runs in the Träger
// transaction of the organization and reads no personal data.
type BillingDocumentTemplates interface {
	DocumentTemplate(ctx context.Context, organizationID int64, documentKind string) (LetterTemplate, error)
}

// BillingDocumentKinds lets the Abrechnung of a Träger read and choose the
// kind of document of each service in its Träger transaction, which the
// caller opens.
type BillingDocumentKinds interface {
	ListDocumentKinds(ctx context.Context, organizationID int64) (DocumentKindOverview, error)
	SetDocumentKind(ctx context.Context, organizationID int64, service, documentKind string) (DocumentKindOverview, error)
}
