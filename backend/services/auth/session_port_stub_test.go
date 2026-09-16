package auth

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// stubAccountSessions is the in-package double of the Identity & Access
// session port (#3251). It records revocations and, when given the stub
// repositories, answers the guardian and school-portal tenant scans the way
// the owner does: active mappings in order, roles per mapping, a role
// lookup failure propagated, a dangling role row skipped.
type stubAccountSessions struct {
	mu                sync.Mutex
	deletedAccountIDs []int64
	repos             *repositories.Factory
	// sessions is the owner's session store when a test needs the real
	// session rows listed and revoked in the caller's tenant.
	sessions *identityaccess.Module

	deleteAccountSessionsFn        func(ctx context.Context, accountID int64, reason, ipAddress, userAgent string) ([]RevokedSession, error)
	scheduleAccountWideRevokeFn    func(ctx context.Context, accountID int64, reason, ipAddress, userAgent string) error
	markAccountWideWipeCompletedFn func(ctx context.Context, accountID int64) error
	revokeAllTokensWithReasonFn    func(ctx context.Context, accountID int64, reason string) error
	loadAccountClaimsFn            func(ctx context.Context, accountID, tenantID int64) (*AccountClaims, error)
	listSessionIDsFn               func(ctx context.Context, accountID int64) ([]int64, error)
	findGuardianTenantFn           func(ctx context.Context, accountID int64) (bool, int64, error)
	findSchoolPortalTenantFn       func(ctx context.Context, accountID int64) (bool, int64, error)
}

var errStubSessionsNotSupported = errors.New("account sessions stub: operation not supported in this test")

func newStubAccountSessions() *stubAccountSessions { return &stubAccountSessions{} }

func (s *stubAccountSessions) DeletedAccountIDs() []int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]int64, len(s.deletedAccountIDs))
	copy(out, s.deletedAccountIDs)
	return out
}

func (s *stubAccountSessions) recordDeletion(accountID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deletedAccountIDs = append(s.deletedAccountIDs, accountID)
}

func (s *stubAccountSessions) LoginWithAudit(context.Context, string, string, string, string, string) (string, string, error) {
	return "", "", errStubSessionsNotSupported
}

func (s *stubAccountSessions) LoginWithMFAGate(context.Context, string, string, string, string, string, string) (*LoginResult, error) {
	return nil, errStubSessionsNotSupported
}

func (s *stubAccountSessions) LoginParentWithAudit(context.Context, string, string, string, string) (string, string, error) {
	return "", "", errStubSessionsNotSupported
}

func (s *stubAccountSessions) LoginSchoolWithMFAGate(context.Context, string, string, string, string, string) (*LoginResult, error) {
	return nil, errStubSessionsNotSupported
}

func (s *stubAccountSessions) LoginSchoolAtTenantWithMFAGate(context.Context, string, string, string, string, string, string) (*LoginResult, error) {
	return nil, errStubSessionsNotSupported
}

func (s *stubAccountSessions) IssueTokensForAuthenticatedAccount(context.Context, int64, int64, string, string) (string, string, error) {
	return "", "", errStubSessionsNotSupported
}

func (s *stubAccountSessions) IssueSchoolTokensForAuthenticatedAccount(context.Context, int64, int64, string, string) (string, string, error) {
	return "", "", errStubSessionsNotSupported
}

func (s *stubAccountSessions) RefreshTokenWithAudit(context.Context, string, string, string) (string, string, error) {
	return "", "", errStubSessionsNotSupported
}

func (s *stubAccountSessions) LogoutWithAudit(context.Context, string, string, string) error {
	return errStubSessionsNotSupported
}

func (s *stubAccountSessions) SwitchTenant(context.Context, int64, string, string) (string, string, error) {
	return "", "", errStubSessionsNotSupported
}

func (s *stubAccountSessions) SwitchSchool(context.Context, int64, string, string, string) (string, string, error) {
	return "", "", errStubSessionsNotSupported
}

func (s *stubAccountSessions) HasSchoolPortalAccess(context.Context, int64, int64) (bool, error) {
	return false, errStubSessionsNotSupported
}

func (s *stubAccountSessions) ValidateSessionTokens(context.Context, string, string, string) (*jwt.AppClaims, error) {
	return nil, errStubSessionsNotSupported
}

func (s *stubAccountSessions) VerifyAccountTenantMembership(ctx context.Context, accountID, tenantID int64) (bool, error) {
	if s.repos == nil || s.repos.AccountTenant == nil {
		return false, errStubSessionsNotSupported
	}
	return s.repos.AccountTenant.ExistsByAccountAndTenant(ctx, accountID, tenantID)
}

func (s *stubAccountSessions) CountExpiredTokens(context.Context) (int, error) {
	return 0, errStubSessionsNotSupported
}

func (s *stubAccountSessions) CleanupExpiredTokens(context.Context) (int, error) {
	return 0, errStubSessionsNotSupported
}

func (s *stubAccountSessions) ListActiveSessions(ctx context.Context, accountID int64) ([]ActiveSession, error) {
	if s.sessions == nil {
		return nil, errStubSessionsNotSupported
	}
	rows, err := s.sessions.ListAccountSessions(ctx, identityaccess.AccountSessionFilter{AccountID: accountID, Liveness: identityaccess.AccountSessionsLive})
	if err != nil {
		return nil, err
	}
	active := make([]ActiveSession, 0, len(rows))
	for _, row := range rows {
		session := ActiveSession{ID: row.ID, Token: row.Token, Expiry: row.Expiry, Mobile: row.Mobile, CreatedAt: row.CreatedAt}
		if row.Identifier != nil {
			session.Identifier = *row.Identifier
		}
		active = append(active, session)
	}
	return active, nil
}

