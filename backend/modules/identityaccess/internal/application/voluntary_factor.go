package application

import (
	"context"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/ports"
)

// voluntaryFactor is the second factor an account switches on itself on a
// portal without a school: the caterer's (#4497, ADR 0077 point 7) and the
// parents' (#7051). No school or Träger can require it, and no school's mode
// reaches it. It is the account's e-mail factor and its remembered devices,
// as on the school portals, but its challenges and devices name no school:
// its policy is the registry default, and its events are logged without a
// school ledger. The portals differ only in who belongs to them (member)
// and in the session they mint (mint); the devices follow the portal's
// realm (domain.Portal.Device).
type voluntaryFactor struct {
	portal   domain.Portal
	auth     *AccountAuthentication
	mfa      *AccountMFAFlows
	verifier ports.PasswordVerifier
	runtime  ports.Runtime
	logger   *slog.Logger
	// member reads the signed-in account and proves it still belongs to the
	// portal.
	member func(ctx context.Context, accountID int64) (domain.LoginAccount, error)
	// mint mints the portal's session of an account that proved its second
	// factor. It reads the portal's refusals again: the account may have
	// left the portal since the password.
	mint func(ctx context.Context, account domain.LoginAccount, proof mintProof, ipAddress, userAgent string) (mintedSession, error)
	// notEnabled refuses switching off a factor that is not on.
	notEnabled error
}

func (f voluntaryFactor) bind(ctx context.Context) context.Context {
	return portalDeviceContext(ctx, f.portal, 0)
}

// answer finishes a password sign-in the portal admitted. An account
// without the factor mints with the password; a remembered device of this
// portal proves the factor; otherwise the code is mailed.
func (f voluntaryFactor) answer(ctx context.Context, account domain.LoginAccount, request identityaccess.VoluntaryLoginRequest, mint func(context.Context, mintProof) (mintedSession, error)) (identityaccess.VoluntaryLoginResult, error) {
	proof, challenge, err := f.gate(ctx, account, request.TrustedDeviceCookie, request.IPAddress)
	if err != nil {
		return identityaccess.VoluntaryLoginResult{}, err
	}
	if challenge != nil {
		return identityaccess.VoluntaryLoginResult{
			Status: identityaccess.LoginStatusMFARequired, ChallengeToken: challenge.ChallengeToken,
			MaskedEmail: challenge.MaskedEmail, TrustedDeviceDays: challenge.TrustedDeviceDays,
		}, nil
	}
	minted, err := mint(ctx, proof)
	if err != nil {
		return identityaccess.VoluntaryLoginResult{}, err
	}
	return identityaccess.VoluntaryLoginResult{
		Status: identityaccess.LoginStatusAuthenticated, AccessToken: minted.accessToken, RefreshToken: minted.refreshToken,
	}, nil
}

// gate checks the second factor of a password sign-in: the proof to mint
// with, or the challenge to answer first.
func (f voluntaryFactor) gate(ctx context.Context, account domain.LoginAccount, trustedDeviceCookie, ipAddress string) (mintProof, *domain.LoginResult, error) {
	if !f.auth.mfa.Configured() {
		return mintProof{}, nil, nil
	}
	ctx = f.bind(ctx)
	enrolled, err := f.auth.mfa.HasEnrollment(ctx, account.ID)
	if err != nil {
		return mintProof{}, nil, failed("check "+f.portal.Name+" mfa enrollment", identityaccess.ErrMFAStatusUnavailable)
	}
	if !enrolled {
		return mintProof{}, nil, nil
	}
	if f.auth.trustedDeviceVerified(ctx, account.ID, 0, trustedDeviceCookie) {
		return mintProof{kind: proofTrustedDevice, trustedDeviceCookie: trustedDeviceCookie}, nil, nil
	}
	result, err := f.auth.mfaChallengeResult(ctx, account, 0, f.portal, ipAddress)
	if err != nil {
		return mintProof{}, nil, err
	}
	if !result.TrustedDeviceEnabled {
		result.TrustedDeviceDays = 0
	}
	return mintProof{}, result, nil
}

