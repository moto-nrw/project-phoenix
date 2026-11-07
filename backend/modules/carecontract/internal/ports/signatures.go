package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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

// SigningLinkRecord is a stored one-time mail link.
type SigningLinkRecord struct {
	ID         int64
	ContractID int64
	DocumentID int64
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

// StoredSignature is the signature of a contract version as stored.
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
	PaperSHA256         string
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
	InsertSigningLink(ctx context.Context, organizationID, contractID, documentID int64, tokenSHA256 string, expiresAt time.Time) error
	SigningLink(ctx context.Context, organizationID int64, tokenSHA256 string) (SigningLinkRecord, error)
	UseSigningLink(ctx context.Context, organizationID, linkID int64, at time.Time) error
	InsertSignature(ctx context.Context, organizationID int64, signature NewSignature) error
	Signature(ctx context.Context, organizationID, contractID, versionID int64) (*StoredSignature, error)
	PaperDocument(ctx context.Context, organizationID, contractID int64) (carecontract.File, error)
	Reminders(ctx context.Context, organizationID, contractID int64) ([]carecontract.Reminder, error)
	DueReminders(ctx context.Context, organizationID, contractID int64, lastContactBefore time.Time, maxReminders int) ([]ReminderCandidate, error)
	InsertReminder(ctx context.Context, organizationID, contractID int64, number int, outboxID int64, recipient string) error
	InboxContractAddressed(ctx context.Context, organizationID, contractID int64, reader carecontract.InboxReader) (bool, error)
}
