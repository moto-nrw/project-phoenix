package application

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// parseRefreshClaims parses and validates the refresh JWT.
func (s *AccountAuthentication) parseRefreshClaims(refreshToken string) (domain.RefreshClaims, error) {
	claims, err := s.codec.ParseRefreshToken(refreshToken)
	if err != nil {
		return domain.RefreshClaims{}, failed("parse refresh token", identityaccess.ErrInvalidToken)
	}
	return claims, nil
}

type refreshResult struct {
	accessToken  string
	refreshToken string
}

func refreshSingleflightKey(refreshToken string, proofHash []byte) string {
	return refreshToken + "\x00" + hex.EncodeToString(proofHash)
}

// RefreshTokenWithAudit rotates a refresh session and returns a new token
// pair. Concurrent calls with the same refresh token and recovery proof are
// deduplicated; the caller's context keeps cancelling the transaction so an
// interrupted rotation rolls back and the presented token stays valid.
func (s *AccountAuthentication) refreshTokenWithAudit(ctx context.Context, refreshToken, ipAddress, userAgent string) (string, string, error) {
	sfCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	// A refresh token alone must not join another caller's in-flight
	// recovery: the key binds the independent proof hash as well.
	proofHash := s.rotation.RecoveryProofHash(ctx)
	v, err, shared := s.refreshSF.Do(refreshSingleflightKey(refreshToken, proofHash), func() (any, error) {
		return s.doRefreshTokenWithAudit(sfCtx, refreshToken, ipAddress, userAgent)
	})
	if shared {
		s.logger.Info("concurrent_refresh_deduplicated", "shared", true)
	}
	if err != nil {
		return "", "", err
	}
	result := v.(*refreshResult)
	return result.accessToken, result.refreshToken, nil
}

func (s *AccountAuthentication) doRefreshTokenWithAudit(ctx context.Context, refreshToken, ipAddress, userAgent string) (*refreshResult, error) {
	claims, err := s.parseRefreshClaims(refreshToken)
	if err != nil {
		s.logRefreshDecision("refresh_session_rejected", "invalid_format", 0, 0)
		return nil, err
	}
	if claims.TenantID > 0 {
		if err := s.validateTenantAccess(ctx, claims); err != nil {
			return nil, err
		}
	}
	// A school token without a school is dead on arrival; refusing before
	// the rotation keeps the caller's refresh token intact.
	if claims.Scope == domain.ScopeSchool && claims.TenantID <= 0 {
		s.logRefreshDecision("refresh_session_rejected", "school_token_without_tenant", claims.AccountID, claims.TenantID)
		return nil, failed("refresh school session", identityaccess.ErrTenantAccessDenied)
	}
	// Every refresh carries its authorization and its claims into the
	// rotation transaction: either the rotation and the claims commit
	// together, or the transaction rolls back and the presented refresh
	// token is still the caller's.
	var (
		guard    mintGuard
		metadata *domain.AccountClaimsPayload
	)
	if claims.Scope == domain.ScopeSchool {
		guard = s.schoolRefreshMintGuard(claims.AccountID, claims.TenantID, &metadata)
	} else {
		guard = s.refreshClaimsGuard(claims.Scope, claims.TenantID, &metadata)
	}
	account, newSession, recovered, err := s.refreshSessionInTransaction(ctx, claims, ipAddress, userAgent, claims.TenantID, guard)
	if err != nil {
		return nil, err
	}
	if recovered {
		s.logger.Info("refresh_rotation_recovered",
			slog.Int64("account_id", claims.AccountID),
			slog.Int64("tenant_id", claims.TenantID),
			slog.Int("generation", newSession.Generation))
	}
	if metadata == nil {
		return nil, failed("refresh session", fmt.Errorf("refresh claims payload missing after rotation"))
	}
	access, refresh := buildClaims(account, newSession, metadata, account.Email)
	// The audit event is filed at the tenant the refresh validated against;
	// the route carries no tenant and the fallback would attribute a
	// multi-school account's refresh to its first mapping.
	auditCtx := ctx
	if claims.TenantID > 0 {
		auditCtx = s.runtime.WithTenantID(ctx, claims.TenantID)
	}
	accessToken, refreshTokenString, err := s.generateAndLogTokens(auditCtx, account.ID, access, refresh, ipAddress, userAgent, domain.AuthEventTokenRefresh)
	if err != nil {
		return nil, err
	}
	return &refreshResult{accessToken: accessToken, refreshToken: refreshTokenString}, nil
}

