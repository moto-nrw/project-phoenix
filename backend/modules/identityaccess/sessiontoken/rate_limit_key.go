package sessiontoken

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/tokenprimitives"
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
//     own budget. Different portal scopes ("", "org", "platform", "parent")
//     are separate budgets too — scope must be in the key because operator
//     IDs (platform.operators) and account IDs (auth.accounts) are
//     different ID spaces.
//   - Requests without a verified identity — missing, expired, or
//     manipulated JWTs, non-JWT bearer values such as IoT device API keys,
//     and MFA challenge/enrollment tokens (no "id" claim) — return "" and
//     the limiter falls back to the trusted client IP. Unverified bearer
//     values must never produce token-derived buckets: an attacker could
//     mint arbitrary values and sidestep IP limiting entirely.
func (a *TokenAuth) RateLimitKey(r *http.Request) string {
	tokenString := strings.TrimSpace(extractBearerToken(r.Header.Get("Authorization")))
	if tokenString == "" || a == nil || a.JwtAuth == nil {
		return ""
	}

	token, err := a.JwtAuth.Decode(tokenString)
	if err != nil {
		return ""
	}
	identity, ok := tokenprimitives.RateLimitIdentity(token)
	if !ok {
		return ""
	}

	return fmt.Sprintf("acct:%d:%s:%d", identity.AccountID, identity.Scope, identity.TenantID)
}
