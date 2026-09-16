package application

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

type revocationGroup struct {
	accountID int64
	tenantID  int64
	scope     string
	familyID  string
	count     int
}

// auditRevokedSessions records one token_revoked event per (account, tenant,
// portal scope, family) group of the revoked sessions, on the caller's
// transaction.
func (s *AccountAuthentication) auditRevokedSessions(ctx context.Context, sessions []domain.AccountSession, reason, ipAddress, userAgent string) error {
	if len(sessions) == 0 {
		return nil
	}
	if ipAddress == "" {
		ipAddress = domain.InternalRevocationAuditIP
	}
	groups := make(map[string]*revocationGroup)
	for _, session := range sessions {
		familyID := session.FamilyID
		if familyID == "" {
			familyID = "legacy:" + session.Token
		}
		key := fmt.Sprintf("%d:%d:%s:%s", session.AccountID, session.TenantID, session.PortalScope, familyID)
		group := groups[key]
		if group == nil {
			scope := session.PortalScope
			if scope == "" {
				scope = domain.PortalScopeUnknown
			}
			group = &revocationGroup{accountID: session.AccountID, tenantID: session.TenantID, scope: scope, familyID: familyID}
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
		event := domain.AuthEvent{
			AccountID: group.accountID, TenantID: group.tenantID, Type: domain.AuthEventTokenRevoked, Success: true,
			IPAddress: ipAddress, UserAgent: userAgent,
			RevokedSessions: &domain.RevokedSessionsEvidence{
				PortalScope: group.scope, FamilyFingerprint: s.rotation.FamilyFingerprint(group.familyID), Reason: reason, Count: group.count,
			},
		}
		if err := s.audit.RecordAuthEvent(ctx, event); err != nil {
			return fmt.Errorf("audit token revocation: %w", err)
		}
	}
	return nil
}

// deleteFamilyWithAudit revokes the session's family (or the single legacy
// session without one) and records the evidence on the same transaction.
func (s *AccountAuthentication) deleteFamilyWithAudit(ctx context.Context, session domain.AccountSession, reason, ipAddress, userAgent string) error {
	if session.FamilyID == "" {
		if err := s.sessions.DeleteAccountSession(ctx, session.ID); err != nil {
			return err
		}
		return s.auditRevokedSessions(ctx, []domain.AccountSession{session}, reason, ipAddress, userAgent)
	}
	revoked, err := s.sessions.RevokeAccountSessionFamily(ctx, session.FamilyID)
	if err != nil {
		return err
	}
	return s.auditRevokedSessions(ctx, revoked, reason, ipAddress, userAgent)
}

// DeleteAccountSessionsWithAudit revokes the account's sessions at the
// caller's school with audit evidence, or schedules the account-wide wipe an
// account-wide reason asks for. Tenant-scoped callers pair it with
// QueuePushCleanup after their transaction.
func (s *AccountAuthentication) DeleteAccountSessionsWithAudit(ctx context.Context, accountID int64, reason, ipAddress, userAgent string) ([]domain.AccountSession, error) {
	if domain.IsAccountWideRevocation(reason) {
		return nil, s.ScheduleAccountWideRevoke(ctx, accountID, reason, ipAddress, userAgent)
	}
	revoked, err := s.sessions.RevokeAccountSessionsInTenant(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if err := s.auditRevokedSessions(ctx, revoked, reason, ipAddress, userAgent); err != nil {
		return nil, err
	}
	return revoked, nil
}

// ScheduleAccountWideRevoke wipes every session of the account at every
// school. Inside an administrative transaction (or a tenantless ambient
// one) the wipe runs immediately; inside a request transaction it is
// recorded as pending and completed after commit; a plain tenant
// transaction without an after-commit queue leaves the wipe to the caller;
// without any transaction it runs independently.
func (s *AccountAuthentication) ScheduleAccountWideRevoke(ctx context.Context, accountID int64, reason, ipAddress, userAgent string) error {
	if s.runtime.IsAdminTx(ctx) || (s.runtime.TenantID(ctx) == 0 && s.runtime.HasTransaction(ctx)) {
		skip, err := s.shouldSkipAccountWideWipe(ctx, accountID, reason)
		if err != nil {
			return err
		}
		if skip {
			if s.runtime.IsAdminTx(ctx) {
				return s.MarkAccountWideWipeCompleted(ctx, accountID)
			}
			return nil
		}
		if _, err := s.deleteAllAccountSessionsInCtx(ctx, accountID, reason, ipAddress, userAgent); err != nil {
			return err
		}
		s.queuePushCleanup(ctx, accountID, nil, reason)
		if s.runtime.IsAdminTx(ctx) {
			return s.MarkAccountWideWipeCompleted(ctx, accountID)
		}
		return nil
	}
	if s.runtime.HasAfterCommitHooks(ctx) {
		if err := s.recordPendingAccountWideWipe(ctx, accountID, reason); err != nil {
			return err
		}
		pendingRecorded := s.runtime.TenantID(ctx) > 0
		s.runtime.RegisterAfterCommit(ctx, func() {
			if err := s.finishScheduledAccountWideWipe(ctx, accountID, reason, ipAddress, userAgent, pendingRecorded); err != nil {
				s.logger.Warn("failed to revoke remaining sessions after commit",
					slog.Int64("account_id", accountID),
					slog.String("reason", reason),
					slog.Any("error", err))
			}
		})
		return nil
	}
	if s.runtime.HasTransaction(ctx) {
		// A plain tenant transaction has no after-commit queue and nesting
		// an administrative transaction deadlocks; the caller revokes after
		// its transaction commits.
		return nil
	}
	return s.wipeAccountWideIndependently(ctx, accountID, reason, ipAddress, userAgent)
}

func (s *AccountAuthentication) recordPendingAccountWideWipe(ctx context.Context, accountID int64, reason string) error {
	tenantID := s.runtime.TenantID(ctx)
	if tenantID <= 0 {
		return nil
	}
	return s.audit.RecordAuthEvent(ctx, domain.AuthEvent{
		AccountID: accountID, TenantID: tenantID, Type: domain.AuthEventTokenRevoked, Success: true,
		IPAddress: domain.InternalRevocationAuditIP, PendingWipe: &domain.PendingWipeEvidence{Reason: reason},
	})
}

// QueuePushCleanup removes the push subscriptions the revoked sessions
// leave behind: after commit when the caller's transaction has an
// after-commit queue, immediately without a transaction, and not at all
// inside a plain transaction whose rollback would otherwise lose them.
func (s *AccountAuthentication) QueuePushCleanup(ctx context.Context, accountID int64, sessions []domain.AccountSession, reason string) {
	s.queuePushCleanup(ctx, accountID, sessions, reason)
}

func (s *AccountAuthentication) queuePushCleanup(ctx context.Context, accountID int64, sessions []domain.AccountSession, reason string) {
	run := func() {
		if err := s.cleanupPushAfterRevocation(s.runtime.Detach(ctx), accountID, sessions, reason); err != nil {
			s.logger.Warn("failed to delete push subscriptions after token revocation",
				slog.Int64("account_id", accountID),
				slog.String("reason", reason),
				slog.Any("error", err))
		}
	}
	if s.runtime.HasAfterCommitHooks(ctx) {
		s.runtime.RegisterAfterCommit(ctx, run)
		return
	}
	if s.runtime.HasTransaction(ctx) {
		return
	}
	run()
}

func (s *AccountAuthentication) wipeAccountWideIndependently(ctx context.Context, accountID int64, reason, ipAddress, userAgent string) error {
	if s.runtime.IsAdminTx(ctx) {
		skip, err := s.shouldSkipAccountWideWipe(ctx, accountID, reason)
		if err != nil {
			return err
		}
		if skip {
			return s.MarkAccountWideWipeCompleted(ctx, accountID)
		}
		sessions, err := s.deleteAllAccountSessionsInCtx(ctx, accountID, reason, ipAddress, userAgent)
		if err != nil {
			return err
		}
		if err := s.cleanupPushAfterRevocation(ctx, accountID, sessions, reason); err != nil {
			return err
		}
		return s.MarkAccountWideWipeCompleted(ctx, accountID)
	}
	adminCtx := s.runtime.Detach(ctx)
	var sessions []domain.AccountSession
	err := s.runtime.WithAdminTx(adminCtx, func(txCtx context.Context) error {
		skip, innerErr := s.shouldSkipAccountWideWipe(txCtx, accountID, reason)
		if innerErr != nil {
			return innerErr
		}
		if skip {
			return s.MarkAccountWideWipeCompleted(txCtx, accountID)
		}
		sessions, innerErr = s.deleteAllAccountSessionsInCtx(txCtx, accountID, reason, ipAddress, userAgent)
		if innerErr != nil {
			return innerErr
		}
		return s.MarkAccountWideWipeCompleted(txCtx, accountID)
	})
	if err != nil {
		return err
	}
	return s.cleanupPushAfterRevocation(adminCtx, accountID, sessions, reason)
}

// finishScheduledAccountWideWipe completes a wipe recorded before commit:
// claims the pending rows, revokes the sessions that existed at the newest
// pending cutoff (a login that started afterwards survives) and records the
// completion.
func (s *AccountAuthentication) finishScheduledAccountWideWipe(ctx context.Context, accountID int64, reason, ipAddress, userAgent string, pendingRecorded bool) error {
	var sessions []domain.AccountSession
	pushReason := reason
	run := func(txCtx context.Context) error {
		claimed, err := s.audit.ClaimPendingAccountWideWipes(txCtx, accountID)
		if err != nil {
			return err
		}
		if len(claimed) == 0 && pendingRecorded {
			// Reactivation or another worker already claimed the row.
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
			return s.completeAccountWideWipes(txCtx, claimed)
		}
		if !cutoff.IsZero() {
			if reason != "account_deactivated" {
				if _, found, _, lockErr := s.store.FindLoginAccount(txCtx, accountID, true); lockErr != nil {
					return lockErr
				} else if !found {
					return domain.ErrAccountNotFound
				}
			}
			sessions, err = s.sessions.RevokeAccountSessionsCreatedAtOrBefore(txCtx, accountID, cutoff)
			if err != nil {
				return err
			}
			if err := s.auditRevokedSessions(txCtx, sessions, reason, ipAddress, userAgent); err != nil {
				return err
			}
			newer, newerErr := s.sessions.HasLiveAccountSessionsCreatedAfter(txCtx, accountID, cutoff)
			if newerErr != nil {
				return newerErr
			}
			if newer {
				pushReason = "pending_wipe"
			}
			return s.completeAccountWideWipes(txCtx, claimed)
		}
		sessions, err = s.deleteAllAccountSessionsInCtx(txCtx, accountID, reason, ipAddress, userAgent)
		if err != nil {
			return err
		}
		return s.completeAccountWideWipes(txCtx, claimed)
	}
	if s.runtime.IsAdminTx(ctx) {
		if err := run(ctx); err != nil {
			return err
		}
		return s.cleanupPushAfterRevocation(ctx, accountID, sessions, pushReason)
	}
	adminCtx := s.runtime.Detach(ctx)
	if err := s.runtime.WithAdminTx(adminCtx, run); err != nil {
		return err
	}
	return s.cleanupPushAfterRevocation(adminCtx, accountID, sessions, pushReason)
}

func pendingWipeReason(claimed []domain.PendingAccountWideWipe, fallback string) string {
	reason := fallback
	for _, wipe := range claimed {
		if domain.IsAccountWideRevocation(wipe.Reason) {
			reason = wipe.Reason
		}
	}
	if !domain.IsAccountWideRevocation(reason) {
		return "administrative_revoke"
	}
	return reason
}

func pendingWipeCutoff(claimed []domain.PendingAccountWideWipe) time.Time {
	var cutoff time.Time
	for _, wipe := range claimed {
		if cutoff.IsZero() || wipe.CreatedAt.After(cutoff) {
			cutoff = wipe.CreatedAt
		}
	}
	return cutoff
}

// shouldSkipAccountWideWipe locks the account and skips a deactivation
// wipe when the account is active again.
func (s *AccountAuthentication) shouldSkipAccountWideWipe(ctx context.Context, accountID int64, reason string) (bool, error) {
	if reason != "account_deactivated" {
		return false, nil
	}
	account, found, _, err := s.store.FindLoginAccount(ctx, accountID, true)
	if err != nil {
		return false, err
	}
	if !found {
		return false, domain.ErrAccountNotFound
	}
	return account.Active, nil
}

// MarkAccountWideWipeCompleted claims the account's pending wipes on the
// caller's transaction and records their completion.
func (s *AccountAuthentication) MarkAccountWideWipeCompleted(ctx context.Context, accountID int64) error {
	pending, err := s.audit.ClaimPendingAccountWideWipes(ctx, accountID)
	if err != nil {
		return err
	}
	return s.completeAccountWideWipes(ctx, pending)
}

func (s *AccountAuthentication) completeAccountWideWipes(ctx context.Context, pending []domain.PendingAccountWideWipe) error {
	for _, wipe := range pending {
		event := domain.AuthEvent{
			AccountID: wipe.AccountID, TenantID: wipe.TenantID, Type: domain.AuthEventAccountWideWipeCompleted, Success: true,
			IPAddress: domain.InternalRevocationAuditIP, CompletedWipe: &domain.CompletedWipeEvidence{PendingEventID: wipe.EventID},
		}
		if err := s.audit.RecordAuthEvent(ctx, event); err != nil {
			return fmt.Errorf("audit account-wide wipe completion: %w", err)
		}
	}
	return nil
}

func (s *AccountAuthentication) deleteAllAccountSessionsInCtx(ctx context.Context, accountID int64, reason, ipAddress, userAgent string) ([]domain.AccountSession, error) {
	sessions, err := s.sessions.RevokeAllAccountSessions(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if err := s.auditRevokedSessions(ctx, sessions, reason, ipAddress, userAgent); err != nil {
		return nil, err
	}
	return sessions, nil
}

// RevokeAllTokensWithReason revokes every session of the account: an
// account-wide reason schedules the wipe, any other reason deletes the
// sessions at the caller's school inside its transaction.
func (s *AccountAuthentication) RevokeAllTokensWithReason(ctx context.Context, accountID int64, reason string) error {
	if domain.IsAccountWideRevocation(reason) {
		if err := s.ScheduleAccountWideRevoke(ctx, accountID, reason, "", ""); err != nil {
			return failed("revoke all tokens", err)
		}
		return nil
	}
	var revoked []domain.AccountSession
	err := s.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		sessions, txErr := s.DeleteAccountSessionsWithAudit(txCtx, accountID, reason, "", "")
		if txErr != nil {
			return txErr
		}
		revoked = sessions
		return nil
	})
	if err != nil {
		return failed("revoke all tokens", err)
	}
	s.queuePushCleanup(ctx, accountID, revoked, reason)
	return nil
}

// RevokeTokensByTenantID deletes every refresh session of a school, so a
// soft delete cuts session refresh for all of its users.
func (s *AccountAuthentication) RevokeTokensByTenantID(ctx context.Context, tenantID int64) (int, error) {
	count := 0
	err := s.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		deleted, err := s.sessions.RevokeTenantAccountSessions(txCtx, tenantID)
		if err != nil {
			return err
		}
		if err := s.auditRevokedSessions(txCtx, deleted, "tenant_deleted", "", ""); err != nil {
			return err
		}
		count = len(deleted)
		return nil
	})
	if err != nil {
		return 0, failed("revoke tokens by tenant", err)
	}
	return count, nil
}

