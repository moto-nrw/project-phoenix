// Package exporttransfer is the public Export Transfer capability: sending an
// already-produced export file to the school's configured counterpart (#3050).
//
// The capability does not choose export content. Callers supply finished bytes
// or their existing file producer, so transfer and download use the same
// Zeitwirtschafts-/DATEV export implementation. A producer runs with its access
// audit in the command transaction before the transfer intent commits.
//
// It is a client only; moto runs no SFTP server. There is exactly one
// counterpart per school, and no listing, download, delete or rename of
// anything already there: none of those are needed, and each would be another
// way to misuse a school's credentials.
package exporttransfer

import (
	"context"
	"errors"
)

// ErrUnknownSettlement marks an attempt whose remote outcome may have been
// decided, but whose journal outcome could not be persisted. The recovery
// worker terminalizes its already committed fence without retrying the remote
// operation.
var ErrUnknownSettlement = errors.New("export transfer settlement is unknown")

// Export kinds. One value per export surface that may be transferred, so the
// journal says WHICH export left the school, not just that a file did.
const (
	KindStaffTimeTracking = "staff_time_tracking"
)

// Outcome reasons. Stable codes, never transport error prose: they are read
// back from the journal years later and mapped to German sentences in the UI.
// A raw SSH error would carry paths, versions and a server's own wording.
const (
	ReasonNotConfigured = "not_configured"
	ReasonAddressDenied = "address_denied"
	ReasonHostKey       = "host_key_mismatch"
	ReasonAuth          = "authentication_rejected"
	ReasonConnect       = "connection_failed"
	ReasonUpload        = "upload_failed"
	ReasonTooLarge      = "file_too_large"
	ReasonInternal      = "internal_error"
	// ReasonMailFailed: the mail with the download link was not accepted
	// after the delivery retries (#4805).
	ReasonMailFailed = "mail_failed"
)

// ErrTransferUnavailable marks a transfer that could not even be attempted
// for a reason that is not the school's configuration — a settings store that
// will not answer, for example. It is NOT the same as "not configured".
var ErrTransferUnavailable = errors.New("export transfer unavailable")

// Request is one transfer: which export, which file, on whose behalf.
type Request struct {
	// Kind and Format describe the export for the journal.
	Kind   string
	Format string
	// Period names the exported month as YYYY-MM, empty when the export
	// has none (#4805).
	Period string
	// Filename and Data are the finished export, byte for byte as the
	// download serves it.
	Filename string
	Data     []byte
	// PrepareFile builds the file and records its access audit in the command
	// transaction, before destination resolution. When set, it supplies Filename
	// and Data; transfer acceptance still requires the synchronous transport.
	PrepareFile func(context.Context) (filename string, data []byte, err error)
	// ActorAccountID is 0 for a system caller; ActorName is snapshotted into
	// the journal so the entry still reads after a rename or deletion.
	ActorAccountID int64
	ActorName      string
}

// Outcome is the result of one attempt.
//
// A failed attempt is a normal result — a counterpart can be down — so it is
// a value, not an error. Callers must render Success, never merely the
// absence of an error.
type Outcome struct {
	// Transferred says whether the file arrived under its final name.
	//
	// Deliberately NOT called "success": the frontend's route wrapper treats
	// any payload carrying a "success" field as an already-formed API
	// envelope and passes it through without the usual "data" wrapper. A
	// domain field of that name silently changes the response shape.
	Transferred bool   `json:"transferred"`
	Filename    string `json:"filename"`
	ByteSize    int64  `json:"byte_size"`
	// TargetHost and TargetDirectory name the destination, so whoever pressed
	// the button sees where the payroll data went.
	TargetHost      string `json:"target_host,omitempty"`
	TargetDirectory string `json:"target_directory,omitempty"`
	// Reason is one of the codes above, empty on success.
	Reason string `json:"reason,omitempty"`
}

// Status is the credential-free view of the school's configuration. It has no
// password field at all — not a masked one.
type Status struct {
	// Enabled mirrors the on/off switch alone; Ready also requires every
	// value to be present and well-formed.
	Enabled bool   `json:"enabled"`
	Ready   bool   `json:"ready"`
	Host    string `json:"host,omitempty"`
	Port    int    `json:"port,omitempty"`
	// RemoteDirectory names where files land.
	RemoteDirectory string `json:"remote_directory,omitempty"`
	// MissingSettings names the setting keys still to fill, in form order.
	MissingSettings []string `json:"missing_settings,omitempty"`
}

// EmailRecord is one settled e-mail send of an export (#4805). The mail
// carries a download link, never the file; the producing domain delivers it
// and records the result here, so the one transfer journal lists every send.
type EmailRecord struct {
	Kind      string
	Format    string
	Period    string
	Filename  string
	ByteSize  int64
	Recipient string
	// Delivered says whether the mail server accepted the mail. A failed
	// send names ReasonMailFailed.
	Delivered      bool
	Reason         string
	ActorAccountID int64
	ActorName      string
}

// Capability is the Export Transfer capability compose.New returns.
type Capability interface {
	// Transfer sends the export and records the attempt.
	Transfer(context.Context, Request) (Outcome, error)
	// Status reports whether a transfer can be offered, and to where.
	Status(context.Context) (Status, error)
	// RecordEmail writes a settled e-mail send into the journal, in the
	// caller's tenant transaction.
	RecordEmail(context.Context, EmailRecord) error
}
