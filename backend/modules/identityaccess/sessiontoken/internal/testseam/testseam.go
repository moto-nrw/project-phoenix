// Package testseam hands the MFA challenge claim parser to
// sessiontoken/tokentest. Only the sessiontoken subtree can import it, and
// nothing in production calls it: production reads challenge tokens through
// the Identity & Access codec. It keeps sessiontoken identical with and
// without the testhooks build tag (#5990).
package testseam

// MFAChallenge is the parsed content of a challenge claim map.
type MFAChallenge struct {
	AccountID   int64
	TenantID    int64
	ChallengeID int64
	Scope       string
	ExpiresAt   int64
}

// ParseMFAChallenge validates a challenge claim map the way the legacy
// challenge parser did.
var ParseMFAChallenge func(claims map[string]any) (MFAChallenge, error)
