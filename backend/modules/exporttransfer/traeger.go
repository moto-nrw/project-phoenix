package exporttransfer

import (
	"context"
	"errors"
	"time"
)

// The Träger scope of Export Transfer (#6824, ADR 0218; #6911, ADR 0222):
// several recipients per Träger, each with its own sealed credentials, one
// recipient per export format, a journal per recipient and Träger
// transactions of its own. It is a separate capability, so the school scope
// above keeps its exact behaviour and neither scope can reach the other's
// target.

// KindParentBillingFinance is the journal kind of a finance export of the
// Elternabrechnung: Sollstellung, DATEV booking batch or PAIN.008.
const KindParentBillingFinance = "parent_billing_finance"

// The finance export formats of the Elternabrechnung a Träger assigns to a
// recipient (ADR 0222), in the order the Träger sees them.
const (
	FormatSollstellung = "sollstellung"
	FormatDatev        = "datev"
	FormatPain008      = "pain008"
)

// TraegerFormats lists the assignable formats in order.
var TraegerFormats = []string{FormatSollstellung, FormatDatev, FormatPain008}

// Recipient field names, in form order. A validation error and an
// incomplete recipient name these.
const (
	RecipientFieldName        = "name"
	RecipientFieldHost        = "host"
	RecipientFieldPort        = "port"
	RecipientFieldUsername    = "username"
	RecipientFieldPassword    = "password"
	RecipientFieldDirectory   = "remote_directory"
	RecipientFieldFingerprint = "host_key_fingerprint"
)

// Journal states of a Träger handover.
const (
	TraegerTransferred = "transferred"
	TraegerFailed      = "failed"
	TraegerInProgress  = "in_progress"
	// TraegerUnknown: the file may have arrived, but the outcome was never
	// recorded; nothing retries it.
	TraegerUnknown = "unknown"
)

// ErrRecipientInvalid marks a recipient a Träger cannot save. The error
// names the fields (RecipientError).
var ErrRecipientInvalid = errors.New("export transfer: the recipient is invalid")

// ErrRecipientNotFound reports a recipient that does not exist for the
// Träger, also one of another Träger.
var ErrRecipientNotFound = errors.New("export transfer: the recipient does not exist")

// ErrRecipientNameTaken reports a name another recipient of the Träger
// already has; two recipients of one name could not be told apart.
var ErrRecipientNameTaken = errors.New("export transfer: another recipient has this name")

// ErrAssignmentInvalid reports an assignment of an unknown format or of a
// format named twice.
var ErrAssignmentInvalid = errors.New("export transfer: the assignment of the formats is invalid")

// RecipientError lists the invalid fields of a recipient, in form order.
type RecipientError struct{ Fields []string }

func (e *RecipientError) Error() string { return ErrRecipientInvalid.Error() }

// Unwrap lets errors.Is find ErrRecipientInvalid.
func (e *RecipientError) Unwrap() error { return ErrRecipientInvalid }

// TraegerRecipient is the credential-free view of one recipient of a
// Träger. It has no password field at all, only whether one is stored.
type TraegerRecipient struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	// Enabled mirrors the switch; Ready also needs every value.
	Enabled            bool   `json:"enabled"`
	Ready              bool   `json:"ready"`
	Host               string `json:"host"`
	Port               int    `json:"port"`
	Username           string `json:"username"`
	RemoteDirectory    string `json:"remote_directory"`
	HostKeyFingerprint string `json:"host_key_fingerprint"`
	PasswordSet        bool   `json:"password_set"`
	// MissingFields names what still blocks a handover, in form order. A
	// stored password that no longer opens counts as missing.
	MissingFields []string `json:"missing_fields"`
	// Formats are the export formats this recipient receives, in order.
	Formats []string `json:"formats"`
	// LastHandover is its newest handover: its error state until the next.
	LastHandover *TraegerTransferEntry `json:"last_handover,omitempty"`
	UpdatedAt    time.Time             `json:"updated_at"`
}

// TraegerRecipientInput is what the Träger saves. A nil Password keeps the
// stored one; an empty one is refused, the recipient is removed instead.
type TraegerRecipientInput struct {
	Name               string
	Enabled            bool
	Host               string
	Port               int
	Username           string
	Password           *string
	RemoteDirectory    string
	HostKeyFingerprint string
}

// TraegerAssignment sends one format to one recipient; a nil RecipientID
// leaves the format to the download.
type TraegerAssignment struct {
	Format      string `json:"format"`
	RecipientID *int64 `json:"recipient_id"`
}

// Destination names where a file goes, so the producing domain can record
// it with its export. It holds no credential.
type Destination struct {
	RecipientID   int64
	RecipientName string
	Host          string
	Directory     string
}

