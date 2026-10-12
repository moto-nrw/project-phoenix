package jwt

// MFA challenge scope values — distinguish tenant accounts from platform
// operators and school-portal (Lehrkraft) logins. The school scope (#2207)
// exists so a challenge started at /school/auth/login can only ever be
// redeemed for school-scope tokens: the tenant verify endpoint refuses it
// and vice versa.
const (
	MFAChallengeScopeTenant   = "tenant"
	MFAChallengeScopePlatform = "platform"
	MFAChallengeScopeSchool   = "school"
)

// MFAChallengeClaims represents a short-lived JWT issued after a successful
// password check on an account that requires a second factor. It is exchanged
// at /auth/mfa/verify (or the operator equivalent) for a regular access /
// refresh token pair. The token is intentionally narrow: it carries the
// account / operator identity needed to look up the challenge row and an
// `mfa_pending` flag so middleware never confuses it with an authenticated
// session token.
type MFAChallengeClaims struct {
	// AccountID is the auth.accounts.id (tenant scope) or
	// platform.operators.id (platform scope) of the user being challenged.
	AccountID int64 `json:"account_id"`

	// Scope distinguishes tenant from platform tokens — same conventions as
	// AppClaims.Scope, but constrained to the two MFAChallengeScope* values.
	Scope string `json:"scope,omitempty"`

	// TenantID is set on tenant-scope tokens so the verify handler can wrap
	// repository calls in the correct tenant tx. Zero on platform tokens.
	TenantID int64 `json:"tenant_id,omitempty"`

	// ChallengeID pins the token to the exact auth.mfa_email_challenges row
	// it was minted for. Without it the verify path had to fall back to
	// "newest active code for this account", which is ambiguous the moment
	// one account has two challenges in flight — a tenant login and a school
	// login, say — and let the code emailed for one portal be redeemed at the
	// other. Zero only on tokens minted before this claim existed.
	ChallengeID int64 `json:"challenge_id,omitempty"`

	// MFAPending must be true on every challenge token — middleware uses this
	// to reject challenge tokens at endpoints that expect a fully
	// authenticated session.
	MFAPending bool `json:"mfa_pending"`

	CommonClaims
}
