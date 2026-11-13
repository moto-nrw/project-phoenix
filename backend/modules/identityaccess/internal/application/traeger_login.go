package application

import (
	"context"
	"errors"
	"log/slog"
	"slices"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/domain"
)

// Träger-Portal sign-in (ADR 0084, #4649). The Träger session has scope org,
// names exactly one Träger and no school, and lives in its own session bucket.
// Its claims are the Träger roles and their rights; no school role or school
// permission enters it. The membership belongs to Organisation & Tenancy and
// arrives through the root-bound port; every mint and refresh re-checks it
// under the account lock.

var errTraegerMembershipsUnavailable = errors.New("identity access: Träger memberships are not composed")

// LoginTraeger checks the password, resolves the Träger of the account and
// asks for the second factor, or mints the Träger session where the second
// factor is not composed. A member of several Träger without a choice gets
// the list to choose from; a choice outside the memberships is refused like a
// missing membership.
func (s *AccountAuthentication) LoginTraeger(ctx context.Context, email, password, ipAddress, userAgent string, organizationID int64) (*identityaccess.TraegerLoginResult, error) {
	return runFlow(ctx, s.flow(), func(ctx context.Context) (*identityaccess.TraegerLoginResult, error) {
		return s.loginTraeger(ctx, email, password, ipAddress, userAgent, organizationID)
	})
}

func (s *AccountAuthentication) loginTraeger(ctx context.Context, email, password, ipAddress, userAgent string, organizationID int64) (*identityaccess.TraegerLoginResult, error) {
	if s.traeger == nil {
		return nil, failed("Träger login", errTraegerMembershipsUnavailable)
	}
	account, err := s.validateLoginCredentials(ctx, email, password, ipAddress, userAgent)
	if err != nil {
		return nil, err
	}
	organizations, err := s.listTraegerOrganizations(ctx, account.ID)
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
	if s.mfa.Configured() {
		return s.traegerSecondFactor(ctx, account, organizationID, ipAddress)
	}
	return s.mintTraegerSession(ctx, account, organizationID, nil, ipAddress, userAgent)
}

// traegerSecondFactor answers a correct password with the second factor
// (ADR 0084, #4651): every Träger session needs it, for every Träger role and
// independent of the mfa_mode of any school. An account without an enrolled
// factor sets it up first. The Träger the person chose travels back to the
// client, which names it again when it redeems the code; the mint checks the
// membership again.
func (s *AccountAuthentication) traegerSecondFactor(ctx context.Context, account domain.LoginAccount, organizationID int64, ipAddress string) (*identityaccess.TraegerLoginResult, error) {
	enrolled, err := s.mfa.HasEnrollment(ctx, account.ID)
	if err != nil {
		return nil, failed("check mfa enrollment", identityaccess.ErrMFAStatusUnavailable)
	}
	if !enrolled {
		result, err := s.mfaEnrollmentResult(account, 0, domain.PortalOrg)
		if err != nil {
			return nil, err
		}
		return &identityaccess.TraegerLoginResult{
			Status: identityaccess.TraegerLoginMFAEnrollmentRequired, AccessToken: result.AccessToken,
			MaskedEmail: result.MaskedEmail, OrganizationID: organizationID,
		}, nil
	}
	result, err := s.mfaChallengeResult(ctx, account, 0, domain.PortalOrg, ipAddress)
	if err != nil {
		return nil, err
	}
	return &identityaccess.TraegerLoginResult{
		Status: identityaccess.TraegerLoginMFARequired, ChallengeToken: result.ChallengeToken,
		MaskedEmail: result.MaskedEmail, OrganizationID: organizationID,
	}, nil
}

// IssueTraegerTokensForVerifiedMFA mints the Träger session once the second
// factor is proven. The proof binds the account; the Träger is the one the
// login resolved, or the only one of the account. The mint guard re-checks
// the membership and the MFA reset generation under the account lock, so a
// membership ended or a factor reset mid-challenge mints nothing.
func (s *AccountAuthentication) IssueTraegerTokensForVerifiedMFA(ctx context.Context, proof identityaccess.VerifiedMFAChallenge, organizationID int64, ipAddress, userAgent string) (*identityaccess.TraegerLoginResult, error) {
	return runFlow(ctx, s.flow(), func(ctx context.Context) (*identityaccess.TraegerLoginResult, error) {
		if s.traeger == nil {
			return nil, failed("Träger login", errTraegerMembershipsUnavailable)
		}
		if proof.Scope != identityaccess.MFAChallengeScopeOrg {
			return nil, failed("issue Träger tokens", identityaccess.ErrInvalidToken)
		}
		account, err := s.authenticatedAccount(ctx, "issue Träger tokens", proof.AccountID)
		if err != nil {
			return nil, err
		}
		organizations, err := s.listTraegerOrganizations(ctx, account.ID)
		if err != nil {
			return nil, failed("issue Träger tokens: list memberships", err)
		}
		if organizationID == 0 && len(organizations) == 1 {
			organizationID = organizations[0].ID
		}
		if !containsTraeger(organizations, organizationID) {
			s.logFailedLogin(ctx, account.ID, ipAddress, userAgent, "No Träger membership at Träger token issue")
			return nil, failed("issue Träger tokens", identityaccess.ErrNoTraegerMembership)
		}
		return s.mintTraegerSession(ctx, account, organizationID, &proof, ipAddress, userAgent)
	})
}

