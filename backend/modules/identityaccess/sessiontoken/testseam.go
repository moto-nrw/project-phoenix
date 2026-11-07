package sessiontoken

import "github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken/internal/testseam"

// The registration below is the only production link to
// sessiontoken/tokentest. It exports nothing: testseam is importable only
// inside the sessiontoken subtree (#5990).
func init() {
	testseam.ParseMFAChallenge = parseMFAChallenge
}

// parseMFAChallenge reads a challenge claim map.
//
// Defense-in-depth (symmetric to MFAEnrollmentClaims): a challenge token
// MUST NOT also carry mfa_enrollment_pending=true. Rejecting the foreign
// flag up front means a malformed JWT can't satisfy both /auth/mfa/verify
// and /auth/mfa/enroll/* middlewares. (#1430 review item #8)
func parseMFAChallenge(claims map[string]any) (testseam.MFAChallenge, error) {
	var common CommonClaims
	accountID, tenantID, scope, err := parseMFAPendingClaims(claims, mfaPendingClaimsSpec{
		foreignFlagKey: "mfa_enrollment_pending",
		foreignFlagErr: "token is a pending-MFA-enrollment token, not a challenge",
		scopeTenant:    MFAChallengeScopeTenant,
		scopePlatform:  MFAChallengeScopePlatform,
		scopeSchool:    MFAChallengeScopeSchool,
		scopeOrg:       MFAChallengeScopeOrg,
		pendingFlagKey: "mfa_pending",
		notPendingErr:  "token is not a pending-MFA challenge",
	}, &common)
	if err != nil {
		return testseam.MFAChallenge{}, err
	}
	return testseam.MFAChallenge{
		AccountID:   accountID,
		TenantID:    tenantID,
		ChallengeID: getOptionalInt64(claims, "challenge_id"),
		Scope:       scope,
		ExpiresAt:   common.ExpiresAt,
	}, nil
}
