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
// school ledger. The portals differ only in who belongs to them (member),
// in how their devices are bound (bind) and in the session they mint.
type voluntaryFactor struct {
	portal   domain.Portal
	auth     *AccountAuthentication
	mfa      *AccountMFAFlows
	verifier ports.PasswordVerifier
	runtime  ports.Runtime
	logger   *slog.Logger
	// bind prepares the context of every flow: the composed unit, the
	// missing school ledger and the portal's trusted-device binding.
	bind func(context.Context) context.Context
	// member reads the signed-in account and proves it still belongs to the
	// portal.
	member func(ctx context.Context, accountID int64) (domain.LoginAccount, error)
	// notEnabled refuses switching off a factor that is not on.
	notEnabled error
}

// voluntaryGate is the outcome of the second-factor check of a sign-in: a
// challenge to answer, or a session to mint with its proof.
type voluntaryGate struct {
	challenge *domain.LoginResult
	proof     mintProof
}

// gate checks the second factor of a password sign-in. An account without
// the factor mints with the password; a remembered device of this portal
// proves the factor; otherwise the code is mailed.
func (f voluntaryFactor) gate(ctx context.Context, account domain.LoginAccount, trustedDeviceCookie, ipAddress string) (voluntaryGate, error) {
	if !f.auth.mfa.Configured() {
		return voluntaryGate{}, nil
	}
	ctx = f.bind(ctx)
	enrolled, err := f.auth.mfa.HasEnrollment(ctx, account.ID)
	if err != nil {
		return voluntaryGate{}, failed("check "+f.portal.Name+" mfa enrollment", identityaccess.ErrMFAStatusUnavailable)
	}
	if !enrolled {
		return voluntaryGate{}, nil
	}
	if f.auth.trustedDeviceVerified(ctx, account.ID, 0, trustedDeviceCookie) {
		return voluntaryGate{proof: mintProof{kind: proofTrustedDevice, trustedDeviceCookie: trustedDeviceCookie}}, nil
	}
	result, err := f.auth.mfaChallengeResult(ctx, account, 0, f.portal, ipAddress)
	if err != nil {
		return voluntaryGate{}, err
	}
	if !result.TrustedDeviceEnabled {
		result.TrustedDeviceDays = 0
	}
	return voluntaryGate{challenge: result}, nil
}

// signInProof is a redeemed sign-in code of this portal and, when the
// person asked to remember the browser, the prepared device. A device that
// cannot be prepared never fails the sign-in.
type signInProof struct {
	challenge identityaccess.VerifiedMFAChallenge
	device    identityaccess.TrustedDeviceIssue
}

func (f voluntaryFactor) verifySignIn(ctx context.Context, challengeToken, code string, rememberDevice bool, ipAddress, userAgent string) (signInProof, error) {
	if f.mfa == nil {
		return signInProof{}, identityaccess.ErrAccountMFAUnavailable
	}
	ctx = f.bind(ctx)
	proof, err := f.mfa.VerifyChallengeForScope(ctx, challengeToken, code, f.portal.Name)
	if err != nil {
		return signInProof{}, err
	}
	result := signInProof{challenge: identityaccess.VerifiedMFAChallenge(proof)}
	if rememberDevice {
		result.device, err = f.mfa.PrepareTrustedDevice(ctx, proof.AccountID, 0, userAgent, parseGateIP(ipAddress))
		if err != nil {
			f.logger.Warn("trusted-device policy unavailable; skipping cookie",
				slog.String("portal", f.portal.Name),
				slog.Int64("account_id", proof.AccountID),
				slog.String("error", err.Error()))
			result.device = nil
		}
	}
	return result, nil
}

// remember stores the prepared device after the session was minted. A
// device that cannot be stored never fails the sign-in.
func (f voluntaryFactor) remember(ctx context.Context, accountID int64, issue identityaccess.TrustedDeviceIssue) (string, time.Time) {
	if issue == nil {
		return "", time.Time{}
	}
	cookie, expiresAt, err := issue(f.bind(ctx))
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
func (f voluntaryFactor) start(ctx context.Context, accountID int64, ip net.IP) (string, string, error) {
	if f.mfa == nil {
		return "", "", identityaccess.ErrAccountMFAUnavailable
	}
	ctx = f.bind(ctx)
	account, err := f.member(ctx, accountID)
	if err != nil {
		return "", "", err
	}
	enrolled, err := f.mfa.HasEnrollment(ctx, accountID)
	if err != nil {
		return "", "", err
	}
	if enrolled {
		return "", "", identityaccess.ErrMFAAlreadyEnrolled
	}
	token, err := f.mfa.StartChallenge(ctx, accountID, 0, f.portal.Name, ip)
	if err != nil {
		return "", "", err
	}
	return token, domain.MaskEmail(account.Email), nil
}

// confirm switches the second factor on with the mailed code and returns
// the proof the caller mints its session with. Every other session of the
// account was minted without the factor and ends at its next refresh.
func (f voluntaryFactor) confirm(ctx context.Context, accountID int64, challengeToken, code string) (identityaccess.VerifiedMFAChallenge, error) {
	if f.mfa == nil {
		return identityaccess.VerifiedMFAChallenge{}, identityaccess.ErrAccountMFAUnavailable
	}
	ctx = f.bind(ctx)
	if _, err := f.member(ctx, accountID); err != nil {
		return identityaccess.VerifiedMFAChallenge{}, err
	}
	proof, err := f.mfa.VerifyChallengeForOwner(ctx, challengeToken, code, f.portal.Name, accountID, 0)
	if err != nil {
		return identityaccess.VerifiedMFAChallenge{}, err
	}
	if err := f.mfa.Enroll(ctx, proof); err != nil {
		return identityaccess.VerifiedMFAChallenge{}, err
	}
	f.logger.Info("second factor switched on",
		slog.String("portal", f.portal.Name),
		slog.Int64("account_id", accountID))
	return identityaccess.VerifiedMFAChallenge(proof), nil
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
