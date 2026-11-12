package parentbilling

import (
	"context"
	"errors"
	"time"
)

// Handover of a finance export to the recipient of the Träger (#6824, ADR
// 0217). The Elternabrechnung makes the file and keeps the protocol; Export
// Transfer, bound by the root to FinanceExportTransfer, keeps the recipient
// and the journal and carries the file; the recipient has its own routes at
// /traeger/export-recipient.

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
	// ErrExportHandoverFormat reports a format that cannot be handed over.
	ErrExportHandoverFormat = errors.New("this export format cannot be handed over")
)

// ExportFileHandover is the handover of a protocol entry.
type ExportFileHandover struct {
	State           string     `json:"state"`
	Reason          string     `json:"reason,omitempty"`
	TargetHost      string     `json:"target_host"`
	TargetDirectory string     `json:"target_directory"`
	StartedAt       time.Time  `json:"started_at"`
	SettledAt       *time.Time `json:"settled_at,omitempty"`
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
// Elternabrechnung hands a file over. The root binds it; it opens its own
// Träger transactions of the request's Träger.
type FinanceExportTransfer interface {
	Handover(ctx context.Context, request FinanceHandoverRequest) (FinanceHandoverOutcome, error)
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
