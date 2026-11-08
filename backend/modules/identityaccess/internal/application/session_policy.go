package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// sessionBinding is what a session of one kind rests on: the account, the
// school and Träger its token names, and what a sign-in read before the
// transaction.
type sessionBinding struct {
	// op names the operation in refusals.
	op        string
	accountID int64
	tenantID  int64
	orgID     int64
	// booked are the guardian schools that book Eltern-App (parent sign-in).
	booked []int64
	// recheckMFAGate: the school sign-in passed its MFA gate without a
	// challenge; the guard decides the gate again under the lock.
	recheckMFAGate bool
}

// sessionPolicy is the session rules of one kind. The mint, the refresh
// rotation and the session validation read them here.
type sessionPolicy struct {
	// guard is the eligibility a sign-in or switch mints under.
	guard func(s *AccountAuthentication, b sessionBinding) eligibility
	// renew runs the reads a renewal needs before its transaction (school
	// state, module booking, token shape) and returns the guard it runs
	// under the account lock; refusals carry the sentinels the refresh route
	// answers with.
	renew func(ctx context.Context, s *AccountAuthentication, b sessionBinding) (eligibility, error)
	// keptAt names the school a refreshed session is kept at.
	keptAt func(s *AccountAuthentication, b sessionBinding) successorTenant
	// check is what a session validation asks on its transaction: the facts
	// renew and guard decide, each read once, without locks and without
	// claims, refused with the sentinels the refresh answers with. It
	// returns the module booking check that runs after the transaction, or
	// nil.
	check func(ctx context.Context, s *AccountAuthentication, b sessionBinding) (func(context.Context) error, error)
}

var sessionPolicies = map[domain.SessionKind]sessionPolicy{
	domain.SessionKind(domain.PortalTenant.Name): {
		guard: func(s *AccountAuthentication, b sessionBinding) eligibility {
			return s.tenantEligibility(b.op, b.tenantID)
		},
		renew: func(ctx context.Context, s *AccountAuthentication, b sessionBinding) (eligibility, error) {
			if err := s.requireLiveSchool(ctx, b); err != nil {
				return nil, err
			}
			return s.tenantRefreshEligibility(b.tenantID), nil
		},
		keptAt: tokenSchool,
		check: func(ctx context.Context, s *AccountAuthentication, b sessionBinding) (func(context.Context) error, error) {
			roles, err := s.liveSchoolRoles(ctx, b)
			if err != nil {
				return nil, err
			}
			// A guardian-only role set renews as a parent session
			// (tenantRefreshEligibility).
			refusal := tenantPortalRefusal(&domain.AccountClaimsPayload{RoleNames: domain.RoleNames(roles), Roles: roles})
			if refusal != nil && !errors.Is(refusal, identityaccess.ErrParentMustUseParentPortal) {
				return nil, failed(b.op, refusal)
			}
			return nil, nil
		},
	},
	domain.SessionKind(domain.PortalSchool.Name): {
		guard: func(s *AccountAuthentication, b sessionBinding) eligibility {
			return s.schoolEligibility(b.tenantID, b.recheckMFAGate)
		},
		renew: func(ctx context.Context, s *AccountAuthentication, b sessionBinding) (eligibility, error) {
			if err := s.requireLiveSchool(ctx, b); err != nil {
				return nil, err
			}
			if !domain.PortalSchool.Holds(b.tenantID, b.orgID) {
				s.logRefreshDecision("refresh_session_rejected", "school_token_without_tenant", b.accountID, b.tenantID)
				return nil, failed("refresh school session", identityaccess.ErrTenantAccessDenied)
			}
			if err := s.requireSchoolPortal(ctx, "refresh school session", b.accountID, b.tenantID); err != nil {
				s.logRefreshDecision("refresh_session_rejected", "portal_module_not_booked", b.accountID, b.tenantID)
				return nil, err
			}
			return s.schoolRefreshEligibility(b.tenantID), nil
		},
		keptAt: tokenSchool,
		check: func(ctx context.Context, s *AccountAuthentication, b sessionBinding) (func(context.Context) error, error) {
			roles, err := s.liveSchoolRoles(ctx, b)
			if err != nil {
				return nil, err
			}
			// A revoked portal role answers as at refresh
			// (schoolRefreshEligibility).
			if !domain.HasSchoolPortalRole(roles) {
				return nil, failed(b.op, identityaccess.ErrTenantAccessDenied)
			}
			return func(ctx context.Context) error {
				return s.requireSchoolPortal(ctx, b.op, b.accountID, b.tenantID)
			}, nil
		},
	},
	domain.SessionKind(domain.PortalParent.Name): {
		guard: func(s *AccountAuthentication, b sessionBinding) eligibility {
			return s.parentEligibility(b.op, b.booked)
		},
		renew: func(ctx context.Context, s *AccountAuthentication, b sessionBinding) (eligibility, error) {
			if err := s.requireLiveSchool(ctx, b); err != nil {
				return nil, err
			}
			booked, err := s.requireParentPortalForAccount(ctx, "refresh parent session", b.accountID)
			if err != nil {
				s.logRefreshDecision("refresh_session_rejected", "portal_module_not_booked", b.accountID, b.tenantID)
				return nil, err
			}
			return s.parentRefreshEligibility(booked), nil
		},
		keptAt: tokenSchool,
		check: func(ctx context.Context, s *AccountAuthentication, b sessionBinding) (func(context.Context) error, error) {
			if b.tenantID > 0 {
				if _, err := s.tenantAccessRefusal(ctx, b.accountID, b.tenantID); err != nil {
					return nil, err
				}
			}
			guardianSchools, err := s.guardianSchoolsInTx(ctx, b.accountID, false)
			if err != nil {
				return nil, err
			}
			return func(ctx context.Context) error {
				booked, err := s.bookedGuardianSchools(ctx, b.op, b.accountID, guardianSchools)
				if err != nil {
					return err
				}
				// No live guardian school left answers as at refresh
				// (parentRefreshEligibility).
				if len(booked) == 0 {
					return failed(b.op, identityaccess.ErrTenantNotFound)
				}
				return nil
			}, nil
		},
	},
	domain.SessionKind(domain.PortalCaterer.Name): {
		guard: func(s *AccountAuthentication, _ sessionBinding) eligibility { return s.catererEligibility() },
		renew: func(_ context.Context, s *AccountAuthentication, b sessionBinding) (eligibility, error) {
			// A caterer token never names a school (ADR 0077).
			if b.tenantID != 0 {
				s.logRefreshDecision("refresh_session_rejected", "caterer_token_with_tenant", b.accountID, b.tenantID)
				return nil, failed("refresh caterer session", identityaccess.ErrInvalidToken)
			}
			return s.catererEligibility(), nil
		},
		keptAt: func(s *AccountAuthentication, _ sessionBinding) successorTenant { return s.catererSessionSchool },
		check: func(ctx context.Context, s *AccountAuthentication, b sessionBinding) (func(context.Context) error, error) {
			if b.tenantID != 0 {
				return nil, failed(b.op, identityaccess.ErrInvalidToken)
			}
			// The refresh keeps the session at a live supplied school
			// (catererSessionSchool); without one it ends.
			_, err := s.catererSessionSchool(ctx, b.accountID, domain.AccountSession{})
			return nil, err
		},
	},
	domain.SessionKind(domain.PortalOrg.Name): {
		guard: func(s *AccountAuthentication, b sessionBinding) eligibility { return s.traegerEligibility(b.orgID) },
		renew: func(_ context.Context, s *AccountAuthentication, b sessionBinding) (eligibility, error) {
			if !domain.PortalOrg.Holds(b.tenantID, b.orgID) {
				s.logRefreshDecision("refresh_session_rejected", "traeger_token_without_traeger", b.accountID, b.tenantID)
				return nil, failed("refresh Träger session", identityaccess.ErrInvalidToken)
			}
			return s.traegerEligibility(b.orgID), nil
		},
		keptAt: func(*AccountAuthentication, sessionBinding) successorTenant {
			return func(context.Context, int64, domain.AccountSession) (int64, error) { return 0, nil }
		},
		// The access token's shape was checked against the portal before.
		check: func(ctx context.Context, s *AccountAuthentication, b sessionBinding) (func(context.Context) error, error) {
			return nil, s.requireTraegerMember(ctx, b.accountID, b.orgID)
		},
	},
	// A Träger school session has no mapping at its school; its guard
	// re-checks the Träger instead (ADR 0084 section 5).
	domain.SessionKindTraegerSchool: {
		guard: func(s *AccountAuthentication, b sessionBinding) eligibility {
			return s.traegerSchoolEligibility(b.orgID, b.tenantID)
		},
		renew: func(_ context.Context, s *AccountAuthentication, b sessionBinding) (eligibility, error) {
			if b.tenantID <= 0 {
				s.logRefreshDecision("refresh_session_rejected", "traeger_school_token_without_school", b.accountID, b.tenantID)
				return nil, failed("refresh Träger school session", identityaccess.ErrInvalidToken)
			}
			return s.traegerSchoolEligibility(b.orgID, b.tenantID), nil
		},
		keptAt: tokenSchool,
		check: func(ctx context.Context, s *AccountAuthentication, b sessionBinding) (func(context.Context) error, error) {
			return nil, s.traegerGrantsSchool(ctx, b.accountID, b.orgID, b.tenantID)
		},
	},
}

