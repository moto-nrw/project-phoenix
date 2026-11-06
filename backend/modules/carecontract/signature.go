package carecontract

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The signature of a care contract (#4679, ADR 0088 section 3): the parents
// sign the frozen PDF of a sent contract with a drawn or typed signature,
// either in the parents portal or through the one-time link of its mail
// (a simple electronic signature, no QES, #3740). The Träger-Büro records a
// contract the parents signed on paper by uploading its PDF. Either way the
// contract is concluded: it becomes aktiv, or unterschrieben until its
// effective date, and its version becomes wirksam. The Audit Platform keeps
// the drawn or typed signature (#4785); the contract keeps the checksums of
// the version and of the PDF and how the signer was identified.

// The methods of a signature.
const (
	SignatureDrawn = "drawn"
	SignatureTyped = "typed"
	SignaturePaper = "paper"
)

// The ways a signer was identified.
const (
	ChannelParentAccount = "parent_account"
	ChannelMailLink      = "mail_link"
	ChannelOffice        = "office"
)

// SigningLinkLifetime is how long the link of a contract mail stays open.
const SigningLinkLifetime = 30 * 24 * time.Hour

// MaxReminders is how many reminders a contract gets at most.
const MaxReminders = 3

// PaperMaxBytes bounds the PDF of a paper contract.
const PaperMaxBytes = 10 << 20

var (
	// ErrSigningLinkInvalid reports a mail link that is unknown, used,
	// expired, or no longer names the PDF of a contract waiting for its
	// signature. The answer never tells which.
	ErrSigningLinkInvalid = errors.New("the signing link is not valid")
	// ErrContractNotSignable reports a contract that waits for no signature:
	// a draft online, or a contract signed already.
	ErrContractNotSignable = errors.New("the contract cannot be signed")
	// ErrSignatureInvalid reports a signature image or typed name the
	// checks refuse.
	ErrSignatureInvalid = errors.New("the signature is not valid")
	// ErrWithdrawalNoticeRequired reports an online signature without the
	// confirmation that the parents read the withdrawal notice.
	ErrWithdrawalNoticeRequired = errors.New("the withdrawal notice must be confirmed")
	// ErrPaperDocumentInvalid reports an upload that is no PDF or too large.
	ErrPaperDocumentInvalid = errors.New("the paper contract must be a PDF")
)

// SignatureInput is the signature as the parents give it: the method, the
// base64 PNG, for a typed signature the typed name; that they read the
// withdrawal notice and whether care shall begin before the withdrawal
// period ends (§ 356 section 4, § 357a section 2 BGB).
type SignatureInput struct {
	Method             string `json:"method"`
	ImagePNG           string `json:"image_png"`
	TypedName          string `json:"typed_name"`
	NoticeAcknowledged bool   `json:"withdrawal_notice_acknowledged"`
	EarlyStart         bool   `json:"early_start_requested"`
}

// PaperInput is a contract the parents signed on paper.
type PaperInput struct {
	SignerName string
	SignedOn   calendar.Date
	Filename   string
	PDF        []byte
}

// ContractSignature is the signature of the contract's current version.
type ContractSignature struct {
	Method              string        `json:"method"`
	Channel             string        `json:"channel"`
	SignerName          string        `json:"signer_name"`
	TypedName           string        `json:"typed_name"`
	SignedAt            time.Time     `json:"signed_at"`
	SignedOn            calendar.Date `json:"signed_on"`
	VersionSHA256       string        `json:"version_sha256"`
	DocumentSHA256      string        `json:"document_sha256"`
	WithdrawalNoticeAt  *time.Time    `json:"withdrawal_notice_at"`
	EarlyStartRequested bool          `json:"early_start_requested"`
	// WithdrawalEndsOn is the last day of the 14-day withdrawal period of an
	// online signature; nil on paper.
	WithdrawalEndsOn *calendar.Date `json:"withdrawal_ends_on"`
	// ImagePNG is the drawn or typed signature; empty on paper.
	ImagePNG      []byte `json:"image_png"`
	PaperFilename string `json:"paper_filename"`
	// Intact is false when the ledger entry, the PDF or the version no
	// longer match the checksums stored at the signature.
	Intact bool `json:"intact"`
}

// Reminder is one reminder mail of a contract.
type Reminder struct {
	Number    int       `json:"number"`
	Recipient string    `json:"recipient"`
	SentAt    time.Time `json:"sent_at"`
}

// SigningView is what the parents read before they sign: the contract with
// its booked care and terms, the party, the withdrawal notice and, once
// signed, the signature.
type SigningView struct {
	ContractID       int64             `json:"contract_id"`
	OrganizationID   int64             `json:"organization_id"`
	Status           string            `json:"status"`
	TraegerName      string            `json:"traeger_name"`
	PartyName        string            `json:"party_name"`
	Version          Version           `json:"version"`
	DocumentSHA256   string            `json:"document_sha256"`
	WithdrawalNotice []string          `json:"withdrawal_notice"`
	Signature        *SigningSignature `json:"signature"`
}

// SigningSignature is the signature as the parents see it after signing.
type SigningSignature struct {
	Method              string         `json:"method"`
	SignerName          string         `json:"signer_name"`
	SignedAt            time.Time      `json:"signed_at"`
	EarlyStartRequested bool           `json:"early_start_requested"`
	WithdrawalEndsOn    *calendar.Date `json:"withdrawal_ends_on"`
}

// Signing serves the one-time link of a contract mail (#4679). The link is
// "<Träger>.<token>"; it names one contract and the PDF it was sent with.
// Reading leaves the link open, so a mail scanner that opens it uses
// nothing up; signing closes it for good. Each operation opens the Träger
// transaction of the link itself.
type Signing interface {
	LinkSigning(ctx context.Context, link string) (SigningView, error)
	LinkSigningDocument(ctx context.Context, link string) (File, error)
	SignByLink(ctx context.Context, link string, input SignatureInput) (SigningView, error)
}

// InboxSigning serves the signature in the parents portal. The workflow
// passes the children whose relationship permits the parents to sign; the
// contract must have been sent to the account's address. Both operations
// run in the Träger transaction the caller opens.
type InboxSigning interface {
	InboxSigningView(ctx context.Context, organizationID, contractID int64, reader InboxReader) (SigningView, error)
	SignInInbox(ctx context.Context, organizationID, contractID int64, reader InboxReader, input SignatureInput) (SigningView, error)
}

// PaperSignatures records a contract signed on paper (#4679, option D) in
// the Träger transaction of the route, and reads its PDF back.
type PaperSignatures interface {
	RecordPaperSignature(ctx context.Context, reviewer Reviewer, contractID int64, input PaperInput) error
	PaperDocument(ctx context.Context, reviewer Reviewer, contractID int64) (File, error)
}

// Reminders sends the due reminders of the contracts that wait for a
// signature, over every Träger. It opens its own units.
type Reminders interface {
	RemindDue(ctx context.Context) error
}

// ContractSignatureEntry is a drawn or typed signature for the ledger.
type ContractSignatureEntry struct {
	OrganizationID  int64
	VersionID       int64
	SignerAccountID *int64
	SignerName      string
	DocumentSHA256  string
	SignedAt        time.Time
	Input           SignatureInput
}

// SignatureProof is a stored signature of the ledger.
type SignatureProof struct {
	VersionID      int64
	Method         string
	TypedName      string
	PNG            []byte
	DocumentSHA256 string
	Intact         bool
}

// SignatureLedger is the Audit Platform's signature ledger at the Träger
// (#4785) as the contract uses it, in the caller's Träger transaction. A
// refused image or name wraps ErrSignatureInvalid.
type SignatureLedger interface {
	RecordContractSignature(ctx context.Context, entry ContractSignatureEntry) (int64, error)
	ContractSignatureProofs(ctx context.Context, ids []int64) (map[int64]SignatureProof, error)
}

// SigningTokens makes the secret of a mail link and its stored SHA-256.
type SigningTokens interface {
	NewToken() (raw string, fingerprint string, err error)
	Fingerprint(raw string) string
}

// SigningLinks are the tokens and the base of the parents portal the mail
// links point to.
type SigningLinks struct {
	Tokens     SigningTokens
	ParentsURL string
}

// TraegerSender reads the letterhead of the Träger in the caller's Träger
// transaction: its first line names the Träger, the rest is its address.
type TraegerSender interface {
	TraegerLetterhead(ctx context.Context, organizationID int64) (string, error)
}

// TraegerOrganizations lists the active Träger, outside any transaction.
type TraegerOrganizations interface {
	ActiveTraegerIDs(ctx context.Context) ([]int64, error)
}

// ReminderSettings reads after how many days a contract without signature
// is reminded, in the Träger transaction; 0 means no reminders.
type ReminderSettings interface {
	SignatureReminderDays(ctx context.Context, organizationID int64) (int, error)
}

// The purposes of a contract mail.
const (
	MailPurposeContract = ""
	MailPurposeReminder = "reminder"
	MailPurposeSigned   = "signed"
)
