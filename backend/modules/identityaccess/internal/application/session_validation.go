package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// ValidateSessionTokens verifies a frontend session hand-off (access token
// plus optional refresh token for one portal) without consuming any refresh
// recovery state. Every mismatch is ErrInvalidToken.
func (s *AccountAuthentication) ValidateSessionTokens(ctx context.Context, access, refresh, portal string) (_ identityaccess.SessionClaims, err error) {
	if s == nil {
		return identityaccess.SessionClaims{}, errAccountAuthenticationUnavailable
	}
	defer func() { err = authenticationError(err) }()
	ctx = bindRuntime(s.attach, ctx)
	claims, err := s.codec.ParseAccessToken(access)
	now := time.Now().Unix()
	if err != nil || claims.AccountID <= 0 || claims.ExpiresAt <= now || !domain.SessionScopeMatches(claims.Scope, portal) {
		return identityaccess.SessionClaims{}, identityaccess.ErrInvalidToken
	}
	if (portal == "tenant" || portal == "school") && claims.TenantID <= 0 {
		return identityaccess.SessionClaims{}, identityaccess.ErrInvalidToken
	}
	refreshClaims, err := s.parseSessionRefresh(refresh, claims, now)
	if err != nil {
		return identityaccess.SessionClaims{}, err
	}
	err = s.runtime.WithAdminTx(ctx, func(txCtx context.Context) error {
		return s.validateSessionInTx(txCtx, claims, refreshClaims, portal)
	})
	if err != nil {
		return identityaccess.SessionClaims{}, err
	}
	return identityaccess.SessionClaims(claims), nil
}

func (s *AccountAuthentication) validateOperatorSession(ctx context.Context, claims domain.SessionClaims, refreshClaims *domain.RefreshClaims) error {
	operator, err := s.sessions.findOperatorByID(ctx, claims.AccountID)
	if err != nil || !operator.Active {
		return identityaccess.ErrInvalidToken
	}
	if refreshClaims == nil {
		return nil
	}
	session, err := s.sessions.findOperatorSessionForUpdate(ctx, refreshClaims.Token)
	if err != nil || session.OperatorID != operator.ID || !session.Expiry.After(time.Now()) || session.RotatedAt != nil || session.ReplacementTokenHash != nil || session.FamilyID != claims.FamilyID {
		return identityaccess.ErrInvalidToken
	}
	return nil
}

func (s *AccountAuthentication) parseSessionRefresh(refresh string, claims domain.SessionClaims, now int64) (*domain.RefreshClaims, error) {
	var refreshClaims *domain.RefreshClaims
	if refresh != "" {
		parsed, err := s.codec.ParseRefreshToken(refresh)
		if err != nil || parsed.ExpiresAt <= now || parsed.AccountID != claims.AccountID || parsed.Scope != claims.Scope || parsed.TenantID != claims.TenantID || claims.ReadOnly || claims.FamilyID == "" {
			return nil, identityaccess.ErrInvalidToken
		}
		refreshClaims = &parsed
	}
	return refreshClaims, nil
}

func (s *AccountAuthentication) validateSessionInTx(txCtx context.Context, claims domain.SessionClaims, refreshClaims *domain.RefreshClaims, portal string) error {
	if portal == "platform" {
		return s.validateOperatorSession(txCtx, claims, refreshClaims)
	}
	account, found, _, err := findLoginAccount(txCtx, s.runtime, s.store, claims.AccountID, false)
	if err != nil || !found || !account.Active {
		return identityaccess.ErrInvalidToken
	}
	if claims.TenantID > 0 {
		active, _, err := s.store.LockActiveTenantMappingShared(txCtx, account.ID, claims.TenantID)
		if err != nil || !active {
			return identityaccess.ErrInvalidToken
		}
	}
	if refreshClaims == nil {
		return nil
	}
	session, err := s.sessions.findAccountSessionByToken(txCtx, refreshClaims.Token)
	if err != nil || session.AccountID != account.ID || (claims.TenantID > 0 && session.TenantID != claims.TenantID) || session.PortalScope != domain.PersistedPortalScope(claims.Scope) || !session.Expiry.After(time.Now()) || session.RotatedAt != nil || session.ReplacementTokenHash != nil || session.FamilyID != claims.FamilyID {
		return identityaccess.ErrInvalidToken
	}
	return nil
}
