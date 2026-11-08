package sessiontoken

import (
	"fmt"
	"net/http"
)

// RateLimitKey buckets an authenticated request by its stable quota
// identity — account ID, scope, and tenant ID from the verified JWT claims —
// instead of a per-token hash (#2064). Every valid session of the same
// identity shares one budget, so a re-login or token refresh no longer
// resets the quota. The key carries only numeric IDs and the scope label,
// never the raw token, email, or name.
//
// Quota-identity rules:
//   - Same account, same scope, same tenant → one shared budget across all
//     sessions (browser tabs, refreshed tokens, re-logins).
//   - A tenant switch mints a JWT with a different tenant_id and gets its
//     own budget. Different portal scopes are separate budgets too —
//     scope must be in the key because operator IDs (platform.operators)
//     and account IDs (auth.accounts) are different ID spaces.
//   - Requests without a verified identity — missing, expired, or
//     manipulated JWTs, non-JWT bearer values such as IoT device API keys,
//     and MFA challenge/enrollment tokens (no "id" claim) — return "" and
//     the limiter falls back to the trusted client IP. Unverified bearer
//     values must never produce token-derived buckets: an attacker could
//     mint arbitrary values and sidestep IP limiting entirely.
//
// It reads the token the Verifier already verified, so the limiter must run
// after it, and after ParseVerified to reuse its parse.
func RateLimitKey(r *http.Request) string {
	decoded, err := decodedFromCtx(r.Context())
	if err != nil || decoded.Expired {
		return ""
	}
	switch {
	case decoded.AccessErr == nil && decoded.access.ID > 0:
		return quotaKey(int64(decoded.access.ID), decoded.access.Scope, decoded.access.TenantID)
	case decoded.RefreshErr == nil && decoded.Refresh.AccountID > 0:
		return quotaKey(decoded.Refresh.AccountID, decoded.Refresh.Scope, decoded.Refresh.TenantID)
	default:
		return ""
	}
}

func quotaKey(accountID int64, scope string, tenantID int64) string {
	return fmt.Sprintf("acct:%d:%s:%d", accountID, scope, tenantID)
}
