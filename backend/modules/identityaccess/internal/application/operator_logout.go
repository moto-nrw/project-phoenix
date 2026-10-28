package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// maxLogoutPasses bounds the revocation passes of an operator logout.
const maxLogoutPasses = 4

// LogoutOperator revokes the operator refresh-token family the logged-in
// access token names (#5194). The operator routes check that family per
// request (RequiresActiveOperator), so its access tokens end at once, also in
// a cookie a late response writes back; other families stay valid. The route
// runs behind that check, so the family is the operator's own and live.
//
// Each pass deletes the family in its own transaction. A rotation that
// locked the newest row before a pass commits its successor once that pass
// stops waiting; the pass ran on an older snapshot and missed it, the next
// one sees it. Success requires a pass that finds nothing left. Exhausting
// the bound without that confirmation fails so the browser can retry.
func (s *OperatorAuthentication) LogoutOperator(ctx context.Context, operatorID int64, familyID string) error {
	if s == nil {
		return errOperatorAuthenticationUnavailable
	}
	if operatorID <= 0 || familyID == "" {
		return identityaccess.ErrOperatorRefreshTokenInvalid
	}
	ctx = bindRuntime(s.attach, ctx)
	revoked := 0
	confirmed := false
	for pass := 0; pass < maxLogoutPasses; pass++ {
		deleted, err := s.operators.RevokeOperatorSessionFamily(ctx, familyID)
		if err != nil {
			return operatorError(fmt.Errorf("failed to revoke operator refresh-token family: %w", err))
		}
		if len(deleted) == 0 {
			confirmed = true
			break
		}
		revoked += len(deleted)
	}
	if revoked > 0 {
		if err := s.auditRevocation(ctx, operatorID, familyID, "logout", revoked); err != nil {
			return operatorError(err)
		}
	}
	if !confirmed {
		return fmt.Errorf("operator session family revocation unconfirmed after %d passes", maxLogoutPasses)
	}
	return nil
}
