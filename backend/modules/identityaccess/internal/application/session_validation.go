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
	if (portal == "tenant" || portal == "school") && claims.TenantID <= 0 {
		return identityaccess.SessionClaims{}, identityaccess.ErrInvalidToken
	}
	// A Träger session names its Träger and no school (ADR 0084).
	if portal == domain.ScopeOrg && (claims.OrgID <= 0 || claims.TenantID != 0) {
		return identityaccess.SessionClaims{}, identityaccess.ErrInvalidToken
	}
	refreshClaims, err := s.parseSessionRefresh(refresh, claims, now)
	if err != nil {
		return identityaccess.SessionClaims{}, err
	}
	err = s.runtime.WithAdminTx(ctx, func(txCtx context.Context) error {
		return s.validateSessionInTx(txCtx, claims, refreshClaims, portal)
	})
	if err != nil {
		return identityaccess.SessionClaims{}, err
	}
	return identityaccess.SessionClaims(claims), nil
}

func (s *AccountAuthentication) validateOperatorSession(ctx context.Context, claims domain.SessionClaims, refreshClaims *domain.RefreshClaims) error {
	operator, err := s.sessions.findOperatorByID(ctx, claims.AccountID)
	if err != nil || !operator.Active {
		return identityaccess.ErrInvalidToken
	}
	if refreshClaims == nil {
		return nil
	}
	session, err := s.sessions.findOperatorSessionForUpdate(ctx, refreshClaims.Token)
	if err != nil || session.OperatorID != operator.ID || !session.Expiry.After(time.Now()) || session.RotatedAt != nil || session.ReplacementTokenHash != nil || session.FamilyID != claims.FamilyID {
		return identityaccess.ErrInvalidToken
	}
	return nil
}

func (s *AccountAuthentication) parseSessionRefresh(refresh string, claims domain.SessionClaims, now int64) (*domain.RefreshClaims, error) {
	var refreshClaims *domain.RefreshClaims
	if refresh != "" {
		parsed, err := s.codec.ParseRefreshToken(refresh)
		if err != nil || parsed.ExpiresAt <= now || parsed.AccountID != claims.AccountID || parsed.Scope != claims.Scope || parsed.TenantID != claims.TenantID || (claims.Scope == domain.ScopeOrg && parsed.OrgID != claims.OrgID) || claims.ReadOnly || claims.FamilyID == "" {
			return nil, identityaccess.ErrInvalidToken
		}
		refreshClaims = &parsed
	}
	return refreshClaims, nil
}

func (s *AccountAuthentication) validateSessionInTx(txCtx context.Context, claims domain.SessionClaims, refreshClaims *domain.RefreshClaims, portal string) error {
	if portal == "platform" {
		return s.validateOperatorSession(txCtx, claims, refreshClaims)
	}
	account, found, _, err := findLoginAccount(txCtx, s.runtime, s.store, claims.AccountID, false)
	if err != nil || !found || !account.Active {
		return identityaccess.ErrInvalidToken
	}
	if !s.sessionReachesSchool(txCtx, account.ID, claims, portal) {
		return identityaccess.ErrInvalidToken
	}
	if portal == domain.ScopeOrg && !s.activeTraegerMember(txCtx, account.ID, claims.OrgID) {
		return identityaccess.ErrInvalidToken
	}
	if refreshClaims == nil {
		return nil
	}
	session, err := s.sessions.findAccountSessionByToken(txCtx, refreshClaims.Token)
	if err != nil || session.AccountID != account.ID || (claims.TenantID > 0 && session.TenantID != claims.TenantID) || !sessionOfTraeger(session, claims) || session.PortalScope != domain.PersistedPortalScope(claims.Scope) || !session.Expiry.After(time.Now()) || session.RotatedAt != nil || session.ReplacementTokenHash != nil || session.FamilyID != claims.FamilyID {
		return identityaccess.ErrInvalidToken
	}
	return nil
}

// activeTraegerMember re-reads the Träger membership a Träger session rests
// on; any failure reads as no membership.
func (s *AccountAuthentication) activeTraegerMember(ctx context.Context, accountID, organizationID int64) bool {
	if s.traeger == nil {
		return false
	}
	organizations, err := s.traeger.ListAccountTraegerOrganizations(ctx, accountID)
	return err == nil && containsTraeger(organizations, organizationID)
}

// sessionReachesSchool re-checks the school of a school-bound token: an
// active mapping, or for a Träger school session the Träger, which must still
// grant the school (ADR 0084 section 5). A token without a school passes.
func (s *AccountAuthentication) sessionReachesSchool(ctx context.Context, accountID int64, claims domain.SessionClaims, portal string) bool {
	if claims.ViaTraeger {
		return portal == "tenant" && claims.TenantID > 0 && claims.OrgID > 0 && s.traegerGrantsSchool(ctx, accountID, claims.OrgID, claims.TenantID)
	}
	if claims.TenantID <= 0 {
		return true
	}
	active, _, err := s.store.LockActiveTenantMappingShared(ctx, accountID, claims.TenantID)
	return err == nil && active
}

// traegerGrantsSchool re-checks a Träger school session: active membership,
// the right "In OGS wechseln" and a live school of the Träger. Any failure
// reads as no access.
func (s *AccountAuthentication) traegerGrantsSchool(ctx context.Context, accountID, organizationID, schoolID int64) bool {
	if s.traeger == nil || s.requireTraegerSchoolSwitch(ctx, accountID, organizationID) != nil {
		return false
	}
	school, found, err := s.schools.FindSchool(ctx, schoolID)
	return err == nil && found && !school.Deleted && school.Active && school.OrganizationID == organizationID
}

// sessionOfTraeger reports whether a token's session names the Träger the
// token names: a Träger token its Träger, a Träger school token the Träger it
// came through (#4654); every other session names none.
func sessionOfTraeger(session domain.AccountSession, claims domain.SessionClaims) bool {
	switch {
	case claims.Scope == domain.ScopeOrg:
		return session.OrganizationID == claims.OrgID
	case claims.ViaTraeger:
		return domain.IsTraegerSchoolSession(session) && session.OrganizationID == claims.OrgID
	default:
		return session.OrganizationID == 0
	}
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
	head, err := s.sessions.LatestAccountSessionInFamily(bindRuntime(s.attach, ctx), token.FamilyID)
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
