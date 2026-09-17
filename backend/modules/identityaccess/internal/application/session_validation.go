package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// ValidateSessionTokens verifies a frontend session hand-off (access token
// plus optional refresh token for one portal) without consuming any refresh
// recovery state. Every mismatch is ErrInvalidToken.
func (s *AccountAuthentication) ValidateSessionTokens(ctx context.Context, access, refresh, portal string) (domain.SessionClaims, error) {
	claims, err := s.codec.ParseAccessToken(access)
	now := time.Now().Unix()
	if err != nil || claims.AccountID <= 0 || claims.ExpiresAt <= now || !domain.SessionScopeMatches(claims.Scope, portal) {
		return domain.SessionClaims{}, domain.ErrInvalidToken
	}
	if (portal == "tenant" || portal == "school") && claims.TenantID <= 0 {
		return domain.SessionClaims{}, domain.ErrInvalidToken
	}
	var refreshClaims *domain.RefreshClaims
	if refresh != "" {
		parsed, err := s.codec.ParseRefreshToken(refresh)
		if err != nil || parsed.ExpiresAt <= now || parsed.AccountID != claims.AccountID || parsed.Scope != claims.Scope || parsed.TenantID != claims.TenantID || claims.ReadOnly || claims.FamilyID == "" {
			return domain.SessionClaims{}, domain.ErrInvalidToken
		}
		refreshClaims = &parsed
	}
	err = s.runtime.WithAdminTx(ctx, func(txCtx context.Context) error {
		if portal == "platform" {
			return s.validateOperatorSession(txCtx, claims, refreshClaims)
		}
		account, found, _, err := s.store.FindLoginAccount(txCtx, claims.AccountID, false)
		if err != nil || !found || !account.Active {
			return domain.ErrInvalidToken
		}
		if claims.TenantID > 0 {
			active, _, err := s.store.LockActiveTenantMappingShared(txCtx, account.ID, claims.TenantID)
			if err != nil || !active {
				return domain.ErrInvalidToken
			}
		}
		if refreshClaims == nil {
			return nil
		}
		session, err := s.sessions.FindAccountSession(txCtx, refreshClaims.Token)
		if err != nil || session.AccountID != account.ID || (claims.TenantID > 0 && session.TenantID != claims.TenantID) || session.PortalScope != domain.PersistedPortalScope(claims.Scope) || !session.Expiry.After(time.Now()) || session.RotatedAt != nil || session.ReplacementToken != nil || session.FamilyID != claims.FamilyID {
			return domain.ErrInvalidToken
		}
		return nil
	})
	if err != nil {
		return domain.SessionClaims{}, err
	}
	return claims, nil
}

func (s *AccountAuthentication) validateOperatorSession(ctx context.Context, claims domain.SessionClaims, refreshClaims *domain.RefreshClaims) error {
	operator, err := s.sessions.FindOperator(ctx, claims.AccountID)
	if err != nil || !operator.Active {
		return domain.ErrInvalidToken
	}
	if refreshClaims == nil {
		return nil
	}
	session, err := s.sessions.FindOperatorSessionForUpdate(ctx, refreshClaims.Token)
	if err != nil || session.OperatorID != operator.ID || !session.Expiry.After(time.Now()) || session.RotatedAt != nil || session.ReplacementToken != nil || session.FamilyID != claims.FamilyID {
		return domain.ErrInvalidToken
	}
	return nil
}
