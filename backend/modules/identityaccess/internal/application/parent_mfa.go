package application

import (
	"context"
	"log/slog"
	"net"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/ports"
	identityPorts "github.com/moto-ogs/project-phoenix/modules/identityaccess/ports"
)

// ParentAccess is the parents portal's sign-in with the account's second
// factor and the parent's own management of it (#7051, ADR 0216 addendum).
// The factor is voluntary (voluntaryFactor): the parent switches it on, and
// an account that has it from its work at a school is asked here too. Every
// session it mints is the parent session of the sign-in (parentTarget),
// with the proof of the factor; its devices are the parents portal's.
type ParentAccess struct {
	auth   *AccountAuthentication
	factor voluntaryFactor
}

// voluntary is the parents' voluntary second factor.
func (p *ParentAccess) voluntary() voluntaryFactor { return p.factor }

// ComposeParentAccess composes the parents portal's sign-in with its second
// factor (#7051); without the sessions there is none.
func ComposeParentAccess(auth *AccountAuthentication, sessions *identityPorts.SessionDependencies, mfa *AccountMFAFlows, runtime ports.Runtime) *identityaccess.ParentAccess {
	if auth == nil || sessions == nil || sessions.Passwords == nil {
		return nil
	}
	logger := sessions.Logger
	if logger == nil {
		logger = slog.Default()
	}
	parents := newParentAccess(auth, mfa, sessions.Passwords, runtime, logger)
	return identityaccess.NewParentAccess(parents, parents)
}

func newParentAccess(auth *AccountAuthentication, mfa *AccountMFAFlows, verifier ports.PasswordVerifier, runtime ports.Runtime, logger *slog.Logger) *ParentAccess {
	parents := &ParentAccess{auth: auth}
	parents.factor = voluntaryFactor{
		portal: domain.PortalParent, auth: auth, mfa: mfa, verifier: verifier, runtime: runtime, logger: logger,
		member: parents.signedInParent, mint: parents.mintProven, notEnabled: identityaccess.ErrParentMFANotEnabled,
	}
	return parents
}

// flow binds the composed unit and translates the refusals like the sign-in.
func (p *ParentAccess) flow() flowUnit {
	if p == nil {
		return flowUnit{unavailable: errAccountAuthenticationUnavailable}
	}
	return flowUnit{attach: p.auth.attach, translate: authenticationError}
}

// signedInParent reads the signed-in account; the session chain in front
// of every settings route proved it is a parent's.
func (p *ParentAccess) signedInParent(ctx context.Context, accountID int64) (domain.LoginAccount, error) {
	return p.auth.authenticatedAccount(ctx, "parent account", accountID)
}

// LoginParent signs a parent in. The portal's refusals come first, so no
// code goes to an account the portal would not open; an account with a
// second factor answers with a mailed code, unless the browser presents a
// device the parents portal remembered.
func (p *ParentAccess) LoginParent(ctx context.Context, request identityaccess.ParentLoginRequest) (identityaccess.ParentLoginResult, error) {
	return runFlow(ctx, p.flow(), func(ctx context.Context) (identityaccess.ParentLoginResult, error) {
		return p.login(ctx, request)
	})
}

func (p *ParentAccess) login(ctx context.Context, request identityaccess.ParentLoginRequest) (identityaccess.ParentLoginResult, error) {
	account, err := p.auth.validateLoginCredentials(ctx, request.Email, request.Password, request.IPAddress, request.UserAgent)
	if err != nil {
		return identityaccess.ParentLoginResult{}, err
	}
	target, err := p.auth.parentTarget(ctx, account, request.IPAddress, request.UserAgent, true)
	if err != nil {
		return identityaccess.ParentLoginResult{}, err
	}
	return p.voluntary().answer(ctx, account, request, func(ctx context.Context, proof mintProof) (mintedSession, error) {
		return p.auth.mintParentSession(ctx, account, request.Email, target, proof, request.IPAddress, request.UserAgent)
	})
}

func (p *ParentAccess) VerifyParentMFA(ctx context.Context, verification identityaccess.ParentMFAVerification) (identityaccess.ParentSession, error) {
	return runFlow(ctx, p.flow(), func(ctx context.Context) (identityaccess.ParentSession, error) {
		return p.voluntary().redeem(ctx, verification)
	})
}