// refreshClaimsGuard assembles the claims of a non-school refresh (tenant and
// parent scopes) inside the rotation transaction. Parent-scope refresh tokens
// round-trip as parent tokens: detected via the scope claim or, for tokens
// that predate it, a guardian-only role set at the refresh tenant.
func (s *AccountAuthentication) refreshClaimsGuard(scope string, tenantID int64, out **domain.AccountClaimsPayload) mintGuard {
	return func(ctx context.Context, account domain.LoginAccount) error {
		if scope == domain.ScopeParent {
			*out = buildParentMetadata(account)
			return nil
		}
		guardianOnly, err := s.isGuardianOnlyAccountInTx(ctx, account.ID, tenantID)
		if err != nil {
			return err
		}
		if guardianOnly {
			*out = buildParentMetadata(account)
			return nil
		}
		// Refresh preserves the tenant of the presented token; the default
		// fallback could silently switch a multi-school account.
		metadata, err := s.loadAccountMetadataForTenantInTx(ctx, account, tenantID)
		if err != nil {
			return err
		}
		// A tenant refresh must not outlive the role change that turned the
		// account into a school-portal-only one (#2207).
		if domain.IsSchoolPortalOnly(metadata.Roles) {
			return failed("refresh token", identityaccess.ErrMustUseSchoolPortal)
		}
		*out = metadata
		return nil
	}
}

// refreshSessionInTransaction validates and rotates the session in one
// administrative transaction: the owning account is locked first (the same
// account -> session order login uses), the guard runs under that lock, then
// the presented session is locked, validated, and either recovered from a
// committed hand-off or rotated.
func (s *AccountAuthentication) refreshSessionInTransaction(ctx context.Context, claims domain.RefreshClaims, ipAddress, userAgent string, tenantID int64, guard mintGuard) (domain.LoginAccount, domain.AccountSession, bool, error) {
	rotation := refreshRotation{service: s, claims: claims, ipAddress: ipAddress, userAgent: userAgent, tenantID: tenantID, guard: guard}
	if err := s.runtime.WithAdminTx(ctx, rotation.run); err != nil {
		s.auditRejectedRefresh(ctx, claims.AccountID, ipAddress, userAgent, err)
		return domain.LoginAccount{}, domain.AccountSession{}, false, failed("refresh transaction", err)
	}
	if rotation.revokedForPush != nil {
		s.queuePushCleanup(ctx, rotation.revokedForPush.AccountID, []domain.AccountSession{*rotation.revokedForPush}, "family")
	}
	if rotation.rejectAfterCommit != nil {
		return domain.LoginAccount{}, domain.AccountSession{}, false, failed("refresh transaction", rotation.rejectAfterCommit)
	}
	return rotation.account, rotation.newSession, rotation.recovered, nil
}

// refreshRotation keeps the transaction's provisional result separate from a
// rejection whose family revocation must commit before it reaches the caller.
type refreshRotation struct {
	service           *AccountAuthentication
	claims            domain.RefreshClaims
	ipAddress         string
	userAgent         string
	tenantID          int64
	guard             mintGuard
	account           domain.LoginAccount
	newSession        domain.AccountSession
	recovered         bool
	rejectAfterCommit error
	revokedForPush    *domain.AccountSession
}

