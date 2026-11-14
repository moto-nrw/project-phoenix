package application

import (
	"context"
	"fmt"
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

// ParentAccessDependencies are the ports the parent flows consume beside the
// sign-in.
type ParentAccessDependencies struct {
	MFA      *AccountMFAFlows
	Verifier ports.PasswordVerifier
	Runtime  ports.Runtime
	Logger   *slog.Logger
}

func NewParentAccess(auth *AccountAuthentication, deps ParentAccessDependencies) (*ParentAccess, error) {
	if auth == nil || deps.Verifier == nil || deps.Runtime == nil {
		return nil, fmt.Errorf("identity access parent access: the sign-in, the password check and the runtime are required")
	}
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	parents := &ParentAccess{auth: auth}
	parents.factor = voluntaryFactor{
		portal: domain.PortalParent, auth: auth, mfa: deps.MFA, verifier: deps.Verifier, runtime: deps.Runtime, logger: logger,
		bind: func(ctx context.Context) context.Context {
			return identityaccess.WithParentDevice(withoutSchoolLedgerContext(ctx))
		},
		member:     parents.signedInParent,
		notEnabled: identityaccess.ErrParentMFANotEnabled,
	}
	return parents, nil
}

// ComposeParentAccess composes the parents portal's sign-in with its second
// factor (#7051); without the sessions there is none.
func ComposeParentAccess(auth *AccountAuthentication, sessions *identityPorts.SessionDependencies, mfa *AccountMFAFlows, runtime ports.Runtime) *identityaccess.ParentAccess {
	if auth == nil || sessions == nil || sessions.Passwords == nil {
		return nil
	}
	access, err := NewParentAccess(auth, ParentAccessDependencies{MFA: mfa, Verifier: sessions.Passwords, Runtime: runtime, Logger: sessions.Logger})
	if err != nil {
		return nil
	}
	return identityaccess.NewParentAccess(access, access)
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
		account, err := p.auth.validateLoginCredentials(ctx, request.Email, request.Password, request.IPAddress, request.UserAgent)
		if err != nil {
			return identityaccess.ParentLoginResult{}, err
		}
		target, err := p.auth.parentTarget(ctx, account, request.IPAddress, request.UserAgent, true)
		if err != nil {
			return identityaccess.ParentLoginResult{}, err
		}
		gate, err := p.voluntary().gate(ctx, account, request.TrustedDeviceCookie, request.IPAddress)
		if err != nil {
			return identityaccess.ParentLoginResult{}, err
		}
		if gate.challenge != nil {
			return identityaccess.ParentLoginResult{
				Status: identityaccess.LoginStatusMFARequired, ChallengeToken: gate.challenge.ChallengeToken,
				MaskedEmail: gate.challenge.MaskedEmail, TrustedDeviceDays: gate.challenge.TrustedDeviceDays,
			}, nil
		}
		minted, err := p.auth.mintParentSession(ctx, account, request.Email, target, gate.proof, request.IPAddress, request.UserAgent)
		if err != nil {
			return identityaccess.ParentLoginResult{}, err
		}
		return identityaccess.ParentLoginResult{
			Status: identityaccess.LoginStatusAuthenticated, AccessToken: minted.accessToken, RefreshToken: minted.refreshToken,
		}, nil
	})
}

// VerifyParentMFA redeems the mailed code of a sign-in and mints the parent
// session. A remembered device is prepared before the mint and stored after
// it.
func (p *ParentAccess) VerifyParentMFA(ctx context.Context, verification identityaccess.ParentMFAVerification) (identityaccess.ParentSession, error) {
	return runFlow(ctx, p.flow(), func(ctx context.Context) (identityaccess.ParentSession, error) {
		proof, err := p.voluntary().verifySignIn(ctx, verification.ChallengeToken, verification.Code, verification.RememberDevice, verification.IPAddress, verification.UserAgent)
		if err != nil {
			return identityaccess.ParentSession{}, err
		}
		session, err := p.mintProvenSession(ctx, proof.challenge, verification.IPAddress, verification.UserAgent)
		if err != nil {
			return identityaccess.ParentSession{}, err
		}
		session.TrustedDeviceCookie, session.TrustedDeviceExpiresAt = p.voluntary().remember(ctx, proof.challenge.AccountID, proof.device)
		return session, nil
	})
}

// mintProvenSession mints the parent session of an account that proved its
// second factor. The portal's refusals are read again: the account may have
// lost its last guardian school since the password.
func (p *ParentAccess) mintProvenSession(ctx context.Context, proof identityaccess.VerifiedMFAChallenge, ipAddress, userAgent string) (identityaccess.ParentSession, error) {
	account, err := p.auth.authenticatedAccount(ctx, "issue parent tokens", proof.AccountID)
	if err != nil {
		return identityaccess.ParentSession{}, err
	}
	target, err := p.auth.parentTarget(ctx, account, ipAddress, userAgent, true)
	if err != nil {
		return identityaccess.ParentSession{}, err
	}
	minted, err := p.auth.mintParentSession(ctx, account, account.Email, target, challengeProof(&proof), ipAddress, userAgent)
	if err != nil {
		return identityaccess.ParentSession{}, err
	}
	return identityaccess.ParentSession{AccessToken: minted.accessToken, RefreshToken: minted.refreshToken, Email: account.Email}, nil
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
		token, masked, err := p.voluntary().start(ctx, accountID, ip)
		if err != nil {
			return identityaccess.ParentMFAChallenge{}, err
		}
		return identityaccess.ParentMFAChallenge{ChallengeToken: token, MaskedEmail: masked}, nil
	})
}

// ConfirmParentMFA switches the second factor on with the mailed code and
// mints a parent session proven by it. Every other session of the account
// was minted without the factor and ends at its next refresh.
func (p *ParentAccess) ConfirmParentMFA(ctx context.Context, confirmation identityaccess.ParentMFAConfirmation) (identityaccess.ParentSession, error) {
	return runFlow(ctx, p.flow(), func(ctx context.Context) (identityaccess.ParentSession, error) {
		proof, err := p.voluntary().confirm(ctx, confirmation.AccountID, confirmation.ChallengeToken, confirmation.Code)
		if err != nil {
			return identityaccess.ParentSession{}, err
		}
		return p.mintProvenSession(ctx, proof, confirmation.IPAddress, confirmation.UserAgent)
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
