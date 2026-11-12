package parentbilling

import (
	"context"
	"errors"
	"time"
)

// Handover of a finance export to the recipient of the Träger (#6824, ADR
// 0217). The Elternabrechnung makes the file and keeps the protocol; Export
// Transfer, bound by the root to FinanceExportTransfer, keeps the recipient
// and the journal and carries the file.

// Handover states of a file in the protocol.
const (
	HandoverInProgress  = "in_progress"
	HandoverTransferred = "transferred"
	HandoverFailed      = "failed"
	// HandoverUnknown: the file may have arrived; the outcome was never
	// recorded. A new handover of the same content needs Repeat.
	HandoverUnknown = "unknown"
)

var (
	// ErrExportRecipientNotReady reports a handover without a complete,
	// switched-on recipient; no file and no number were made.
	ErrExportRecipientNotReady = errors.New("the recipient of the finance exports is not ready")
	// ErrExportAlreadyHandedOver reports a handover of a content that
	// arrived, is under way or whose outcome is unknown; Repeat sends it again.
	ErrExportAlreadyHandedOver = errors.New("the same file was already handed over")
	// ErrExportRecipientInvalid reports a recipient the Träger cannot save;
	// ExportRecipientError names the fields.
	ErrExportRecipientInvalid = errors.New("the recipient of the finance exports is invalid")
	// ErrExportHandoverFormat reports a format that cannot be handed over.
	ErrExportHandoverFormat = errors.New("this export format cannot be handed over")
)

// ExportRecipientError lists the invalid fields of a recipient.
type ExportRecipientError struct{ Fields []string }

func (e *ExportRecipientError) Error() string { return ErrExportRecipientInvalid.Error() }

// Unwrap lets errors.Is find ErrExportRecipientInvalid.
func (e *ExportRecipientError) Unwrap() error { return ErrExportRecipientInvalid }

// ExportFileHandover is the handover of a protocol entry.
type ExportFileHandover struct {
	State           string     `json:"state"`
	Reason          string     `json:"reason,omitempty"`
	TargetHost      string     `json:"target_host"`
	TargetDirectory string     `json:"target_directory"`
	StartedAt       time.Time  `json:"started_at"`
	SettledAt       *time.Time `json:"settled_at,omitempty"`
}

// ExportRecipient is the recipient of the finance exports as the page
// shows it. It carries no password, only whether one is stored.
type ExportRecipient struct {
	Configured         bool       `json:"configured"`
	Enabled            bool       `json:"enabled"`
	Ready              bool       `json:"ready"`
	Host               string     `json:"host"`
	Port               int        `json:"port"`
	Username           string     `json:"username"`
	RemoteDirectory    string     `json:"remote_directory"`
	HostKeyFingerprint string     `json:"host_key_fingerprint"`
	PasswordSet        bool       `json:"password_set"`
	MissingFields      []string   `json:"missing_fields"`
	UpdatedAt          *time.Time `json:"updated_at,omitempty"`
}

// ExportRecipientInput is what the Träger saves; a nil Password keeps the
// stored one, as long as address, port, user and fingerprint stay.
type ExportRecipientInput struct {
	Enabled            bool
	Host               string
	Port               int
	Username           string
	Password           *string
	RemoteDirectory    string
	HostKeyFingerprint string
}

// ExportHandoverEntry is one attempt in the journal of the recipient.
type ExportHandoverEntry struct {
	ID              int64     `json:"id"`
	CreatedAt       time.Time `json:"created_at"`
	Format          string    `json:"format"`
	FileName        string    `json:"file_name"`
	ByteSize        int64     `json:"byte_size"`
	TargetHost      string    `json:"target_host"`
	TargetDirectory string    `json:"target_directory"`
	State           string    `json:"state"`
	Reason          string    `json:"reason,omitempty"`
}

// ExportRecipientPage is the recipient with its newest handovers.
type ExportRecipientPage struct {
	Recipient ExportRecipient       `json:"recipient"`
	Handovers []ExportHandoverEntry `json:"handovers"`
}

// FinanceHandoverRequest is one handover as the transport sees it.
type FinanceHandoverRequest struct {
	OrganizationID int64
	AccountID      int64
	Format         string
	// Prepare makes and records the file in the Träger transaction of the
	// transfer intent, knowing where it goes.
	Prepare func(ctx context.Context, host, directory string) (fileName string, data []byte, err error)
	// Settle records the outcome in the Träger transaction that completes
	// the transport's journal.
	Settle func(ctx context.Context, transferred bool, reason string) error
}

// FinanceHandoverOutcome is the result of the transport.
type FinanceHandoverOutcome struct {
	Transferred bool
	// NotReady: no recipient was ready, nothing was prepared.
	NotReady bool
	// Uncertain: sent or not, the outcome was not recorded.
	Uncertain       bool
	Reason          string
	TargetHost      string
	TargetDirectory string
}

// FinanceExportTransfer is Export Transfer's Träger scope as the
// Elternabrechnung uses it. The root binds it; every method opens its own
// Träger transaction of organizationID.
type FinanceExportTransfer interface {
	Handover(ctx context.Context, request FinanceHandoverRequest) (FinanceHandoverOutcome, error)
	ExportRecipient(ctx context.Context, organizationID int64) (ExportRecipient, error)
	SaveExportRecipient(ctx context.Context, organizationID, accountID int64, input ExportRecipientInput) (ExportRecipient, error)
	DeleteExportRecipient(ctx context.Context, organizationID int64) error
	ExportHandovers(ctx context.Context, organizationID int64, limit int) ([]ExportHandoverEntry, error)
}

// HandoverRequest names the file of a run to hand over.
type HandoverRequest struct {
	Format  string
	RunKind string
	RunID   int64
	// Datev picks the bookings of a DATEV file, as for its download.
	Datev DatevFileOptions
	// Repeat sends a content again that was already handed over.
	Repeat bool
}

// HandoverResult is what the person sees after a handover. A failed
// transport is a result, not an error: the recipient can be down.
type HandoverResult struct {
	FileName        string `json:"file_name"`
	State           string `json:"state"`
	Reason          string `json:"reason,omitempty"`
	TargetHost      string `json:"target_host"`
	TargetDirectory string `json:"target_directory"`
}

// FinanceExportHandovers hands a finance export of a released run to the
// recipient of the Träger. It must run outside a Träger transaction: the
// transport commits each step on its own.
type FinanceExportHandovers interface {
	HandOver(ctx context.Context, reviewer HolidayReviewer, request HandoverRequest) (HandoverResult, error)
}
