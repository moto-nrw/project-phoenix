// Package ports declares what the Export Transfer capability needs from the
// outside world. Every dependency is an interface owned HERE, over domain
// values, so the transport and the settings store stay replaceable and the
// application layer imports neither.
package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/exporttransfer/internal/domain"
)

// TargetResolver reads the school's counterpart configuration.
type TargetResolver interface {
	// Resolve returns the complete target, or ErrNotConfigured. A resolution
	// failure must be returned as itself, never folded into ErrNotConfigured:
	// a settings store that cannot answer is a different situation from a
	// school that has not filled in the form.
	Resolve(context.Context) (domain.Target, error)
	State(context.Context) (domain.TargetState, error)
}

// Uploader atomically replaces the file after its dispatch fence commits. The
// implementation decides where a file may go — the address policy that keeps
// this from becoming a request-forgery tool lives with the transport, not here.
type Uploader interface {
	Prepare(ctx context.Context, target domain.Target, filename string, data []byte) (commit func() error, rollback func() error, err error)
}

// CommandRuntime commits the intent independently of an ambient HTTP transaction.
type CommandRuntime struct {
	Active       func(context.Context) bool
	Detached     func(context.Context) context.Context
	WithinTenant func(context.Context, func(context.Context) error) error
}

// ReasonedError is the transport's error vocabulary: a failure names WHY it
// happened as a stable code, so the application layer never has to recognize
// a transport's sentinel values by identity.
type ReasonedError interface {
	error
	// TransferReason returns one of the domain reason codes.
	TransferReason() string
}

// Journal holds one durable, one-shot transfer intent and its audit evidence.
// File bytes and credentials never enter this store: recovery only settles unknowns.
type Journal interface {
	Begin(context.Context, domain.JournalEntry) (int64, error)
	Claim(context.Context, int64) (string, error)
	Complete(context.Context, int64, string, domain.JournalEntry) error
	// Record writes an already settled send, such as a delivered e-mail,
	// as one terminal entry.
	Record(context.Context, domain.JournalEntry) error
}
