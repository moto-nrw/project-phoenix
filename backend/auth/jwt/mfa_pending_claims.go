package jwt

import (
	"errors"
	"fmt"
	"time"
)

// mfaPendingClaimsSpec parameterizes parseMFAPendingClaims for the two
// deliberately parallel pending-MFA token types (challenge / enrollment).
// All error strings are the historical per-type texts — they surface in
// 401 bodies and must stay verbatim.
type mfaPendingClaimsSpec struct {
	foreignFlagKey string // the OTHER type's pending flag (cross-reject)
	foreignFlagErr string
	scopeTenant    string
	scopePlatform  string
	pendingFlagKey string
	notPendingErr  string
}

// parseMFAPendingClaims is the shared body of the challenge/enrollment
// ParseClaims implementations.
//
// Defense-in-depth (#1430 review item #8): a pending-MFA token MUST NOT
// also carry the other type's pending flag. Rejecting the foreign flag up
// front means a malformed JWT can never satisfy both the /auth/mfa/verify
// and /auth/mfa/enroll/* middlewares.
func parseMFAPendingClaims(claims map[string]any, spec mfaPendingClaimsSpec, common *CommonClaims) (accountID, tenantID int64, scope string, err error) {
	if getOptionalBool(claims, spec.foreignFlagKey) {
		return 0, 0, "", errors.New(spec.foreignFlagErr)
	}

	accountID = getOptionalInt64(claims, "account_id")
	if accountID == 0 {
		return 0, 0, "", errors.New("missing required claim: account_id")
	}

	scope = getOptionalString(claims, "scope")
	if scope != spec.scopeTenant && scope != spec.scopePlatform {
		return 0, 0, "", fmt.Errorf("invalid scope claim: %q", scope)
	}

	tenantID = getOptionalInt64(claims, "tenant_id")
	if !getOptionalBool(claims, spec.pendingFlagKey) {
		return 0, 0, "", errors.New(spec.notPendingErr)
	}

	if exp, ok := claims["exp"]; ok {
		switch v := exp.(type) {
		case float64:
			common.ExpiresAt = int64(v)
		case int64:
			common.ExpiresAt = v
		case time.Time:
			common.ExpiresAt = v.Unix()
		}
	}
	return accountID, tenantID, scope, nil
}
