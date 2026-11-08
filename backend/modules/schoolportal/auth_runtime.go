package schoolportal

import (
	"context"
	"errors"
	"net"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// The school login, the school session mint, the school switch and the
// account second factor are owned by Identity & Access (#3251, #3331). The
// composition root binds them with the school scope, so a challenge of
// another portal never reaches these handlers.

// VerifiedChallenge is what a redeemed school challenge hands the portal.
type VerifiedChallenge struct {
	MFAResetGeneration int64
	AccountID          int64
	TenantID           int64
}

// AuthRuntime is the school portal's session port.
type AuthRuntime struct {
	// Login authenticates without a pinned school and picks the first
	// school where the account holds a school-portal role.
	Login func(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie string) (*identityaccess.LoginResult, error)
	// LoginAtSchool is the selected-school variant: the account must hold a
	// school-portal role at tenantSlug.
	LoginAtSchool func(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie, tenantSlug string) (*identityaccess.LoginResult, error)
	// IssueTokens mints the school-scope pair for an account that proved
	// its identity through the second factor; the role is re-checked.
	IssueTokens func(ctx context.Context, proof VerifiedChallenge, ipAddress, userAgent string) (accessToken, refreshToken string, err error)
	// SwitchSchool re-authenticates a school session at another school.
	SwitchSchool func(ctx context.Context, accountID int64, tenantSlug, ipAddress, userAgent, sourceFamilyID string) (accessToken, refreshToken string, err error)
}

func (rt *AuthRuntime) complete() bool {
	return rt != nil && rt.Login != nil && rt.LoginAtSchool != nil && rt.IssueTokens != nil && rt.SwitchSchool != nil
}

// MFARuntime is the school portal's second-factor port. Every challenge call
// is bound to the school scope by the composition root.
type MFARuntime struct {
	VerifyChallenge func(ctx context.Context, challengeToken, code string) (VerifiedChallenge, error)
	// VerifyChallengeForOwner additionally pins the challenge to the account
	// and school the enrollment token names, before any code is compared.
	VerifyChallengeForOwner func(ctx context.Context, challengeToken, code string, accountID, tenantID int64) (VerifiedChallenge, error)
	// ResendChallenge re-issues the code and answers with the renewed JWT.
	ResendChallenge func(ctx context.Context, challengeToken string, ip net.IP) (string, error)
	// StartChallenge mails the enrollment code and names its row.
	StartChallenge       func(ctx context.Context, accountID, tenantID int64, ip net.IP) (string, error)
	Enroll               func(ctx context.Context, accountID int64) error
	PrepareTrustedDevice func(ctx context.Context, accountID, tenantID int64, userAgent string, ip net.IP) (identityaccess.TrustedDeviceIssue, error)
}

func (rt *MFARuntime) complete() bool {
	return rt != nil && rt.VerifyChallenge != nil && rt.VerifyChallengeForOwner != nil &&
		rt.ResendChallenge != nil && rt.StartChallenge != nil && rt.Enroll != nil && rt.PrepareTrustedDevice != nil
}

var (
	errLoginUnavailable      = errors.New("account sessions are not composed")
	errMFAServiceUnavailable = errors.New("mfa service is not configured for this deployment")
)
