package identityaccess

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"
)

// Operator MFA records (#2723): the operator's MFA enrollment, the hashed
// e-mail codes of its login challenges and the server-side records of its
// remember-device cookies. The rows are platform-wide like the operator
// itself. Every operation joins the caller's transaction when one is active,
// so a multi-table write such as disabling MFA commits as one unit of work.
var (
	// ErrOperatorMFACredentialNotFound reports an operator without MFA
	// enrollment.
	ErrOperatorMFACredentialNotFound = errors.New("operator mfa credential not found")
	// ErrOperatorMFAChallengeNotFound reports an operator without an
	// unconsumed, unexpired e-mail code.
	ErrOperatorMFAChallengeNotFound = errors.New("operator mfa challenge not found")
	// ErrOperatorMFAChallengeStateChanged reports an activation or
	// consumption that found the code no longer in the expected state. The
	// loser of two concurrent verifications receives it.
	ErrOperatorMFAChallengeStateChanged = errors.New("operator mfa challenge was already consumed or activated")
	// ErrOperatorTrustedDeviceNotFound reports a lookup without an active
	// trusted device, or a revocation of a device already revoked or gone.
	ErrOperatorTrustedDeviceNotFound = errors.New("operator trusted device not found")
)

// OperatorMFACredential records that an operator enrolled in e-mail MFA.
type OperatorMFACredential struct {
	ID         int64
	OperatorID int64
	Method     string
	EnrolledAt time.Time
	LastUsedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// OperatorMFAChallenge is one hashed e-mail code. ConsumedAt is set while
// the code is not redeemable: before its delivery was confirmed and after
// it was used.
type OperatorMFAChallenge struct {
	ID         int64
	OperatorID int64
	CodeHash   string
	ExpiresAt  time.Time
	ConsumedAt *time.Time
	IPAddress  net.IP
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// OperatorTrustedDevice is the server-side record of a remember-device
// cookie; TokenHash is the hash of the cookie's raw token.
type OperatorTrustedDevice struct {
	ID         int64
	OperatorID int64
	TokenHash  string
	UserAgent  *string
	IPAddress  net.IP
	ExpiresAt  time.Time
	LastUsedAt *time.Time
	RevokedAt  *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// OperatorMFAQuery resolves operator MFA records. "Active" means unconsumed
// (codes) or unrevoked (devices) and not yet expired.
type OperatorMFAQuery interface {
	FindOperatorMFACredential(ctx context.Context, operatorID int64) (OperatorMFACredential, error)
	// FindActiveOperatorMFAChallenge returns the active code with the latest
	// expiry.
	FindActiveOperatorMFAChallenge(ctx context.Context, operatorID int64) (OperatorMFAChallenge, error)
	// CountOperatorMFAChallengesSince counts every code created at or after
	// since, redeemed or not; the send rate limit reads it.
	CountOperatorMFAChallengesSince(ctx context.Context, operatorID int64, since time.Time) (int, error)
	FindActiveOperatorTrustedDevice(ctx context.Context, operatorID int64, tokenHash string) (OperatorTrustedDevice, error)
	// ListActiveOperatorTrustedDevices orders by last use, most recent first,
	// then by creation.
	ListActiveOperatorTrustedDevices(ctx context.Context, operatorID int64) ([]OperatorTrustedDevice, error)
}

// OperatorMFACommand changes operator MFA records. The creates validate the
// record and return it with its identity and timestamps.
type OperatorMFACommand interface {
	CreateOperatorMFACredential(ctx context.Context, credential OperatorMFACredential) (OperatorMFACredential, error)
	TouchOperatorMFACredential(ctx context.Context, id int64, usedAt time.Time) error
	DeleteOperatorMFACredentials(ctx context.Context, operatorID int64) error
	CreateOperatorMFAChallenge(ctx context.Context, challenge OperatorMFAChallenge) (OperatorMFAChallenge, error)
	// ActivateOperatorMFAChallenge makes a delivered code redeemable.
	ActivateOperatorMFAChallenge(ctx context.Context, id int64) error
	// ConsumeOperatorMFAChallenge redeems a code exactly once.
	ConsumeOperatorMFAChallenge(ctx context.Context, id int64, consumedAt time.Time) error
	CreateOperatorTrustedDevice(ctx context.Context, device OperatorTrustedDevice) (OperatorTrustedDevice, error)
	TouchOperatorTrustedDevice(ctx context.Context, id int64, usedAt time.Time) error
	RevokeOperatorTrustedDevice(ctx context.Context, id int64, revokedAt time.Time) error
	RevokeOperatorTrustedDevices(ctx context.Context, operatorID int64, revokedAt time.Time) error
}

// OperatorMFARecords is the capability the operator MFA flows consume.
type OperatorMFARecords interface {
	OperatorMFAQuery
	OperatorMFACommand
}

func (m *Module) FindOperatorMFACredential(ctx context.Context, operatorID int64) (OperatorMFACredential, error) {
	credential, err := m.engine.FindOperatorMFACredential(ctx, operatorID)
	if err != nil {
		return OperatorMFACredential{}, fmt.Errorf("identity access: find operator mfa credential: %w", err)
	}
	return credential, nil
}

func (m *Module) FindActiveOperatorMFAChallenge(ctx context.Context, operatorID int64) (OperatorMFAChallenge, error) {
	challenge, err := m.engine.FindActiveOperatorMFAChallenge(ctx, operatorID)
	if err != nil {
		return OperatorMFAChallenge{}, fmt.Errorf("identity access: find active operator mfa challenge: %w", err)
	}
	return challenge, nil
}

func (m *Module) CountOperatorMFAChallengesSince(ctx context.Context, operatorID int64, since time.Time) (int, error) {
	count, err := m.engine.CountOperatorMFAChallengesSince(ctx, operatorID, since)
	if err != nil {
		return 0, fmt.Errorf("identity access: count operator mfa challenges: %w", err)
	}
	return count, nil
}

func (m *Module) FindActiveOperatorTrustedDevice(ctx context.Context, operatorID int64, tokenHash string) (OperatorTrustedDevice, error) {
	device, err := m.engine.FindActiveOperatorTrustedDevice(ctx, operatorID, tokenHash)
	if err != nil {
		return OperatorTrustedDevice{}, fmt.Errorf("identity access: find active operator trusted device: %w", err)
	}
	return device, nil
}

func (m *Module) ListActiveOperatorTrustedDevices(ctx context.Context, operatorID int64) ([]OperatorTrustedDevice, error) {
	devices, err := m.engine.ListActiveOperatorTrustedDevices(ctx, operatorID)
	if err != nil {
		return nil, fmt.Errorf("identity access: list active operator trusted devices: %w", err)
	}
	return devices, nil
}

func (m *Module) CreateOperatorMFACredential(ctx context.Context, credential OperatorMFACredential) (OperatorMFACredential, error) {
	stored, err := m.engine.CreateOperatorMFACredential(ctx, credential)
	if err != nil {
		return OperatorMFACredential{}, fmt.Errorf("identity access: create operator mfa credential: %w", err)
	}
	return stored, nil
}

func (m *Module) TouchOperatorMFACredential(ctx context.Context, id int64, usedAt time.Time) error {
	if err := m.engine.TouchOperatorMFACredential(ctx, id, usedAt); err != nil {
		return fmt.Errorf("identity access: touch operator mfa credential: %w", err)
	}
	return nil
}

func (m *Module) DeleteOperatorMFACredentials(ctx context.Context, operatorID int64) error {
	if err := m.engine.DeleteOperatorMFACredentials(ctx, operatorID); err != nil {
		return fmt.Errorf("identity access: delete operator mfa credentials: %w", err)
	}
	return nil
}

func (m *Module) CreateOperatorMFAChallenge(ctx context.Context, challenge OperatorMFAChallenge) (OperatorMFAChallenge, error) {
	stored, err := m.engine.CreateOperatorMFAChallenge(ctx, challenge)
	if err != nil {
		return OperatorMFAChallenge{}, fmt.Errorf("identity access: create operator mfa challenge: %w", err)
	}
	return stored, nil
}

func (m *Module) ActivateOperatorMFAChallenge(ctx context.Context, id int64) error {
	if err := m.engine.ActivateOperatorMFAChallenge(ctx, id); err != nil {
		return fmt.Errorf("identity access: activate operator mfa challenge: %w", err)
	}
	return nil
}

func (m *Module) ConsumeOperatorMFAChallenge(ctx context.Context, id int64, consumedAt time.Time) error {
	if err := m.engine.ConsumeOperatorMFAChallenge(ctx, id, consumedAt); err != nil {
		return fmt.Errorf("identity access: consume operator mfa challenge: %w", err)
	}
	return nil
}

func (m *Module) CreateOperatorTrustedDevice(ctx context.Context, device OperatorTrustedDevice) (OperatorTrustedDevice, error) {
	stored, err := m.engine.CreateOperatorTrustedDevice(ctx, device)
	if err != nil {
		return OperatorTrustedDevice{}, fmt.Errorf("identity access: create operator trusted device: %w", err)
	}
	return stored, nil
}

func (m *Module) TouchOperatorTrustedDevice(ctx context.Context, id int64, usedAt time.Time) error {
	if err := m.engine.TouchOperatorTrustedDevice(ctx, id, usedAt); err != nil {
		return fmt.Errorf("identity access: touch operator trusted device: %w", err)
	}
	return nil
}

func (m *Module) RevokeOperatorTrustedDevice(ctx context.Context, id int64, revokedAt time.Time) error {
	if err := m.engine.RevokeOperatorTrustedDevice(ctx, id, revokedAt); err != nil {
		return fmt.Errorf("identity access: revoke operator trusted device: %w", err)
	}
	return nil
}

func (m *Module) RevokeOperatorTrustedDevices(ctx context.Context, operatorID int64, revokedAt time.Time) error {
	if err := m.engine.RevokeOperatorTrustedDevices(ctx, operatorID, revokedAt); err != nil {
		return fmt.Errorf("identity access: revoke operator trusted devices: %w", err)
	}
	return nil
}
