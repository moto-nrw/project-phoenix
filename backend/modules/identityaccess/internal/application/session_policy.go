package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// renewal names what a binding serves. A sign-in or switch mints; a refresh
// and a session validation renew, and answer one state alike.
type renewal int

const (
	mintSession renewal = iota
	refreshSession
	validateSession
)

// sessionBinding is what a session of one kind rests on: the account, the
// school and Träger its token names, and what a sign-in read before the
// transaction.
type sessionBinding struct {
	// op names the operation in refusals.
	op        string
	renewal   renewal
	accountID int64
	tenantID  int64
	orgID     int64
	// booked are the guardian schools that book Eltern-App, read before the
	// transaction (parent sign-in and refresh).
	booked []int64
	// recheckMFAGate: the school sign-in passed its MFA gate without a
	// challenge; the guard decides the gate again under the lock.
	recheckMFAGate bool
}

// refuse returns a renewal's refusal. A refresh logs its reason with the
// other refresh rejections; a validation logs once when it answers.
func (b sessionBinding) refuse(s *AccountAuthentication, reason string, err error) error {
	if b.renewal == refreshSession {
		s.logRefreshDecision("refresh_session_rejected", reason, b.accountID, b.tenantID)
	}
	return err
}

// factReads reads what a session rests on. Mint and refresh read under the
// guard's share locks, in the lock order of the mint pipeline; a validation
// writes nothing and reads the same facts without locks.
type factReads struct{ locked bool }

func (r factReads) school(ctx context.Context, s *AccountAuthentication, schoolID int64) (domain.School, bool, error) {
	if r.locked {
		return s.schools.LockSchoolShared(ctx, schoolID)
	}
	return s.schools.FindSchool(ctx, schoolID)
}

func (r factReads) mapping(ctx context.Context, s *AccountAuthentication, accountID, tenantID int64) (bool, error) {
	var mapped bool
	var err error
	if r.locked {
		mapped, _, err = s.store.LockActiveTenantMappingShared(ctx, accountID, tenantID)
	} else {
		mapped, _, err = s.store.HasActiveAccountTenant(ctx, accountID, tenantID)
	}
	return mapped, err
}

func (r factReads) roles(ctx context.Context, s *AccountAuthentication, accountID, tenantID int64) ([]domain.RoleAssignment, error) {
	roles, _, err := s.store.ListAccountRolesAtTenant(ctx, accountID, tenantID, r.locked)
	return roles, err
}

func (r factReads) permissionSources(ctx context.Context, s *AccountAuthentication, accountID, tenantID int64) error {
	if !r.locked {
		return nil
	}
	_, err := s.store.LockAccountPermissionSources(ctx, accountID, tenantID)
	return err
}

// heldFacts is what the facts of a kind read and the claims or the booking
// check go on with.
type heldFacts struct {
	roles           []domain.RoleAssignment
	guardianSchools []int64
}

// portalBooking is the module a portal's sessions need booked (#4895).
type portalBooking int

const (
	noBooking portalBooking = iota
	schoolPortalBooking
	parentPortalBooking
)

// sessionPolicy is the session rules of one kind. Each fact has one
// definition: the mint and the refresh rotation run it under the guard's
// locks (guard, renew), the session validation without them (check).
type sessionPolicy struct {
	// before runs the lock-free reads a renewal makes ahead of the facts:
	// the token's shape and the school it is bound to. The refresh runs them
	// before its transaction, the validation on its own.
	before func(ctx context.Context, s *AccountAuthentication, b sessionBinding) error
	// facts is what the account must still hold, read through r.
	facts func(ctx context.Context, s *AccountAuthentication, b sessionBinding, r factReads) (heldFacts, error)
	// claims builds the token claims under the account lock after the facts.
	// A refusal belongs in facts: validation never builds claims.
	claims func(ctx context.Context, s *AccountAuthentication, b sessionBinding, account domain.LoginAccount, held heldFacts) (*domain.AccountClaimsPayload, error)
	// booking is the module the kind's school must book. The refresh reads
	// it before its transaction, the validation after its own: the portal
	// port reads in a transaction of its own.
	booking portalBooking
	// keptAt names the school a renewed session is kept at; a refusal there
	// ends the session at refresh and validation alike.
	keptAt func(s *AccountAuthentication, b sessionBinding) successorTenant
}

// guard is the eligibility a session mints or renews under: the facts under
// the guard's locks, then the claims.
func (p sessionPolicy) guard(s *AccountAuthentication, b sessionBinding) eligibility {
	return func(ctx context.Context, account domain.LoginAccount) (*domain.AccountClaimsPayload, error) {
		b.accountID = account.ID
		held, err := p.facts(ctx, s, b, factReads{locked: true})
		if err != nil {
			return nil, err
		}
		if p.booking == parentPortalBooking {
			if err := b.guardianAtBookedSchool(s, held.guardianSchools, b.booked); err != nil {
				return nil, err
			}
		}
		return p.claims(ctx, s, b, account, held)
	}
}

