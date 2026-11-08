package application

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// ValidateSessionTokens verifies a frontend session hand-off (access token
// plus optional refresh token for one portal) without consuming any refresh
// recovery state. Every mismatch is ErrInvalidToken.
func (s *AccountAuthentication) ValidateSessionTokens(ctx context.Context, access, refresh, portal string) (_ identityaccess.SessionClaims, err error) {
	if s == nil {
		return identityaccess.SessionClaims{}, errAccountAuthenticationUnavailable
	}
	defer func() { err = authenticationError(err) }()
	ctx = bindRuntime(s.attach, ctx)
	claims, err := s.codec.ParseAccessToken(access)
	now := time.Now().Unix()
	if err != nil || claims.AccountID <= 0 || claims.ExpiresAt <= now || !domain.SessionScopeMatches(claims.Scope, portal) {
		return identityaccess.SessionClaims{}, identityaccess.ErrInvalidToken
	}
	if named, _ := domain.PortalNamed(portal); !named.Holds(claims.TenantID, claims.OrgID) {
		return identityaccess.SessionClaims{}, identityaccess.ErrInvalidToken
	}
	refreshClaims, err := s.parseSessionRefresh(refresh, claims, now)
	if err != nil {
		return identityaccess.SessionClaims{}, err
	}
	if refreshClaims == nil {
		if err := s.verifyAccessFamily(ctx, claims, portal); err != nil {
			return identityaccess.SessionClaims{}, err
		}
	}
	var renewal accountRenewal
	if portal != domain.PortalPlatform.Name {
		if renewal, err = s.renewAccountSession(ctx, claims); err != nil {
			return identityaccess.SessionClaims{}, err
		}
	}
	err = s.runtime.WithAdminTx(ctx, func(txCtx context.Context) error {
		if portal == domain.PortalPlatform.Name {
			return s.validateOperatorSession(txCtx, claims, refreshClaims)
		}
		return s.validateAccountSessionInTx(txCtx, renewal, claims, refreshClaims)
	})
	if err != nil {
		return identityaccess.SessionClaims{}, err
	}
	return identityaccess.SessionClaims(claims), nil
}

// verifyAccessFamily confirms that the refresh-token family an access token
// names still backs it, for a hand-off without its refresh token (#5399).
// The refresh row check covers the family otherwise. The BFF asks before it
// writes or shows a session, so the answer of a request that outlived its
// logout neither sets a cookie nor reads as signed in. ErrInvalidToken means
// revoked, expired or foreign; any other error means the check could not run.
func (s *AccountAuthentication) verifyAccessFamily(ctx context.Context, claims domain.SessionClaims, portal string) error {
	if portal != domain.PortalPlatform.Name {
		return s.VerifyAccessSession(ctx, identityaccess.AccessSession{
			AccountID: claims.AccountID, ActingAdminID: claims.ActingAdminID, Scope: claims.Scope, FamilyID: claims.FamilyID,
		})
	}
	if claims.FamilyID == "" {
		return identityaccess.ErrInvalidToken
	}
	head, err := s.sessions.LatestOperatorSessionInFamily(ctx, claims.FamilyID)
	if errors.Is(err, identityaccess.ErrOperatorSessionNotFound) {
		return identityaccess.ErrInvalidToken
	}
	if err != nil {
		return err
	}
	if head.OperatorID != claims.AccountID || !head.Expiry.After(time.Now()) {
		return identityaccess.ErrInvalidToken
	}
	return nil
}

func (s *AccountAuthentication) validateOperatorSession(ctx context.Context, claims domain.SessionClaims, refreshClaims *domain.RefreshClaims) error {
	operator, err := s.sessions.findOperatorByID(ctx, claims.AccountID)
	if errors.Is(err, identityaccess.ErrOperatorNotFound) {
		return identityaccess.ErrInvalidToken
	}
	if err != nil {
		return err
	}
	if !operator.Active {
		return identityaccess.ErrInvalidToken
	}
	if refreshClaims == nil {
		return nil
	}
	session, err := s.sessions.findOperatorSessionForUpdate(ctx, refreshClaims.Token)
	if errors.Is(err, identityaccess.ErrOperatorSessionNotFound) {
		return identityaccess.ErrInvalidToken
	}
	if err != nil {
		return err
	}
	if session.OperatorID != operator.ID || !session.Expiry.After(time.Now()) || session.RotatedAt != nil || session.ReplacementTokenHash != nil || session.FamilyID != claims.FamilyID {
		return identityaccess.ErrInvalidToken
	}
	return nil
}

func (s *AccountAuthentication) parseSessionRefresh(refresh string, claims domain.SessionClaims, now int64) (*domain.RefreshClaims, error) {
	var refreshClaims *domain.RefreshClaims
	if refresh != "" {
		parsed, err := s.codec.ParseRefreshToken(refresh)
		if err != nil || parsed.ExpiresAt <= now || parsed.AccountID != claims.AccountID || parsed.Scope != claims.Scope || parsed.TenantID != claims.TenantID || (claims.Scope == domain.PortalOrg.Claim && parsed.OrgID != claims.OrgID) || claims.ReadOnly || claims.FamilyID == "" {
			return nil, identityaccess.ErrInvalidToken
		}
		refreshClaims = &parsed
	}
	return refreshClaims, nil
}

