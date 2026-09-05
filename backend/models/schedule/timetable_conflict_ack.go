package schedule

import (
	"context"
	"errors"
	"regexp"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// conflictAckFingerprintPattern matches the hex fingerprints produced by the
// conflict detection (sha256 prefix, 32 hex chars today; the range tolerates
// future length changes without a migration).
var conflictAckFingerprintPattern = regexp.MustCompile(`^[a-f0-9]{16,64}$`)

// MaxConflictAcksPerAccount caps how many acknowledgements one account can
// hold per tenant. Fingerprints are opaque to the server (they cannot be
// verified against a concrete conflict without recomputing the caller's whole
// planning window), so without a cap the endpoint would let any SchedulesRead
// holder accumulate unbounded rows (#2151 review). Acknowledge prunes the
// oldest rows beyond the cap — stale acks stop matching anyway once the
// underlying conflict changes, so dropping the oldest first is lossless in
// practice. 500 is far above what a user can meaningfully review ("Alle
// ausblenden" on a busy month acks a few dozen).
const MaxConflictAcksPerAccount = 500

// TimetableConflictAck records that ONE user of ONE school has reviewed and
// dismissed ONE concrete planning conflict (#2139). The fingerprint is the
// conflict's stable identity (kind, person, involved instances, date, overlap
// window, relevant rooms — see conflictFingerprint in services/schedule); any
// substantive change to the conflict yields a new fingerprint, so stale acks
// simply stop matching and the warning resurfaces.
//
// This is per-user data state, NOT a tenant-wide setting: two admins of the
// same school each dismiss their own banner entries.
type TimetableConflictAck struct {
	Model `bun:"schema:schedule,table:timetable_conflict_acks"`
	TenantModel

	AccountID   int64  `bun:"account_id,notnull" json:"account_id"`
	Fingerprint string `bun:"fingerprint,notnull" json:"fingerprint"`
}

// Validate ensures the acknowledgement is well-formed.
func (a *TimetableConflictAck) Validate() error {
	if a.AccountID <= 0 {
		return errors.New("account_id is required")
	}
	if !ValidConflictAckFingerprint(a.Fingerprint) {
		return errors.New("fingerprint must be 16-64 lowercase hex characters")
	}
	return nil
}

// ValidConflictAckFingerprint reports whether the string is a plausible
// conflict fingerprint. Shared by model validation and the HTTP handler so
// both reject the same shapes.
func ValidConflictAckFingerprint(fingerprint string) bool {
	return conflictAckFingerprintPattern.MatchString(fingerprint)
}

// TimetableConflictAckRepository defines operations for per-user conflict
// acknowledgements. All methods are tenant-scoped through the usual RLS +
// WithTenantFilter path; the account scoping is an explicit parameter because
// acks are per-user data, not per-tenant data.
type TimetableConflictAckRepository interface {
	base.Repository[*TimetableConflictAck]

	// ListFingerprintsByAccount returns every fingerprint the account has
	// acknowledged in the current tenant.
	ListFingerprintsByAccount(ctx context.Context, accountID int64) ([]string, error)

	// Acknowledge idempotently records the fingerprint for the account and
	// prunes the account's oldest acknowledgements beyond
	// MaxConflictAcksPerAccount.
	Acknowledge(ctx context.Context, accountID int64, fingerprint string) error

	// Unacknowledge removes the fingerprint for the account. Removing an
	// unknown fingerprint is a no-op, not an error.
	Unacknowledge(ctx context.Context, accountID int64, fingerprint string) error
}
