package parentbilling

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The document inbox of the parents portal (spec #4611 user stories 93 and
// 96, #4671, ADR 0126). A parent with an account finds every confirmation
// letter the Träger sent to the family: an invoice whose letter was sent at
// least once and whose recipient address is the address of the account,
// the account of the payer in ADR 0088 section 7. The parents portal decides
// which Träger an account may read at all from its relationships to the
// children; this capability only answers for one Träger in its Träger
// transaction, which the caller opens. Every read lands in the trail of the
// run with the account.

// PushKindBillingDocument is the outbox kind of the push of a Träger that
// announces a new letter in the inbox.
const PushKindBillingDocument = "parent_billing_document"

// PushRelatedBillingDocument is the related entity type of that push.
const PushRelatedBillingDocument = "parent_billing_document"

// ParentDocumentKindConfirmationLetter is the kind of a confirmation letter
// that is the invoice of a holiday registration (ADR 0121).
const ParentDocumentKindConfirmationLetter = "confirmation_letter"

// ErrParentDocumentNotFound reports a document that is not in the inbox of
// the account: unknown, never sent, or addressed to someone else.
var ErrParentDocumentNotFound = errors.New("document not found in the inbox")

// BillingDocumentNotice is the push of a Träger that a letter is in the
// inbox of the parents with the recipient's address. It names no device:
// the worker finds the accounts and their devices when it sends.
type BillingDocumentNotice struct {
	OrganizationID int64  `json:"organization_id"`
	DocumentID     int64  `json:"document_id"`
	RecipientEmail string `json:"recipient_email"`
}

// ParentDocumentReader is the account of the parents portal that reads its
// inbox, with its current address.
type ParentDocumentReader struct {
	AccountID int64
	Email     string
}

// ParentDocument is one document in the inbox: what it is, for which
// period and children, its amount and when it was first sent.
type ParentDocument struct {
	DocumentID int64 `json:"id"`
	// RunID is the run whose trail records the read; the parents never see
	// it.
	RunID        int64         `json:"-"`
	Kind         string        `json:"kind"`
	Number       string        `json:"number"`
	PeriodName   string        `json:"period_name"`
	ServiceFrom  calendar.Date `json:"service_from"`
	ServiceUntil calendar.Date `json:"service_until"`
	IssuedOn     calendar.Date `json:"issued_on"`
	TotalCents   int64         `json:"total_cents"`
	ChildNames   []string      `json:"child_names"`
	SentAt       time.Time     `json:"sent_at"`
}

// ParentDocuments serves the inbox of the parents portal for one Träger.
// Every operation runs in the Träger transaction of organizationID, which
// the caller opens; the reader's address selects the documents.
type ParentDocuments interface {
	// ListParentDocuments lists the sent documents of the Träger addressed
	// to the reader, the newest first, and records the read.
	ListParentDocuments(ctx context.Context, organizationID int64, reader ParentDocumentReader) ([]ParentDocument, error)
	// ParentDocumentLetter reads the stored PDF of one sent document
	// addressed to the reader and records the read.
	ParentDocumentLetter(ctx context.Context, organizationID, documentID int64, reader ParentDocumentReader) (BillingLetterFile, error)
}