// cleanupPushAfterRevocation removes the push subscriptions the revocation
// orphaned: every portal's rows for an account-wide reason, otherwise the
// rows bound to the revoked families plus the unbound rows at the schools
// the sessions belonged to.
func (s *AccountAuthentication) cleanupPushAfterRevocation(ctx context.Context, accountID int64, sessions []domain.AccountSession, reason string) error {
	if domain.IsAccountWideRevocation(reason) {
		return s.deletePushAcrossTenants(ctx, accountID)
	}
	if err := s.deletePushForFamilies(ctx, accountID, domain.SessionFamilyIDs(sessions)); err != nil {
		return err
	}
	return s.deletePushUnboundForSessions(ctx, accountID, sessions)
}

func (s *AccountAuthentication) deletePushAcrossTenants(ctx context.Context, accountID int64) error {
	return s.withPushAdminTx(ctx, func(adminCtx context.Context) error {
		if err := s.push.DeleteStaffByAccount(adminCtx, accountID); err != nil {
			return err
		}
		if err := s.push.DeleteSchoolByAccount(adminCtx, accountID); err != nil {
			return err
		}
		return s.push.DeleteParentByAccount(adminCtx, accountID)
	})
}

func (s *AccountAuthentication) deletePushForFamilies(ctx context.Context, accountID int64, familyIDs []string) error {
	for _, familyID := range familyIDs {
		if familyID == "" {
			continue
		}
		if err := s.withPushAdminTx(ctx, func(adminCtx context.Context) error {
			return s.push.DeleteByTokenFamily(adminCtx, accountID, familyID)
		}); err != nil {
			return err
		}
	}
	return nil
}

