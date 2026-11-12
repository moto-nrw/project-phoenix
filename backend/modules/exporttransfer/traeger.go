package exporttransfer

import (
	"context"
	"errors"
	"time"
)

// The Träger scope of Export Transfer (#6824, ADR 0217): one recipient per
// Träger with its own sealed credentials, its own journal and its own Träger
// transactions. It is a separate capability, so the school scope above keeps
// its exact behaviour and neither scope can reach the other's target.

// KindParentBillingFinance is the journal kind of a finance export of the
// Elternabrechnung: Sollstellung, DATEV booking batch or PAIN.008.
const KindParentBillingFinance = "parent_billing_finance"

// Recipient field names, in form order. A validation error and an
// incomplete recipient name these.
const (
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

// RecipientError lists the invalid fields of a recipient, in form order.
type RecipientError struct{ Fields []string }

func (e *RecipientError) Error() string { return ErrRecipientInvalid.Error() }

// Unwrap lets errors.Is find ErrRecipientInvalid.
func (e *RecipientError) Unwrap() error { return ErrRecipientInvalid }

// TraegerRecipient is the credential-free view of the recipient of a
// Träger. It has no password field at all, only whether one is stored.
type TraegerRecipient struct {
	// Configured says whether the Träger saved a recipient at all.
	Configured bool `json:"configured"`
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
	MissingFields []string   `json:"missing_fields"`
	UpdatedAt     *time.Time `json:"updated_at,omitempty"`
}

// TraegerRecipientInput is what the Träger saves. A nil Password keeps the
// stored one; an empty one is refused, the recipient is removed instead.
type TraegerRecipientInput struct {
	Enabled            bool
	Host               string
	Port               int
	Username           string
	Password           *string
	RemoteDirectory    string
	HostKeyFingerprint string
}

// Destination names where a file goes, so the producing domain can record
// it with its export. It holds no credential.
type Destination struct {
	Host      string
	Directory string
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
	// State is transferred, failed, in_progress or unknown.
	State  string `json:"state"`
	Reason string `json:"reason,omitempty"`
}

// TraegerRecipientPage is the recipient of a Träger with its newest
// handovers, read in one Träger transaction.
type TraegerRecipientPage struct {
	Recipient TraegerRecipient       `json:"recipient"`
	Handovers []TraegerTransferEntry `json:"handovers"`
}

// TraegerTransfers is the Träger scope of Export Transfer. Every method
// opens its own Träger transaction of organizationID, or joins the one of
// the same Träger, and refuses to run inside a school or administrative one.
type TraegerTransfers interface {
	// Recipient answers the recipient and at most journalLimit handovers.
	Recipient(ctx context.Context, organizationID int64, journalLimit int) (TraegerRecipientPage, error)
	SaveRecipient(ctx context.Context, organizationID, actorAccountID int64, input TraegerRecipientInput) (TraegerRecipient, error)
	// DeleteRecipient removes the recipient and its sealed password and
	// answers the view without one.
	DeleteRecipient(ctx context.Context, organizationID int64) (TraegerRecipient, error)
	// Transfer hands one file over. A recipient that is not ready answers
	// ReasonNotConfigured without preparing a file.
	Transfer(ctx context.Context, request TraegerTransferRequest) (Outcome, error)
}

// RekeyResult counts one batch of the bank-data rekey (ADR 0183).
type RekeyResult struct {
	Scanned int
	Changed int
	LastID  int64
}

// RecipientRekey re-seals the recipient password of a Träger on the active
// bank key. Only the maintenance command holds it; it runs in the Träger
// transaction the command opens.
type RecipientRekey interface {
	Batch(ctx context.Context, organizationID, cursor int64, batchSize int) (RekeyResult, error)
}

// BankDataCipher seals and opens the recipient password with the bank ring
// (ADR 0183, ADR 0217 section 3). The marker keeps a health-only cipher
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
