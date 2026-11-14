package application

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/ports"
)

// ValidateSessionTokens verifies a frontend session hand-off (access token
// plus optional refresh token for one portal) without consuming any refresh
// recovery state. It asks whether the session would still renew: the facts
// its refresh decides, through the refresh's own definitions, without locks
// and without building claims, because validation writes nothing. A refusal carries the sentinel the
// refresh answers with (identityaccess.SessionOutcomeOf); any other error is
// a read that could not run.
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
	named, _ := domain.PortalNamed(portal)
	if !named.Holds(claims.TenantID, claims.OrgID) {
		return identityaccess.SessionClaims{}, identityaccess.ErrInvalidToken
	}
	refreshClaims, err := s.parseSessionRefresh(refresh, claims, now)
	if err != nil {
		return identityaccess.SessionClaims{}, err
	}
	check := s.accountSessionCheck
	if named.MFA == domain.MFAOperator {
		check = s.operatorSessionCheck
	}
	inTx, err := check(ctx, claims, refreshClaims)
	if err != nil {
		return identityaccess.SessionClaims{}, err
	}
	var booked func(context.Context) error
	err = s.runtime.WithAdminTx(ctx, func(txCtx context.Context) error {
		var err error
		booked, err = inTx(txCtx)
		return err
	})
	if err == nil && booked != nil {
		// The portal port reads the module booking in a transaction of its
		// own, after the session's own facts.
		err = booked(ctx)
	}
	if err != nil {
		s.logValidationRefusal(err, claims, portal)
		return identityaccess.SessionClaims{}, err
	}
	return identityaccess.SessionClaims(claims), nil
}

// sessionFacts reads what a session rests on inside the validation's
// transaction and returns the booking read to run after it, if any.
type sessionFacts func(txCtx context.Context) (booked func(context.Context) error, err error)

// operatorSessionCheck prepares the validation of an operator hand-off:
// operators run their own pipeline.
func (s *AccountAuthentication) operatorSessionCheck(ctx context.Context, claims domain.SessionClaims, refreshClaims *domain.RefreshClaims) (sessionFacts, error) {
	if refreshClaims == nil {
		if err := s.verifyOperatorFamily(ctx, claims); err != nil {
			return nil, err
		}
	}
	return func(txCtx context.Context) (func(context.Context) error, error) {
		return nil, s.validateOperatorSession(txCtx, claims, refreshClaims)
	}, nil
}

// accountSessionCheck prepares the validation of an account hand-off of any
// portal.
func (s *AccountAuthentication) accountSessionCheck(_ context.Context, claims domain.SessionClaims, refreshClaims *domain.RefreshClaims) (sessionFacts, error) {
	handOff, err := accountHandOffOf(claims)
	if err != nil {
		return nil, err
	}
	return func(txCtx context.Context) (func(context.Context) error, error) {
		return s.validateAccountSessionInTx(txCtx, handOff, claims, refreshClaims)
	}, nil
}

// accountHandOff is the session an account access token names: its kind,
// the rules of that kind and what the session rests on.
type accountHandOff struct {
	kind    domain.SessionKind
	policy  sessionPolicy
	binding sessionBinding
}

func accountHandOffOf(claims domain.SessionClaims) (accountHandOff, error) {
	kind, ok := claims.SessionKind()
	policy, known := sessionPolicies[kind]
	if !ok || !known || claims.FamilyID == "" || (claims.ViaTraeger && kind != domain.SessionKindTraegerSchool) {
		return accountHandOff{}, identityaccess.ErrInvalidToken
	}
	return accountHandOff{
		kind:    kind,
		policy:  policy,
		binding: sessionBinding{op: "validate session", renewal: validateSession, accountID: claims.AccountID, tenantID: claims.TenantID, orgID: claims.OrgID},
	}, nil
}

// validateAccountSessionInTx reads the family (or the presented refresh
// row), the account and the facts of the session's kind on one
// transaction, and returns the booking check that runs after it.
func (s *AccountAuthentication) validateAccountSessionInTx(txCtx context.Context, handOff accountHandOff, claims domain.SessionClaims, refreshClaims *domain.RefreshClaims) (func(context.Context) error, error) {
	var session domain.AccountSession
	if refreshClaims == nil {
		head, err := s.sessions.LatestAccountSessionInFamily(txCtx, claims.FamilyID)
		if err := familyBacks(head, err, domain.AccessSession{
			AccountID: claims.AccountID, ActingAdminID: claims.ActingAdminID, Scope: claims.Scope, FamilyID: claims.FamilyID,
		}); err != nil {
			return nil, err
		}
		session = domain.AccountSession(head)
	}
	account, found, _, err := findLoginAccount(txCtx, s.runtime, s.store, claims.AccountID, false)
	if err != nil {
		return nil, err
	}
	if !found || !account.Active {
		return nil, identityaccess.ErrInvalidToken
	}
	if refreshClaims != nil {
		if session, err = s.validateRefreshRow(txCtx, handOff.kind, claims, *refreshClaims, account.ID); err != nil {
			return nil, err
		}
	}
	return handOff.policy.check(txCtx, s, handOff.binding, session)
}

