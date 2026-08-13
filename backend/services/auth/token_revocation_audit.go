package auth

import (
	"context"
	"fmt"
	"sort"

	"github.com/moto-nrw/project-phoenix/auth/rotation"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

const internalRevocationAuditIP = "0.0.0.0"

type revocationGroup struct {
	accountID int64
	tenantID  int64
	scope     string
	familyID  string
	count     int
}

func persistedPortalScope(jwtScope string) string {
	switch jwtScope {
	case "", authModels.PortalScopeTenant:
		return authModels.PortalScopeTenant
	case authModels.PortalScopeOrg, authModels.PortalScopeParent, authModels.PortalScopeSchool:
		return jwtScope
	default:
		return authModels.PortalScopeUnknown
	}
}

func (s *Service) auditRevokedTokens(ctx context.Context, tokens []*authModels.Token, reason, ipAddress, userAgent string) error {
	if len(tokens) == 0 {
		return nil
	}
	if ipAddress == "" {
		ipAddress = internalRevocationAuditIP
	}
	groups := make(map[string]*revocationGroup)
	for _, token := range tokens {
		familyID := token.FamilyID
		if familyID == "" {
			familyID = "legacy:" + token.Token
		}
		key := fmt.Sprintf("%d:%d:%s:%s", token.AccountID, token.TenantID, token.PortalScope, familyID)
		group := groups[key]
		if group == nil {
			scope := token.PortalScope
			if scope == "" {
				scope = authModels.PortalScopeUnknown
			}
			group = &revocationGroup{accountID: token.AccountID, tenantID: token.TenantID, scope: scope, familyID: familyID}
			groups[key] = group
		}
		group.count++
	}
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		group := groups[key]
		event := auditModels.NewAuthEvent(group.accountID, auditModels.EventTypeTokenRevoked, true, ipAddress)
		event.SetTenantID(group.tenantID)
		event.UserAgent = userAgent
		event.SetMetadata("portal_scope", group.scope)
		event.SetMetadata("family_fingerprint", rotation.FamilyFingerprint(group.familyID))
		event.SetMetadata("reason", reason)
		event.SetMetadata("revoked_token_count", group.count)
		if err := s.repos.AuthEvent.Create(ctx, event); err != nil {
			return fmt.Errorf("audit token revocation: %w", err)
		}
	}
	return nil
}

func (s *Service) deleteAccountTokensWithAudit(ctx context.Context, accountID int64, reason, ipAddress, userAgent string) ([]*authModels.Token, error) {
	tokens, err := s.repos.Token.DeleteByAccountIDReturning(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if err := s.auditRevokedTokens(ctx, tokens, reason, ipAddress, userAgent); err != nil {
		return nil, err
	}
	if err := s.deleteStaffPushAcrossTenants(ctx, accountID); err != nil {
		return nil, err
	}
	return tokens, nil
}

func (s *Service) deleteFamilyWithAudit(ctx context.Context, token *authModels.Token, reason, ipAddress, userAgent string) error {
	if token.FamilyID == "" {
		if err := s.repos.Token.Delete(ctx, token.ID); err != nil {
			return err
		}
		return s.auditRevokedTokens(ctx, []*authModels.Token{token}, reason, ipAddress, userAgent)
	}
	tokens, err := s.repos.Token.DeleteByFamilyIDReturning(ctx, token.FamilyID)
	if err != nil {
		return err
	}
	if err := s.auditRevokedTokens(ctx, tokens, reason, ipAddress, userAgent); err != nil {
		return err
	}
	return s.deleteStaffPushForFamily(ctx, token.AccountID, token.FamilyID)
}

// deleteStaffPushAcrossTenants removes every staff push row for the account.
// Tenant-scoped callers get a dedicated admin transaction so RLS cannot hide
// other schools. An ambient admin transaction is reused.
func (s *Service) deleteStaffPushAcrossTenants(ctx context.Context, accountID int64) error {
	return s.withStaffPushAdminTx(ctx, func(adminCtx context.Context) error {
		return s.repos.PushSubscription.DeleteStaffByAccountID(adminCtx, accountID)
	})
}

func (s *Service) deleteStaffPushForFamily(ctx context.Context, accountID int64, familyID string) error {
	if familyID == "" {
		return nil
	}
	return s.withStaffPushAdminTx(ctx, func(adminCtx context.Context) error {
		return s.repos.PushSubscription.DeleteStaffByTokenFamilyID(adminCtx, accountID, familyID)
	})
}

func (s *Service) withStaffPushAdminTx(ctx context.Context, fn func(context.Context) error) error {
	if s.repos.PushSubscription == nil {
		return nil
	}
	if s.db == nil {
		return fn(ctx)
	}
	if tenant.FromContext(ctx) == 0 {
		if _, ok := modelBase.TxFromContext(ctx); ok {
			return fn(ctx)
		}
	}
	return tenant.WithAdminTx(modelBase.ContextWithoutTx(ctx), s.db, func(adminCtx context.Context, _ bun.Tx) error {
		return fn(adminCtx)
	})
}