// Push portals per persisted portal scope: tenant and org sessions belong to
// the staff portal; unknown legacy rows may belong to any portal.
const (
	pushPortalStaff  = "staff"
	pushPortalParent = "parent"
	pushPortalSchool = "school"
)

func pushPortalsForScope(portalScope string) []string {
	switch portalScope {
	case domain.PortalScopeParent:
		return []string{pushPortalParent}
	case domain.PortalScopeSchool:
		return []string{pushPortalSchool}
	case domain.PortalScopeUnknown, "":
		return []string{pushPortalStaff, pushPortalParent, pushPortalSchool}
	default:
		return []string{pushPortalStaff}
	}
}

func sessionMatchesPushPortal(portalScope, portal string) bool {
	for _, candidate := range pushPortalsForScope(portalScope) {
		if candidate == portal {
			return true
		}
	}
	return false
}

func sessionTenantIDsForPortal(sessions []domain.AccountSession, portal string) []int64 {
	seen := make(map[int64]struct{}, len(sessions))
	ids := make([]int64, 0, len(sessions))
	for _, session := range sessions {
		if session.TenantID <= 0 {
			continue
		}
		if portal != "" && !sessionMatchesPushPortal(session.PortalScope, portal) {
			continue
		}
		if _, ok := seen[session.TenantID]; ok {
			continue
		}
		seen[session.TenantID] = struct{}{}
		ids = append(ids, session.TenantID)
	}
	return ids
}

