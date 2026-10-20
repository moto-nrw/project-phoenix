package timetable

import (
	"context"
	"errors"
	"regexp"
)

// ErrInvalidConflictAck rejects a non-positive account or a fingerprint that
// does not match the shape the conflict detection emits.
var ErrInvalidConflictAck = errors.New("invalid conflict acknowledgement")

// conflictAckFingerprintPattern matches the hex fingerprints produced by the
// conflict detection (sha256 prefix, 32 hex chars today; the range tolerates
// future length changes without a migration).
var conflictAckFingerprintPattern = regexp.MustCompile(`^[a-f0-9]{16,64}$`)

// ValidConflictAckFingerprint reports whether the string is a plausible
// conflict fingerprint. Shared by the module boundary and the HTTP handler so
// both reject the same shapes.
func ValidConflictAckFingerprint(fingerprint string) bool {
	return conflictAckFingerprintPattern.MatchString(fingerprint)
}

// ConflictAckQuery reads the per-user conflict acknowledgements (#2139). An
// acknowledgement hides ONE concrete, reviewed planning conflict for ONE
// account of ONE school; it is user view state, never a tenant-wide setting.
type ConflictAckQuery interface {
	ListConflictAcks(context.Context, int64) ([]string, error)
}

// ConflictAckCommand records and removes acknowledgements. Both operations
// are idempotent: re-acknowledging succeeds, and removing an unknown
// fingerprint is a no-op.
type ConflictAckCommand interface {
	AcknowledgeConflict(context.Context, int64, string) error
	UnacknowledgeConflict(context.Context, int64, string) error
}

type ConflictAckCapability interface {
	ConflictAckQuery
	ConflictAckCommand
}