// requireLiveSchool refuses the renewal of a school-bound session whose
// mapping ended or whose school is gone or inactive before the transaction,
// so the client can retry after reactivation. A token without a school
// passes.
func (s *AccountAuthentication) requireLiveSchool(ctx context.Context, b sessionBinding) error {
	if b.tenantID <= 0 {
		return nil
	}
	return s.validateTenantAccess(ctx, b.accountID, b.tenantID)
}

// liveSchoolRoles reads, without locks, the mapping and school a
// school-bound session rests on and the account's roles there.
func (s *AccountAuthentication) liveSchoolRoles(ctx context.Context, b sessionBinding) ([]domain.RoleAssignment, error) {
	if _, err := s.tenantAccessRefusal(ctx, b.accountID, b.tenantID); err != nil {
		return nil, err
	}
	return s.loadAccountRolesForTenant(ctx, b.accountID, b.tenantID)
}

// tokenSchool keeps a refreshed session at the school of the presented
// token. Pre-tenant-claim tokens carry school zero; their successor gets the
// account's default mapping.
func tokenSchool(s *AccountAuthentication, b sessionBinding) successorTenant {
	return func(ctx context.Context, accountID int64, _ domain.AccountSession) (int64, error) {
		if b.tenantID > 0 {
			return b.tenantID, nil
		}
		resolved, _, err := s.resolveAccountTenant(ctx, accountID, "")
		return resolved, err
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

// sameTraeger reports whether a stored session names the Träger its token
// names: a Träger session its Träger, a Träger school session the Träger it
// came through (#4654); a session of every other kind names none.
func sameTraeger(kind domain.SessionKind, orgID int64, session domain.AccountSession) bool {
	namesTraeger := func(k domain.SessionKind) bool {
		return k == domain.SessionKindTraegerSchool || k == domain.SessionKind(domain.PortalOrg.Name)
	}
	stored := session.SessionKind()
	if namesTraeger(kind) || namesTraeger(stored) {
		return kind == stored && session.OrganizationID == orgID
	}
	return true
}