// renew runs the reads a refresh makes before its transaction and returns
// the guard it runs under the account lock. Every refusal leaves the
// presented token valid.
func (p sessionPolicy) renew(ctx context.Context, s *AccountAuthentication, b sessionBinding) (eligibility, error) {
	if err := p.before(ctx, s, b); err != nil {
		return nil, err
	}
	switch p.booking {
	case schoolPortalBooking:
		if err := s.requireSchoolPortal(ctx, "refresh school session", b.accountID, b.tenantID); err != nil {
			return nil, b.refuse(s, "portal_module_not_booked", err)
		}
	case parentPortalBooking:
		booked, err := s.requireParentPortalForAccount(ctx, "refresh parent session", b.accountID)
		if err != nil {
			return nil, b.refuse(s, "portal_module_not_booked", err)
		}
		b.booked = booked
	case noBooking:
	}
	return p.guard(s, b), nil
}

// check is what a session validation asks on its transaction: what the
// refresh reads before its transaction and the facts, without locks, then
// the MFA floor and the school the session is kept at. session is the
// family's latest generation or the presented refresh row. It returns the
// module booking check that runs after the transaction, or nil.
func (p sessionPolicy) check(ctx context.Context, s *AccountAuthentication, b sessionBinding, session domain.AccountSession) (func(context.Context) error, error) {
	if err := p.before(ctx, s, b); err != nil {
		return nil, err
	}
	held, err := p.facts(ctx, s, b, factReads{})
	if err != nil {
		return nil, err
	}
	if err := s.requireSessionFloor(ctx, b.accountID, session.TenantID, session, factReads{}); err != nil {
		return nil, err
	}
	if _, err := p.keptAt(s, b)(ctx, b.accountID, session); err != nil {
		return nil, err
	}
	switch p.booking {
	case schoolPortalBooking:
		return func(ctx context.Context) error {
			return s.requireSchoolPortal(ctx, b.op, b.accountID, b.tenantID)
		}, nil
	case parentPortalBooking:
		return func(ctx context.Context) error {
			booked, err := s.bookedGuardianSchools(ctx, b.op, b.accountID, held.guardianSchools)
			if err != nil {
				return err
			}
			return b.guardianAtBookedSchool(s, held.guardianSchools, booked)
		}, nil
	case noBooking:
	}
	return nil, nil
}

