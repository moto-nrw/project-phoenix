package application

import (
	"context"
	"errors"
	"log/slog"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// Träger-Portal sign-in (ADR 0084, #4649). The Träger session has scope org,
// names exactly one Träger and no school, and lives in its own session bucket.
// Its claims are the Träger roles and their rights; no school role or school
// permission enters it. The membership belongs to Organisation & Tenancy and
// arrives through the root-bound port; every mint and refresh re-checks it
// under the account lock.

var errTraegerMembershipsUnavailable = errors.New("identity access: Träger memberships are not composed")

// LoginTraeger checks the password, resolves the Träger of the account and
// mints a Träger session. A member of several Träger without a choice gets
// the list to choose from; a choice outside the memberships is refused like a
// missing membership.
func (s *AccountAuthentication) LoginTraeger(ctx context.Context, email, password, ipAddress, userAgent string, organizationID int64) (_ *identityaccess.TraegerLoginResult, err error) {
	if s == nil {
		return nil, errAccountAuthenticationUnavailable
	}
	defer func() { err = authenticationError(err) }()
	ctx = bindRuntime(s.attach, ctx)
	if s.traeger == nil {
		return nil, failed("Träger login", errTraegerMembershipsUnavailable)
	}
	account, err := s.validateLoginCredentials(ctx, email, password, ipAddress, userAgent)
	if err != nil {
		return nil, err
	}
	var organizations []identityaccess.TraegerOrganization
	err = globalIdentityRead(ctx, s.runtime, func(txCtx context.Context) error {
		var listErr error
		organizations, listErr = s.traeger.ListAccountTraegerOrganizations(txCtx, account.ID)
		return listErr
	})
	if err != nil {
		return nil, failed("Träger login: list memberships", err)
	}
	if organizationID == 0 && len(organizations) > 1 {
		return &identityaccess.TraegerLoginResult{Status: identityaccess.TraegerLoginSelectionRequired, Organizations: organizations}, nil
	}
	if organizationID == 0 && len(organizations) == 1 {
		organizationID = organizations[0].ID
	}
	if !containsTraeger(organizations, organizationID) {
		s.logFailedLogin(ctx, account.ID, ipAddress, userAgent, "No Träger membership at Träger login")
		return nil, failed("Träger login", identityaccess.ErrNoTraegerMembership)
	}
	var metadata *domain.AccountClaimsPayload
	session := s.newRefreshSession(account.ID, domain.ScopeOrg)
	session.OrganizationID = organizationID
	session, err = s.persistNewSession(ctx, account, session, 0, s.traegerMintGuard(organizationID, &metadata), "")
	if err != nil {
		return nil, err
	}
	access, refresh := buildClaims(account, session, metadata, account.Email)
	accessToken, refreshToken, err := s.codec.IssueTokenPair(access, refresh)
	if err != nil {
		return nil, failed("generate tokens", err)
	}
	s.recordTraegerLogin(ctx, account.ID, ipAddress, userAgent)
	return &identityaccess.TraegerLoginResult{Status: identityaccess.TraegerLoginAuthenticated, AccessToken: accessToken, RefreshToken: refreshToken}, nil
}

// traegerMintGuard re-checks the Träger membership and loads the Träger roles
// inside the session transaction, under the account lock: the claims and the
// session commit together or not at all.
func (s *AccountAuthentication) traegerMintGuard(organizationID int64, out **domain.AccountClaimsPayload) mintGuard {
	return func(ctx context.Context, account domain.LoginAccount) error {
		if s.traeger == nil {
			return errTraegerMembershipsUnavailable
		}
		organizations, err := s.traeger.ListAccountTraegerOrganizations(ctx, account.ID)
		if err != nil {
			return err
		}
		if !containsTraeger(organizations, organizationID) {
			return failed("Träger session", identityaccess.ErrNoTraegerMembership)
		}
		roles, _, err := s.store.ListTraegerRoles(ctx, organizationID, account.ID)
		if err != nil {
			return err
		}
		if roles == nil {
			roles = []string{}
		}
		*out = &domain.AccountClaimsPayload{
			RoleNames:   roles,
			Permissions: identityaccess.TraegerRights(roles),
			Username:    account.Username,
			OrgID:       organizationID,
			Scope:       domain.ScopeOrg,
		}
		return nil
	}
}

// recordTraegerLogin files the sign-in in the authentication ledger of the
// account's first school. That ledger is school-scoped; a person who works
// only in the Träger-Büro has no school to file it in (ADR 0084), so the
// sign-in is only logged.
func (s *AccountAuthentication) recordTraegerLogin(ctx context.Context, accountID int64, ipAddress, userAgent string) {
	if ipAddress == "" {
		return
	}
	if _, _, err := s.resolveAccountTenantDefault(ctx, accountID); err != nil {
		s.logger.Info("Träger sign-in without a school ledger", slog.Int64("account_id", accountID))
		return
	}
	if err := s.logAuthEvent(ctx, accountID, domain.AuthEventLogin, true, ipAddress, userAgent, ""); err != nil {
		s.logger.Error("failed to audit Träger sign-in",
			slog.Int64("account_id", accountID),
			slog.Any("error", err))
	}
}

func containsTraeger(organizations []identityaccess.TraegerOrganization, organizationID int64) bool {
	return organizationID > 0 && slices.ContainsFunc(organizations, func(organization identityaccess.TraegerOrganization) bool {
		return organization.ID == organizationID
	})
}

// AssignTraegerRole grants a Träger role. It participates in the caller's
// administrative transaction, which the Träger membership command of
// Organisation & Tenancy owns; it never opens one itself.
func (s *AccountAuthentication) AssignTraegerRole(ctx context.Context, organizationID, accountID int64, role string) error {
	if s == nil {
		return errAccountAuthenticationUnavailable
	}
	if !identityaccess.IsTraegerRole(role) {
		return identityaccess.ErrInvalidTraegerRole
	}
	if organizationID <= 0 || accountID <= 0 {
		return identityaccess.ErrAccountNotFound
	}
	if !s.runtime.HasTransaction(ctx) || !s.runtime.IsAdminTx(ctx) {
		return identityaccess.ErrTraegerTransactionRequired
	}
	// The membership and this grant commit together; an account the erasure
	// of unused accounts already anonymized gets no Träger access.
	live, _, err := s.store.LockLiveTraegerAccount(ctx, accountID)
	if err != nil {
		return err
	}
	if !live {
		return identityaccess.ErrAccountNotFound
	}
	_, err = s.store.InsertTraegerRole(ctx, organizationID, accountID, role)
	return err
}

// ListTraegerAccounts returns e-mail and Träger roles of the given accounts
// on the caller's administrative transaction.
func (s *AccountAuthentication) ListTraegerAccounts(ctx context.Context, organizationID int64, accountIDs []int64) ([]identityaccess.TraegerAccount, error) {
	if s == nil {
		return nil, errAccountAuthenticationUnavailable
	}
	if !s.runtime.HasTransaction(ctx) || !s.runtime.IsAdminTx(ctx) {
		return nil, identityaccess.ErrTraegerTransactionRequired
	}
	accounts, _, err := s.store.ListTraegerAccounts(ctx, organizationID, accountIDs)
	if err != nil {
		return nil, err
	}
	result := make([]identityaccess.TraegerAccount, 0, len(accounts))
	for _, account := range accounts {
		result = append(result, identityaccess.TraegerAccount(account))
	}
	return result, nil
}
