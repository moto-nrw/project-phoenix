package users

import "time"

// Consent states as the staff and parent portals render them. They are the
// shared vocabulary of the projection People Directory folds together, kept
// here so every retained consumer names the same state by the same word.
const (
	StudentConsentStateGranted     = "granted"
	StudentConsentStateWithdrawn   = "withdrawn"
	StudentConsentStateNotRecorded = "not_recorded"
)

// StudentConsentState is the current staff/parent-facing state of one consent
// or required acknowledgement. ChangedAt is the grant/withdrawal time when
// known. CanWithdraw and CanGrant are decided by the calling portal's
// permission check and the recorded state.
type StudentConsentState struct {
	Key         string
	State       string
	ChangedAt   *time.Time
	CanWithdraw bool
	CanGrant    bool
}