var sessionPolicies = map[domain.SessionKind]sessionPolicy{
	domain.SessionKind(domain.PortalTenant.Name): {
		before: requireLiveSchool,
		facts:  tenantFacts,
		claims: func(ctx context.Context, s *AccountAuthentication, b sessionBinding, account domain.LoginAccount, held heldFacts) (*domain.AccountClaimsPayload, error) {
			if b.renewal != mintSession && domain.IsGuardianOnly(domain.RoleNames(held.roles)) {
				return buildParentMetadata(account), nil
			}
			return s.loadAccountMetadataForTenantInTx(ctx, account, b.tenantID)
		},
		keptAt: tokenSchool,
	},
	domain.SessionKind(domain.PortalSchool.Name): {
		before: func(ctx context.Context, s *AccountAuthentication, b sessionBinding) error {
			if err := requireLiveSchool(ctx, s, b); err != nil {
				return err
			}
			if !domain.PortalSchool.Holds(b.tenantID, b.orgID) {
				return b.refuse(s, "school_token_without_tenant", failed("refresh school session", identityaccess.ErrTenantAccessDenied))
			}
			return nil
		},
		facts: schoolFacts,
		claims: func(ctx context.Context, s *AccountAuthentication, b sessionBinding, account domain.LoginAccount, _ heldFacts) (*domain.AccountClaimsPayload, error) {
			if err := (factReads{locked: true}).permissionSources(ctx, s, account.ID, b.tenantID); err != nil {
				return nil, fmt.Errorf("lock permission sources of account %d at school %d: %w", account.ID, b.tenantID, err)
			}
			payload, err := s.schoolClaimsPayloadInTx(ctx, account, b.tenantID)
			if err != nil {
				return nil, err
			}
			if b.recheckMFAGate {
				if err := s.requireSchoolMintMFAPolicy(ctx, account.ID, b.tenantID, payload); err != nil {
					return nil, err
				}
			}
			return payload, nil
		},
		booking: schoolPortalBooking,
		keptAt:  tokenSchool,
	},
	domain.SessionKind(domain.PortalParent.Name): {
		before: requireLiveSchool,
		facts: func(ctx context.Context, s *AccountAuthentication, b sessionBinding, r factReads) (heldFacts, error) {
			if r.locked && b.tenantID <= 0 {
				// Parent tokens name no school. Resolve the successor's
				// school under the account lock before locking it (#6305).
				var err error
				b.tenantID, _, err = s.resolveAccountTenantDefaultInTransaction(ctx, b.accountID)
				if err != nil {
					return heldFacts{}, err
				}
			}
			if err := liveSchoolUnderLock(ctx, s, b, r); err != nil {
				return heldFacts{}, err
			}
			guardianSchools, err := s.guardianSchoolsInTx(ctx, b.accountID, r.locked)
			return heldFacts{guardianSchools: guardianSchools}, err
		},
		claims: func(_ context.Context, _ *AccountAuthentication, _ sessionBinding, account domain.LoginAccount, _ heldFacts) (*domain.AccountClaimsPayload, error) {
			return buildParentMetadata(account), nil
		},
		booking: parentPortalBooking,
		keptAt:  tokenSchool,
	},
	domain.SessionKind(domain.PortalCaterer.Name): {
		before: func(_ context.Context, s *AccountAuthentication, b sessionBinding) error {
			// A caterer token never names a school (ADR 0077).
			if b.tenantID != 0 {
				return b.refuse(s, "caterer_token_with_tenant", failed("refresh caterer session", identityaccess.ErrInvalidToken))
			}
			return nil
		},
		facts: func(ctx context.Context, s *AccountAuthentication, b sessionBinding, _ factReads) (heldFacts, error) {
			return heldFacts{}, s.requireCaterer(ctx, b.accountID)
		},
		claims: func(_ context.Context, _ *AccountAuthentication, _ sessionBinding, account domain.LoginAccount, _ heldFacts) (*domain.AccountClaimsPayload, error) {
			return buildCatererMetadata(account), nil
		},
		keptAt: func(s *AccountAuthentication, _ sessionBinding) successorTenant { return s.catererSessionSchool },
	},
	domain.SessionKind(domain.PortalOrg.Name): {
		before: func(_ context.Context, s *AccountAuthentication, b sessionBinding) error {
			if !domain.PortalOrg.Holds(b.tenantID, b.orgID) {
				return b.refuse(s, "traeger_token_without_traeger", failed("refresh Träger session", identityaccess.ErrInvalidToken))
			}
			return nil
		},
		facts: func(ctx context.Context, s *AccountAuthentication, b sessionBinding, _ factReads) (heldFacts, error) {
			return heldFacts{}, s.requireTraegerMember(ctx, b.accountID, b.orgID)
		},
		claims: func(ctx context.Context, s *AccountAuthentication, b sessionBinding, account domain.LoginAccount, _ heldFacts) (*domain.AccountClaimsPayload, error) {
			return s.traegerClaims(ctx, account, b.orgID)
		},
		keptAt: func(*AccountAuthentication, sessionBinding) successorTenant {
			return func(context.Context, int64, domain.AccountSession) (int64, error) { return 0, nil }
		},
	},
	// A Träger school session has no mapping at its school; its facts are
	// the Träger's instead (ADR 0084 section 5).
	domain.SessionKindTraegerSchool: {
		before: func(_ context.Context, s *AccountAuthentication, b sessionBinding) error {
			if b.tenantID <= 0 {
				return b.refuse(s, "traeger_school_token_without_school", failed("refresh Träger school session", identityaccess.ErrInvalidToken))
			}
			return nil
		},
		facts: func(ctx context.Context, s *AccountAuthentication, b sessionBinding, r factReads) (heldFacts, error) {
			return heldFacts{}, s.traegerGrantsSchool(ctx, b.accountID, b.orgID, b.tenantID, r)
		},
		claims: func(ctx context.Context, s *AccountAuthentication, b sessionBinding, account domain.LoginAccount, _ heldFacts) (*domain.AccountClaimsPayload, error) {
			return s.traegerSchoolClaims(ctx, account, b.tenantID, b.orgID)
		},
		keptAt: tokenSchool,
	},
}

// requireLiveSchool refuses the renewal of a school-bound session whose
// mapping ended or whose school is gone or inactive. The school check closes
// the race with a concurrent soft delete that bulk deletes the school's
// sessions. A token without a school passes.
func requireLiveSchool(ctx context.Context, s *AccountAuthentication, b sessionBinding) error {
	if b.tenantID <= 0 {
		return nil
	}
	return globalIdentityRead(ctx, s.runtime, func(ctx context.Context) error {
		reason, err := s.tenantAccessRefusal(ctx, b.accountID, b.tenantID)
		switch {
		case reason != "":
			return b.refuse(s, reason, err)
		case err != nil && b.renewal == refreshSession:
			s.logger.Error("refresh_session_validation_failed",
				slog.Int64("account_id", b.accountID),
				slog.Int64("tenant_id", b.tenantID),
				slog.Any("error", err))
		}
		return err
	})
}