func (r *refreshRotation) run(ctx context.Context) error {
	now := time.Now()
	if err := r.lockAccount(ctx); err != nil {
		return err
	}
	presented, err := r.presentedSession(ctx)
	if err != nil {
		return err
	}
	if err := r.validatePresented(ctx, presented, now); err != nil || r.rejectAfterCommit != nil {
		return err
	}
	current, wasRecovered, err := r.service.resolveRefreshHandoff(ctx, presented, now)
	if err != nil {
		if errors.Is(err, identityaccess.ErrInvalidToken) {
			return r.reject(ctx, current, "replay_detected", "revoke replayed refresh-token family", identityaccess.ErrInvalidToken)
		}
		return err
	}
	if err := r.validateLineage(ctx, current); err != nil || r.rejectAfterCommit != nil {
		return err
	}
	if err := r.service.requireSessionMFA(ctx, r.account.ID, current.TenantID, current.PortalScope, current.MFAVerified); err != nil {
		return err
	}
	return r.persist(ctx, current, wasRecovered, now)
}

func (r *refreshRotation) lockAccount(ctx context.Context) error {
	if r.service.mfa.Configured() && r.claims.Scope != domain.ScopeParent && r.tenantID > 0 {
		if err := r.service.lockMFAPolicyForMint(ctx, r.claims.AccountID, r.tenantID); err != nil {
			return err
		}
	}
	unlocked, err := r.service.sessions.findAccountSessionByToken(ctx, r.claims.Token)
	if err != nil {
		if errors.Is(err, identityaccess.ErrAccountSessionNotFound) {
			r.service.logRefreshDecision("refresh_session_rejected", "token_not_found", r.claims.AccountID, r.claims.TenantID)
			return failed("get token", identityaccess.ErrTokenNotFound)
		}
		return fmt.Errorf("find refresh token owner: %w", err)
	}
	r.account, err = r.service.fetchAndValidateAccountForUpdate(ctx, unlocked.AccountID)
	if err != nil {
		reason := "account_lookup_failed"
		if errors.Is(err, identityaccess.ErrAccountInactive) {
			reason = "account_inactive"
		} else if errors.Is(err, domain.ErrAccountNotFound) {
			reason = "account_not_found"
		}
		r.service.logRefreshDecision("refresh_session_rejected", reason, r.claims.AccountID, r.claims.TenantID)
		return err
	}
	if r.guard != nil {
		return r.guard(ctx, r.account)
	}
	return nil
}

func (r *refreshRotation) presentedSession(ctx context.Context) (domain.AccountSession, error) {
	presented, err := r.service.sessions.findAccountSessionForUpdate(ctx, r.claims.Token)
	if err != nil {
		if errors.Is(err, identityaccess.ErrAccountSessionNotFound) {
			r.service.logRefreshDecision("refresh_session_rejected", "token_not_found", r.claims.AccountID, r.claims.TenantID)
			return domain.AccountSession{}, failed("get token", identityaccess.ErrTokenNotFound)
		}
		return domain.AccountSession{}, fmt.Errorf("find refresh token: %w", err)
	}
	return presented, nil
}

func (r *refreshRotation) validatePresented(ctx context.Context, presented domain.AccountSession, now time.Time) error {
	if presented.AccountID != r.claims.AccountID || (r.claims.TenantID > 0 && presented.TenantID != r.claims.TenantID) {
		return r.reject(ctx, presented, "claim_mismatch", "revoke mismatched refresh-token family", identityaccess.ErrInvalidToken)
	}
	if now.After(presented.Expiry) {
		return r.reject(ctx, presented, "token_expired", "delete expired refresh-token family", identityaccess.ErrTokenExpired)
	}
	return nil
}

func (r *refreshRotation) validateLineage(ctx context.Context, current domain.AccountSession) error {
	if current.FamilyID == "" {
		return nil
	}
	latest, err := r.service.sessions.LatestAccountSessionInFamily(ctx, current.FamilyID)
	if err != nil {
		return fmt.Errorf("inspect refresh-token family: %w", err)
	}
	if latest.Generation > current.Generation {
		return r.reject(ctx, current, "lineage_mismatch", "revoke inconsistent refresh-token family", identityaccess.ErrInvalidToken)
	}
	return nil
}

func (r *refreshRotation) reject(ctx context.Context, session domain.AccountSession, reason, operation string, cause error) error {
	if err := r.service.deleteFamilyWithAudit(ctx, session, reason, r.ipAddress, r.userAgent); err != nil {
		return fmt.Errorf("%s: %w", operation, err)
	}
	r.revokedForPush = &session
	r.rejectAfterCommit = cause
	r.service.logRefreshDecision("refresh_session_rejected", reason, r.claims.AccountID, r.claims.TenantID)
	return nil
}

