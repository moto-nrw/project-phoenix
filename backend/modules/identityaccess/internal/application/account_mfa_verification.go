package application

import (
	"context"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// VerifyChallenge verifies a tenant-portal challenge.
func (f *AccountMFAFlows) VerifyChallenge(ctx context.Context, challengeToken, code string) (domain.VerifiedMFAChallenge, error) {
	return f.VerifyChallengeForScope(ctx, challengeToken, code, domain.MFAChallengeScopeTenant)
}

// VerifyChallengeForScope refuses a challenge started for another portal
// before any code is compared, so a school code can never be burned — or
// redeemed — at the tenant endpoint and the other way round.
func (f *AccountMFAFlows) VerifyChallengeForScope(ctx context.Context, challengeToken, code, expectedScope string) (domain.VerifiedMFAChallenge, error) {
	return f.verifyChallengeBound(ctx, challengeToken, code, expectedScope, nil)
}

// VerifyChallengeForOwner additionally pins the challenge to an account and
// a school the caller authenticated by other means. The ownership check runs
// before the code comparison and therefore before the single-use consume:
// checking the result afterwards burns a foreign challenge on its way to the
// 401, which kills the rightful owner's in-flight login.
func (f *AccountMFAFlows) VerifyChallengeForOwner(ctx context.Context, challengeToken, code, expectedScope string, accountID, tenantID int64) (domain.VerifiedMFAChallenge, error) {
	return f.verifyChallengeBound(ctx, challengeToken, code, expectedScope, &challengeOwner{accountID: accountID, tenantID: tenantID})
}

func (f *AccountMFAFlows) verifyChallengeBound(ctx context.Context, challengeToken, code, expectedScope string, owner *challengeOwner) (domain.VerifiedMFAChallenge, error) {
	ctx = schoollessMFAContext(ctx, expectedScope)
	claims, err := f.challengeClaims(challengeToken, expectedScope, owner)
	if err != nil {
		return domain.VerifiedMFAChallenge{}, err
	}
	account, found, err := f.records.FindAccountIdentity(ctx, claims.AccountID)
	if err != nil || !found {
		return domain.VerifiedMFAChallenge{}, identityaccess.ErrMFAChallengeTokenInvalid
	}
	if domain.MFALocked(account.MFALockedUntil, f.now()) {
		return domain.VerifiedMFAChallenge{}, identityaccess.ErrMFALocked
	}
	// The token names the exact row it was minted for. "Newest active code
	// for this account" would let two concurrent challenges — a tenant login
	// and a school login of the same person — redeem each other's code. A
	// token without the claim predates it and is refused rather than
	// downgraded to the ambiguous lookup.
	if claims.ChallengeID == 0 {
		return domain.VerifiedMFAChallenge{}, identityaccess.ErrMFAChallengeTokenInvalid
	}
	active, activeFound, err := f.records.FindActiveChallengeForAccount(ctx, claims.ChallengeID, claims.AccountID)
	if err != nil || !activeFound {
		f.recordFailure(ctx, claims.AccountID, claims.TenantID, "no active challenge")
		return domain.VerifiedMFAChallenge{}, identityaccess.ErrMFACodeInvalid
	}
	// The scope check above trusts the JWT alone. Re-check it against the
	// stored row so a forged or mixed-up pairing is refused before the code
	// comparison rather than after it.
	if active.Scope != expectedScope || (claims.TenantID > 0 && active.TenantID != claims.TenantID) {
		f.recordFailure(ctx, claims.AccountID, claims.TenantID, "challenge portal mismatch")
		return domain.VerifiedMFAChallenge{}, identityaccess.ErrMFAUnsupportedScope
	}

	if err := f.verifyBoundChallengeCode(ctx, claims, account, active, code); err != nil {
		return domain.VerifiedMFAChallenge{}, err
	}

	if err := f.completeChallengeVerification(ctx, claims, account.ID, active.ID); err != nil {
		return domain.VerifiedMFAChallenge{}, err
	}
	return domain.VerifiedMFAChallenge{AccountID: claims.AccountID, Scope: claims.Scope, TenantID: claims.TenantID, MFAResetGeneration: active.MFAResetGeneration}, nil
}

// VerifyCodeForAccount runs the same pipeline without the JWT round trip:
// the caller authenticated the user out of band (an enrollment token, a
// tenant session at passkey registration).
//
// Without a challenge id the row is resolved as "the newest active code of
// this account", which is only safe once narrowed to the portal asking.
// expectedScope and tenantID are therefore required, not decorative:
// without them a Lehrkraft with a school login challenge in one tab could
// have that code consumed here and a tenant session minted from a second
// factor issued for another portal.
func (f *AccountMFAFlows) VerifyCodeForAccount(ctx context.Context, accountID, tenantID int64, code, expectedScope string) error {
	_, err := f.VerifyEnrollmentCode(ctx, accountID, tenantID, code, expectedScope)
	return err
}

func (f *AccountMFAFlows) VerifyEnrollmentCode(ctx context.Context, accountID, tenantID int64, code, expectedScope string) (domain.VerifiedMFAChallenge, error) {
	ctx = schoollessMFAContext(ctx, expectedScope)
	account, found, err := f.records.FindAccountIdentity(ctx, accountID)
	if err != nil || !found {
		return domain.VerifiedMFAChallenge{}, identityaccess.ErrMFACodeInvalid
	}
	if domain.MFALocked(account.MFALockedUntil, f.now()) {
		return domain.VerifiedMFAChallenge{}, identityaccess.ErrMFALocked
	}
	// The challenge and lockout policy use the explicit school. Audit events
	// below use the authenticated school context.
	active, activeFound, err := f.records.FindActiveChallengeInScope(ctx, accountID, tenantID, expectedScope)
	if err != nil || !activeFound {
		f.recordFailure(ctx, accountID, 0, "no active challenge")
		return domain.VerifiedMFAChallenge{}, identityaccess.ErrMFACodeInvalid
	}
	threshold, duration, err := f.resolveLockoutPolicy(ctx, tenantID)
	if err != nil {
		return domain.VerifiedMFAChallenge{}, err
	}
	ok, verifyErr := f.codes.VerifyShortCode(code, active.CodeHash)
	if verifyErr != nil || !ok {
		if err := f.handleFailedAttempt(ctx, account, tenantID, threshold, duration); err != nil {
			return domain.VerifiedMFAChallenge{}, err
		}
		f.recordFailure(ctx, accountID, 0, "code mismatch")
		return domain.VerifiedMFAChallenge{}, identityaccess.ErrMFACodeInvalid
	}
	if err := f.records.ConsumeChallenge(ctx, active.ID, f.now()); err != nil {
		f.logger.Warn("failed to mark challenge consumed; refusing verify",
			slog.Int64("account_id", accountID),
			slog.Int64("challenge_id", active.ID),
			slog.String("error", err.Error()))
		f.recordFailure(ctx, accountID, 0, "consume race")
		return domain.VerifiedMFAChallenge{}, identityaccess.ErrMFACodeInvalid
	}
	// The caller may run this inside its own transaction (passkey
	// registration), where a failed statement aborts everything after it. The
	// failure is therefore returned, not just logged, so it cannot surface
	// later as an unrelated error.
	if err := f.records.ResetMFAAttempts(ctx, accountID); err != nil {
		f.logger.Warn("failed to reset MFA attempts",
			slog.Int64("account_id", accountID),
			slog.String("error", err.Error()),
		)
		return domain.VerifiedMFAChallenge{}, identityaccess.ErrMFAStatusUnavailable
	}
	f.recordAuthEvent(ctx, domain.AuthEvent{
		AccountID: accountID, Type: domain.AuthEventMFAVerified, Success: true,
		IPAddress: domain.MFAAuditFallbackIP,
	})
	return domain.VerifiedMFAChallenge{AccountID: accountID, TenantID: tenantID, Scope: expectedScope, MFAResetGeneration: active.MFAResetGeneration}, nil
}

// resolveLockoutPolicy runs before code comparison so an unavailable policy
// cannot turn uncounted guesses into a successful verification.
func (f *AccountMFAFlows) resolveLockoutPolicy(ctx context.Context, tenantID int64) (int, time.Duration, error) {
	threshold, err := f.settings.LockoutThreshold(ctx, tenantID)
	if err != nil || threshold <= 0 {
		return 0, 0, f.securitySettingsFailure("account lockout threshold", err)
	}
	duration, err := f.settings.LockoutDuration(ctx, tenantID)
	if err != nil || duration <= 0 {
		return 0, 0, f.securitySettingsFailure("account lockout duration", err)
	}
	return threshold, duration, nil
}

// handleFailedAttempt bumps the lockout counter atomically using the policy
// resolved before comparison. A read-modify-write would let two concurrent
// failed verifies collapse into one counted attempt.
func (f *AccountMFAFlows) handleFailedAttempt(ctx context.Context, account domain.AccountIdentity, tenantID int64, threshold int, duration time.Duration) error {
	result, err := f.records.IncrementMFAAttempts(ctx, account.ID, threshold, duration)
	if err != nil {
		f.logger.Warn("failed to persist MFA attempt counter",
			slog.String("error", err.Error()),
		)
		return identityaccess.ErrMFAStatusUnavailable
	}
	// result.Attempts == threshold means this increment was the N-th, and
	// only this caller sees it. Later attempts push the window forward
	// without re-emitting.
	if result.Attempts == threshold {
		f.recordAuthEvent(ctx, domain.AuthEvent{
			AccountID: account.ID, TenantID: tenantID, Type: domain.AuthEventMFALocked,
			IPAddress: domain.MFAAuditFallbackIP, MFA: &domain.MFAEvidence{LockedUntil: result.LockedUntil},
		})
	}
	return nil
}

func (f *AccountMFAFlows) verifyBoundChallengeCode(ctx context.Context, claims domain.MFAChallengeClaims, account domain.AccountIdentity, active domain.AccountMFAChallenge, code string) error {
	threshold, duration, err := f.resolveLockoutPolicy(ctx, claims.TenantID)
	if err != nil {
		return err
	}
	ok, verifyErr := f.codes.VerifyShortCode(code, active.CodeHash)
	if verifyErr != nil || !ok {
		if err := f.handleFailedAttempt(ctx, account, claims.TenantID, threshold, duration); err != nil {
			return err
		}
		f.recordFailure(ctx, claims.AccountID, claims.TenantID, "code mismatch")
		return identityaccess.ErrMFACodeInvalid
	}

	return nil
}
