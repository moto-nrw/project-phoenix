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
// one sees it. The passes end when one finds nothing left.
func (s *OperatorAuthentication) LogoutOperator(ctx context.Context, operatorID int64, familyID string) error {
	if s == nil {
		return errOperatorAuthenticationUnavailable
	}
	if operatorID <= 0 || familyID == "" {
		return identityaccess.ErrOperatorRefreshTokenInvalid
	}
	ctx = bindRuntime(s.attach, ctx)
	revoked := 0
	for pass := 0; pass < maxLogoutPasses; pass++ {
		deleted, err := s.operators.RevokeOperatorSessionFamily(ctx, familyID)
		if err != nil {
			return operatorError(fmt.Errorf("failed to revoke operator refresh-token family: %w", err))
		}
		if len(deleted) == 0 {
			break
		}
		revoked += len(deleted)
	}
	if revoked == 0 {
		return nil
	}
	return operatorError(s.auditRevocation(ctx, operatorID, familyID, "logout", revoked))
}
