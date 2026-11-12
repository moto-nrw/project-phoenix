package ports

import (
	"context"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/carecontract"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The actions of the trail the signature adds (#4679).
const (
	ActionSign   = "sign"
	ActionRemind = "remind"
)

// StoredDocument is the frozen PDF of the contract's current version with
// its checksum.
type StoredDocument struct {
	ID        int64
	VersionID int64
	SHA256    string
	File      carecontract.File
}

// SentState is what the parents were sent: the SHA-256 of the PDF and the
// checksum of the version it shows. A signature compares the contract with
// it, so a PDF or a version changed after the send is never signed.
type SentState struct {
	DocumentSHA256 string
	VersionSHA256  string
}

// SigningLinkRecord is a stored one-time mail link.
type SigningLinkRecord struct {
	ID         int64
	ContractID int64
	DocumentID int64
	Sent       SentState
	ExpiresAt  time.Time
	UsedAt     *time.Time
}

// NewSignature is the conclusion of one contract version.
type NewSignature struct {
	ContractID          int64
	VersionID           int64
	DocumentID          *int64
	Method              string
	Channel             string
	AuditSignatureID    *int64
	SignerName          string
	SignerAccountID     *int64
	SigningLinkID       *int64
	VersionSHA256       string
	DocumentSHA256      string
	WithdrawalNoticeAt  *time.Time
	EarlyStartRequested bool
	PaperFilename       string
	PaperPDF            []byte
	PaperSHA256         string
	SignedOn            calendar.Date
	SignedAt            time.Time
	RecordedByAccountID *int64
}

type StoredSignature struct {
	VersionID           int64
	DocumentID          *int64
	Method              string
	Channel             string
	AuditSignatureID    *int64
	SignerName          string
	VersionSHA256       string
	DocumentSHA256      string
	WithdrawalNoticeAt  *time.Time
	EarlyStartRequested bool
	PaperFilename       string
	PaperIntact         bool
	SignedOn            calendar.Date
	SignedAt            time.Time
}

// ReminderCandidate is a sent contract without signature whose last mail
// or reminder lies back far enough.
type ReminderCandidate struct {
	ContractID int64
	Number     int
	Recipient  string
}

// SignatureStore persists the signatures, links and reminders of the
// contracts of a Träger in its Träger transaction.
type SignatureStore interface {
	CurrentDocument(ctx context.Context, organizationID, contractID, versionID int64) (StoredDocument, error)
	InsertSigningLink(ctx context.Context, organizationID, contractID, documentID int64, tokenSHA256 string, sent SentState, expiresAt time.Time) error
	SigningLink(ctx context.Context, organizationID int64, tokenSHA256 string) (SigningLinkRecord, error)
	// LatestSigningLink reads the newest link of a contract; found is false
	// when the contract was never sent with one.
	LatestSigningLink(ctx context.Context, organizationID, contractID int64) (link SigningLinkRecord, found bool, err error)
	UseSigningLink(ctx context.Context, organizationID, linkID int64, at time.Time) error
	InsertSignature(ctx context.Context, organizationID int64, signature NewSignature) error
	// Signature reads the signature of a version; versionID 0 reads the
	// newest signature of the contract, whichever version it signed.
	Signature(ctx context.Context, organizationID, contractID, versionID int64) (*StoredSignature, error)
	PaperDocument(ctx context.Context, organizationID, contractID int64) (carecontract.File, error)
	Reminders(ctx context.Context, organizationID, contractID int64) ([]carecontract.Reminder, error)
	DueReminders(ctx context.Context, organizationID, contractID int64, lastContactBefore time.Time, maxReminders int) ([]ReminderCandidate, error)
	InsertReminder(ctx context.Context, organizationID, contractID int64, number int, outboxID int64, recipient string) error
	InboxContractAddressed(ctx context.Context, organizationID, contractID int64, reader carecontract.InboxReader) (bool, error)
}