// redeem redeems the mailed code of a sign-in and mints the session. A
// remembered device is prepared before the mint and stored after it; a
// device that cannot be prepared or stored never fails the sign-in.
func (f voluntaryFactor) redeem(ctx context.Context, verification identityaccess.VoluntaryMFAVerification) (identityaccess.VoluntarySession, error) {
	if f.mfa == nil {
		return identityaccess.VoluntarySession{}, identityaccess.ErrAccountMFAUnavailable
	}
	bound := f.bind(ctx)
	proof, err := f.mfa.VerifyChallengeForScope(bound, verification.ChallengeToken, verification.Code, f.portal.Name)
	if err != nil {
		return identityaccess.VoluntarySession{}, err
	}
	var device identityaccess.TrustedDeviceIssue
	if verification.RememberDevice {
		device, err = f.mfa.PrepareTrustedDevice(bound, proof.AccountID, 0, verification.UserAgent, parseGateIP(verification.IPAddress))
		if err != nil {
			f.logger.Warn("trusted-device policy unavailable; skipping cookie",
				slog.String("portal", f.portal.Name),
				slog.Int64("account_id", proof.AccountID),
				slog.String("error", err.Error()))
			device = nil
		}
	}
	challenge := identityaccess.VerifiedMFAChallenge(proof)
	session, err := f.proven(ctx, proof.AccountID, challengeProof(&challenge), verification.IPAddress, verification.UserAgent)
	if err != nil {
		return identityaccess.VoluntarySession{}, err
	}
	session.TrustedDeviceCookie, session.TrustedDeviceExpiresAt = f.remember(bound, proof.AccountID, device)
	return session, nil
}

// proven mints the portal's session of an account that proved its second
// factor.
func (f voluntaryFactor) proven(ctx context.Context, accountID int64, proof mintProof, ipAddress, userAgent string) (identityaccess.VoluntarySession, error) {
	account, err := f.auth.authenticatedAccount(ctx, "issue "+f.portal.Name+" tokens", accountID)
	if err != nil {
		return identityaccess.VoluntarySession{}, err
	}
	minted, err := f.mint(ctx, account, proof, ipAddress, userAgent)
	if err != nil {
		return identityaccess.VoluntarySession{}, err
	}
	return identityaccess.VoluntarySession{AccessToken: minted.accessToken, RefreshToken: minted.refreshToken, Email: account.Email}, nil
}

func (f voluntaryFactor) remember(ctx context.Context, accountID int64, issue identityaccess.TrustedDeviceIssue) (string, time.Time) {
	if issue == nil {
		return "", time.Time{}
	}
	cookie, expiresAt, err := issue(ctx)
	if err != nil {
		f.logger.Warn("failed to remember device",
			slog.String("portal", f.portal.Name),
			slog.Int64("account_id", accountID),
			slog.String("error", err.Error()))
		return "", time.Time{}
	}
	return cookie, expiresAt
}

// resend mails a new code. Only a challenge of this portal is accepted.
func (f voluntaryFactor) resend(ctx context.Context, challengeToken string, ip net.IP) (string, error) {
	if f.mfa == nil {
		return "", identityaccess.ErrAccountMFAUnavailable
	}
	return f.mfa.ResendChallengeForScope(f.bind(ctx), challengeToken, ip, f.portal.Name)
}

// status reports whether the account switched its second factor on.
func (f voluntaryFactor) status(ctx context.Context, accountID int64) (bool, string, error) {
	if f.mfa == nil {
		return false, "", identityaccess.ErrAccountMFAUnavailable
	}
	ctx = f.bind(ctx)
	account, err := f.member(ctx, accountID)
	if err != nil {
		return false, "", err
	}
	enrolled, err := f.mfa.HasEnrollment(ctx, accountID)
	if err != nil {
		return false, "", err
	}
	return enrolled, domain.MaskEmail(account.Email), nil
}

// start mails the code that switches the second factor on.
func (f voluntaryFactor) start(ctx context.Context, accountID int64, ip net.IP) (identityaccess.VoluntaryMFAChallenge, error) {
	if f.mfa == nil {
		return identityaccess.VoluntaryMFAChallenge{}, identityaccess.ErrAccountMFAUnavailable
	}
	ctx = f.bind(ctx)
	account, err := f.member(ctx, accountID)
	if err != nil {
		return identityaccess.VoluntaryMFAChallenge{}, err
	}
	enrolled, err := f.mfa.HasEnrollment(ctx, accountID)
	if err != nil {
		return identityaccess.VoluntaryMFAChallenge{}, err
	}
	if enrolled {
		return identityaccess.VoluntaryMFAChallenge{}, identityaccess.ErrMFAAlreadyEnrolled
	}
	token, err := f.mfa.StartChallenge(ctx, accountID, 0, f.portal.Name, ip)
	if err != nil {
		return identityaccess.VoluntaryMFAChallenge{}, err
	}
	return identityaccess.VoluntaryMFAChallenge{ChallengeToken: token, MaskedEmail: domain.MaskEmail(account.Email)}, nil
}