func (r *refreshRotation) persist(ctx context.Context, current domain.AccountSession, wasRecovered bool, now time.Time) error {
	// Pre-tenant-claim tokens carry tenant 0; the successor gets the
	// account's default mapping.
	effectiveTenantID := r.tenantID
	if effectiveTenantID == 0 {
		resolved, _, err := r.service.resolveAccountTenant(ctx, r.account.ID, "")
		if err != nil {
			return err
		}
		effectiveTenantID = resolved
	}
	r.recovered = wasRecovered
	if r.recovered {
		r.newSession = current
	} else {
		var err error
		r.newSession, err = r.service.createAndPersistSuccessor(ctx, current, r.account.ID, effectiveTenantID, r.claims.Scope, now)
		if err != nil {
			return err
		}
	}
	if _, err := r.service.store.RecordAccountLogin(ctx, r.account.ID, time.Now()); err != nil {
		return err
	}
	return nil
}

func (s *AccountAuthentication) auditRejectedRefresh(ctx context.Context, accountID int64, ipAddress, userAgent string, err error) {
	if !errors.Is(err, identityaccess.ErrAccountInactive) || ipAddress == "" {
		return
	}
	if auditErr := s.logAuthEvent(ctx, accountID, domain.AuthEventTokenRefresh, false, ipAddress, userAgent, "Account inactive"); auditErr != nil {
		s.logger.Error("failed to audit rejected refresh",
			slog.Int64("account_id", accountID),
			slog.Any("error", auditErr))
	}
}

// fetchAndValidateAccountForUpdate locks the account before refresh locks
// its session row, preserving the account-first order login uses.
func (s *AccountAuthentication) fetchAndValidateAccountForUpdate(ctx context.Context, accountID int64) (domain.LoginAccount, error) {
	account, found, _, err := findLoginAccount(ctx, s.runtime, s.store, accountID, true)
	if err != nil {
		return domain.LoginAccount{}, failed("get account", fmt.Errorf("account lookup failed: %w", err))
	}
	if !found {
		return domain.LoginAccount{}, failed("get account", domain.ErrAccountNotFound)
	}
	if !account.Active {
		return domain.LoginAccount{}, failed("check account status", identityaccess.ErrAccountInactive)
	}
	return account, nil
}

// resolveRefreshHandoff follows a bounded, validated successor chain. A chain
// exists only when a previous rotation committed but its response may not
// have reached the browser; outside the grace period reuse is replay. The
// request proves possession of the presented token; later hops are trusted
// only after their persisted links were validated.
func (s *AccountAuthentication) resolveRefreshHandoff(ctx context.Context, presented domain.AccountSession, now time.Time) (domain.AccountSession, bool, error) {
	current := presented
	proofValidated := false
	for hop := 0; hop < s.rotation.MaxRecoveryHops(); hop++ {
		if current.RotatedAt == nil {
			return current, current.ID != presented.ID, nil
		}
		if current.ReplacementToken == nil || current.RotatedAt.After(now) || now.Sub(*current.RotatedAt) > s.rotation.RecoveryGrace() {
			return current, false, identityaccess.ErrInvalidToken
		}
		if err := s.requireAccountRecoveryProof(ctx, current.RecoveryProofHash, proofValidated); err != nil {
			return current, false, err
		}
		proofValidated = true
		next, err := s.sessions.findAccountSessionForUpdate(ctx, *current.ReplacementToken)
		if err != nil {
			if errors.Is(err, identityaccess.ErrAccountSessionNotFound) {
				return current, false, identityaccess.ErrInvalidToken
			}
			return current, false, fmt.Errorf("follow refresh-token handoff: %w", err)
		}
		// Tenant 0 is the legacy pre-tenant-claim state whose first successor
		// may carry the resolved tenant; modern lineage stays pinned.
		if !sameAccountSessionLineage(current, next) {
			return current, false, identityaccess.ErrInvalidToken
		}
		current = next
	}
	return current, false, identityaccess.ErrInvalidToken
}

