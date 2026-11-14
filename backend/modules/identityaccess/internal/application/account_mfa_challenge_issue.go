package application

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/domain"
)

// The issuance of an e-mail code: a challenge row, its mail in the outbox and
// the challenge JWT naming the row. A code a password check led to is bound to
// the credential generation that check read (#6289).

// StartChallenge mails a fresh code for one portal and returns the challenge
// JWT that names its row. The row is stored consumed and only activated once
// the transport accepted the message, so a failed send leaves no redeemable
// code behind. It serves the in-session flows (passkey registration, the
// caterer's factor), which no password check precedes.
func (f *AccountMFAFlows) StartChallenge(ctx context.Context, accountID, tenantID int64, scope string, ip net.IP) (string, error) {
	return f.startChallenge(ctx, accountID, tenantID, scope, ip, nil)
}

// StartBoundChallenge is StartChallenge for a code a password check led to:
// the sign-in, the enrollment token it handed out, or the challenge a resend
// renews. Under the account lock it issues nothing unless the account still
// holds credentialGeneration, so no code goes out for a password a change
// replaced in the meantime (#6289).
func (f *AccountMFAFlows) StartBoundChallenge(ctx context.Context, accountID, tenantID int64, scope string, ip net.IP, credentialGeneration int64) (string, error) {
	return f.startChallenge(ctx, accountID, tenantID, scope, ip, &credentialGeneration)
}

func (f *AccountMFAFlows) startChallenge(ctx context.Context, accountID, tenantID int64, scope string, ip net.IP, credentialGeneration *int64) (string, error) {
	if !domain.AccountMFAChallengeScope(scope) {
		return "", identityaccess.ErrMFAUnsupportedScope
	}
	account, found, err := f.records.FindAccountIdentity(ctx, accountID)
	if err != nil {
		return "", fmt.Errorf("look up account: %w", err)
	}
	if !found {
		return "", fmt.Errorf("look up account: %w", domain.ErrAccountNotFound)
	}
	if domain.MFALocked(account.MFALockedUntil, f.now()) {
		return "", identityaccess.ErrMFALocked
	}

	if err := f.requireChallengeRateLimit(ctx, accountID); err != nil {
		return "", err
	}
	plainCode, codeHash, err := newMFAEmailCode(f.codes)
	if err != nil {
		return "", err
	}
	// The portal and the school travel with the row, not only with the JWT:
	// the enrollment-confirm and passkey-registration paths look a code up
	// by account, and only these columns keep that lookup inside its portal.
	createdAt := f.now()
	enabled, days := f.challengeDeviceHint(ctx, tenantID, scope)
	challenge, send, err := f.enqueueChallenge(ctx, domain.AccountMFAChallenge{
		AccountID: accountID, Scope: scope, TenantID: tenantID, CodeHash: codeHash,
		ExpiresAt: createdAt.Add(domain.MFAChallengeTTL), ConsumedAt: &createdAt, IPAddress: ip,
		CreatedAt: createdAt, UpdatedAt: createdAt,
	}, domain.MFACodeMailFor(
		account.Email, "", account.ID, false, plainCode, domain.MFAChallengeTTL, ip, enabled, days,
	).ForPortal(scope, account.PortalLocale), credentialGeneration)
	if err != nil {
		return "", fmt.Errorf("prepare email challenge: %w", err)
	}
	if err := f.deliverChallenge(ctx, challenge, send); err != nil {
		return "", err
	}

	token, err := f.codec.IssueChallengeToken(domain.MFAChallengeClaims{
		AccountID: accountID, Scope: scope, TenantID: tenantID, ChallengeID: challenge.ID,
		CredentialGeneration: challenge.MFAResetGeneration,
	}, domain.MFAChallengeTTL)
	if err != nil {
		return "", fmt.Errorf("mint challenge jwt: %w", err)
	}
	return token, nil
}

// challengeOwner pins a challenge to the identity the caller already
// authenticated out of band.
type challengeOwner struct {
	accountID int64
	tenantID  int64
}

// ResendChallengeForScope is the variant every portal resend endpoint uses:
// no surface may re-drive a foreign portal's challenge and burn its budget.
func (f *AccountMFAFlows) ResendChallengeForScope(ctx context.Context, challengeToken string, ip net.IP, expectedScope string) (string, error) {
	claims, err := f.codec.ParseChallengeToken(challengeToken)
	if err != nil {
		return "", identityaccess.ErrMFAChallengeTokenInvalid
	}
	if claims.Scope != expectedScope {
		return "", identityaccess.ErrMFAUnsupportedScope
	}
	return f.resend(ctx, claims, ip)
}

// resend has no cooldown of its own: the sliding window inside StartChallenge
// remains the abuse defense. It returns the renewed JWT so the frontend can
// replace its in-flight token — otherwise the fresh code becomes
// unverifiable the moment the original token expires.
//
// The renewed code is bound to the credential generation of the challenge it
// renews: a code mailed before a password change renews into nothing (#6289).
func (f *AccountMFAFlows) resend(ctx context.Context, claims domain.MFAChallengeClaims, ip net.IP) (string, error) {
	return f.StartBoundChallenge(ctx, claims.AccountID, claims.TenantID, claims.Scope, ip, claims.CredentialGeneration)
}

// requireCredentialGeneration refuses, under the account lock the caller
// holds, a code bound to a credential generation the account no longer
// holds: a password write or an MFA reset committed since the password check
// it stems from (#6289). The row then takes the generation it was checked
// against, so the code redeems only while that still holds.
func (f *AccountMFAFlows) requireCredentialGeneration(ctx context.Context, accountID int64, credentialGeneration *int64) error {
	if credentialGeneration == nil {
		return nil
	}
	account, found, err := f.records.FindAccountIdentity(ctx, accountID)
	if err != nil || !found {
		return identityaccess.ErrMFAStatusUnavailable
	}
	if !domain.CredentialStillHolds(credentialGeneration, account.MFAResetGeneration) {
		return identityaccess.ErrInvalidCredentials
	}
	return nil
}

// deliverChallenge sends the committed mail and activates the code once the
// transport accepted it; either failure leaves no redeemable code.
func (f *AccountMFAFlows) deliverChallenge(ctx context.Context, challenge domain.AccountMFAChallenge, send func(context.Context) error) error {
	if err := send(f.runtime.Detach(ctx)); err != nil {
		f.logger.Warn("mfa challenge delivery failed",
			slog.Int64("account_id", challenge.AccountID),
			slog.String("error", err.Error()),
		)
		return identityaccess.ErrMFAStatusUnavailable
	}
	if err := f.activateDeliveredChallenge(ctx, challenge); err != nil {
		f.logger.Error("failed to activate delivered mfa challenge",
			slog.Int64("challenge_id", challenge.ID),
			slog.String("error", err.Error()),
		)
		return identityaccess.ErrMFAStatusUnavailable
	}
	return nil
}
