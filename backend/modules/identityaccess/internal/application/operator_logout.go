package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// LogoutOperator revokes the operator refresh-token family the presented
// refresh token belongs to (#5194). The operator routes check that family
// per request, so its access tokens end at once, also in a cookie a late
// response writes back. Other families of the operator stay valid. An
// unknown token is a successful logout; a token of another operator is
// refused.
func (s *OperatorAuthentication) LogoutOperator(ctx context.Context, operatorID int64, refreshToken string) error {
	if s == nil {
		return errOperatorAuthenticationUnavailable
	}
	ctx = bindRuntime(s.attach, ctx)
	return operatorError(s.runtime.WithAdminTx(ctx, func(txCtx context.Context) error {
		session, err := s.operators.findOperatorSessionForUpdate(txCtx, refreshToken)
		if errors.Is(err, identityaccess.ErrOperatorSessionNotFound) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("failed to find operator refresh token: %w", err)
		}
		if session.OperatorID != operatorID {
			return identityaccess.ErrOperatorRefreshTokenInvalid
		}
		if err := s.revokeFamilyWithAudit(txCtx, session, "logout"); err != nil {
			return fmt.Errorf("failed to revoke operator refresh-token family: %w", err)
		}
		// A rotation that locked the family's newest row before this delete
		// commits its successor once the delete stops waiting; the statement
		// above ran on an older snapshot and missed it. A second statement
		// sees it. A rotation that starts later finds its row gone.
		if _, err := s.operators.RevokeOperatorSessionFamily(txCtx, session.FamilyID); err != nil {
			return fmt.Errorf("failed to revoke operator refresh-token successor: %w", err)
		}
		return nil
	}))
}
