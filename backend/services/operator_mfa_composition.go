package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/services/platform"
)

// Identity & Access owns the operator MFA enrollment, e-mail challenge and
// trusted-device rows (#2723). operatorMFARecords serves the retained
// operator MFA service's consumer-owned port over the public module: a
// missing row is (nil, nil), a record that fails validation or a state
// change that did not apply is an error, and the stored identity and
// timestamps are written back into the caller's value.
type operatorMFARecords struct {
	records identityaccess.OperatorMFARecords
}

func newOperatorMFARecords(records identityaccess.OperatorMFARecords) platform.OperatorMFARecords {
	return operatorMFARecords{records: records}
}

func (r operatorMFARecords) FindCredential(ctx context.Context, operatorID int64) (*platformModels.OperatorMFACredential, error) {
	credential, err := r.records.FindOperatorMFACredential(ctx, operatorID)
	if errors.Is(err, identityaccess.ErrOperatorMFACredentialNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, operatorDatabaseError("find operator mfa credential", err)
	}
	return operatorMFACredentialModel(credential), nil
}

func (r operatorMFARecords) CreateCredential(ctx context.Context, credential *platformModels.OperatorMFACredential) error {
	if credential == nil {
		return fmt.Errorf("operator mfa credential cannot be nil")
	}
	stored, err := r.records.CreateOperatorMFACredential(ctx, identityaccess.OperatorMFACredential{
		OperatorID: credential.OperatorID, Method: credential.Method, EnrolledAt: credential.EnrolledAt, LastUsedAt: credential.LastUsedAt,
	})
	if err != nil {
		return operatorDatabaseError("create operator mfa credential", err)
	}
	*credential = *operatorMFACredentialModel(stored)
	return nil
}

func (r operatorMFARecords) TouchCredential(ctx context.Context, id int64, usedAt time.Time) error {
	if err := r.records.TouchOperatorMFACredential(ctx, id, usedAt); err != nil {
		return operatorDatabaseError("touch operator mfa credential", err)
	}
	return nil
}

func (r operatorMFARecords) DeleteCredentials(ctx context.Context, operatorID int64) error {
	if err := r.records.DeleteOperatorMFACredentials(ctx, operatorID); err != nil {
		return operatorDatabaseError("delete operator mfa credentials", err)
	}
	return nil
}

func (r operatorMFARecords) CreateChallenge(ctx context.Context, challenge *platformModels.OperatorMFAEmailChallenge) error {
	if challenge == nil {
		return fmt.Errorf("operator mfa email challenge cannot be nil")
	}
	stored, err := r.records.CreateOperatorMFAChallenge(ctx, identityaccess.OperatorMFAChallenge{
		OperatorID: challenge.OperatorID, CodeHash: challenge.CodeHash, ExpiresAt: challenge.ExpiresAt,
		ConsumedAt: challenge.ConsumedAt, IPAddress: challenge.IPAddress,
	})
	if err != nil {
		return operatorDatabaseError("create operator mfa email challenge", err)
	}
	*challenge = *operatorMFAChallengeModel(stored)
	return nil
}

func (r operatorMFARecords) FindActiveChallenge(ctx context.Context, operatorID int64) (*platformModels.OperatorMFAEmailChallenge, error) {
	challenge, err := r.records.FindActiveOperatorMFAChallenge(ctx, operatorID)
	if errors.Is(err, identityaccess.ErrOperatorMFAChallengeNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, operatorDatabaseError("find active operator mfa email challenge", err)
	}
	return operatorMFAChallengeModel(challenge), nil
}

func (r operatorMFARecords) CountChallengesSince(ctx context.Context, operatorID int64, since time.Time) (int, error) {
	count, err := r.records.CountOperatorMFAChallengesSince(ctx, operatorID, since)
	if err != nil {
		return 0, operatorDatabaseError("count recent operator mfa email challenges", err)
	}
	return count, nil
}

func (r operatorMFARecords) ActivateChallenge(ctx context.Context, id int64) error {
	if err := r.records.ActivateOperatorMFAChallenge(ctx, id); err != nil {
		return operatorDatabaseError("mark operator mfa email challenge active", err)
	}
	return nil
}

func (r operatorMFARecords) ConsumeChallenge(ctx context.Context, id int64, consumedAt time.Time) error {
	if err := r.records.ConsumeOperatorMFAChallenge(ctx, id, consumedAt); err != nil {
		return operatorDatabaseError("mark operator mfa email challenge consumed", err)
	}
	return nil
}

