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

// Uploader moves the bytes. The implementation decides where a file may go —
// the address policy that keeps this from becoming a request-forgery tool
// lives with the transport, not here.
type Uploader interface {
	Upload(ctx context.Context, target domain.Target, filename string, data []byte) error
}

// ReasonedError is the transport's error vocabulary: a failure names WHY it
// happened as a stable code, so the application layer never has to recognize
// a transport's sentinel values by identity.
type ReasonedError interface {
	error
	// TransferReason returns one of the domain reason codes.
	TransferReason() string
}

// Journal appends the audit trail of transfer attempts.
type Journal interface {
	Record(context.Context, domain.JournalEntry) error
}
