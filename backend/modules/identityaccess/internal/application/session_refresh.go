package application

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"time"

	"github.com/gofrs/uuid"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
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
func (s *AccountAuthentication) refreshTokenWithAudit(ctx context.Context, refreshToken string, proof domain.RecoveryProof, ipAddress, userAgent string) (string, string, error) {
	sfCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	// A refresh token alone must not join another caller's in-flight
	// recovery: the key binds the independent proof hash as well.
	v, err, shared := s.refreshSF.Do(refreshSingleflightKey(refreshToken, proof.Hash()), func() (any, error) {
		return s.doRefreshTokenWithAudit(sfCtx, refreshToken, proof, ipAddress, userAgent)
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

func (s *AccountAuthentication) doRefreshTokenWithAudit(ctx context.Context, refreshToken string, proof domain.RecoveryProof, ipAddress, userAgent string) (*refreshResult, error) {
	claims, err := s.parseRefreshClaims(refreshToken)
	if err != nil {
		s.logRefreshDecision("refresh_session_rejected", "invalid_format", 0, 0)
		return nil, err
	}
	plan, err := s.planRefresh(ctx, claims)
	if err != nil {
		return nil, err
	}
	// The portal's guard runs inside the rotation transaction: either the
	// rotation and the claims commit together, or the transaction rolls back
	// and the presented refresh token is still the caller's.
	rotated, err := s.refreshSessionInTransaction(ctx, claims, proof, ipAddress, userAgent, plan)
	if err != nil {
		return nil, err
	}
	if rotated.recovered {
		s.logger.Info("refresh_rotation_recovered",
			slog.Int64("account_id", claims.AccountID),
			slog.Int64("tenant_id", rotated.newSession.TenantID),
			slog.Int("generation", rotated.newSession.Generation))
	}
	access, refresh := buildClaims(rotated.account, rotated.newSession, rotated.payload, rotated.account.Email)
	accessToken, refreshTokenString, err := s.codec.IssueTokenPair(access, refresh)
	if err != nil {
		return nil, failed("generate tokens", err)
	}
	// Filed at the school the successor is kept at, which the refresh
	// validated: the route carries no tenant.
	s.auditSession(ctx, rotated.account.ID, mintAudit{event: domain.AuthEventTokenRefresh, ipAddress: ipAddress, userAgent: userAgent, schoolID: rotated.newSession.TenantID})
	return &refreshResult{accessToken: accessToken, refreshToken: refreshTokenString}, nil
}

// refreshPlan is what the refresh of one portal runs inside its rotation:
// the portal's eligibility and the school its successor is kept at.
type refreshPlan struct {
	portal    domain.Portal
	eligible  eligibility
	successor successorTenant
}

// planRefresh runs the refusals that need no rotation lock and picks the
// portal's guard. Every refusal here and in the guard happens before the
// rotation commits, so the presented refresh token stays valid: a school
// that books its module again lets the session continue.
func (s *AccountAuthentication) planRefresh(ctx context.Context, claims domain.RefreshClaims) (refreshPlan, error) {
	portal, ok := domain.PortalOfClaim(claims.Scope)
	if !ok || !portal.Persisted {
		s.logRefreshDecision("refresh_session_rejected", "unknown_scope", claims.AccountID, claims.TenantID)
		return refreshPlan{}, failed("refresh session", identityaccess.ErrInvalidToken)
	}
	plan := refreshPlan{portal: portal, successor: s.refreshTenant(claims)}
	switch {
	case portal.Name == domain.PortalCaterer.Name:
		// A caterer token never names a school (ADR 0077).
		if claims.TenantID != 0 {
			s.logRefreshDecision("refresh_session_rejected", "caterer_token_with_tenant", claims.AccountID, claims.TenantID)
			return refreshPlan{}, failed("refresh caterer session", identityaccess.ErrInvalidToken)
		}
		plan.eligible, plan.successor = s.catererEligibility(), s.catererSessionSchool
	case traegerSchoolRefresh(claims):
		// A Träger school session has no mapping at its school; its guard
		// re-checks the Träger instead (ADR 0084 section 5).
		if claims.TenantID <= 0 {
			s.logRefreshDecision("refresh_session_rejected", "traeger_school_token_without_school", claims.AccountID, claims.TenantID)
			return refreshPlan{}, failed("refresh Träger school session", identityaccess.ErrInvalidToken)
		}
		plan.eligible = s.traegerSchoolEligibility(claims.OrgID, claims.TenantID)
	case portal.Name == domain.PortalOrg.Name:
		if !portal.Holds(claims.TenantID, claims.OrgID) {
			s.logRefreshDecision("refresh_session_rejected", "traeger_token_without_traeger", claims.AccountID, claims.TenantID)
			return refreshPlan{}, failed("refresh Träger session", identityaccess.ErrInvalidToken)
		}
		plan.eligible = s.traegerEligibility(claims.OrgID)
	default:
		eligible, err := s.planStaffOrParentRefresh(ctx, portal, claims)
		if err != nil {
			return refreshPlan{}, err
		}
		plan.eligible = eligible
	}
	return plan, nil
}

// planStaffOrParentRefresh covers the portals bound to the account's own
// mapping: tenant, school and parent. An inactive school is refused before
// the rotation so the client can retry after reactivation; a portal whose
// module the school no longer books renews no session (#4895).
func (s *AccountAuthentication) planStaffOrParentRefresh(ctx context.Context, portal domain.Portal, claims domain.RefreshClaims) (eligibility, error) {
	if claims.TenantID > 0 {
		if err := s.validateTenantAccess(ctx, claims); err != nil {
			return nil, err
		}
	}
	switch portal.Name {
	case domain.PortalSchool.Name:
		if !portal.Holds(claims.TenantID, claims.OrgID) {
			s.logRefreshDecision("refresh_session_rejected", "school_token_without_tenant", claims.AccountID, claims.TenantID)
			return nil, failed("refresh school session", identityaccess.ErrTenantAccessDenied)
		}
		if err := s.requireSchoolPortal(ctx, "refresh school session", claims.AccountID, claims.TenantID); err != nil {
			s.logRefreshDecision("refresh_session_rejected", "portal_module_not_booked", claims.AccountID, claims.TenantID)
			return nil, err
		}
		return s.schoolRefreshEligibility(claims.TenantID), nil
	case domain.PortalParent.Name:
		booked, err := s.requireParentPortalForAccount(ctx, "refresh parent session", claims.AccountID)
		if err != nil {
			s.logRefreshDecision("refresh_session_rejected", "portal_module_not_booked", claims.AccountID, claims.TenantID)
			return nil, err
		}
		return s.parentRefreshEligibility(booked), nil
	default:
		return s.tenantRefreshEligibility(claims.TenantID), nil
	}
}

// parentRefreshEligibility reports a parent left without a live guardian
// school as ErrTenantNotFound, the answer the refresh route gave when the
// successor found no school to be kept at.
func (s *AccountAuthentication) parentRefreshEligibility(booked []int64) eligibility {
	parent := s.parentEligibility("refresh parent session", booked)
	return func(ctx context.Context, account domain.LoginAccount) (*domain.AccountClaimsPayload, error) {
		payload, err := parent(ctx, account)
		if errors.Is(err, identityaccess.ErrAccountNoGuardianRole) {
			s.logRefreshDecision("refresh_session_rejected", "no_live_guardian_school", account.ID, 0)
			return nil, failed("refresh parent session", identityaccess.ErrTenantNotFound)
		}
		return payload, err
	}
}

// refreshTenant pins the successor of a tenant or parent session to the
// tenant of the presented token. Pre-tenant-claim tokens carry tenant 0;
// their successor gets the account's default mapping.
func (s *AccountAuthentication) refreshTenant(claims domain.RefreshClaims) successorTenant {
	return func(ctx context.Context, accountID int64, _ domain.AccountSession) (int64, error) {
		if claims.Scope == domain.PortalOrg.Claim {
			return 0, nil
		}
		if claims.TenantID > 0 {
			return claims.TenantID, nil
		}
		resolved, _, err := s.resolveAccountTenant(ctx, accountID, "")
		return resolved, err
	}
}

// traegerSchoolRefresh reports the refresh of an OGS session the Träger-Büro
// opened through the Absprung: a school-bound refresh token that names its
// Träger. Other OGS refresh tokens name none.
func traegerSchoolRefresh(claims domain.RefreshClaims) bool {
	return domain.PersistedPortalScope(claims.Scope) == domain.PortalTenant.Name && claims.OrgID > 0
}

// refreshSessionInTransaction validates and rotates the session in one
// administrative transaction: the owning account is locked first (the same
// account -> session order login uses), the portal's eligibility runs under
// that lock, then
// the presented session is locked, validated, and either recovered from a
// committed hand-off or rotated.
func (s *AccountAuthentication) refreshSessionInTransaction(ctx context.Context, claims domain.RefreshClaims, proof domain.RecoveryProof, ipAddress, userAgent string, plan refreshPlan) (*refreshRotation, error) {
	rotated := &refreshRotation{service: s, claims: claims, proof: proof, ipAddress: ipAddress, userAgent: userAgent, plan: plan}
	if err := s.mintUnit(ctx, rotated.run); err != nil {
		s.auditRejectedRefresh(ctx, claims.AccountID, ipAddress, userAgent, err)
		return nil, failed("refresh transaction", err)
	}
	if rotated.revokedForPush != nil {
		s.queuePushCleanup(ctx, rotated.revokedForPush.AccountID, []domain.AccountSession{*rotated.revokedForPush}, "family")
	}
	if rotated.rejectAfterCommit != nil {
		return nil, failed("refresh transaction", rotated.rejectAfterCommit)
	}
	return rotated, nil
}

// successorTenant names the tenant the successor of the current session is
// pinned to. It runs inside the rotation transaction, under the account lock.
type successorTenant func(ctx context.Context, accountID int64, current domain.AccountSession) (int64, error)

// refreshRotation keeps the transaction's provisional result separate from a
// rejection whose family revocation must commit before it reaches the caller.
type refreshRotation struct {
	service           *AccountAuthentication
	claims            domain.RefreshClaims
	proof             domain.RecoveryProof
	ipAddress         string
	userAgent         string
	plan              refreshPlan
	account           domain.LoginAccount
	payload           *domain.AccountClaimsPayload
	newSession        domain.AccountSession
	recovered         bool
	rejectAfterCommit error
	revokedForPush    *domain.AccountSession
}

// run is the mint pipeline's order for a rotation: policy lock, account
// lock, the portal's eligibility, then the presented session, the floor and
// the live-account stamp.
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
	current, wasRecovered, err := r.service.handoffs().Resolve(ctx, presented, r.proof, now)
	if err != nil {
		if errors.Is(err, identityaccess.ErrInvalidToken) {
			return r.reject(ctx, current, "replay_detected", "revoke replayed refresh-token family", identityaccess.ErrInvalidToken)
		}
		return err
	}
	if err := r.validateLineage(ctx, current); err != nil || r.rejectAfterCommit != nil {
		return err
	}
	if err := r.service.requireMintFloor(ctx, r.account.ID, current.TenantID, current); err != nil {
		return err
	}
	return r.persist(ctx, current, wasRecovered, now)
}

func (r *refreshRotation) lockAccount(ctx context.Context) error {
	if err := r.service.lockMFAPolicyFor(ctx, r.plan.portal, r.claims.AccountID, r.claims.TenantID); err != nil {
		return err
	}
	unlocked, err := r.service.sessions.findAccountSessionByToken(ctx, r.claims.Token)
	if err != nil {
		if errors.Is(err, identityaccess.ErrAccountSessionNotFound) {
			r.service.logRefreshDecision("refresh_session_rejected", "token_not_found", r.claims.AccountID, r.claims.TenantID)
			return failed("get token", identityaccess.ErrTokenNotFound)
		}
		return fmt.Errorf("find refresh token owner: %w", err)
	}
	r.account, err = r.service.lockLiveAccount(ctx, unlocked.AccountID)
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
	r.payload, err = r.service.eligibleClaims(ctx, r.plan.eligible, r.account)
	return err
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
	traegerToken, traegerSession := r.claims.Scope == domain.PortalOrg.Claim, presented.PortalScope == domain.PortalOrg.Name
	viaTraeger := traegerSchoolRefresh(r.claims) || domain.IsTraegerSchoolSession(presented)
	if presented.AccountID != r.claims.AccountID || (r.claims.TenantID > 0 && presented.TenantID != r.claims.TenantID) ||
		traegerToken != traegerSession || ((traegerToken || viaTraeger) && presented.OrganizationID != r.claims.OrgID) {
		return r.reject(ctx, presented, "claim_mismatch", "revoke mismatched refresh-token family", identityaccess.ErrInvalidToken)
	}
	if now.After(presented.Expiry) {
		return r.reject(ctx, presented, "token_expired", "delete expired refresh-token family", identityaccess.ErrTokenExpired)
	}
	return nil
}

func (r *refreshRotation) validateLineage(ctx context.Context, current domain.AccountSession) error {
	if current.FamilyID == "" {
		// A head from before refresh families that the backfill has not
		// reached (it rotated while the migration ran) has no lineage to
		// check; its rotation gives the chain a family (#5194).
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
	effectiveTenantID, err := r.plan.successor(ctx, r.account.ID, current)
	if err != nil {
		return err
	}
	capped := capLegacyLineage(&current)
	r.recovered = wasRecovered
	if r.recovered {
		// Recovery hands out the successor a rotation before the deploy
		// stored; its new deadline is persisted on that head (#5422).
		if capped {
			if err := r.service.sessions.RetireAccountSessionFamily(ctx, current.AccountID, current.FamilyID, *current.FamilyExpiryCap); err != nil {
				return err
			}
		}
		r.newSession = current
	} else {
		var err error
		r.newSession, err = r.service.createAndPersistSuccessor(ctx, current, r.account.ID, effectiveTenantID, r.claims.Scope, r.proof, now)
		if err != nil {
			return err
		}
	}
	_, err = r.service.stampLiveAccount(ctx, r.account.ID)
	return err
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

// handoffs follows the rotation hand-offs of account refresh sessions.
func (s *AccountAuthentication) handoffs() domain.HandoffChain[domain.AccountSession] {
	return domain.HandoffChain[domain.AccountSession]{
		Link: func(session domain.AccountSession) domain.HandoffLink {
			return domain.HandoffLink{ID: session.ID, Token: session.Token, RotatedAt: session.RotatedAt, ReplacementTokenHash: session.ReplacementTokenHash, RecoveryProofHash: session.RecoveryProofHash}
		},
		Successor:   s.sessions.findAccountSessionForUpdate,
		SameLineage: sameAccountSessionLineage,
		Grace:       s.rotation.RecoveryGrace(),
		MaxHops:     s.rotation.MaxRecoveryHops(),
		Invalid:     identityaccess.ErrInvalidToken,
		NotFound:    identityaccess.ErrAccountSessionNotFound,
		Portal:      "account",
	}
}

// createAndPersistSuccessor mints the successor and records the bounded
// predecessor hand-off atomically. The successor follows from the presented
// token, so a lost response can be recomputed although only hashes are
// stored (#4902).
func (s *AccountAuthentication) createAndPersistSuccessor(ctx context.Context, predecessor domain.AccountSession, accountID, tenantID int64, scope string, proof domain.RecoveryProof, now time.Time) (domain.AccountSession, error) {
	expiry := now.Add(s.codec.RefreshExpiry())
	if predecessor.FamilyExpiryCap != nil && predecessor.FamilyExpiryCap.Before(expiry) {
		expiry = *predecessor.FamilyExpiryCap
	}
	// A chain from before refresh families gets one here; the predecessor
	// takes it when it is marked rotated (#5194).
	familyID := predecessor.FamilyID
	if familyID == "" {
		familyID = uuid.Must(uuid.NewV4()).String()
	}
	successor := domain.AccountSession{
		Token:                domain.SuccessorRefreshToken(predecessor.Token),
		AccountID:            accountID,
		TenantID:             tenantID,
		OrganizationID:       predecessor.OrganizationID,
		Expiry:               expiry,
		Mobile:               predecessor.Mobile,
		Identifier:           predecessor.Identifier,
		FamilyID:             familyID,
		SwitchedFromFamilyID: predecessor.SwitchedFromFamilyID,
		FamilyExpiryCap:      predecessor.FamilyExpiryCap,
		LineageDeadline:      predecessor.LineageDeadline,
		Generation:           predecessor.Generation + 1,
		PortalScope:          domain.PersistedPortalScope(scope),
		MFAVerified:          predecessor.MFAVerified,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	created, err := s.sessions.CreateAccountSession(ctx, ports.AccountSession(successor))
	if err != nil {
		return domain.AccountSession{}, err
	}
	stored := domain.AccountSession(created)
	if err := s.sessions.MarkAccountSessionRotated(ctx, predecessor.ID, stored.Token, proof.Hash(), now); err != nil {
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

// LogoutWithAudit invalidates the presented refresh-token family with the
// chain it links into, the families of its school-switch lineage included
// (#5422); other logins, devices and portals keep their sessions. Logout
// runs in an administrative transaction because auth.tokens is RLS-guarded
// and the route carries no tenant.
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
		presented, found, err := s.logoutStart(txCtx, claims)
		if err != nil || !found {
			return err
		}
		// The account lock refresh takes first (#5194): a rotation that
		// committed its successor before this lock is granted is deleted
		// with the family below, one that waits for it finds no token.
		if _, _, _, err := findLoginAccount(txCtx, s.runtime, s.store, presented.AccountID, true); err != nil {
			return failed("lock account", err)
		}
		revokedChain, deleteErr := s.revokeSessionChain(txCtx, presented)
		if deleteErr != nil {
			return failed("delete token family", deleteErr)
		}
		sessions = revokedChain
		revoked = &presented
		return nil
	})
	if err == nil && revoked != nil {
		// Every revoked family drops its push subscriptions, the families of
		// the switch lineage included (#5422).
		s.queuePushCleanup(ctx, revoked.AccountID, append([]domain.AccountSession{*revoked}, sessions...), "family")
		s.auditLogout(ctx, *revoked, sessions, claims.TenantID, ipAddress, userAgent)
	}
	return err
}

// logoutStart resolves the session a logout walks the chain from: the
// presented token's row, read before the account lock and re-read by the
// walk after it. When the row is gone (the session cap evicted a retired
// switch source, cleanup removed an expired one), the token's signed claims
// still name its family and switch lineage (#5422), so a family switched
// from it cannot outlive the logout. A token without them, minted before
// #5422, has nothing left to revoke: an unknown token is a successful
// logout.
func (s *AccountAuthentication) logoutStart(ctx context.Context, claims domain.RefreshClaims) (domain.AccountSession, bool, error) {
	presented, err := s.sessions.findAccountSessionByToken(ctx, claims.Token)
	if err == nil {
		return presented, true, nil
	}
	if !errors.Is(err, identityaccess.ErrAccountSessionNotFound) {
		return domain.AccountSession{}, false, failed("find refresh token", err)
	}
	if claims.FamilyID == "" || claims.AccountID <= 0 {
		return domain.AccountSession{}, false, nil
	}
	start := domain.AccountSession{
		AccountID: claims.AccountID, TenantID: claims.TenantID, OrganizationID: claims.OrgID,
		TokenHash: domain.HashToken(claims.Token), FamilyID: claims.FamilyID, PortalScope: domain.PersistedPortalScope(claims.Scope),
	}
	if claims.LineageID != claims.FamilyID {
		start.SwitchedFromFamilyID = claims.LineageID
	}
	return start, true, nil
}

// auditLogout appends the audit rows after the family deletion committed;
// logout must never leave a usable session because the audit store is down.
// A switch lineage spans schools (#5422): each school's ledger records the
// revocation of its own sessions in its own tenant transaction, and the
// school of the presented session records the logout itself.
func (s *AccountAuthentication) auditLogout(ctx context.Context, revoked domain.AccountSession, sessions []domain.AccountSession, claimTenantID int64, ipAddress, userAgent string) {
	// A Träger session belongs to no school, and the authentication ledger
	// is school-scoped (ADR 0084); the Träger audit arrives with #4655. The
	// school sessions its Absprung opened are audited at their schools.
	tenantID := revoked.TenantID
	if tenantID == 0 {
		tenantID = claimTenantID
	}
	if revoked.PortalScope == domain.PortalOrg.Name {
		tenantID = 0
	} else if tenantID == 0 {
		s.logger.Error("failed to audit logout",
			slog.Int64("account_id", revoked.AccountID),
			slog.String("error", "tenant is required"))
		return
	}
	bySchool := sessionsBySchool(sessions, tenantID)
	for _, schoolID := range slices.Sorted(maps.Keys(bySchool)) {
		if schoolID == 0 {
			continue
		}
		auditCtx := s.runtime.WithTenantID(ctx, schoolID)
		err := s.ledgerUnit(auditCtx, schoolID, func(txCtx context.Context) error {
			return s.auditLogoutAtSchool(txCtx, revoked.AccountID, bySchool[schoolID], schoolID == tenantID, ipAddress, userAgent)
		})
		if err != nil {
			s.logger.Error("failed to audit logout",
				slog.Int64("account_id", revoked.AccountID),
				slog.Int64("tenant_id", schoolID),
				slog.Any("error", err))
		}
	}
}

// sessionsBySchool groups the revoked sessions by school. A session without
// school belongs to the presented one, which is always present.
func sessionsBySchool(sessions []domain.AccountSession, presentedTenantID int64) map[int64][]domain.AccountSession {
	bySchool := map[int64][]domain.AccountSession{presentedTenantID: nil}
	for _, session := range sessions {
		schoolID := session.TenantID
		if schoolID == 0 {
			schoolID = presentedTenantID
		}
		bySchool[schoolID] = append(bySchool[schoolID], session)
	}
	return bySchool
}

// auditLogoutAtSchool writes one school's rows inside its tenant transaction.
func (s *AccountAuthentication) auditLogoutAtSchool(txCtx context.Context, accountID int64, sessions []domain.AccountSession, presented bool, ipAddress, userAgent string) error {
	if err := s.auditRevokedSessions(txCtx, sessions, "logout", ipAddress, userAgent); err != nil {
		return err
	}
	if !presented || ipAddress == "" {
		return nil
	}
	return s.logAuthEvent(txCtx, accountID, domain.AuthEventLogout, true, ipAddress, userAgent, "")
}

func (s *AccountAuthentication) validateTenantAccess(ctx context.Context, claims domain.RefreshClaims) error {
	return globalIdentityRead(ctx, s.runtime, func(txCtx context.Context) error { return s.validateTenantAccessInTransaction(txCtx, claims) })
}

// sameAccountSessionLineage holds when next continues current's family.
// Tenant 0 is the legacy pre-tenant-claim state whose first successor may
// carry the resolved tenant; a caterer session may be re-pinned to another
// supplied school.
func sameAccountSessionLineage(current, next domain.AccountSession) bool {
	sameTenant := current.TenantID == 0 || next.TenantID == current.TenantID
	catererRepin := current.PortalScope == domain.PortalCaterer.Name && next.PortalScope == domain.PortalCaterer.Name
	return next.FamilyID == current.FamilyID && next.AccountID == current.AccountID && (sameTenant || catererRepin) && next.OrganizationID == current.OrganizationID && next.Generation == current.Generation+1
}