func (s *AccountAuthentication) deletePushUnboundForSessions(ctx context.Context, accountID int64, sessions []domain.AccountSession) error {
	for _, portal := range []string{pushPortalStaff, pushPortalSchool, pushPortalParent} {
		if err := s.deletePushUnboundAtTenants(ctx, accountID, sessionTenantIDsForPortal(sessions, portal), portal); err != nil {
			return err
		}
	}
	return nil
}

func (s *AccountAuthentication) deletePushUnboundAtTenants(ctx context.Context, accountID int64, tenantIDs []int64, portal string) error {
	for _, tenantID := range tenantIDs {
		if tenantID <= 0 {
			continue
		}
		if err := s.withPushAdminTx(ctx, func(adminCtx context.Context) error {
			return s.push.DeleteUnboundByAccount(adminCtx, accountID, tenantID, portal)
		}); err != nil {
			return err
		}
	}
	return nil
}

// withPushAdminTx reuses an ambient transaction and otherwise opens an
// administrative one so RLS cannot hide other schools' rows.
func (s *AccountAuthentication) withPushAdminTx(ctx context.Context, fn func(context.Context) error) error {
	if s.runtime.HasTransaction(ctx) {
		return fn(ctx)
	}
	return s.runtime.WithAdminTx(s.runtime.WithoutTransaction(ctx), fn)
}
