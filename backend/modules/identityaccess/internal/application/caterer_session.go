package application

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// The caterer's session (ADR 0077). A caterer belongs to no school, so its
// refresh is a case of its own beside the tenant, parent and school refresh
// rather than a branch inside them: its token carries no tenant, its claims
// come from the caterer it belongs to, and its successor session follows
// the schools the caterer still supplies.
//
// auth.tokens and audit.auth_events both require a school, so a caterer's
// session row and its audit events are kept at a school it supplies: the
// one the presented session is kept at while that supply lives, else the
// first live one. That school is a storage key, never an access right; the
// token stays tenant-less.

// refreshCatererSession rotates a caterer session. The rotation shares the
// account lock, replay detection and successor hand-off of every refresh.
func (s *AccountAuthentication) refreshCatererSession(ctx context.Context, claims domain.RefreshClaims, ipAddress, userAgent string) (*refreshResult, error) {
	if claims.TenantID != 0 {
		// A caterer token never names a school.
		s.logRefreshDecision("refresh_session_rejected", "caterer_token_with_tenant", claims.AccountID, claims.TenantID)
		return nil, failed("refresh caterer session", identityaccess.ErrInvalidToken)
	}
	var metadata *domain.AccountClaimsPayload
	guard := func(ctx context.Context, account domain.LoginAccount) error {
		return s.catererRefreshClaims(ctx, account, &metadata)
	}
	account, newSession, recovered, err := s.refreshSessionInTransaction(ctx, claims, ipAddress, userAgent, s.catererSessionSchool, guard)
	if err != nil {
		return nil, err
	}
	if recovered {
		s.logger.Info("refresh_rotation_recovered",
			slog.Int64("account_id", claims.AccountID),
			slog.Int64("tenant_id", newSession.TenantID),
			slog.Int("generation", newSession.Generation))
	}
	if metadata == nil {
		return nil, failed("refresh caterer session", fmt.Errorf("refresh claims payload missing after rotation"))
	}
	access, refresh := buildClaims(account, newSession, metadata, account.Email)
	auditCtx := s.runtime.WithTenantID(ctx, newSession.TenantID)
	accessToken, refreshTokenString, err := s.generateAndLogTokens(auditCtx, account.ID, access, refresh, ipAddress, userAgent, domain.AuthEventTokenRefresh)
	if err != nil {
		return nil, err
	}
	return &refreshResult{accessToken: accessToken, refreshToken: refreshTokenString}, nil
}

// catererRefreshClaims keeps a caterer session a caterer session, and only
// while the account still belongs to a caterer (ADR 0077).
func (s *AccountAuthentication) catererRefreshClaims(ctx context.Context, account domain.LoginAccount, out **domain.AccountClaimsPayload) error {
	if s.caterers == nil {
		return domain.ErrAccountNotCaterer
	}
	if _, isCaterer, _, err := s.caterers.FindCatererOfAccount(ctx, account.ID); err != nil {
		return fmt.Errorf("find caterer: %w", err)
	} else if !isCaterer {
		return domain.ErrAccountNotCaterer
	}
	*out = buildCatererMetadata(account)
	return nil
}

// catererSessionSchool is the school a caterer's successor session is kept
// at: the current one while its supply is live, else the first live school
// the caterer supplies. A caterer without a live supplied school has
// nothing left to see (ADR 0077).
func (s *AccountAuthentication) catererSessionSchool(ctx context.Context, accountID int64, current domain.AccountSession) (int64, error) {
	if s.caterers == nil {
		return 0, domain.ErrAccountNotCaterer
	}
	caterer, isCaterer, _, err := s.caterers.FindCatererOfAccount(ctx, accountID)
	if err != nil {
		return 0, fmt.Errorf("find caterer: %w", err)
	}
	if !isCaterer {
		return 0, domain.ErrAccountNotCaterer
	}
	schools, _, err := s.caterers.ListCatererSchools(ctx, caterer.ID)
	if err != nil {
		return 0, fmt.Errorf("list caterer schools: %w", err)
	}
	active, err := s.schools.ListActiveSchoolsByID(ctx, schools)
	if err != nil {
		return 0, fmt.Errorf("resolve caterer schools: %w", err)
	}
	live := make(map[int64]bool, len(active))
	for _, school := range active {
		live[school.ID] = true
	}
	schools = slices.DeleteFunc(schools, func(id int64) bool { return !live[id] })
	if len(schools) == 0 {
		return 0, identityaccess.ErrCatererWithoutSchool
	}
	if slices.Contains(schools, current.TenantID) {
		return current.TenantID, nil
	}
	return schools[0], nil
}

// isCatererAccount reports whether the account belongs to a caterer, read
// through the global identity read: a caterer has no school to read in.
func (s *AccountAuthentication) isCatererAccount(ctx context.Context, accountID int64) (bool, error) {
	if s.caterers == nil {
		return false, nil
	}
	var isCaterer bool
	err := globalIdentityRead(ctx, s.runtime, func(readCtx context.Context) error {
		var err error
		_, isCaterer, _, err = s.caterers.FindCatererOfAccount(readCtx, accountID)
		return err
	})
	return isCaterer, err
}
