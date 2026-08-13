package auth

import (
	"context"
	"fmt"
	"log/slog"
	"sort"

	"github.com/moto-nrw/project-phoenix/auth/rotation"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	iotModels "github.com/moto-nrw/project-phoenix/models/iot"
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

func isAccountWideRevocation(reason string) bool {
	switch reason {
	case "password_reset", "account_deactivated", "administrative_revoke":
		return true
	default:
		return false
	}
}

func tokenFamilyIDs(tokens []*authModels.Token) []string {
	seen := make(map[string]struct{}, len(tokens))
	ids := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if token == nil || token.FamilyID == "" {
			continue
		}
		if _, ok := seen[token.FamilyID]; ok {
			continue
		}
		seen[token.FamilyID] = struct{}{}
		ids = append(ids, token.FamilyID)
	}
	return ids
}

func pushPortalsForScope(portalScope string) []string {
	switch portalScope {
	case authModels.PortalScopeParent:
		return []string{iotModels.PushPortalParent}
	case authModels.PortalScopeUnknown, "":
		return []string{iotModels.PushPortalStaff, iotModels.PushPortalParent}
	default:
		return []string{iotModels.PushPortalStaff}
	}
}

func tokenMatchesPushPortal(portalScope, portal string) bool {
	for _, candidate := range pushPortalsForScope(portalScope) {
		if candidate == portal {
			return true
		}
	}
	return false
}

func tokenTenantIDsForPortal(tokens []*authModels.Token, portal string) []int64 {
	seen := make(map[int64]struct{}, len(tokens))
	ids := make([]int64, 0, len(tokens))
	for _, token := range tokens {
		if token == nil || token.TenantID <= 0 {
			continue
		}
		if portal != "" && !tokenMatchesPushPortal(token.PortalScope, portal) {
			continue
		}
		if _, ok := seen[token.TenantID]; ok {
			continue
		}
		seen[token.TenantID] = struct{}{}
		ids = append(ids, token.TenantID)
	}
	return ids
}

func (s *Service) deleteAccountTokensWithAudit(ctx context.Context, accountID int64, reason, ipAddress, userAgent string) ([]*authModels.Token, error) {
	if isAccountWideRevocation(reason) {
		return s.deleteAllAccountTokensWithAudit(ctx, accountID, reason, ipAddress, userAgent)
	}
	tokens, err := s.repos.Token.DeleteByAccountIDReturning(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if err := s.auditRevokedTokens(ctx, tokens, reason, ipAddress, userAgent); err != nil {
		return nil, err
	}
	return tokens, nil
}

func (s *Service) deleteAllAccountTokensWithAudit(ctx context.Context, accountID int64, reason, ipAddress, userAgent string) ([]*authModels.Token, error) {
	var tokens []*authModels.Token
	run := func(adminCtx context.Context) error {
		var err error
		tokens, err = s.repos.Token.DeleteByAccountIDReturning(adminCtx, accountID)
		if err != nil {
			return err
		}
		return s.auditRevokedTokens(adminCtx, tokens, reason, ipAddress, userAgent)
	}
	// Never nest a second transaction. Offboarding and tenant requests already
	// hold a connection (and often the account row). A nested AdminTx times
	// out on the 3-conn test pool. Cross-tenant wipe happens when we can
	// start a fresh admin transaction.
	if _, ok := modelBase.TxFromContext(ctx); ok {
		return tokens, run(ctx)
	}
	if s.db == nil {
		return tokens, run(ctx)
	}
	adminCtx := tenant.WithTenantID(modelBase.ContextWithoutTx(ctx), 0)
	err := tenant.WithAdminTx(adminCtx, s.db, func(txCtx context.Context, _ bun.Tx) error {
		return run(txCtx)
	})
	return tokens, err
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
	return s.auditRevokedTokens(ctx, tokens, reason, ipAddress, userAgent)
}

func (s *Service) cleanupPushAfterTokenRevocation(ctx context.Context, accountID int64, tokens []*authModels.Token, reason string) {
	var err error
	if isAccountWideRevocation(reason) {
		err = s.deletePushAcrossTenants(ctx, accountID)
	} else {
		err = s.deletePushForFamilies(ctx, accountID, tokenFamilyIDs(tokens))
		if err == nil {
			err = s.deletePushUnboundForTokens(ctx, accountID, tokens)
		}
	}
	if err != nil {
		s.getLogger().Warn("failed to delete push subscriptions after token revocation",
			slog.Int64("account_id", accountID),
			slog.String("reason", reason),
			slog.Any("error", err),
		)
	}
}

func (s *Service) cleanupPushAfterFamilyRevocation(ctx context.Context, token *authModels.Token) {
	if token == nil {
		return
	}
	s.cleanupPushAfterTokenRevocation(ctx, token.AccountID, []*authModels.Token{token}, "family")
}

// deletePushAcrossTenants removes every staff and parent push row for the account.
// Tenant-scoped callers get a dedicated admin transaction so RLS cannot hide
// other schools. An ambient admin transaction is reused.
func (s *Service) deletePushAcrossTenants(ctx context.Context, accountID int64) error {
	return s.withStaffPushAdminTx(ctx, func(adminCtx context.Context) error {
		if err := s.repos.PushSubscription.DeleteStaffByAccountID(adminCtx, accountID); err != nil {
			return err
		}
		return s.repos.PushSubscription.DeleteParentByAccountID(adminCtx, accountID)
	})
}

func (s *Service) deletePushForFamilies(ctx context.Context, accountID int64, familyIDs []string) error {
	for _, familyID := range familyIDs {
		if familyID == "" {
			continue
		}
		if err := s.withStaffPushAdminTx(ctx, func(adminCtx context.Context) error {
			return s.repos.PushSubscription.DeleteByTokenFamilyID(adminCtx, accountID, familyID)
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) deletePushUnboundForTokens(ctx context.Context, accountID int64, tokens []*authModels.Token) error {
	if err := s.deletePushUnboundAtTenants(ctx, accountID, tokenTenantIDsForPortal(tokens, iotModels.PushPortalStaff), iotModels.PushPortalStaff); err != nil {
		return err
	}
	return s.deletePushUnboundAtTenants(ctx, accountID, tokenTenantIDsForPortal(tokens, iotModels.PushPortalParent), iotModels.PushPortalParent)
}

func (s *Service) deletePushUnboundAtTenants(ctx context.Context, accountID int64, tenantIDs []int64, portal string) error {
	for _, tenantID := range tenantIDs {
		if tenantID <= 0 {
			continue
		}
		if err := s.withStaffPushAdminTx(ctx, func(adminCtx context.Context) error {
			if portal == iotModels.PushPortalParent {
				return s.repos.PushSubscription.DeleteParentUnboundByAccount(adminCtx, accountID, tenantID)
			}
			return s.repos.PushSubscription.DeleteStaffUnboundByAccount(adminCtx, accountID, tenantID)
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) withStaffPushAdminTx(ctx context.Context, fn func(context.Context) error) error {
	if s.repos.PushSubscription == nil {
		return nil
	}
	if s.db == nil {
		return fn(ctx)
	}
	// Reuse any ambient transaction. Opening a second AdminTx while the
	// caller still holds a connection deadlocks on the 3-conn test pool.
	if _, ok := modelBase.TxFromContext(ctx); ok {
		return fn(ctx)
	}
	return tenant.WithAdminTx(modelBase.ContextWithoutTx(ctx), s.db, func(adminCtx context.Context, _ bun.Tx) error {
		return fn(adminCtx)
	})
}
