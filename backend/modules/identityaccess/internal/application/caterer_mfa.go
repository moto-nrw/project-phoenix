package application

import (
	"context"
	"log/slog"
	"net"
	"strings"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/domain"
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
	gate, challenge, err := c.signInGate(ctx, account, request)
	if err != nil {
		return identityaccess.CatererLoginResult{}, err
	}
	if challenge != nil {
		return *challenge, nil
	}
	return c.mintSession(ctx, account, schools[0], gate.proof, request.IPAddress, request.UserAgent)
}

func (c *CatererAccess) signInGate(ctx context.Context, account domain.LoginAccount, request identityaccess.CatererLoginRequest) (voluntaryGate, *identityaccess.CatererLoginResult, error) {
	gate, err := c.factor().gate(ctx, account, request.TrustedDeviceCookie, request.IPAddress)
	if err != nil || gate.challenge == nil {
		return gate, nil, err
	}
	return gate, &identityaccess.CatererLoginResult{
		Status: identityaccess.CatererLoginMFARequired, ChallengeToken: gate.challenge.ChallengeToken,
		MaskedEmail: gate.challenge.MaskedEmail, TrustedDeviceDays: gate.challenge.TrustedDeviceDays,
	}, nil
}

// factor is the caterer's voluntary second factor. Its devices name neither
// a school nor a Träger (#6955).
func (c *CatererAccess) factor() voluntaryFactor {
	return voluntaryFactor{
		portal: domain.PortalCaterer, auth: c.auth, mfa: c.mfa, verifier: c.verifier, runtime: c.runtime, logger: c.logger,
		bind: withoutSchoolLedgerContext, member: c.signedInCaterer, notEnabled: identityaccess.ErrCatererMFANotEnabled,
	}
}

// mintSession persists the caterer session at the school it is kept at and
// issues its pair. The login is filed at that school.
func (c *CatererAccess) mintSession(ctx context.Context, account domain.LoginAccount, schoolID int64, proof mintProof, ipAddress, userAgent string) (identityaccess.CatererLoginResult, error) {
	minted, err := c.auth.mintAccountSession(c.runtime.WithTenantID(ctx, schoolID), mintRequest{
		portal: domain.PortalCaterer, account: account, tenantID: schoolID, proof: proof,
		audit: &mintAudit{event: domain.AuthEventLogin, ipAddress: ipAddress, userAgent: userAgent, schoolID: schoolID},
	})
	if err != nil {
		return identityaccess.CatererLoginResult{}, err
	}
	return identityaccess.CatererLoginResult{Status: identityaccess.CatererLoginAuthenticated, AccessToken: minted.accessToken, RefreshToken: minted.refreshToken}, nil
}

// mintProvenSession mints the session of a caterer that proved its second
// factor: a redeemed code, or a passkey (proof nil).
func (c *CatererAccess) mintProvenSession(ctx context.Context, accountID int64, challenge *identityaccess.VerifiedMFAChallenge, ipAddress, userAgent string) (identityaccess.CatererLoginResult, error) {
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
	proof := mintProof{kind: proofPasskey}
	if challenge != nil {
		proof = challengeProof(challenge)
	}
	return c.mintSession(ctx, account, schools[0], proof, ipAddress, userAgent)
}

// VerifyCatererMFA redeems the mailed code of a caterer sign-in and mints
// the session. A remembered device is prepared before the mint and stored
// after it; a device that cannot be remembered never fails the sign-in.
func (c *CatererAccess) VerifyCatererMFA(ctx context.Context, verification identityaccess.CatererMFAVerification) (identityaccess.CatererSession, error) {
	session, err := c.verifyMFA(c.tenantless(ctx), verification)
	return session, catererMFAError(err)
}

