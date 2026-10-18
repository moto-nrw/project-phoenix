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

type AccountMFARecords struct {
	Source identityPorts.AccountMFARecords
}

func (r AccountMFARecords) FindAccountIdentity(ctx context.Context, accountID int64) (domain.AccountIdentity, bool, error) {
	identity, found, err := r.Source.FindAccountIdentity(ctx, accountID)
	return domain.AccountIdentity(identity), found, err
}

func (r AccountMFARecords) AccountBelongsToTenant(ctx context.Context, accountID, tenantID int64) (bool, error) {
	return r.Source.AccountBelongsToTenant(ctx, accountID, tenantID)
}

func (r AccountMFARecords) LockAccountForOverrideWrite(ctx context.Context, accountID int64) error {
	return r.Source.LockAccountForOverrideWrite(ctx, accountID)
}

func (r AccountMFARecords) IncrementMFAAttempts(ctx context.Context, accountID int64, threshold int, duration time.Duration) (domain.AccountLockout, error) {
	lockout, err := r.Source.IncrementMFAAttempts(ctx, accountID, threshold, duration)
	return domain.AccountLockout(lockout), err
}

func (r AccountMFARecords) ResetMFAAttempts(ctx context.Context, accountID int64) error {
	return r.Source.ResetMFAAttempts(ctx, accountID)
}

func (r AccountMFARecords) FindCredential(ctx context.Context, accountID int64) (domain.AccountMFACredential, bool, error) {
	credential, found, err := r.Source.FindCredential(ctx, accountID)
	return domain.AccountMFACredential(credential), found, err
}

func (r AccountMFARecords) CreateCredential(ctx context.Context, credential domain.AccountMFACredential) error {
	return r.Source.CreateCredential(ctx, identityaccess.AccountMFACredential(credential))
}

func (r AccountMFARecords) TouchCredential(ctx context.Context, id int64, usedAt time.Time) error {
	return r.Source.TouchCredential(ctx, id, usedAt)
}

func (r AccountMFARecords) DeleteCredentials(ctx context.Context, accountID int64) error {
	return r.Source.DeleteCredentials(ctx, accountID)
}

func (r AccountMFARecords) CreateChallenge(ctx context.Context, challenge domain.AccountMFAChallenge) (domain.AccountMFAChallenge, error) {
	stored, err := r.Source.CreateChallenge(ctx, identityaccess.AccountMFAChallenge(challenge))
	return domain.AccountMFAChallenge(stored), err
}

func (r AccountMFARecords) ActivateChallenge(ctx context.Context, id int64) error {
	return r.Source.ActivateChallenge(ctx, id)
}

func (r AccountMFARecords) ConsumeChallenge(ctx context.Context, id int64, consumedAt time.Time) error {
	return r.Source.ConsumeChallenge(ctx, id, consumedAt)
}

func (r AccountMFARecords) CountChallengesSince(ctx context.Context, accountID int64, since time.Time) (int, error) {
	return r.Source.CountChallengesSince(ctx, accountID, since)
}

func (r AccountMFARecords) FindActiveChallengeForAccount(ctx context.Context, id, accountID int64) (domain.AccountMFAChallenge, bool, error) {
	challenge, found, err := r.Source.FindActiveChallengeForAccount(ctx, id, accountID)
	return domain.AccountMFAChallenge(challenge), found, err
}

func (r AccountMFARecords) FindActiveChallengeInScope(ctx context.Context, accountID, tenantID int64, scope string) (domain.AccountMFAChallenge, bool, error) {
	challenge, found, err := r.Source.FindActiveChallengeInScope(ctx, accountID, tenantID, scope)
	return domain.AccountMFAChallenge(challenge), found, err
}

func (r AccountMFARecords) CreateTrustedDevice(ctx context.Context, device domain.AccountTrustedDevice) (domain.AccountTrustedDevice, error) {
	stored, err := r.Source.CreateTrustedDevice(ctx, identityaccess.AccountTrustedDevice(device))
	return domain.AccountTrustedDevice(stored), err
}

func (r AccountMFARecords) FindActiveTrustedDevice(ctx context.Context, accountID, tenantID int64, tokenHash string) (domain.AccountTrustedDevice, bool, error) {
	device, found, err := r.Source.FindActiveTrustedDevice(ctx, accountID, tenantID, tokenHash)
	return domain.AccountTrustedDevice(device), found, err
}

func (r AccountMFARecords) ListActiveTrustedDevices(ctx context.Context, accountID, tenantID int64) ([]domain.AccountTrustedDevice, error) {
	devices, err := r.Source.ListActiveTrustedDevices(ctx, accountID, tenantID)
	if err != nil {
		return nil, err
	}
	result := make([]domain.AccountTrustedDevice, 0, len(devices))
	for _, device := range devices {
		result = append(result, domain.AccountTrustedDevice(device))
	}
	return result, nil
}

func (r AccountMFARecords) TouchTrustedDevice(ctx context.Context, id int64, usedAt time.Time) error {
	return r.Source.TouchTrustedDevice(ctx, id, usedAt)
}

func (r AccountMFARecords) RevokeTrustedDevice(ctx context.Context, id int64, revokedAt time.Time) error {
	return r.Source.RevokeTrustedDevice(ctx, id, revokedAt)
}

func (r AccountMFARecords) RevokeAllTrustedDevices(ctx context.Context, accountID int64, revokedAt time.Time) error {
	return r.Source.RevokeAllTrustedDevices(ctx, accountID, revokedAt)
}

func (r AccountMFARecords) RevokeTenantTrustedDevices(ctx context.Context, accountID, tenantID int64, revokedAt time.Time) error {
	return r.Source.RevokeTenantTrustedDevices(ctx, accountID, tenantID, revokedAt)
}

func (r AccountMFARecords) FindGlobalOverride(ctx context.Context, accountID int64) (domain.AccountMFAOverride, bool, error) {
	override, found, err := r.Source.FindGlobalOverride(ctx, accountID)
	return domain.AccountMFAOverride(override), found, err
}

func (r AccountMFARecords) FindTenantOverride(ctx context.Context, accountID, tenantID int64) (domain.AccountMFAOverride, bool, error) {
	override, found, err := r.Source.FindTenantOverride(ctx, accountID, tenantID)
	return domain.AccountMFAOverride(override), found, err
}

func (r AccountMFARecords) UpsertGlobalOverride(ctx context.Context, override domain.AccountMFAOverride) error {
	return r.Source.UpsertGlobalOverride(ctx, identityaccess.AccountMFAOverride(override))
}

func (r AccountMFARecords) UpsertTenantOverride(ctx context.Context, override domain.AccountMFAOverride) error {
	return r.Source.UpsertTenantOverride(ctx, identityaccess.AccountMFAOverride(override))
}

func (r AccountMFARecords) DeleteGlobalOverride(ctx context.Context, accountID int64) error {
	return r.Source.DeleteGlobalOverride(ctx, accountID)
}

func (r AccountMFARecords) DeleteTenantOverride(ctx context.Context, accountID, tenantID int64) error {
	return r.Source.DeleteTenantOverride(ctx, accountID, tenantID)
}

// parseGateIP reads the client address the login carries as a string.
func parseGateIP(ipAddress string) net.IP {
	return net.ParseIP(strings.TrimSpace(ipAddress))
}
