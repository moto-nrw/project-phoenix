package application

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	identityPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
)

// errMFANotComposed refuses a challenge on a module composed without the
// second factor.
var errMFANotComposed = errors.New("identity access: mfa is not composed")

// operatorMFAGate adapts the composed operator flows to the gate the
// operator login consults. Operator MFA is mandatory, so an unconfigured
// gate issues the token pair directly, as the early phases did.
type OperatorMFAGate struct{ Flows *OperatorMFAFlows }

func (g OperatorMFAGate) Configured() bool { return g.Flows != nil }

func (g OperatorMFAGate) HasEnrollment(ctx context.Context, operatorID int64) (bool, error) {
	if g.Flows == nil {
		return false, nil
	}
	return g.Flows.HasEnrollment(ctx, operatorID)
}

func (g OperatorMFAGate) VerifyTrustedDevice(ctx context.Context, operatorID int64, cookie string) (bool, error) {
	if g.Flows == nil {
		return false, nil
	}
	return g.Flows.VerifyTrustedDevice(ctx, operatorID, cookie)
}

func (g OperatorMFAGate) StartChallenge(ctx context.Context, operatorID int64, ipAddress string) (string, error) {
	if g.Flows == nil {
		return "", errors.New("identity access: operator mfa is not composed")
	}
	return g.Flows.StartChallenge(ctx, operatorID, net.ParseIP(strings.TrimSpace(ipAddress)))
}

func (g OperatorMFAGate) TrustedDeviceDays() int {
	if g.Flows == nil {
		return 0
	}
	return g.Flows.TrustedDeviceDays()
}

type MFAChallengeCodec struct {
	Source identityPorts.MFAChallengeCodec
}

func (c MFAChallengeCodec) IssueChallengeToken(claims domain.MFAChallengeClaims, ttl time.Duration) (string, error) {
	return c.Source.IssueChallengeToken(identityaccess.MFAChallengeClaims(claims), ttl)
}

func (c MFAChallengeCodec) ParseChallengeToken(token string) (domain.MFAChallengeClaims, error) {
	claims, err := c.Source.ParseChallengeToken(token)
	return domain.MFAChallengeClaims(claims), err
}

type MFAMail struct{ Source identityPorts.MFAMail }

func (m MFAMail) EnqueueCode(ctx context.Context, message domain.MFACodeMail, challengeID int64, expiresAt time.Time) (func(context.Context) error, error) {
	if challengeID <= 0 || expiresAt.IsZero() {
		return nil, domain.ErrMFAStatusUnavailable
	}
	return m.Source.EnqueueCode(ctx, identityaccess.MFACodeMail(message), challengeID, expiresAt)
}

func (m MFAMail) NotifyTrustedDeviceAdded(ctx context.Context, message domain.TrustedDeviceMail, deviceID int64) error {
	if deviceID <= 0 {
		return domain.ErrMFAStatusUnavailable
	}
	return m.Source.NotifyTrustedDeviceAdded(ctx, identityaccess.TrustedDeviceMail(message), deviceID)
}

// mfaAuditTrail routes the account events to the authentication ledger and
// the operator entries to the operator action log.
type MFAAuditTrail struct {
	Events   AuthAudit
	Operator identityPorts.OperatorActionLog
}

func (t MFAAuditTrail) RecordAuthEvent(ctx context.Context, event domain.AuthEvent) error {
	return t.Events.RecordAuthEvent(ctx, event)
}

func (t MFAAuditTrail) RecordOperatorAction(ctx context.Context, entry domain.OperatorAuditEntry) error {
	return t.Operator.RecordOperatorAction(ctx, publicOperatorAuditEntry(entry))
}

// parseGateIP reads the client address the login carries as a string.
func parseGateIP(ipAddress string) net.IP {
	return net.ParseIP(strings.TrimSpace(ipAddress))
}
