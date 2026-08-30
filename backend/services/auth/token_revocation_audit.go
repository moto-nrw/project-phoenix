package auth

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

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
	case authModels.PortalScopeSchool:
		return []string{iotModels.PushPortalSchool}
	case authModels.PortalScopeUnknown, "":
		return []string{iotModels.PushPortalStaff, iotModels.PushPortalParent, iotModels.PushPortalSchool}
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
		return nil, s.scheduleAccountWideRevoke(ctx, accountID, reason, ipAddress, userAgent)
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

func (s *Service) scheduleAccountWideRevoke(ctx context.Context, accountID int64, reason, ipAddress, userAgent string) error {
	if tenant.IsAdminTx(ctx) || (tenant.FromContext(ctx) == 0 && hasAmbientTx(ctx)) {
		skip, err := s.shouldSkipAccountWideWipe(ctx, accountID, reason)
		if err != nil {
			return err
		}
		if skip {
			if tenant.IsAdminTx(ctx) {
				return s.markAccountWideWipeCompleted(ctx, accountID)
			}
			return nil
		}
		if _, err := s.deleteAllAccountTokensInCtx(ctx, accountID, reason, ipAddress, userAgent); err != nil {
			return err
		}
		s.queuePushCleanup(ctx, accountID, nil, reason)
		if tenant.IsAdminTx(ctx) {
			return s.markAccountWideWipeCompleted(ctx, accountID)
		}
		return nil
	}
	if tenant.HasAfterCommitHooks(ctx) {
		if err := s.recordPendingAccountWideWipe(ctx, accountID, reason); err != nil {
			return err
		}
		pendingRecorded := tenant.FromContext(ctx) > 0
		tenant.RegisterAfterCommit(ctx, func() {
			if err := s.finishScheduledAccountWideWipe(ctx, accountID, reason, ipAddress, userAgent, pendingRecorded); err != nil {
				s.getLogger().Warn("failed to revoke remaining sessions after commit",
					slog.Int64("account_id", accountID),
					slog.String("reason", reason),
					slog.Any("error", err),
				)
			}
		})
		return nil
	}
	if hasAmbientTx(ctx) {
		// Plain tenant RunInTx has no after-commit queue. Nesting AdminTx
		// deadlocks. The caller must invoke revoke after that tx commits.
		return nil
	}
	return s.wipeAccountWideIndependently(ctx, accountID, reason, ipAddress, userAgent)
}

func (s *Service) recordPendingAccountWideWipe(ctx context.Context, accountID int64, reason string) error {
	if s.repos.AuthEvent == nil {
		return nil
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return nil
	}
	event := auditModels.NewAuthEvent(accountID, auditModels.EventTypeTokenRevoked, true, internalRevocationAuditIP)
	event.SetTenantID(tenantID)
	event.SetMetadata("reason", reason)
	event.SetMetadata("pending_account_wide_wipe", true)
	return s.repos.AuthEvent.Create(ctx, event)
}

func (s *Service) queuePushCleanup(ctx context.Context, accountID int64, tokens []*authModels.Token, reason string) {
	run := func() {
		if err := s.cleanupPushAfterTokenRevocation(s.independentCleanupCtx(ctx), accountID, tokens, reason); err != nil {
			s.getLogger().Warn("failed to delete push subscriptions after token revocation",
				slog.Int64("account_id", accountID),
				slog.String("reason", reason),
				slog.Any("error", err),
			)
		}
	}
	if tenant.HasAfterCommitHooks(ctx) {
		tenant.RegisterAfterCommit(ctx, run)
		return
	}
	if hasAmbientTx(ctx) {
		// Tokens are not committed yet. A separate admin tx would leave
		// push deleted if the caller rolls back.
		return
	}
	run()
}

func (s *Service) independentCleanupCtx(ctx context.Context) context.Context {
	return tenant.ContextWithoutAfterCommitHooks(tenant.ContextWithoutTenant(modelBase.ContextWithoutTx(ctx)))
}

func hasAmbientTx(ctx context.Context) bool {
	_, ok := modelBase.TxFromContext(ctx)
	return ok
}