// mintProven mints the parent session of an account that proved its second
// factor. The portal's refusals are read again: the account may have lost
// its last guardian school since the password.
func (p *ParentAccess) mintProven(ctx context.Context, account domain.LoginAccount, proof mintProof, ipAddress, userAgent string) (mintedSession, error) {
	target, err := p.auth.parentTarget(ctx, account, ipAddress, userAgent, true)
	if err != nil {
		return mintedSession{}, err
	}
	return p.auth.mintParentSession(ctx, account, account.Email, target, proof, ipAddress, userAgent)
}

// ResendParentMFA mails a new code. Only a parents-portal challenge is
// accepted: the portal never drives another portal's.
func (p *ParentAccess) ResendParentMFA(ctx context.Context, challengeToken string, ip net.IP) (string, error) {
	return runFlow(ctx, p.flow(), func(ctx context.Context) (string, error) {
		return p.voluntary().resend(ctx, challengeToken, ip)
	})
}

// ParentMFAStatus reports whether the account has the second factor and
// whether it may switch it on here.
func (p *ParentAccess) ParentMFAStatus(ctx context.Context, accountID int64) (identityaccess.ParentMFAState, error) {
	return runFlow(ctx, p.flow(), func(ctx context.Context) (identityaccess.ParentMFAState, error) {
		enabled, masked, err := p.voluntary().status(ctx, accountID)
		if err != nil {
			return identityaccess.ParentMFAState{}, err
		}
		available, err := p.hasGuardianSchool(ctx, accountID)
		if err != nil {
			return identityaccess.ParentMFAState{}, err
		}
		return identityaccess.ParentMFAState{Enabled: enabled, Available: available, MaskedEmail: masked}, nil
	})
}

// hasGuardianSchool reports whether the account is a guardian at a live
// school: the moto team resets a lost factor through such a school.
func (p *ParentAccess) hasGuardianSchool(ctx context.Context, accountID int64) (bool, error) {
	schools, err := p.auth.findGuardianTenantForAccount(ctx, accountID)
	if err != nil {
		return false, failed("parent second factor: enumerate schools", err)
	}
	return len(schools) > 0, nil
}

// StartParentMFA mails the code that switches the second factor on. A
// parent at no school has no way back into the account without the
// mailbox, so the factor stays off for it.
func (p *ParentAccess) StartParentMFA(ctx context.Context, accountID int64, ip net.IP) (identityaccess.ParentMFAChallenge, error) {
	return runFlow(ctx, p.flow(), func(ctx context.Context) (identityaccess.ParentMFAChallenge, error) {
		available, err := p.hasGuardianSchool(ctx, accountID)
		if err != nil {
			return identityaccess.ParentMFAChallenge{}, err
		}
		if !available {
			return identityaccess.ParentMFAChallenge{}, identityaccess.ErrParentMFANeedsSchool
		}
		return p.voluntary().start(ctx, accountID, ip)
	})
}

func (p *ParentAccess) ConfirmParentMFA(ctx context.Context, confirmation identityaccess.ParentMFAConfirmation) (identityaccess.ParentSession, error) {
	return runFlow(ctx, p.flow(), func(ctx context.Context) (identityaccess.ParentSession, error) {
		return p.voluntary().enable(ctx, confirmation)
	})
}

// DisableParentMFA switches the second factor off once the current password
// is checked like a sign-in. The factor belongs to the account, so it goes
// at every portal; a school or Träger that requires it asks for it again at
// the next sign-in there.
func (p *ParentAccess) DisableParentMFA(ctx context.Context, accountID int64, currentPassword, ipAddress, userAgent string) error {
	return runFlowErr(ctx, p.flow(), func(ctx context.Context) error {
		return p.voluntary().disable(ctx, accountID, currentPassword, ipAddress, userAgent)
	})
}

// ListParentTrustedDevices lists the devices the parents portal remembered.
func (p *ParentAccess) ListParentTrustedDevices(ctx context.Context, accountID int64) ([]identityaccess.AccountTrustedDevice, error) {
	return runFlow(ctx, p.flow(), func(ctx context.Context) ([]identityaccess.AccountTrustedDevice, error) {
		return p.voluntary().devices(ctx, accountID)
	})
}

// RevokeParentTrustedDevice forgets one device the parents portal
// remembered.
func (p *ParentAccess) RevokeParentTrustedDevice(ctx context.Context, accountID, deviceID int64) error {
	return runFlowErr(ctx, p.flow(), func(ctx context.Context) error {
		return p.voluntary().revoke(ctx, accountID, deviceID)
	})
}
