package application

import (
	"context"
	"net"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// The caterer's passkeys (#4497): the registration, gated by a mailed code
// like every registration, the discoverable sign-in on the caterer portal,
// and the list the caterer manages in its account. A ceremony runs only on
// the caterer portal's own host, and every account route proves the
// signed-in account is a caterer's.

// requirePasskeys refuses a ceremony of a composition without passkeys, and
// one that did not start on the caterer portal's host.
func (c *CatererAccess) requirePasskeys(expectedOrigin string) error {
	if c.passkeys == nil || c.passkeys.catererHost == "" {
		return identityaccess.ErrPasskeyFlowsUnavailable
	}
	if expectedOrigin != "" {
		return domain.ValidateOperatorPasskeyOrigin(expectedOrigin, c.passkeys.catererHost)
	}
	return nil
}

// passkeysOf proves the signed-in account is a caterer's before one of its
// passkey routes runs.
func (c *CatererAccess) passkeysOf(ctx context.Context, accountID int64, expectedOrigin string) error {
	if err := c.requirePasskeys(expectedOrigin); err != nil {
		return err
	}
	_, err := c.signedInCaterer(ctx, accountID)
	return err
}

// BeginCatererPasskeyLogin starts a passkey sign-in on the caterer portal.
func (c *CatererAccess) BeginCatererPasskeyLogin(ctx context.Context, expectedOrigin string) (identityaccess.PasskeyCeremonyOptions, error) {
	options, err := c.beginPasskeyLogin(c.tenantless(ctx), expectedOrigin)
	return identityaccess.PasskeyCeremonyOptions(options), catererMFAError(err)
}

func (c *CatererAccess) beginPasskeyLogin(ctx context.Context, expectedOrigin string) (domain.PasskeyCeremonyOptions, error) {
	if expectedOrigin == "" {
		return domain.PasskeyCeremonyOptions{}, identityaccess.ErrPasskeyOriginInvalid
	}
	if err := c.requirePasskeys(expectedOrigin); err != nil {
		return domain.PasskeyCeremonyOptions{}, err
	}
	return c.passkeys.beginCatererLogin(ctx, expectedOrigin)
}

// FinishCatererPasskeyLogin completes the sign-in and mints the caterer
// session. A passkey with user verification proves the second factor.
func (c *CatererAccess) FinishCatererPasskeyLogin(ctx context.Context, request identityaccess.AccountPasskeyLoginFinish) (identityaccess.PasskeyLoginResult, error) {
	result, err := c.finishPasskeyLogin(c.tenantless(ctx), request)
	return result, catererMFAError(err)
}

func (c *CatererAccess) finishPasskeyLogin(ctx context.Context, request identityaccess.AccountPasskeyLoginFinish) (identityaccess.PasskeyLoginResult, error) {
	if err := c.requirePasskeys(""); err != nil {
		return identityaccess.PasskeyLoginResult{}, err
	}
	accountID, err := c.passkeys.finishCatererLogin(ctx, domain.AccountPasskeyLoginFinish(request), func(txCtx context.Context, accountID int64) (bool, error) {
		_, isCaterer, _, err := c.store.FindCatererOfAccount(txCtx, accountID)
		return isCaterer, err
	})
	if err != nil {
		return identityaccess.PasskeyLoginResult{}, err
	}
	session, err := c.mintProvenSession(ctx, accountID, nil, request.IPAddress, request.UserAgent)
	if err != nil {
		return identityaccess.PasskeyLoginResult{}, err
	}
	return identityaccess.PasskeyLoginResult{AccessToken: session.AccessToken, RefreshToken: session.RefreshToken}, nil
}

// StartCatererPasskeyEnrollment mails the code a registration echoes back.
func (c *CatererAccess) StartCatererPasskeyEnrollment(ctx context.Context, accountID int64, ip net.IP) (identityaccess.PasskeyEnrollmentChallenge, error) {
	challenge, err := c.startPasskeyEnrollment(c.tenantless(ctx), accountID, ip)
	return identityaccess.PasskeyEnrollmentChallenge(challenge), catererMFAError(err)
}

func (c *CatererAccess) startPasskeyEnrollment(ctx context.Context, accountID int64, ip net.IP) (domain.PasskeyEnrollmentChallenge, error) {
	if err := c.passkeysOf(ctx, accountID, ""); err != nil {
		return domain.PasskeyEnrollmentChallenge{}, err
	}
	return c.passkeys.startCatererEnrollmentChallenge(ctx, accountID, ip)
}

// BeginCatererPasskeyRegistration verifies the mailed code and starts the
// registration on the caterer portal.
func (c *CatererAccess) BeginCatererPasskeyRegistration(ctx context.Context, request identityaccess.CatererPasskeyRegistrationStart) (identityaccess.PasskeyCeremonyOptions, error) {
	options, err := c.beginPasskeyRegistration(c.tenantless(ctx), request)
	return identityaccess.PasskeyCeremonyOptions(options), catererMFAError(err)
}

func (c *CatererAccess) beginPasskeyRegistration(ctx context.Context, request identityaccess.CatererPasskeyRegistrationStart) (domain.PasskeyCeremonyOptions, error) {
	if request.ExpectedOrigin == "" {
		return domain.PasskeyCeremonyOptions{}, identityaccess.ErrPasskeyOriginInvalid
	}
	if err := c.passkeysOf(ctx, request.AccountID, request.ExpectedOrigin); err != nil {
		return domain.PasskeyCeremonyOptions{}, err
	}
	return c.passkeys.beginCatererRegistration(ctx, request.AccountID, request.ExpectedOrigin, request.Code)
}

// FinishCatererPasskeyRegistration stores the caterer's new passkey.
func (c *CatererAccess) FinishCatererPasskeyRegistration(ctx context.Context, request identityaccess.AccountPasskeyRegistrationFinish) (identityaccess.PasskeyCredentialSummary, error) {
	summary, err := c.finishPasskeyRegistration(c.tenantless(ctx), request)
	return identityaccess.PasskeyCredentialSummary(summary), catererMFAError(err)
}

func (c *CatererAccess) finishPasskeyRegistration(ctx context.Context, request identityaccess.AccountPasskeyRegistrationFinish) (domain.PasskeyCredentialSummary, error) {
	if err := c.passkeysOf(ctx, request.AccountID, ""); err != nil {
		return domain.PasskeyCredentialSummary{}, err
	}
	return c.passkeys.FinishRegistration(ctx, domain.AccountPasskeyRegistrationFinish(request))
}

// ListCatererPasskeys lists the caterer's passkeys, oldest first.
func (c *CatererAccess) ListCatererPasskeys(ctx context.Context, accountID int64) ([]identityaccess.PasskeyCredentialSummary, error) {
	summaries, err := c.listPasskeys(c.tenantless(ctx), accountID)
	if err != nil {
		return nil, catererMFAError(err)
	}
	return publicPasskeySummaries(summaries), nil
}

func (c *CatererAccess) listPasskeys(ctx context.Context, accountID int64) ([]domain.PasskeyCredentialSummary, error) {
	if err := c.passkeysOf(ctx, accountID, ""); err != nil {
		return nil, err
	}
	return c.passkeys.ListCredentials(ctx, accountID)
}

// RevokeCatererPasskey removes one passkey of the caterer.
func (c *CatererAccess) RevokeCatererPasskey(ctx context.Context, accountID, passkeyID int64) error {
	return catererMFAError(c.revokePasskey(c.tenantless(ctx), accountID, passkeyID))
}

func (c *CatererAccess) revokePasskey(ctx context.Context, accountID, passkeyID int64) error {
	if err := c.passkeysOf(ctx, accountID, ""); err != nil {
		return err
	}
	return c.passkeys.RevokeCredential(ctx, accountID, passkeyID)
}