func (s *Service) wipeAccountWideIndependently(ctx context.Context, accountID int64, reason, ipAddress, userAgent string) error {
	if s.db == nil || tenant.IsAdminTx(ctx) {
		skip, err := s.shouldSkipAccountWideWipe(ctx, accountID, reason)
		if err != nil {
			return err
		}
		if skip {
			return s.markAccountWideWipeCompleted(ctx, accountID)
		}
		tokens, err := s.deleteAllAccountTokensInCtx(ctx, accountID, reason, ipAddress, userAgent)
		if err != nil {
			return err
		}
		if err := s.cleanupPushAfterTokenRevocation(ctx, accountID, tokens, reason); err != nil {
			return err
		}
		return s.markAccountWideWipeCompleted(ctx, accountID)
	}
	adminCtx := tenant.ContextWithoutTenant(modelBase.ContextWithoutTx(ctx))
	adminCtx = tenant.ContextWithoutAfterCommitHooks(adminCtx)
	var tokens []*authModels.Token
	err := tenant.WithAdminTx(s.withTenantRuntime(adminCtx), s.db, func(txCtx context.Context, _ bun.Tx) error {
		skip, innerErr := s.shouldSkipAccountWideWipe(txCtx, accountID, reason)
		if innerErr != nil {
			return innerErr
		}
		if skip {
			return s.markAccountWideWipeCompleted(txCtx, accountID)
		}
		tokens, innerErr = s.deleteAllAccountTokensInCtx(txCtx, accountID, reason, ipAddress, userAgent)
		if innerErr != nil {
			return innerErr
		}
		return s.markAccountWideWipeCompleted(txCtx, accountID)
	})
	if err != nil {
		return err
	}
	return s.cleanupPushAfterTokenRevocation(adminCtx, accountID, tokens, reason)
}

func (s *Service) finishScheduledAccountWideWipe(ctx context.Context, accountID int64, reason, ipAddress, userAgent string, pendingRecorded bool) error {
	if s.db == nil {
		return s.wipeAccountWideIndependently(ctx, accountID, reason, ipAddress, userAgent)
	}
	var tokens []*authModels.Token
	pushReason := reason
	run := func(txCtx context.Context) error {
		var claimed []auditModels.PendingAccountWideWipe
		var err error
		if s.repos.AuthEvent != nil {
			claimed, err = s.repos.AuthEvent.ClaimPendingAccountWideWipes(txCtx, accountID)
			if err != nil {
				return err
			}
		}
		if len(claimed) == 0 && pendingRecorded {
			// Pending row was recorded and is gone: reactivation or another
			// worker already claimed it.
			return nil
		}
		cutoff := time.Time{}
		if len(claimed) > 0 {
			reason = pendingWipeReason(claimed, reason)
			cutoff = pendingWipeCutoff(claimed)
		}
		skip, err := s.shouldSkipAccountWideWipe(txCtx, accountID, reason)
		if err != nil {
			return err
		}
		if skip {
			return nil
		}
		if !cutoff.IsZero() {
			if s.repos.Account != nil && reason != "account_deactivated" {
				if _, lockErr := s.repos.Account.FindByIDForUpdate(txCtx, accountID); lockErr != nil {
					return lockErr
				}
			}
			tokens, err = s.repos.Token.DeleteByAccountIDCreatedAtOrBeforeReturning(txCtx, accountID, cutoff)
			if err != nil {
				return err
			}
			if err := s.auditRevokedTokens(txCtx, tokens, reason, ipAddress, userAgent); err != nil {
				return err
			}
			newer, newerErr := s.repos.Token.HasLiveTokensCreatedAfter(txCtx, accountID, cutoff)
			if newerErr != nil {
				return newerErr
			}
			if newer {
				pushReason = "pending_wipe"
			}
			return nil
		}
		tokens, err = s.deleteAllAccountTokensInCtx(txCtx, accountID, reason, ipAddress, userAgent)
		return err
	}
	if tenant.IsAdminTx(ctx) {
		if err := run(ctx); err != nil {
			return err
		}
		return s.cleanupPushAfterTokenRevocation(ctx, accountID, tokens, pushReason)
	}
	adminCtx := s.independentCleanupCtx(ctx)
	if err := tenant.WithAdminTx(s.withTenantRuntime(adminCtx), s.db, func(txCtx context.Context, _ bun.Tx) error {
		return run(txCtx)
	}); err != nil {
		return err
	}
	return s.cleanupPushAfterTokenRevocation(adminCtx, accountID, tokens, pushReason)
}

