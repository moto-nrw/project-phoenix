package auth

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// SessionTokenValidator verifies frontend handoffs without consuming refresh recovery state.
type SessionTokenValidator interface {
	ValidateSessionTokens(context.Context, string, string, string) (*jwt.AppClaims, error)
}

func sessionScopeMatches(scope, portal string) bool {
	switch portal {
	case "tenant":
		return scope == "" || scope == "tenant" || scope == "org"
	case "platform", "parent", "school":
		return scope == portal
	default:
		return false
	}
}

func (s *Service) ValidateSessionTokens(ctx context.Context, access, refresh, portal string) (*jwt.AppClaims, error) {
	claims, err := s.tokenAuth.ParseAccessJWT(access)
	if err != nil || claims.ID <= 0 || claims.ExpiresAt <= time.Now().Unix() || !sessionScopeMatches(claims.Scope, portal) {
		return nil, ErrInvalidToken
	}
	if (portal == "tenant" || portal == "school") && claims.TenantID <= 0 {
		return nil, ErrInvalidToken
	}
	var refreshClaims *jwt.RefreshClaims
	if refresh != "" {
		refreshClaims, err = s.parseRefreshTokenClaims(refresh)
		if err != nil || refreshClaims.ExpiresAt <= time.Now().Unix() || refreshClaims.ID != claims.ID || refreshClaims.Scope != claims.Scope || refreshClaims.TenantID != claims.TenantID || claims.ReadOnly || claims.FamilyID == "" {
			return nil, ErrInvalidToken
		}
	}
	err = tenant.WithAdminTx(s.withTenantRuntime(ctx), s.db, func(ctx context.Context, _ bun.Tx) error {
		if portal == "platform" {
			return s.validateOperatorSession(ctx, claims, refreshClaims)
		}
		account, err := s.repos.Account.FindByID(ctx, int64(claims.ID))
		if err != nil || account == nil || !account.Active {
			return ErrInvalidToken
		}
		if claims.TenantID > 0 {
			active, err := s.repos.AccountTenant.ExistsActiveByAccountAndTenantForShare(ctx, account.ID, claims.TenantID)
			if err != nil || !active {
				return ErrInvalidToken
			}
		}
		if refreshClaims == nil {
			return nil
		}
		row, err := s.repos.Token.FindByToken(ctx, refreshClaims.Token)
		if err != nil || row == nil || row.AccountID != account.ID || (claims.TenantID > 0 && row.TenantID != claims.TenantID) || row.PortalScope != persistedPortalScope(claims.Scope) || !row.Expiry.After(time.Now()) || row.RotatedAt != nil || row.ReplacementToken != nil || row.FamilyID != claims.FamilyID {
			return ErrInvalidToken
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return claims, nil
}

func (s *Service) validateOperatorSession(ctx context.Context, claims *jwt.AppClaims, refreshClaims *jwt.RefreshClaims) error {
	account, err := s.repos.Operator.FindByID(ctx, int64(claims.ID))
	if err != nil || account == nil || !account.Active {
		return ErrInvalidToken
	}
	if refreshClaims == nil {
		return nil
	}
	row, err := s.repos.OperatorRefreshToken.FindByTokenForUpdate(ctx, refreshClaims.Token)
	if err != nil || row == nil || row.OperatorID != account.ID || !row.Expiry.After(time.Now()) || row.RotatedAt != nil || row.ReplacementToken != nil || row.FamilyID != claims.FamilyID {
		return ErrInvalidToken
	}
	return nil
}
