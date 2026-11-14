package application

import (
	"context"
	"log/slog"
	"net"

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

// tenantless marks a passkey or operator flow of the caterer: it names no
// school, so its events are logged without a school ledger. Its policy is
// the registry default, which the settings binding resolves for the zero
// school whatever school a context carries.
func (c *CatererAccess) tenantless(ctx context.Context) context.Context {
	return withoutSchoolLedgerContext(c.attach(ctx))
}

// flow binds the composed unit and translates the caterer's refusals.
func (c *CatererAccess) flow() flowUnit {
	return flowUnit{attach: c.attach, translate: catererMFAError}
}

// voluntary is the caterer's voluntary second factor.
func (c *CatererAccess) voluntary() voluntaryFactor { return c.factor }

// LoginCaterer signs a caterer account in to the caterer portal. A caterer
// that switched its second factor on answers with a mailed code, unless the
// browser presents a device the caterer remembered.
func (c *CatererAccess) LoginCaterer(ctx context.Context, request identityaccess.CatererLoginRequest) (identityaccess.CatererLoginResult, error) {
	return runFlow(ctx, c.flow(), func(ctx context.Context) (identityaccess.CatererLoginResult, error) {
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
		return c.voluntary().answer(ctx, account, request, func(ctx context.Context, proof mintProof) (mintedSession, error) {
			return c.mintAt(ctx, account, schools[0], proof, request.IPAddress, request.UserAgent)
		})
	})
}

// mintAt persists the caterer session at the school it is kept at and
// issues its pair. The login is filed at that school.
func (c *CatererAccess) mintAt(ctx context.Context, account domain.LoginAccount, schoolID int64, proof mintProof, ipAddress, userAgent string) (mintedSession, error) {
	return c.auth.mintAccountSession(c.runtime.WithTenantID(ctx, schoolID), mintRequest{
		portal: domain.PortalCaterer, account: account, tenantID: schoolID, proof: proof,
		audit: &mintAudit{event: domain.AuthEventLogin, ipAddress: ipAddress, userAgent: userAgent, schoolID: schoolID},
	})
}

// mintProven mints the session of a caterer that proved its second factor:
// a redeemed code or a passkey.
func (c *CatererAccess) mintProven(ctx context.Context, account domain.LoginAccount, proof mintProof, ipAddress, userAgent string) (mintedSession, error) {
	_, schools, err := c.catererSchools(ctx, account.ID)
	if err != nil {
		return mintedSession{}, err
	}
	if len(schools) == 0 {
		return mintedSession{}, failed("caterer login", domain.ErrCatererWithoutSchool)
	}
	return c.mintAt(ctx, account, schools[0], proof, ipAddress, userAgent)
}

func (c *CatererAccess) VerifyCatererMFA(ctx context.Context, verification identityaccess.CatererMFAVerification) (identityaccess.CatererSession, error) {
	return runFlow(ctx, c.flow(), func(ctx context.Context) (identityaccess.CatererSession, error) {
		return c.voluntary().redeem(ctx, verification)
	})
}

// ResendCatererMFA mails a new code for a caterer challenge. Only a caterer
// challenge is accepted: the caterer portal never drives another portal's.
func (c *CatererAccess) ResendCatererMFA(ctx context.Context, challengeToken string, ip net.IP) (string, error) {
	return runFlow(ctx, c.flow(), func(ctx context.Context) (string, error) {
		return c.voluntary().resend(ctx, challengeToken, ip)
	})
}

func (c *CatererAccess) CatererMFAStatus(ctx context.Context, accountID int64) (identityaccess.CatererMFAState, error) {
	return runFlow(ctx, c.flow(), func(ctx context.Context) (identityaccess.CatererMFAState, error) {
		enabled, masked, err := c.voluntary().status(ctx, accountID)
		if err != nil {
			return identityaccess.CatererMFAState{}, err
		}
		return identityaccess.CatererMFAState{Enabled: enabled, MaskedEmail: masked}, nil
	})
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

func (c *CatererAccess) StartCatererMFA(ctx context.Context, accountID int64, ip net.IP) (identityaccess.CatererMFAChallenge, error) {
	return runFlow(ctx, c.flow(), func(ctx context.Context) (identityaccess.CatererMFAChallenge, error) {
		return c.voluntary().start(ctx, accountID, ip)
	})
}

func (c *CatererAccess) ConfirmCatererMFA(ctx context.Context, confirmation identityaccess.CatererMFAConfirmation) (identityaccess.CatererSession, error) {
	return runFlow(ctx, c.flow(), func(ctx context.Context) (identityaccess.CatererSession, error) {
		return c.voluntary().enable(ctx, confirmation)
	})
}

// DisableCatererMFA switches the second factor off. The passkeys stay, as
// at every reset.
func (c *CatererAccess) DisableCatererMFA(ctx context.Context, accountID int64, currentPassword, ipAddress, userAgent string) error {
	return runFlowErr(ctx, c.flow(), func(ctx context.Context) error {
		return c.voluntary().disable(ctx, accountID, currentPassword, ipAddress, userAgent)
	})
}

func (c *CatererAccess) ListCatererTrustedDevices(ctx context.Context, accountID int64) ([]identityaccess.AccountTrustedDevice, error) {
	return runFlow(ctx, c.flow(), func(ctx context.Context) ([]identityaccess.AccountTrustedDevice, error) {
		return c.voluntary().devices(ctx, accountID)
	})
}

func (c *CatererAccess) RevokeCatererTrustedDevice(ctx context.Context, accountID, deviceID int64) error {
	return runFlowErr(ctx, c.flow(), func(ctx context.Context) error {
		return c.voluntary().revoke(ctx, accountID, deviceID)
	})
}