// createAndPersistSuccessor mints the successor and records the bounded
// predecessor hand-off atomically.
func (s *AccountAuthentication) createAndPersistSuccessor(ctx context.Context, predecessor domain.AccountSession, accountID, tenantID int64, scope string, now time.Time) (domain.AccountSession, error) {
	expiry := now.Add(s.codec.RefreshExpiry())
	if predecessor.FamilyExpiryCap != nil && predecessor.FamilyExpiryCap.Before(expiry) {
		expiry = *predecessor.FamilyExpiryCap
	}
	successor := domain.AccountSession{
		Token:           uuid.Must(uuid.NewV4()).String(),
		AccountID:       accountID,
		TenantID:        tenantID,
		Expiry:          expiry,
		Mobile:          predecessor.Mobile,
		Identifier:      predecessor.Identifier,
		FamilyID:        predecessor.FamilyID,
		FamilyExpiryCap: predecessor.FamilyExpiryCap,
		Generation:      predecessor.Generation + 1,
		PortalScope:     domain.PersistedPortalScope(scope),
		MFAVerified:     predecessor.MFAVerified,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	created, err := s.sessions.CreateAccountSession(ctx, identityaccess.AccountSession(successor))
	if err != nil {
		return domain.AccountSession{}, err
	}
	stored := domain.AccountSession(created)
	if err := s.sessions.MarkAccountSessionRotated(ctx, predecessor.ID, stored.Token, s.rotation.RecoveryProofHash(ctx), now); err != nil {
		return domain.AccountSession{}, err
	}
	if err := s.sessions.DeleteExpiredRotatedAccountSessions(ctx, accountID, now); err != nil {
		return domain.AccountSession{}, err
	}
	return stored, nil
}

func (s *AccountAuthentication) logRefreshDecision(event, reason string, accountID, tenantID int64) {
	s.logger.Warn(event,
		slog.String("reason", reason),
		slog.Int64("account_id", accountID),
		slog.Int64("tenant_id", tenantID))
}

// validateTenantAccess ensures the account still has active access to the
// tenant of the refresh token and that the school is alive and active. The
// school check closes the race with a concurrent soft delete that bulk
// deletes the school's sessions; an inactive school is refused before the
// rotation so the client can retry after reactivation.
func (s *AccountAuthentication) validateTenantAccessInTransaction(ctx context.Context, claims domain.RefreshClaims) error {
	exists, _, err := s.store.HasActiveAccountTenant(ctx, claims.AccountID, claims.TenantID)
	if err != nil {
		s.logger.Error("refresh_session_validation_failed",
			slog.String("reason", "tenant_lookup_error"),
			slog.Int64("account_id", claims.AccountID),
			slog.Int64("tenant_id", claims.TenantID),
			slog.Any("error", err))
		return failed("validate tenant access", fmt.Errorf("tenant access lookup failed: %w", err))
	}
	if !exists {
		s.logRefreshDecision("refresh_session_rejected", "tenant_access_revoked", claims.AccountID, claims.TenantID)
		return failed("validate tenant access", identityaccess.ErrTenantAccessDenied)
	}
	school, found, err := s.schools.FindSchool(ctx, claims.TenantID)
	if err != nil {
		s.logger.Error("failed to look up school during refresh validation",
			slog.Int64("account_id", claims.AccountID),
			slog.Int64("tenant_id", claims.TenantID),
			slog.Any("error", err))
		return failed("validate tenant access", fmt.Errorf("school lookup failed: %w", err))
	}
	if !found {
		s.logRefreshDecision("refresh_session_rejected", "tenant_not_found", claims.AccountID, claims.TenantID)
		return failed("validate tenant access", identityaccess.ErrTenantNotFound)
	}
	if school.Deleted {
		s.logRefreshDecision("refresh_session_rejected", "tenant_deleted", claims.AccountID, claims.TenantID)
		return failed("validate tenant access", identityaccess.ErrTenantNotFound)
	}
	if !school.Active {
		s.logRefreshDecision("refresh_session_rejected", "tenant_inactive", claims.AccountID, claims.TenantID)
		return failed("validate tenant access", identityaccess.ErrTenantNotFound)
	}
	return nil
}

// LogoutWithAudit invalidates the presented refresh-token family; other
// devices and portals keep their sessions. Logout runs in an administrative
// transaction because auth.tokens is RLS-guarded and the route carries no
// tenant.
func (s *AccountAuthentication) LogoutWithAudit(ctx context.Context, refreshToken, ipAddress, userAgent string) (err error) {
	if s == nil {
		return errAccountAuthenticationUnavailable
	}
	defer func() { err = authenticationError(err) }()
	ctx = bindRuntime(s.attach, ctx)
	claims, err := s.codec.ParseRefreshToken(refreshToken)
	if err != nil {
		return failed("parse refresh token", identityaccess.ErrInvalidToken)
	}
	var (
		revoked  *domain.AccountSession
		sessions []domain.AccountSession
	)
	err = s.runtime.WithAdminTx(ctx, func(txCtx context.Context) error {
		presented, err := s.sessions.findAccountSessionByToken(txCtx, claims.Token)
		if err != nil {
			// An unknown token is a successful logout.
			return nil
		}
		var deleteErr error
		if presented.FamilyID == "" {
			deleteErr = s.sessions.DeleteAccountSession(txCtx, presented.ID)
			sessions = []domain.AccountSession{presented}
		} else {
			var family []identityaccess.AccountSession
			family, deleteErr = s.sessions.RevokeAccountSessionFamily(txCtx, presented.FamilyID)
			sessions = domainAccountSessions(family)
		}
		if deleteErr != nil {
			return failed("delete token family", deleteErr)
		}
		revoked = &presented
		return nil
	})
	if err == nil && revoked != nil {
		s.queuePushCleanup(ctx, revoked.AccountID, []domain.AccountSession{*revoked}, "family")
		s.auditLogout(ctx, *revoked, sessions, claims.TenantID, ipAddress, userAgent)
	}
	return err
}

// auditLogout appends the audit rows after the family deletion committed;
// logout must never leave a usable session because the audit store is down.
func (s *AccountAuthentication) auditLogout(ctx context.Context, revoked domain.AccountSession, sessions []domain.AccountSession, claimTenantID int64, ipAddress, userAgent string) {
	tenantID := revoked.TenantID
	if tenantID == 0 {
		tenantID = claimTenantID
	}
	if tenantID == 0 {
		s.logger.Error("failed to audit logout",
			slog.Int64("account_id", revoked.AccountID),
			slog.String("error", "tenant is required"))
		return
	}
	auditCtx := s.runtime.WithTenantID(ctx, tenantID)
	err := s.runtime.WithTenantTx(auditCtx, tenantID, func(txCtx context.Context) error {
		if err := s.auditRevokedSessions(txCtx, sessions, "logout", ipAddress, userAgent); err != nil {
			return err
		}
		if ipAddress == "" {
			return nil
		}
		return s.logAuthEvent(txCtx, revoked.AccountID, domain.AuthEventLogout, true, ipAddress, userAgent, "")
	})
	if err != nil {
		s.logger.Error("failed to audit logout",
			slog.Int64("account_id", revoked.AccountID),
			slog.Any("error", err))
	}
}

func (s *AccountAuthentication) validateTenantAccess(ctx context.Context, claims domain.RefreshClaims) error {
	return globalIdentityRead(ctx, s.runtime, func(txCtx context.Context) error { return s.validateTenantAccessInTransaction(txCtx, claims) })
}

func (s *AccountAuthentication) requireAccountRecoveryProof(ctx context.Context, proofHash []byte, alreadyValidated bool) error {
	if !alreadyValidated {
		if !s.rotation.MatchesRecoveryProof(ctx, proofHash) {
			return identityaccess.ErrInvalidToken
		}

	}
	return nil
}

func sameAccountSessionLineage(current, next domain.AccountSession) bool {
	return next.FamilyID == current.FamilyID && next.AccountID == current.AccountID && (current.TenantID == 0 || next.TenantID == current.TenantID) && next.Generation == current.Generation+1
}