// mintTraegerSession persists the Träger session and issues its pair. A
// proof marks the session as verified by a second factor; the mint refuses
// an unverified Träger session wherever the second factor is composed.
func (s *AccountAuthentication) mintTraegerSession(ctx context.Context, account domain.LoginAccount, organizationID int64, proof *identityaccess.VerifiedMFAChallenge, ipAddress, userAgent string) (*identityaccess.TraegerLoginResult, error) {
	minted, err := s.mintAccountSession(ctx, mintRequest{
		portal: domain.PortalOrg, account: account, orgID: organizationID, proof: challengeProof(proof),
		audit: s.traegerLoginAudit(ctx, account.ID, ipAddress, userAgent),
	})
	if err != nil {
		return nil, err
	}
	return &identityaccess.TraegerLoginResult{Status: identityaccess.TraegerLoginAuthenticated, AccessToken: minted.accessToken, RefreshToken: minted.refreshToken}, nil
}

// CheckPasswordLogin checks an account's password like the login does, with
// its attempt counting and lockout, and returns the account. The Träger
// invitation proves an existing account with it (#4650). It must run outside
// the caller's transaction: a refused attempt commits on its own.
func (s *AccountAuthentication) CheckPasswordLogin(ctx context.Context, email, password, ipAddress, userAgent string) (int64, error) {
	if s == nil {
		return 0, errAccountAuthenticationUnavailable
	}
	account, err := s.validateLoginCredentials(bindRuntime(s.attach, ctx), email, password, ipAddress, userAgent)
	return account.ID, err
}

// traegerLoginAudit files the sign-in in the authentication ledger of the
// account's first school. That ledger is school-scoped; a person who works
// only in the Träger-Büro has no school to file it in (ADR 0084), so the
// sign-in is only logged.
func (s *AccountAuthentication) traegerLoginAudit(ctx context.Context, accountID int64, ipAddress, userAgent string) *mintAudit {
	schoolID, _, err := s.resolveAccountTenantDefault(ctx, accountID)
	if err != nil {
		s.logger.Info("Träger sign-in without a school ledger", slog.Int64("account_id", accountID))
		return nil
	}
	return &mintAudit{event: domain.AuthEventLogin, ipAddress: ipAddress, userAgent: userAgent, schoolID: schoolID}
}

// listTraegerOrganizations reads the Träger of the account outside the mint;
// the mint guard reads them again under the account lock.
func (s *AccountAuthentication) listTraegerOrganizations(ctx context.Context, accountID int64) ([]identityaccess.TraegerOrganization, error) {
	var organizations []identityaccess.TraegerOrganization
	err := globalIdentityRead(ctx, s.runtime, func(txCtx context.Context) error {
		var listErr error
		organizations, listErr = s.traeger.ListAccountTraegerOrganizations(txCtx, accountID)
		return listErr
	})
	return organizations, err
}

// isTraegerMember reports whether the account may sign in to the
// Träger-Portal: it holds an active membership of an active Träger. Without
// the memberships composed nobody is a member.
func (s *AccountAuthentication) isTraegerMember(ctx context.Context, accountID int64) (bool, error) {
	if s.traeger == nil {
		return false, nil
	}
	organizations, err := s.listTraegerOrganizations(ctx, accountID)
	return len(organizations) > 0, err
}

func containsTraeger(organizations []identityaccess.TraegerOrganization, organizationID int64) bool {
	return organizationID > 0 && slices.ContainsFunc(organizations, func(organization identityaccess.TraegerOrganization) bool {
		return organization.ID == organizationID
	})
}

// InAdminUnit reports an ambient administrative transaction: the unit the
// participating Träger commands of the operator join (#5144).
func (s *AccountAuthentication) InAdminUnit(ctx context.Context) bool {
	return s.runtime.HasTransaction(ctx) && s.runtime.IsAdminTx(ctx)
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
	// of unused accounts already anonymized gets no Träger access, and a
	// deactivated one is not reactivated by it (#6535). Both are read under
	// the account lock, so an offboarding that committed first is seen.
	live, active, _, err := s.store.LockLiveTraegerAccount(ctx, accountID)
	if err != nil {
		return err
	}
	if !live {
		return identityaccess.ErrAccountNotFound
	}
	if !active {
		return identityaccess.ErrAccountInactive
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
		result = append(result, identityaccess.TraegerAccount{AccountID: account.AccountID, Email: account.Email, Roles: account.Roles})
	}
	return result, nil
}

// EndTraegerSessions deletes every Träger session of the Träger: the moto team
// deactivated it (#4657). Refresh and hand-off of those sessions fail from
// the commit on; a Träger access token still in flight reaches no school,
// because the Träger projection serves only an active Träger. It participates
// in the caller's administrative transaction and never opens one itself.
func (s *AccountAuthentication) EndTraegerSessions(ctx context.Context, organizationID int64) (int64, error) {
	if s == nil {
		return 0, errAccountAuthenticationUnavailable
	}
	if organizationID <= 0 {
		return 0, identityaccess.ErrInvalidToken
	}
	if !s.runtime.HasTransaction(ctx) || !s.runtime.IsAdminTx(ctx) {
		return 0, identityaccess.ErrTraegerTransactionRequired
	}
	ended, _, err := s.store.DeleteTraegerSessions(ctx, organizationID)
	if err != nil {
		return 0, failed("end Träger sessions", err)
	}
	return ended, nil
}

// hasRetainedTraegerMembership is an account lifecycle fact, independent of
// the stricter membership and organization eligibility used by sign-in.
func (s *AccountAuthentication) hasRetainedTraegerMembership(ctx context.Context, accountID int64) (bool, error) {
	if s.traeger == nil {
		return false, errTraegerMembershipsUnavailable
	}
	return s.traeger.HasRetainedTraegerMembership(ctx, accountID)
}