// enable switches the second factor on with the mailed code and mints a
// session proven by it. Every other session of the account was minted
// without the factor and ends at its next refresh.
func (f voluntaryFactor) enable(ctx context.Context, confirmation identityaccess.VoluntaryMFAConfirmation) (identityaccess.VoluntarySession, error) {
	if f.mfa == nil {
		return identityaccess.VoluntarySession{}, identityaccess.ErrAccountMFAUnavailable
	}
	bound := f.bind(ctx)
	if _, err := f.member(bound, confirmation.AccountID); err != nil {
		return identityaccess.VoluntarySession{}, err
	}
	proof, err := f.mfa.VerifyChallengeForOwner(bound, confirmation.ChallengeToken, confirmation.Code, f.portal.Name, confirmation.AccountID, 0)
	if err != nil {
		return identityaccess.VoluntarySession{}, err
	}
	if err := f.mfa.Enroll(bound, proof); err != nil {
		return identityaccess.VoluntarySession{}, err
	}
	f.logger.Info("second factor switched on",
		slog.String("portal", f.portal.Name),
		slog.Int64("account_id", confirmation.AccountID))
	challenge := identityaccess.VerifiedMFAChallenge(proof)
	return f.proven(ctx, confirmation.AccountID, challengeProof(&challenge), confirmation.IPAddress, confirmation.UserAgent)
}

// disable switches the second factor off once the current password is
// checked like a sign-in. The factor, the remembered devices and the
// sessions' proof go together; the passkeys stay, as at every reset.
func (f voluntaryFactor) disable(ctx context.Context, accountID int64, currentPassword, ipAddress, userAgent string) error {
	if f.mfa == nil {
		return identityaccess.ErrAccountMFAUnavailable
	}
	ctx = f.bind(ctx)
	if _, err := f.member(ctx, accountID); err != nil {
		return err
	}
	enrolled, err := f.mfa.HasEnrollment(ctx, accountID)
	if err != nil {
		return err
	}
	if !enrolled {
		return f.notEnabled
	}
	if err := f.checkCurrentPassword(ctx, accountID, currentPassword, ipAddress, userAgent); err != nil {
		return err
	}
	if err := f.mfa.Disable(ctx, accountID); err != nil {
		return err
	}
	f.logger.Info("second factor switched off",
		slog.String("portal", f.portal.Name),
		slog.Int64("account_id", accountID))
	return nil
}

// checkCurrentPassword checks the password like the password change does:
// a wrong one counts towards the account's lock and its count commits.
func (f voluntaryFactor) checkCurrentPassword(ctx context.Context, accountID int64, password, ipAddress, userAgent string) error {
	var accepted bool
	err := globalIdentityRead(ctx, f.runtime, func(txCtx context.Context) error {
		account, found, _, err := f.auth.store.FindLoginAccount(txCtx, accountID, true)
		if err != nil {
			return err
		}
		if !found || !account.Active {
			return domain.ErrAccountNotFound
		}
		accepted, err = checkPasswordLogin(txCtx, f.auth.store, f.verifier, f.auth.passwordPolicy, account.ID, account.PasswordHash, password, false)
		return err
	})
	if err != nil {
		return failed("check "+f.portal.Name+" password", err)
	}
	if !accepted {
		f.logger.Info("second factor change refused: wrong password",
			slog.String("portal", f.portal.Name),
			slog.Int64("account_id", accountID),
			slog.String("ip_address", strings.TrimSpace(ipAddress)),
			slog.Bool("has_user_agent", userAgent != ""))
		return failed("check "+f.portal.Name+" password", identityaccess.ErrInvalidCredentials)
	}
	return nil
}

// devices lists the account's remembered devices of this portal.
func (f voluntaryFactor) devices(ctx context.Context, accountID int64) ([]identityaccess.AccountTrustedDevice, error) {
	if f.mfa == nil {
		return nil, identityaccess.ErrAccountMFAUnavailable
	}
	ctx = f.bind(ctx)
	if _, err := f.member(ctx, accountID); err != nil {
		return nil, err
	}
	devices, err := f.mfa.records.ListActiveTrustedDevices(ctx, accountID, deviceBinding(ctx, 0))
	if err != nil {
		return nil, err
	}
	result := make([]identityaccess.AccountTrustedDevice, 0, len(devices))
	for _, device := range devices {
		result = append(result, identityaccess.AccountTrustedDevice(device))
	}
	return result, nil
}

// revoke forgets one remembered device of this portal.
func (f voluntaryFactor) revoke(ctx context.Context, accountID, deviceID int64) error {
	if f.mfa == nil {
		return identityaccess.ErrAccountMFAUnavailable
	}
	ctx = f.bind(ctx)
	if _, err := f.member(ctx, accountID); err != nil {
		return err
	}
	return f.mfa.RevokeTrustedDevice(ctx, accountID, 0, deviceID)
}