func (s *stubAccountSessions) ListSessionIDs(ctx context.Context, accountID int64) ([]int64, error) {
	if s.listSessionIDsFn != nil {
		return s.listSessionIDsFn(ctx, accountID)
	}
	if s.sessions == nil {
		return nil, nil
	}
	rows, err := s.sessions.ListAccountSessions(ctx, identityaccess.AccountSessionFilter{AccountID: accountID})
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	return ids, nil
}

// RevokeAllTokensWithReason deletes the account's sessions visible in the
// caller's tenant when a session store is attached, like the owner's
// tenant-scoped revocation, and records the account either way.
func (s *stubAccountSessions) RevokeAllTokensWithReason(ctx context.Context, accountID int64, reason string) error {
	if s.revokeAllTokensWithReasonFn != nil {
		return s.revokeAllTokensWithReasonFn(ctx, accountID, reason)
	}
	s.recordDeletion(accountID)
	if s.sessions == nil {
		return nil
	}
	_, err := s.sessions.RevokeAccountSessionsInTenant(ctx, accountID)
	return err
}

func (s *stubAccountSessions) RevokeTokensByTenantID(context.Context, int64) (int, error) {
	return 0, errStubSessionsNotSupported
}

func (s *stubAccountSessions) DeleteAccountSessionsWithAudit(ctx context.Context, accountID int64, reason, ipAddress, userAgent string) ([]RevokedSession, error) {
	if s.deleteAccountSessionsFn != nil {
		return s.deleteAccountSessionsFn(ctx, accountID, reason, ipAddress, userAgent)
	}
	s.recordDeletion(accountID)
	return nil, nil
}

func (s *stubAccountSessions) QueuePushCleanup(context.Context, int64, []RevokedSession, string) {}

func (s *stubAccountSessions) ScheduleAccountWideRevoke(ctx context.Context, accountID int64, reason, ipAddress, userAgent string) error {
	if s.scheduleAccountWideRevokeFn != nil {
		return s.scheduleAccountWideRevokeFn(ctx, accountID, reason, ipAddress, userAgent)
	}
	s.recordDeletion(accountID)
	return nil
}

func (s *stubAccountSessions) MarkAccountWideWipeCompleted(ctx context.Context, accountID int64) error {
	if s.markAccountWideWipeCompletedFn != nil {
		return s.markAccountWideWipeCompletedFn(ctx, accountID)
	}
	return nil
}

func (s *stubAccountSessions) LoadAccountClaims(ctx context.Context, accountID, tenantID int64) (*AccountClaims, error) {
	if s.loadAccountClaimsFn != nil {
		return s.loadAccountClaimsFn(ctx, accountID, tenantID)
	}
	return nil, errStubSessionsNotSupported
}

// FindGuardianTenant walks the stub repositories like the owner's scan:
// every active mapping, the account's roles there, and the role row of each
// assignment. A role lookup failure propagates; a dangling assignment is
// skipped.
func (s *stubAccountSessions) FindGuardianTenant(ctx context.Context, accountID int64) (bool, int64, error) {
	if s.findGuardianTenantFn != nil {
		return s.findGuardianTenantFn(ctx, accountID)
	}
	if s.repos == nil {
		return false, 0, errStubSessionsNotSupported
	}
	found, tenantID := false, int64(0)
	err := tenant.WithinAdmin(ctx, func(ctx context.Context) error {
		mappings, err := s.repos.AccountTenant.FindActiveByAccountID(ctx, accountID)
		if err != nil {
			return err
		}
		for _, mapping := range mappings {
			roles, roleErr := s.repos.AccountRole.FindByAccountIDForTenant(ctx, accountID, mapping.TenantID)
			if roleErr != nil {
				if isNotFoundError(roleErr) {
					continue
				}
				return roleErr
			}
			for _, assignment := range roles {
				role, lookupErr := s.repos.Role.FindByID(ctx, assignment.RoleID)
				if lookupErr != nil {
					if isNotFoundError(lookupErr) {
						continue
					}
					return lookupErr
				}
				if role != nil && strings.EqualFold(role.Name, guardianRoleName) {
					found, tenantID = true, mapping.TenantID
					return nil
				}
			}
		}
		return nil
	})
	return found, tenantID, err
}

func (s *stubAccountSessions) FindSchoolPortalTenant(ctx context.Context, accountID int64) (bool, int64, error) {
	if s.findSchoolPortalTenantFn != nil {
		return s.findSchoolPortalTenantFn(ctx, accountID)
	}
	if s.repos == nil {
		return false, 0, errStubSessionsNotSupported
	}
	found, tenantID := false, int64(0)
	err := tenant.WithinAdmin(ctx, func(ctx context.Context) error {
		mappings, err := s.repos.AccountTenant.FindActiveByAccountID(ctx, accountID)
		if err != nil {
			return err
		}
		for _, mapping := range mappings {
			roles, roleErr := s.repos.AccountRole.FindByAccountIDForTenant(ctx, accountID, mapping.TenantID)
			if roleErr != nil {
				if isNotFoundError(roleErr) {
					continue
				}
				return roleErr
			}
			for _, assignment := range roles {
				if assignment.Role != nil && isSchoolPortalRole(assignment.Role) {
					found, tenantID = true, mapping.TenantID
					return nil
				}
			}
		}
		return nil
	})
	return found, tenantID, err
}