// accountRenewal is what validation runs in its transaction: the kind of the
// session and its guard.
type accountRenewal struct {
	kind     domain.SessionKind
	eligible eligibility
}

// renewAccountSession asks whether the session would still renew: the
// renewal reads of its kind, as the refresh runs them, without rotating. A
// refusal makes the token invalid; a failed read stays an error.
func (s *AccountAuthentication) renewAccountSession(ctx context.Context, claims domain.SessionClaims) (accountRenewal, error) {
	kind, ok := claims.SessionKind()
	policy, known := s.sessionPolicy(kind)
	if !ok || !known || (claims.ViaTraeger && kind != domain.SessionKindTraegerSchool) {
		return accountRenewal{}, identityaccess.ErrInvalidToken
	}
	eligible, err := policy.renew(ctx, s, sessionBinding{op: "validate session", accountID: claims.AccountID, tenantID: claims.TenantID, orgID: claims.OrgID})
	if err != nil {
		return accountRenewal{}, sessionRefusal(err)
	}
	return accountRenewal{kind: kind, eligible: eligible}, nil
}

// validateAccountSessionInTx runs the guard of the session's kind and checks
// the presented refresh row.
func (s *AccountAuthentication) validateAccountSessionInTx(txCtx context.Context, renewal accountRenewal, claims domain.SessionClaims, refreshClaims *domain.RefreshClaims) error {
	account, found, _, err := findLoginAccount(txCtx, s.runtime, s.store, claims.AccountID, false)
	if err != nil {
		return err
	}
	if !found || !account.Active {
		return identityaccess.ErrInvalidToken
	}
	if _, err := s.eligibleClaims(txCtx, renewal.eligible, account); err != nil {
		return sessionRefusal(err)
	}
	if refreshClaims == nil {
		return nil
	}
	return s.validateRefreshRow(txCtx, renewal.kind, claims, *refreshClaims, account.ID)
}

func (s *AccountAuthentication) validateRefreshRow(ctx context.Context, kind domain.SessionKind, claims domain.SessionClaims, refreshClaims domain.RefreshClaims, accountID int64) error {
	session, err := s.sessions.findAccountSessionByToken(ctx, refreshClaims.Token)
	if errors.Is(err, identityaccess.ErrAccountSessionNotFound) {
		return identityaccess.ErrInvalidToken
	}
	if err != nil {
		return err
	}
	if session.AccountID != accountID || (claims.TenantID > 0 && session.TenantID != claims.TenantID) || !sameTraeger(kind, claims.OrgID, session) || session.PortalScope != domain.PersistedPortalScope(claims.Scope) || !session.Expiry.After(time.Now()) || session.RotatedAt != nil || session.ReplacementTokenHash != nil || session.FamilyID != claims.FamilyID {
		return identityaccess.ErrInvalidToken
	}
	return nil
}

// sessionRefusals are the answers of the renewal reads and guards that end a
// session; every other error is a read that could not run.
var sessionRefusals = []error{
	identityaccess.ErrInvalidToken,
	identityaccess.ErrAccountInactive,
	domain.ErrAccountNotFound,
	identityaccess.ErrTenantNotFound,
	identityaccess.ErrTenantAccessDenied,
	identityaccess.ErrParentMustUseParentPortal,
	identityaccess.ErrMustUseSchoolPortal,
	identityaccess.ErrAccountNoSchoolPortalRole,
	identityaccess.ErrAccountNoGuardianRole,
	identityaccess.ErrNoTraegerMembership,
	identityaccess.ErrTraegerSchoolSwitchForbidden,
	identityaccess.ErrTraegerSchoolNotFound,
	domain.ErrAccountNotCaterer,
}

func sessionRefusal(err error) error {
	if _, notBooked := identityaccess.PortalNotBooked(err); notBooked {
		return identityaccess.ErrInvalidToken
	}
	for _, refusal := range sessionRefusals {
		if errors.Is(err, refusal) {
			return identityaccess.ErrInvalidToken
		}
	}
	return err
}

// VerifyAccessSession confirms that the family an access token names still
// backs it (#5194). Logout and every revocation delete the family, so a
// cookie a late response writes back no longer opens anything, on every
// instance: the check reads the latest generation each time and caches
// nothing. A token without a family is refused; every session minted since
// families exist carries one.
func (s *AccountAuthentication) VerifyAccessSession(ctx context.Context, session identityaccess.AccessSession) (err error) {
	if s == nil {
		return errAccountAuthenticationUnavailable
	}
	defer func() { err = authenticationError(err) }()
	token := domain.AccessSession(session)
	if token.FamilyID == "" {
		return identityaccess.ErrInvalidToken
	}
	// The family is read across schools: a parent or caterer session is
	// pinned to one school, and a tenant the caller's context may carry must
	// not hide it. The token names account and portal; the predicate checks
	// both.
	head, err := s.sessions.LatestAccountSessionInFamily(s.runtime.Detach(bindRuntime(s.attach, ctx)), token.FamilyID)
	if errors.Is(err, identityaccess.ErrAccountSessionNotFound) {
		return identityaccess.ErrInvalidToken
	}
	if err != nil {
		return err
	}
	if !domain.AccessSessionLive(domain.AccountSession(head), token, time.Now()) {
		return identityaccess.ErrInvalidToken
	}
	return nil
}