func (c *CatererAccess) verifyMFA(ctx context.Context, verification identityaccess.CatererMFAVerification) (identityaccess.CatererSession, error) {
	proof, err := c.factor().verifySignIn(ctx, verification.ChallengeToken, verification.Code, verification.RememberDevice, verification.IPAddress, verification.UserAgent)
	if err != nil {
		return identityaccess.CatererSession{}, err
	}
	result, err := c.mintProvenSession(ctx, proof.challenge.AccountID, &proof.challenge, verification.IPAddress, verification.UserAgent)
	if err != nil {
		return identityaccess.CatererSession{}, err
	}
	session := identityaccess.CatererSession{AccessToken: result.AccessToken, RefreshToken: result.RefreshToken}
	session.TrustedDeviceCookie, session.TrustedDeviceExpiresAt = c.factor().remember(ctx, proof.challenge.AccountID, proof.device)
	return session, nil
}

// ResendCatererMFA mails a new code for a caterer challenge. Only a caterer
// challenge is accepted: the caterer portal never drives another portal's.
func (c *CatererAccess) ResendCatererMFA(ctx context.Context, challengeToken string, ip net.IP) (string, error) {
	token, err := c.resendMFA(c.tenantless(ctx), challengeToken, ip)
	return token, catererMFAError(err)
}

func (c *CatererAccess) resendMFA(ctx context.Context, challengeToken string, ip net.IP) (string, error) {
	return c.factor().resend(ctx, challengeToken, ip)
}

// CatererMFAStatus reports whether the caterer switched its second factor on.
func (c *CatererAccess) CatererMFAStatus(ctx context.Context, accountID int64) (identityaccess.CatererMFAState, error) {
	enabled, masked, err := c.factor().status(c.attach(ctx), accountID)
	if err != nil {
		return identityaccess.CatererMFAState{}, catererMFAError(err)
	}
	return identityaccess.CatererMFAState{Enabled: enabled, MaskedEmail: masked}, nil
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
	token, masked, err := c.factor().start(c.attach(ctx), accountID, ip)
	if err != nil {
		return identityaccess.CatererMFAChallenge{}, catererMFAError(err)
	}
	return identityaccess.CatererMFAChallenge{ChallengeToken: token, MaskedEmail: masked}, nil
}

// ConfirmCatererMFA switches the second factor on with the mailed code and
// mints a session proven by it. Every other session of the account was
// minted without the factor and ends at its next refresh.
func (c *CatererAccess) ConfirmCatererMFA(ctx context.Context, confirmation identityaccess.CatererMFAConfirmation) (identityaccess.CatererSession, error) {
	session, err := c.confirmMFA(c.tenantless(ctx), confirmation)
	return session, catererMFAError(err)
}

func (c *CatererAccess) confirmMFA(ctx context.Context, confirmation identityaccess.CatererMFAConfirmation) (identityaccess.CatererSession, error) {
	proof, err := c.factor().confirm(ctx, confirmation.AccountID, confirmation.ChallengeToken, confirmation.Code)
	if err != nil {
		return identityaccess.CatererSession{}, err
	}
	result, err := c.mintProvenSession(ctx, confirmation.AccountID, &proof, confirmation.IPAddress, confirmation.UserAgent)
	if err != nil {
		return identityaccess.CatererSession{}, err
	}
	return identityaccess.CatererSession{AccessToken: result.AccessToken, RefreshToken: result.RefreshToken}, nil
}

// DisableCatererMFA switches the second factor off once the current password
// is checked like a sign-in. The factor, the remembered devices and the
// sessions' proof go together; the passkeys stay, as at every reset.
func (c *CatererAccess) DisableCatererMFA(ctx context.Context, accountID int64, currentPassword, ipAddress, userAgent string) error {
	return catererMFAError(c.factor().disable(c.attach(ctx), accountID, currentPassword, ipAddress, userAgent))
}

// ListCatererTrustedDevices lists the caterer's remembered devices. They
// name no school.
func (c *CatererAccess) ListCatererTrustedDevices(ctx context.Context, accountID int64) ([]identityaccess.AccountTrustedDevice, error) {
	devices, err := c.factor().devices(c.attach(ctx), accountID)
	return devices, catererMFAError(err)
}

// RevokeCatererTrustedDevice forgets one remembered device of the caterer.
func (c *CatererAccess) RevokeCatererTrustedDevice(ctx context.Context, accountID, deviceID int64) error {
	return catererMFAError(c.factor().revoke(c.attach(ctx), accountID, deviceID))
}