// TraegerTransferRequest is one handover of a finance export of a Träger.
type TraegerTransferRequest struct {
	// OrganizationID is the Träger of the session; never from a request.
	OrganizationID int64
	Kind           string
	Format         string
	// PrepareFile builds the file in the Träger transaction that commits
	// the transfer intent, after the recipient was found ready.
	PrepareFile func(context.Context, Destination) (filename string, data []byte, err error)
	// Settle records the outcome with the producing domain, in the Träger
	// transaction that completes the journal: both agree or neither moves.
	Settle         func(context.Context, Outcome) error
	ActorAccountID int64
}

// TraegerTransferEntry is one row of the journal of a Träger.
type TraegerTransferEntry struct {
	ID              int64     `json:"id"`
	CreatedAt       time.Time `json:"created_at"`
	Format          string    `json:"format"`
	Filename        string    `json:"filename"`
	ByteSize        int64     `json:"byte_size"`
	TargetHost      string    `json:"target_host"`
	TargetDirectory string    `json:"target_directory"`
	// RecipientID is nil once the recipient was removed; RecipientName is
	// its name at the handover.
	RecipientID   *int64 `json:"recipient_id"`
	RecipientName string `json:"recipient_name"`
	// State is transferred, failed, in_progress or unknown.
	State  string `json:"state"`
	Reason string `json:"reason,omitempty"`
}

// TraegerRecipientsPage is every recipient of a Träger, the recipient of
// each format and the newest handovers, read in one Träger transaction.
type TraegerRecipientsPage struct {
	Recipients []TraegerRecipient `json:"recipients"`
	// Assignments has one entry per format of TraegerFormats, in order.
	Assignments []TraegerAssignment    `json:"assignments"`
	Handovers   []TraegerTransferEntry `json:"handovers"`
}

// TraegerRecipientPage is one recipient with its own newest handovers.
type TraegerRecipientPage struct {
	Recipient TraegerRecipient       `json:"recipient"`
	Handovers []TraegerTransferEntry `json:"handovers"`
}

// TraegerTransfers is the Träger scope of Export Transfer. Every method
// opens its own Träger transaction of organizationID, or joins the one of
// the same Träger, and refuses to run inside a school or administrative one.
type TraegerTransfers interface {
	// Recipients answers every recipient, the assignments and at most
	// journalLimit handovers.
	Recipients(ctx context.Context, organizationID int64, journalLimit int) (TraegerRecipientsPage, error)
	// Recipient answers one recipient and at most journalLimit of its
	// handovers; another Träger's answers ErrRecipientNotFound.
	Recipient(ctx context.Context, organizationID, recipientID int64, journalLimit int) (TraegerRecipientPage, error)
	CreateRecipient(ctx context.Context, organizationID, actorAccountID int64, input TraegerRecipientInput) (TraegerRecipient, error)
	SaveRecipient(ctx context.Context, organizationID, actorAccountID, recipientID int64, input TraegerRecipientInput) (TraegerRecipient, error)
	// DeleteRecipient removes a recipient and its sealed password; its
	// formats go back to the download. It answers the page without it.
	DeleteRecipient(ctx context.Context, organizationID, recipientID int64) (TraegerRecipientsPage, error)
	// AssignFormats sends each named format to its recipient, or to the
	// download; formats it does not name keep theirs.
	AssignFormats(ctx context.Context, organizationID, actorAccountID int64, assignments []TraegerAssignment) (TraegerRecipientsPage, error)
	// Transfer hands one file to the recipient of its format. A format
	// without a ready recipient answers ReasonNotConfigured without
	// preparing a file.
	Transfer(ctx context.Context, request TraegerTransferRequest) (Outcome, error)
}

// RekeyResult counts one batch of the bank-data rekey (ADR 0183).
type RekeyResult struct {
	Scanned int
	Changed int
	LastID  int64
}

// RecipientRekey re-seals the recipient passwords of a Träger on the active
// bank key. Only the maintenance command holds it; it runs in the Träger
// transaction the command opens.
type RecipientRekey interface {
	Batch(ctx context.Context, organizationID, cursor int64, batchSize int) (RekeyResult, error)
}

// BankDataCipher seals and opens the recipient password with the bank ring
// (ADR 0183, ADR 0218 section 3). The marker keeps a health-only cipher
// from satisfying it; the root binds Security Runtime's bank cipher.
type BankDataCipher interface {
	BankDataRing()
	Seal(plaintext, context []byte) ([]byte, error)
	Open(sealed, context []byte) ([]byte, error)
}

// BankDataRewrapper re-seals a value on the active bank key; only the
// maintenance command binds it.
type BankDataRewrapper interface {
	Rewrap(sealed, aad []byte) ([]byte, error)
}
