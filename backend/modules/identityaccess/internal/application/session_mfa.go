package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/domain"
)

// requireSessionMFA runs under the policy/account lock at every persistence
// boundary, including refresh hand-off recovery, and without locks at a
// session validation (r). Legacy sessions have no proof.
// A Träger session minted before the duty or one whose proof an MFA reset
// revoked is refused at mint, refresh and validation. A caterer or parent
// session needs its proof once the account has a second factor (#4497,
// #7051): a session minted before that ends at its next refresh, and at its
// next validation where the portal reads the enrollment there
// (domain.Portal.FactorAtValidation).
func (s *AccountAuthentication) requireSessionMFA(ctx context.Context, accountID, tenantID int64, scope string, verified bool, r factReads) error {
	if !s.mfa.Configured() {
		return nil
	}
	switch domain.MFARuleOf(scope) {
	case domain.MFAAlways:
		if !verified {
			return identityaccess.ErrInvalidToken
		}
		return nil
	case domain.MFAAccountChoice:
		if portal, _ := domain.PortalOfPersisted(scope); !r.locked && !portal.FactorAtValidation {
			return nil
		}
		return s.requireAccountChoiceSessionMFA(ctx, accountID, verified)
	case domain.MFASchoolPolicy:
	default:
		return nil
	}
	if err := r.permissionSources(ctx, s, accountID, tenantID); err != nil {
		return identityaccess.ErrMFAStatusUnavailable
	}
	permissions, err := s.loadAccountPermissionsForTenant(ctx, accountID, tenantID)
	if err != nil {
		return identityaccess.ErrMFAStatusUnavailable
	}
	policy, err := s.mfa.ResolvePolicyInTx(ctx, accountID, tenantID)
	if err != nil {
		return identityaccess.ErrMFAStatusUnavailable
	}
	if policy.RequiredFor(permissions) && !verified {
		return identityaccess.ErrInvalidToken
	}
	return nil
}

// requireAccountChoiceSessionMFA reads the account's own choice: its
// enrollment, never a school's mode, on the mint's or the refresh's
// transaction. A password mint that races the switch-on commits a session
// without proof; like every session from before the switch-on, it ends at
// its next refresh, which reads the committed enrollment.
func (s *AccountAuthentication) requireAccountChoiceSessionMFA(ctx context.Context, accountID int64, verified bool) error {
	if verified {
		return nil
	}
	enrolled, err := s.mfa.HasEnrollment(ctx, accountID)
	if err != nil {
		return identityaccess.ErrMFAStatusUnavailable
	}
	if enrolled {
		return identityaccess.ErrInvalidToken
	}
	return nil
}

// The signed access token supplies the family, not a client-provided MFA flag.
// Read its current generation after taking the account lock shared with MFA
// reset. A revoked, expired or foreign-portal family cannot restore proof,
// even while its old access JWT remains valid.
func (s *AccountAuthentication) switchMFAProof(ctx context.Context, accountID int64, familyID, scope string) (bool, error) {
	source, err := s.sessions.LatestAccountSessionInFamily(ctx, familyID)
	if err != nil {
		if errors.Is(err, identityaccess.ErrAccountSessionNotFound) {
			return false, nil
		}
		return false, identityaccess.ErrMFAStatusUnavailable
	}
	now := time.Now()
	return source.AccountID == accountID && source.PortalScope == scope &&
		source.MFAVerified && source.RotatedAt == nil && source.Expiry.After(now) &&
		(source.FamilyExpiryCap == nil || source.FamilyExpiryCap.After(now)), nil
}

// adoptSourceFamily checks the family a switch presents: it must still live,
// and a tenant or school switch inherits its second-factor proof.
func (s *AccountAuthentication) adoptSourceFamily(ctx context.Context, accountID int64, familyID string, session *domain.AccountSession, now time.Time) error {
	source, err := s.liveSourceFamily(ctx, accountID, familyID, now)
	if err != nil {
		return err
	}
	if familyID == "" {
		return nil
	}
	// The link is written under the account lock logout takes: a logout
	// that runs after this switch commits finds the new family through it
	// (#5422); one that ran before left no source family to switch from.
	session.SwitchedFromFamilyID = domain.SwitchLineage(source)
	// A family issued before #5422 cannot escape its transition deadline
	// by switching on: the new family keeps it (capLegacyLineage).
	if deadline := legacyDeadline(source); deadline != nil {
		session.FamilyExpiryCap = deadline
		session.LineageDeadline = deadline
		if deadline.Before(session.Expiry) {
			session.Expiry = *deadline
		}
	}
	// The Absprung's proof re-check already checked the Träger proof under this lock;
	// it must not be replaced by a same-school-portal proof lookup.
	if session.SessionKind() == domain.SessionKindTraegerSchool && source.SessionKind() == domain.SessionKind(domain.PortalOrg.Name) {
		return nil
	}
	if portal, ok := domain.PortalOfPersisted(session.PortalScope); !ok || portal.MFA != domain.MFASchoolPolicy {
		return nil
	}
	verified, err := s.switchMFAProof(ctx, accountID, familyID, session.PortalScope)
	if err != nil {
		return err
	}
	session.MFAVerified = verified
	return nil
}

// liveSourceFamily returns the presented family's head, refusing a switch
// whose presented family is gone (#5194). It runs
// under the account lock logout takes as well, so a switch that started
// before a logout either commits first or finds the family revoked; it
// cannot mint a new session out of a revoked one.
func (s *AccountAuthentication) liveSourceFamily(ctx context.Context, accountID int64, familyID string, now time.Time) (domain.AccountSession, error) {
	if familyID == "" {
		return domain.AccountSession{}, nil
	}
	source, err := s.sessions.LatestAccountSessionInFamily(ctx, familyID)
	if errors.Is(err, identityaccess.ErrAccountSessionNotFound) {
		return domain.AccountSession{}, identityaccess.ErrInvalidToken
	}
	if err != nil {
		return domain.AccountSession{}, fmt.Errorf("inspect source session family: %w", err)
	}
	if source.AccountID != accountID || !source.Expiry.After(now) {
		return domain.AccountSession{}, identityaccess.ErrInvalidToken
	}
	return domain.AccountSession(source), nil
}