func (r operatorMFARecords) CreateTrustedDevice(ctx context.Context, device *platformModels.OperatorMFATrustedDevice) error {
	if device == nil {
		return fmt.Errorf("operator mfa trusted device cannot be nil")
	}
	stored, err := r.records.CreateOperatorTrustedDevice(ctx, identityaccess.OperatorTrustedDevice{
		OperatorID: device.OperatorID, TokenHash: device.TokenHash, UserAgent: device.UserAgent, IPAddress: device.IPAddress,
		ExpiresAt: device.ExpiresAt, LastUsedAt: device.LastUsedAt, RevokedAt: device.RevokedAt,
	})
	if err != nil {
		return operatorDatabaseError("create operator mfa trusted device", err)
	}
	*device = *operatorTrustedDeviceModel(stored)
	return nil
}

func (r operatorMFARecords) FindActiveTrustedDevice(ctx context.Context, operatorID int64, tokenHash string) (*platformModels.OperatorMFATrustedDevice, error) {
	device, err := r.records.FindActiveOperatorTrustedDevice(ctx, operatorID, tokenHash)
	if errors.Is(err, identityaccess.ErrOperatorTrustedDeviceNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, operatorDatabaseError("find active operator mfa trusted device", err)
	}
	return operatorTrustedDeviceModel(device), nil
}

func (r operatorMFARecords) ListActiveTrustedDevices(ctx context.Context, operatorID int64) ([]*platformModels.OperatorMFATrustedDevice, error) {
	devices, err := r.records.ListActiveOperatorTrustedDevices(ctx, operatorID)
	if err != nil {
		return nil, operatorDatabaseError("list active operator mfa trusted devices", err)
	}
	result := make([]*platformModels.OperatorMFATrustedDevice, 0, len(devices))
	for _, device := range devices {
		result = append(result, operatorTrustedDeviceModel(device))
	}
	return result, nil
}

func (r operatorMFARecords) TouchTrustedDevice(ctx context.Context, id int64, usedAt time.Time) error {
	if err := r.records.TouchOperatorTrustedDevice(ctx, id, usedAt); err != nil {
		return operatorDatabaseError("touch operator mfa trusted device", err)
	}
	return nil
}

func (r operatorMFARecords) RevokeTrustedDevice(ctx context.Context, id int64, revokedAt time.Time) error {
	if err := r.records.RevokeOperatorTrustedDevice(ctx, id, revokedAt); err != nil {
		return operatorDatabaseError("revoke operator mfa trusted device", err)
	}
	return nil
}

func (r operatorMFARecords) RevokeAllTrustedDevices(ctx context.Context, operatorID int64, revokedAt time.Time) error {
	if err := r.records.RevokeOperatorTrustedDevices(ctx, operatorID, revokedAt); err != nil {
		return operatorDatabaseError("revoke all operator mfa trusted devices", err)
	}
	return nil
}

func operatorMFACredentialModel(src identityaccess.OperatorMFACredential) *platformModels.OperatorMFACredential {
	credential := &platformModels.OperatorMFACredential{
		OperatorID: src.OperatorID, Method: src.Method, EnrolledAt: src.EnrolledAt, LastUsedAt: src.LastUsedAt,
	}
	credential.ID, credential.CreatedAt, credential.UpdatedAt = src.ID, src.CreatedAt, src.UpdatedAt
	return credential
}

func operatorMFAChallengeModel(src identityaccess.OperatorMFAChallenge) *platformModels.OperatorMFAEmailChallenge {
	challenge := &platformModels.OperatorMFAEmailChallenge{
		OperatorID: src.OperatorID, CodeHash: src.CodeHash, ExpiresAt: src.ExpiresAt, ConsumedAt: src.ConsumedAt, IPAddress: src.IPAddress,
	}
	challenge.ID, challenge.CreatedAt, challenge.UpdatedAt = src.ID, src.CreatedAt, src.UpdatedAt
	return challenge
}

func operatorTrustedDeviceModel(src identityaccess.OperatorTrustedDevice) *platformModels.OperatorMFATrustedDevice {
	device := &platformModels.OperatorMFATrustedDevice{
		OperatorID: src.OperatorID, TokenHash: src.TokenHash, UserAgent: src.UserAgent, IPAddress: src.IPAddress,
		ExpiresAt: src.ExpiresAt, LastUsedAt: src.LastUsedAt, RevokedAt: src.RevokedAt,
	}
	device.ID, device.CreatedAt, device.UpdatedAt = src.ID, src.CreatedAt, src.UpdatedAt
	return device
}