// logValidationRefusal records a refused validation once, apart from the
// refresh's events. Every page view of a refused session asks, so a refusal
// is Info; a read that failed, the second-factor state included, is Warn. A missing module is already logged by
// the portal check (logPortalRefusal).
func (s *AccountAuthentication) logValidationRefusal(err error, claims domain.SessionClaims, portal string) {
	attrs := []any{
		slog.String("portal", portal),
		slog.Int64("account_id", claims.AccountID),
		slog.Int64("tenant_id", claims.TenantID),
		slog.String("reason", err.Error()),
	}
	switch identityaccess.SessionOutcomeOf(authenticationError(err)) {
	case identityaccess.SessionModuleNotBooked:
	case identityaccess.SessionUnclassified, identityaccess.SessionUndecided:
		s.logger.Warn("session_validation_failed", attrs...)
	default:
		s.logger.Info("session_validation_refused", attrs...)
	}
}

// verifyOperatorFamily confirms that the operator family an access token
// names still backs it, for a hand-off without its refresh token (#5399).
// The BFF asks before it writes or shows a session, so the answer of a
// request that outlived its logout neither sets a cookie nor reads as signed
// in.
func (s *AccountAuthentication) verifyOperatorFamily(ctx context.Context, claims domain.SessionClaims) error {
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
	session, err := s.sessions.findOperatorSessionByToken(ctx, refreshClaims.Token)
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
		if err != nil || parsed.ExpiresAt <= now || parsed.AccountID != claims.AccountID || parsed.Scope != claims.Scope || parsed.TenantID != claims.TenantID || !refreshNamesTokenTraeger(parsed, claims) || claims.ReadOnly || claims.FamilyID == "" {
			return nil, identityaccess.ErrInvalidToken
		}
		refreshClaims = &parsed
	}
	return refreshClaims, nil
}

// refreshNamesTokenTraeger: the refresh token of a Träger session and of a
// parent session names the Träger its access token names. A staff access
// token names its school's Träger, which its refresh token does not.
func refreshNamesTokenTraeger(refresh domain.RefreshClaims, access domain.SessionClaims) bool {
	switch access.Scope {
	case domain.PortalOrg.Claim, domain.PortalParent.Claim:
		return refresh.OrgID == access.OrgID
	default:
		return true
	}
}

func (s *AccountAuthentication) validateRefreshRow(ctx context.Context, kind domain.SessionKind, claims domain.SessionClaims, refreshClaims domain.RefreshClaims, accountID int64) (domain.AccountSession, error) {
	session, err := s.sessions.findAccountSessionByToken(ctx, refreshClaims.Token)
	if errors.Is(err, identityaccess.ErrAccountSessionNotFound) {
		return domain.AccountSession{}, identityaccess.ErrInvalidToken
	}
	if err != nil {
		return domain.AccountSession{}, err
	}
	if session.AccountID != accountID || (claims.TenantID > 0 && session.TenantID != claims.TenantID) || !sameTraeger(kind, claims.OrgID, session) || session.PortalScope != domain.PersistedPortalScope(claims.Scope) || !session.Expiry.After(time.Now()) || session.RotatedAt != nil || session.ReplacementTokenHash != nil || session.FamilyID != claims.FamilyID {
		return domain.AccountSession{}, identityaccess.ErrInvalidToken
	}
	return session, nil
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
	// not hide it.
	head, err := s.sessions.LatestAccountSessionInFamily(s.runtime.Detach(bindRuntime(s.attach, ctx)), token.FamilyID)
	return familyBacks(head, err, token)
}

// familyBacks judges the latest generation of the family a token names; the
// predicate checks account and portal. ErrInvalidToken means revoked,
// expired or foreign; any other error is the read that failed.
func familyBacks(head ports.AccountSession, err error, token domain.AccessSession) error {
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