// tenantFacts: an active mapping at the school and a role set the tenant
// portal serves. A refresh token from before the tenant claim names school
// zero and has no mapping to check; its successor gets the default mapping.
// A renewal lets a guardian-only role set pass: a refresh token from before
// the scope claim renews it as a parent session. The guard always read the
// roles without a lock.
func tenantFacts(ctx context.Context, s *AccountAuthentication, b sessionBinding, r factReads) (heldFacts, error) {
	if err := liveSchoolUnderLock(ctx, s, b, r); err != nil {
		return heldFacts{}, err
	}
	if b.tenantID > 0 {
		mapped, err := r.mapping(ctx, s, b.accountID, b.tenantID)
		if err != nil {
			return heldFacts{}, fmt.Errorf("re-check membership of account %d at school %d: %w", b.accountID, b.tenantID, err)
		}
		if !mapped {
			return heldFacts{}, failed(b.op, identityaccess.ErrTenantAccessDenied)
		}
	}
	roles, err := s.loadAccountRolesForTenant(ctx, b.accountID, b.tenantID)
	if err != nil {
		return heldFacts{}, err
	}
	refusal := tenantPortalRefusal(&domain.AccountClaimsPayload{RoleNames: domain.RoleNames(roles), Roles: roles})
	if refusal != nil && (b.renewal == mintSession || !errors.Is(refusal, identityaccess.ErrParentMustUseParentPortal)) {
		return heldFacts{}, failed(b.op, refusal)
	}
	return heldFacts{roles: roles}, nil
}

// liveSchoolUnderLock re-reads, in a mint or refresh, the school the session
// is kept at under a share lock, after the account lock (#6305). Ending a
// school's sessions holds the school until its deletion commits
// (auth.revoke_school_sessions): a mint or refresh that comes later waits and
// finds the school gone, so no session survives the end of its school. The
// validation writes nothing and reads the school before its facts.
func liveSchoolUnderLock(ctx context.Context, s *AccountAuthentication, b sessionBinding, r factReads) error {
	if !r.locked || b.tenantID <= 0 {
		return nil
	}
	school, found, err := r.school(ctx, s, b.tenantID)
	if err != nil {
		return fmt.Errorf("re-check school %d of a session: %w", b.tenantID, err)
	}
	if !found || !school.Live() {
		return b.refuse(s, "tenant_deleted", failed(b.op, identityaccess.ErrTenantNotFound))
	}
	return nil
}

// schoolFacts: the school is alive and active, the mapping is active and
// the account holds a school-portal role there. A renewal answers a revoked
// portal role with ErrTenantAccessDenied, the sentinel the refresh route
// maps.
func schoolFacts(ctx context.Context, s *AccountAuthentication, b sessionBinding, r factReads) (heldFacts, error) {
	school, found, err := r.school(ctx, s, b.tenantID)
	if err != nil {
		return heldFacts{}, fmt.Errorf("re-check school %d of a school session: %w", b.tenantID, err)
	}
	if !found || !school.Live() {
		return heldFacts{}, identityaccess.ErrTenantNotFound
	}
	mapped, err := r.mapping(ctx, s, b.accountID, b.tenantID)
	if err != nil {
		return heldFacts{}, fmt.Errorf("re-check membership of account %d at school %d: %w", b.accountID, b.tenantID, err)
	}
	if !mapped {
		return heldFacts{}, b.refuse(s, "tenant_access_revoked", identityaccess.ErrTenantAccessDenied)
	}
	roles, err := r.roles(ctx, s, b.accountID, b.tenantID)
	if err != nil {
		return heldFacts{}, fmt.Errorf("re-check school portal role of account %d at school %d: %w", b.accountID, b.tenantID, err)
	}
	if !domain.HasSchoolPortalRole(roles) {
		if b.renewal == mintSession {
			return heldFacts{}, identityaccess.ErrAccountNoSchoolPortalRole
		}
		return heldFacts{}, b.refuse(s, "school_portal_role_revoked", identityaccess.ErrTenantAccessDenied)
	}
	return heldFacts{roles: roles}, nil
}

// guardianAtBookedSchool: a guardian role at a live school that books
// Eltern-App (#4895). A renewal answers its loss with ErrTenantNotFound, the
// answer the refresh route gave when the successor found no school to be
// kept at.
func (b sessionBinding) guardianAtBookedSchool(s *AccountAuthentication, guardianSchools, booked []int64) error {
	if slices.ContainsFunc(guardianSchools, func(id int64) bool { return slices.Contains(booked, id) }) {
		return nil
	}
	if b.renewal == mintSession {
		return failed(b.op, identityaccess.ErrAccountNoGuardianRole)
	}
	return b.refuse(s, "no_live_guardian_school", failed("refresh parent session", identityaccess.ErrTenantNotFound))
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
