package application

import (
	"context"
	"log/slog"
	"net"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// The caterer's second factor (#4497, ADR 0077 point 7). The caterer
// switches it on itself; no school or Träger can require it. It is the
// account's e-mail factor, its passkeys and its remembered devices, as on
// the school portals, but bound to the caterer's account and to no school:
// its challenges, devices and ceremonies name none, its policy is the
// registry default, and its authentication events are logged like the
// Träger's, without a school ledger. The caterer's session row stays kept at
// a supplied school, as a storage key only.

// catererHost is the host of the caterer portal's URL, or empty.
func catererHost(frontendURL string) string {
	if strings.TrimSpace(frontendURL) == "" {
		return ""
	}
	host, err := domain.OriginHostWithoutPort(frontendURL)
	if err != nil {
		return ""
	}
	return host
}

// errCatererMFAUnavailable refuses the second factor of a module composed
// without it.
var errCatererMFAUnavailable = identityaccess.ErrAccountMFAUnavailable

// tenantless marks a second-factor flow of the caterer: it names no
// school, so its events are logged without a school ledger. Its policy is
// the registry default, which the settings binding resolves for the zero
// school whatever school a context carries.
func (c *CatererAccess) tenantless(ctx context.Context) context.Context {
	return withoutSchoolLedgerContext(c.attach(ctx))
}

// LoginCaterer signs a caterer account in to the caterer portal. A caterer
// that switched its second factor on answers with a mailed code, unless the
// browser presents a device the caterer remembered.
func (c *CatererAccess) LoginCaterer(ctx context.Context, request identityaccess.CatererLoginRequest) (identityaccess.CatererLoginResult, error) {
	result, err := c.login(c.attach(ctx), request)
	return result, catererMFAError(err)
}

func (c *CatererAccess) login(ctx context.Context, request identityaccess.CatererLoginRequest) (identityaccess.CatererLoginResult, error) {
	account, err := c.auth.validateLoginCredentials(ctx, request.Email, request.Password, request.IPAddress, request.UserAgent)
	if err != nil {
		return identityaccess.CatererLoginResult{}, err
	}
	_, schools, err := c.catererSchools(ctx, account.ID)
	if err != nil {
		c.auth.logFailedLogin(ctx, account.ID, request.IPAddress, request.UserAgent, "Not a caterer")
		return identityaccess.CatererLoginResult{}, err
	}
	if len(schools) == 0 {
		// No school to file the event at: the refusal is logged only.
		c.logger.Info("caterer login refused: no school", slog.Int64("account_id", account.ID))
		return identityaccess.CatererLoginResult{}, failed("caterer login", domain.ErrCatererWithoutSchool)
	}
	gate, err := c.signInGate(ctx, account, request)
	if err != nil {
		return identityaccess.CatererLoginResult{}, err
	}
	if gate.challenge != nil {
		return *gate.challenge, nil
	}
	return c.mintSession(ctx, account, schools[0], gate.verified, gate.guard, request.IPAddress, request.UserAgent)
}

// catererSignInGate is the outcome of the second-factor check of a sign-in:
// a challenge to answer, or a session to mint, proven by a remembered
// device where the caterer switched the factor on.
type catererSignInGate struct {
	challenge *identityaccess.CatererLoginResult
	verified  bool
	guard     mintGuard
}

func (c *CatererAccess) signInGate(ctx context.Context, account domain.LoginAccount, request identityaccess.CatererLoginRequest) (catererSignInGate, error) {
	if !c.auth.mfa.Configured() {
		return catererSignInGate{}, nil
	}
	schoolless := c.tenantless(ctx)
	enrolled, err := c.auth.mfa.HasEnrollment(schoolless, account.ID)
	if err != nil {
		return catererSignInGate{}, failed("check caterer mfa enrollment", identityaccess.ErrMFAStatusUnavailable)
	}
	if !enrolled {
		return catererSignInGate{}, nil
	}
	if c.auth.trustedDeviceVerified(schoolless, account.ID, 0, request.TrustedDeviceCookie) {
		// A reset may commit between this check and the mint: the guard
		// re-checks the factor and the device under the account lock.
		guard := c.auth.trustedDeviceMintGuard(account.ID, 0, request.TrustedDeviceCookie, nil)
		return catererSignInGate{verified: true, guard: func(guardCtx context.Context, presented domain.LoginAccount) error {
			return guard(c.tenantless(guardCtx), presented)
		}}, nil
	}
	result, err := c.auth.mfaChallengeResult(schoolless, account, 0, domain.ScopeCaterer, request.IPAddress)
	if err != nil {
		return catererSignInGate{}, err
	}
	challenge := identityaccess.CatererLoginResult{
		Status: identityaccess.CatererLoginMFARequired, ChallengeToken: result.ChallengeToken, MaskedEmail: result.MaskedEmail,
	}
	if result.TrustedDeviceEnabled {
		challenge.TrustedDeviceDays = result.TrustedDeviceDays
	}
	return catererSignInGate{challenge: &challenge}, nil
}

// mintSession persists the caterer session at the school it is kept at and
// issues its pair. The login is filed at that school.
func (c *CatererAccess) mintSession(ctx context.Context, account domain.LoginAccount, schoolID int64, verified bool, guard mintGuard, ipAddress, userAgent string) (identityaccess.CatererLoginResult, error) {
	ctx = c.runtime.WithTenantID(ctx, schoolID)
	session, err := c.auth.createRefreshSessionGuarded(ctx, account, schoolID, domain.ScopeCaterer, guard, "", verified)
	if err != nil {
		return identityaccess.CatererLoginResult{}, err
	}
	access, refresh := buildClaims(account, session, buildCatererMetadata(account), account.Email)
	accessToken, refreshToken, err := c.auth.generateAndLogTokens(ctx, account.ID, access, refresh, ipAddress, userAgent, domain.AuthEventLogin)
	if err != nil {
		return identityaccess.CatererLoginResult{}, err
	}
	return identityaccess.CatererLoginResult{Status: identityaccess.CatererLoginAuthenticated, AccessToken: accessToken, RefreshToken: refreshToken}, nil
}

// mintProvenSession mints the session of a caterer that proved its second
// factor: a redeemed code, or a passkey (proof nil). The proof's reset
// generation is re-checked under the account lock, so a code proved before
// an operator reset mints nothing.
func (c *CatererAccess) mintProvenSession(ctx context.Context, accountID int64, proof *identityaccess.VerifiedMFAChallenge, ipAddress, userAgent string) (identityaccess.CatererLoginResult, error) {
	account, err := c.auth.authenticatedAccount(ctx, "issue caterer tokens", accountID)
	if err != nil {
		return identityaccess.CatererLoginResult{}, err
	}
	_, schools, err := c.catererSchools(ctx, account.ID)
	if err != nil {
		return identityaccess.CatererLoginResult{}, err
	}
	if len(schools) == 0 {
		return identityaccess.CatererLoginResult{}, failed("caterer login", domain.ErrCatererWithoutSchool)
	}
	guard := c.auth.verifiedMFAMintGuard(proof, identityaccess.MFAChallengeScopeCaterer, nil)
	return c.mintSession(ctx, account, schools[0], true, guard, ipAddress, userAgent)
}

// VerifyCatererMFA redeems the mailed code of a caterer sign-in and mints
// the session. A remembered device is prepared before the mint and stored
// after it; a device that cannot be remembered never fails the sign-in.
func (c *CatererAccess) VerifyCatererMFA(ctx context.Context, verification identityaccess.CatererMFAVerification) (identityaccess.CatererSession, error) {
	session, err := c.verifyMFA(c.tenantless(ctx), verification)
	return session, catererMFAError(err)
}

func (c *CatererAccess) verifyMFA(ctx context.Context, verification identityaccess.CatererMFAVerification) (identityaccess.CatererSession, error) {
	if c.mfa == nil {
		return identityaccess.CatererSession{}, errCatererMFAUnavailable
	}
	proof, err := c.mfa.VerifyChallengeForScope(ctx, verification.ChallengeToken, verification.Code, domain.MFAChallengeScopeCaterer)
	if err != nil {
		return identityaccess.CatererSession{}, err
	}
	var issue identityaccess.TrustedDeviceIssue
	if verification.RememberDevice {
		issue, err = c.mfa.PrepareTrustedDevice(ctx, proof.AccountID, 0, verification.UserAgent, parseGateIP(verification.IPAddress))
		if err != nil {
			c.logger.Warn("caterer trusted-device policy unavailable; skipping cookie",
				slog.Int64("account_id", proof.AccountID),
				slog.String("error", err.Error()))
			issue = nil
		}
	}
	public := identityaccess.VerifiedMFAChallenge(proof)
	result, err := c.mintProvenSession(ctx, proof.AccountID, &public, verification.IPAddress, verification.UserAgent)
	if err != nil {
		return identityaccess.CatererSession{}, err
	}
	session := identityaccess.CatererSession{AccessToken: result.AccessToken, RefreshToken: result.RefreshToken}
	if issue != nil {
		cookie, expiresAt, err := issue(c.tenantless(ctx))
		if err != nil {
			c.logger.Warn("failed to remember caterer device",
				slog.Int64("account_id", proof.AccountID),
				slog.String("error", err.Error()))
		} else {
			session.TrustedDeviceCookie, session.TrustedDeviceExpiresAt = cookie, expiresAt
		}
	}
	return session, nil
}

// ResendCatererMFA mails a new code for a caterer challenge. Only a caterer
// challenge is accepted: the caterer portal never drives another portal's.
func (c *CatererAccess) ResendCatererMFA(ctx context.Context, challengeToken string, ip net.IP) (string, error) {
	token, err := c.resendMFA(c.tenantless(ctx), challengeToken, ip)
	return token, catererMFAError(err)
}

func (c *CatererAccess) resendMFA(ctx context.Context, challengeToken string, ip net.IP) (string, error) {
	if c.mfa == nil {
		return "", errCatererMFAUnavailable
	}
	return c.mfa.ResendChallengeForScope(ctx, challengeToken, ip, domain.MFAChallengeScopeCaterer)
}

// CatererMFAStatus reports whether the caterer switched its second factor on.
func (c *CatererAccess) CatererMFAStatus(ctx context.Context, accountID int64) (identityaccess.CatererMFAState, error) {
	ctx = c.tenantless(ctx)
	if c.mfa == nil {
		return identityaccess.CatererMFAState{}, catererMFAError(errCatererMFAUnavailable)
	}
	account, err := c.signedInCaterer(ctx, accountID)
	if err != nil {
		return identityaccess.CatererMFAState{}, catererMFAError(err)
	}
	enrolled, err := c.mfa.HasEnrollment(ctx, accountID)
	if err != nil {
		return identityaccess.CatererMFAState{}, catererMFAError(err)
	}
	return identityaccess.CatererMFAState{Enabled: enrolled, MaskedEmail: domain.MaskEmail(account.Email)}, nil
}

// signedInCaterer reads the signed-in account and proves it is a caterer's.
func (c *CatererAccess) signedInCaterer(ctx context.Context, accountID int64) (domain.LoginAccount, error) {
	account, err := c.auth.authenticatedAccount(ctx, "caterer account", accountID)
	if err != nil {
		return domain.LoginAccount{}, err
	}
	if _, _, err := c.catererSchools(ctx, account.ID); err != nil {
		return domain.LoginAccount{}, err
	}
	return account, nil
}

// StartCatererMFA mails the code that switches the second factor on.
func (c *CatererAccess) StartCatererMFA(ctx context.Context, accountID int64, ip net.IP) (identityaccess.CatererMFAChallenge, error) {
	challenge, err := c.startMFA(c.tenantless(ctx), accountID, ip)
	return challenge, catererMFAError(err)
}

func (c *CatererAccess) startMFA(ctx context.Context, accountID int64, ip net.IP) (identityaccess.CatererMFAChallenge, error) {
	if c.mfa == nil {
		return identityaccess.CatererMFAChallenge{}, errCatererMFAUnavailable
	}
	account, err := c.signedInCaterer(ctx, accountID)
	if err != nil {
		return identityaccess.CatererMFAChallenge{}, err
	}
	enrolled, err := c.mfa.HasEnrollment(ctx, accountID)
	if err != nil {
		return identityaccess.CatererMFAChallenge{}, err
	}
	if enrolled {
		return identityaccess.CatererMFAChallenge{}, identityaccess.ErrMFAAlreadyEnrolled
	}
	token, err := c.mfa.StartChallenge(ctx, accountID, 0, domain.MFAChallengeScopeCaterer, ip)
	if err != nil {
		return identityaccess.CatererMFAChallenge{}, err
	}
	return identityaccess.CatererMFAChallenge{ChallengeToken: token, MaskedEmail: domain.MaskEmail(account.Email)}, nil
}

// ConfirmCatererMFA switches the second factor on with the mailed code and
// mints a session proven by it. Every other session of the account was
// minted without the factor and ends at its next refresh.
func (c *CatererAccess) ConfirmCatererMFA(ctx context.Context, confirmation identityaccess.CatererMFAConfirmation) (identityaccess.CatererSession, error) {
	session, err := c.confirmMFA(c.tenantless(ctx), confirmation)
	return session, catererMFAError(err)
}

func (c *CatererAccess) confirmMFA(ctx context.Context, confirmation identityaccess.CatererMFAConfirmation) (identityaccess.CatererSession, error) {
	if c.mfa == nil {
		return identityaccess.CatererSession{}, errCatererMFAUnavailable
	}
	if _, err := c.signedInCaterer(ctx, confirmation.AccountID); err != nil {
		return identityaccess.CatererSession{}, err
	}
	proof, err := c.mfa.VerifyChallengeForOwner(ctx, confirmation.ChallengeToken, confirmation.Code, domain.MFAChallengeScopeCaterer, confirmation.AccountID, 0)
	if err != nil {
		return identityaccess.CatererSession{}, err
	}
	if err := c.mfa.Enroll(ctx, confirmation.AccountID); err != nil {
		return identityaccess.CatererSession{}, err
	}
	c.logger.Info("caterer second factor switched on", slog.Int64("account_id", confirmation.AccountID))
	public := identityaccess.VerifiedMFAChallenge(proof)
	result, err := c.mintProvenSession(ctx, confirmation.AccountID, &public, confirmation.IPAddress, confirmation.UserAgent)
	if err != nil {
		return identityaccess.CatererSession{}, err
	}
	return identityaccess.CatererSession{AccessToken: result.AccessToken, RefreshToken: result.RefreshToken}, nil
}

// DisableCatererMFA switches the second factor off once the current password
// is checked like a sign-in. The factor, the remembered devices and the
// sessions' proof go together; the passkeys stay, as at every reset.
func (c *CatererAccess) DisableCatererMFA(ctx context.Context, accountID int64, currentPassword, ipAddress, userAgent string) error {
	return catererMFAError(c.disableMFA(c.tenantless(ctx), accountID, currentPassword, ipAddress, userAgent))
}

func (c *CatererAccess) disableMFA(ctx context.Context, accountID int64, currentPassword, ipAddress, userAgent string) error {
	if c.mfa == nil {
		return errCatererMFAUnavailable
	}
	if _, err := c.signedInCaterer(ctx, accountID); err != nil {
		return err
	}
	enrolled, err := c.mfa.HasEnrollment(ctx, accountID)
	if err != nil {
		return err
	}
	if !enrolled {
		return identityaccess.ErrCatererMFANotEnabled
	}
	if err := c.checkCurrentPassword(ctx, accountID, currentPassword, ipAddress, userAgent); err != nil {
		return err
	}
	if err := c.mfa.Disable(ctx, accountID); err != nil {
		return err
	}
	c.logger.Info("caterer second factor switched off", slog.Int64("account_id", accountID))
	return nil
}

// checkCurrentPassword checks the caterer's password like the password
// change does: a wrong one counts towards the account's lock and its
// count commits.
func (c *CatererAccess) checkCurrentPassword(ctx context.Context, accountID int64, password, ipAddress, userAgent string) error {
	var accepted bool
	err := globalIdentityRead(ctx, c.runtime, func(txCtx context.Context) error {
		account, found, _, err := c.auth.store.FindLoginAccount(txCtx, accountID, true)
		if err != nil {
			return err
		}
		if !found || !account.Active {
			return domain.ErrAccountNotFound
		}
		accepted, err = checkPasswordLogin(txCtx, c.auth.store, c.verifier, c.auth.passwordPolicy, account.ID, account.PasswordHash, password, false)
		return err
	})
	if err != nil {
		return failed("check caterer password", err)
	}
	if !accepted {
		c.logger.Info("caterer second factor change refused: wrong password",
			slog.Int64("account_id", accountID),
			slog.String("ip_address", strings.TrimSpace(ipAddress)),
			slog.Bool("has_user_agent", userAgent != ""))
		return failed("check caterer password", identityaccess.ErrInvalidCredentials)
	}
	return nil
}

// ListCatererTrustedDevices lists the caterer's remembered devices. They
// name no school.
func (c *CatererAccess) ListCatererTrustedDevices(ctx context.Context, accountID int64) ([]identityaccess.AccountTrustedDevice, error) {
	devices, err := c.listTrustedDevices(c.tenantless(ctx), accountID)
	if err != nil {
		return nil, catererMFAError(err)
	}
	result := make([]identityaccess.AccountTrustedDevice, 0, len(devices))
	for _, device := range devices {
		result = append(result, identityaccess.AccountTrustedDevice(device))
	}
	return result, nil
}

func (c *CatererAccess) listTrustedDevices(ctx context.Context, accountID int64) ([]domain.AccountTrustedDevice, error) {
	if err := c.secondFactorOf(ctx, accountID); err != nil {
		return nil, err
	}
	return c.mfa.records.ListActiveTrustedDevices(ctx, accountID, 0)
}

// RevokeCatererTrustedDevice forgets one remembered device of the caterer.
func (c *CatererAccess) RevokeCatererTrustedDevice(ctx context.Context, accountID, deviceID int64) error {
	return catererMFAError(c.revokeTrustedDevice(c.tenantless(ctx), accountID, deviceID))
}

func (c *CatererAccess) revokeTrustedDevice(ctx context.Context, accountID, deviceID int64) error {
	if err := c.secondFactorOf(ctx, accountID); err != nil {
		return err
	}
	return c.mfa.RevokeTrustedDevice(ctx, accountID, 0, deviceID)
}

// secondFactorOf proves the signed-in account is a caterer's before one of
// its second-factor routes runs.
func (c *CatererAccess) secondFactorOf(ctx context.Context, accountID int64) error {
	if c.mfa == nil {
		return errCatererMFAUnavailable
	}
	_, err := c.signedInCaterer(ctx, accountID)
	return err
}
