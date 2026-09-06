// Package exporttransfer is the public Export Transfer capability: sending an
// already-produced export file to the school's configured counterpart (#3050).
//
// The capability deliberately does NOT produce files. It receives the finished
// bytes and a filename from whoever built them — today the Zeitwirtschafts-/
// DATEV export — so the transferred file cannot drift from the downloaded one:
// there is one file, used twice.
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
	// Filename and Data are the finished export, byte for byte as the
	// download serves it.
	Filename string
	Data     []byte
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

type engine interface {
	Transfer(context.Context, Request) (Outcome, error)
	Status(context.Context) (Status, error)
}

// Module is the capability handle.
type Module struct{ engine engine }

// NewModule is called by the compose package; other callers use compose.New.
func NewModule(engine engine) *Module { return &Module{engine: engine} }

// Transfer sends the export and records the attempt.
func (m *Module) Transfer(ctx context.Context, request Request) (Outcome, error) {
	return m.engine.Transfer(ctx, request)
}

// Status reports whether a transfer can be offered, and to where.
func (m *Module) Status(ctx context.Context) (Status, error) {
	return m.engine.Status(ctx)
}
