package application

import (
	"context"
	"net"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// The caterer portal's passkey ceremonies (#4497). A caterer belongs to no
// school, so its ceremonies record none: a school's login ceremony refuses
// a ceremony without a school, and the caterer's refuses one with a school.
// The account's credentials are the same records as on the school portals;
// the caterer portal's host and the caterer account bind them here.

// startCatererEnrollmentChallenge mails the code a caterer's registration
// echoes back.
func (f *AccountPasskeyFlows) startCatererEnrollmentChallenge(ctx context.Context, accountID int64, ip net.IP) (domain.PasskeyEnrollmentChallenge, error) {
	account, found, err := f.accounts.FindAccountIdentity(ctx, accountID)
	if err != nil || !found {
		return domain.PasskeyEnrollmentChallenge{}, domain.ErrAccountNotFound
	}
	token, err := f.mfa.StartChallenge(ctx, accountID, 0, domain.MFAChallengeScopeCaterer, ip)
	if err != nil {
		return domain.PasskeyEnrollmentChallenge{}, err
	}
	return domain.PasskeyEnrollmentChallenge{ChallengeToken: token, MaskedEmail: domain.MaskEmail(account.Email)}, nil
}

// beginCatererRegistration verifies the caterer's mailed code and starts the
// attestation. The caller checked the origin against the caterer host.
func (f *AccountPasskeyFlows) beginCatererRegistration(ctx context.Context, accountID int64, expectedOrigin, code string) (domain.PasskeyCeremonyOptions, error) {
	if err := f.mfa.VerifyCodeForAccount(ctx, accountID, 0, code, domain.MFAChallengeScopeCaterer); err != nil {
		return domain.PasskeyCeremonyOptions{}, err
	}
	account, found, err := f.accounts.FindAccountIdentity(ctx, accountID)
	if err != nil || !found {
		return domain.PasskeyCeremonyOptions{}, domain.ErrAccountNotFound
	}
	return f.beginRegistrationCeremony(ctx, account, expectedOrigin, nil)
}

// beginCatererLogin starts a discoverable login on the caterer portal. The
// caller checked the origin against the caterer host.
func (f *AccountPasskeyFlows) beginCatererLogin(ctx context.Context, expectedOrigin string) (domain.PasskeyCeremonyOptions, error) {
	return f.beginLoginCeremony(ctx, expectedOrigin, nil)
}

// finishCatererLogin consumes the ceremony, verifies the assertion, proves
// the account is a caterer's and records the credential use in one
// administrative transaction. It returns the account; the caller mints the
// caterer session after the commit.
func (f *AccountPasskeyFlows) finishCatererLogin(ctx context.Context, request domain.AccountPasskeyLoginFinish, isCaterer func(context.Context, int64) (bool, error)) (int64, error) {
	var accountID int64
	err := completeCeremony(ctx, f.runtime, func(txCtx context.Context) error {
		session, err := f.consumeLoginSession(txCtx, request.SessionID)
		if err != nil {
			return err
		}
		if session.TenantID != nil {
			// A school's ceremony never opens the caterer portal.
			return rejectCeremony(identityaccess.ErrPasskeySessionInvalid)
		}
		verified, err := f.verifyAssertion(txCtx, session, request)
		if err != nil {
			return err
		}
		caterer, err := isCaterer(txCtx, verified.accountID)
		if err != nil {
			return err
		}
		if !caterer {
			// Answered like an unknown passkey: the caterer portal names no
			// other kind of account.
			return rejectCeremony(identityaccess.ErrInvalidCredentials)
		}
		if err := f.records.RecordUse(txCtx, verified.credentialID, verified.credentialJSON, f.now()); err != nil {
			return err
		}
		accountID = verified.accountID
		return nil
	})
	return accountID, err
}