func pendingWipeReason(claimed []auditModels.PendingAccountWideWipe, fallback string) string {
	reason := fallback
	for _, wipe := range claimed {
		if isAccountWideRevocation(wipe.Reason) {
			reason = wipe.Reason
		}
	}
	if !isAccountWideRevocation(reason) {
		return "administrative_revoke"
	}
	return reason
}

func pendingWipeCutoff(claimed []auditModels.PendingAccountWideWipe) time.Time {
	var cutoff time.Time
	for _, wipe := range claimed {
		if cutoff.IsZero() || wipe.CreatedAt.After(cutoff) {
			cutoff = wipe.CreatedAt
		}
	}
	return cutoff
}

func (s *Service) shouldSkipAccountWideWipe(ctx context.Context, accountID int64, reason string) (bool, error) {
	if reason != "account_deactivated" || s.repos.Account == nil {
		return false, nil
	}
	account, err := s.repos.Account.FindByIDForUpdate(ctx, accountID)
	if err != nil {
		return false, err
	}
	return account.Active, nil
}

func (s *Service) markAccountWideWipeCompleted(ctx context.Context, accountID int64) error {
	if s.repos.AuthEvent == nil {
		return nil
	}
	return s.repos.AuthEvent.MarkAccountWideWipeCompleted(ctx, accountID)
}

func (s *Service) deleteAllAccountTokensInCtx(ctx context.Context, accountID int64, reason, ipAddress, userAgent string) ([]*authModels.Token, error) {
	tokens, err := s.repos.Token.DeleteAllByAccountIDReturning(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if err := s.auditRevokedTokens(ctx, tokens, reason, ipAddress, userAgent); err != nil {
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
	return s.auditRevokedTokens(ctx, tokens, reason, ipAddress, userAgent)
}

func (s *Service) cleanupPushAfterTokenRevocation(ctx context.Context, accountID int64, tokens []*authModels.Token, reason string) error {
	if isAccountWideRevocation(reason) {
		return s.deletePushAcrossTenants(ctx, accountID)
	}
	if err := s.deletePushForFamilies(ctx, accountID, tokenFamilyIDs(tokens)); err != nil {
		return err
	}
	return s.deletePushUnboundForTokens(ctx, accountID, tokens)
}

// deletePushAcrossTenants removes every staff and parent push row for the account.
// Tenant-scoped callers get a dedicated admin transaction so RLS cannot hide
// other schools. An ambient admin transaction is reused.
func (s *Service) deletePushAcrossTenants(ctx context.Context, accountID int64) error {
	return s.withStaffPushAdminTx(ctx, func(adminCtx context.Context) error {
		if err := s.repos.PushSubscription.DeleteStaffByAccountID(adminCtx, accountID); err != nil {
			return err
		}
		if err := s.repos.PushSubscription.DeleteSchoolByAccountID(adminCtx, accountID); err != nil {
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
	if err := s.deletePushUnboundAtTenants(ctx, accountID, tokenTenantIDsForPortal(tokens, iotModels.PushPortalSchool), iotModels.PushPortalSchool); err != nil {
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
			switch portal {
			case iotModels.PushPortalParent:
				return s.repos.PushSubscription.DeleteParentUnboundByAccount(adminCtx, accountID, tenantID)
			case iotModels.PushPortalSchool:
				return s.repos.PushSubscription.DeleteSchoolUnboundByAccount(adminCtx, accountID, tenantID)
			default:
				return s.repos.PushSubscription.DeleteStaffUnboundByAccount(adminCtx, accountID, tenantID)
			}
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
	return tenant.WithAdminTx(s.withTenantRuntime(modelBase.ContextWithoutTx(ctx)), s.db, func(adminCtx context.Context, _ bun.Tx) error {
		return fn(adminCtx)
	})
}
