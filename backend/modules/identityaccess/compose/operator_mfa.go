package compose

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// Engine methods for the operator MFA records (#2723). They need no
// dependencies beyond the database, so every composition serves them.

func (e engine) FindOperatorMFACredential(ctx context.Context, operatorID int64) (identityaccess.OperatorMFACredential, error) {
	value, err := e.mfa.FindCredential(ctx, operatorID)
	return identityaccess.OperatorMFACredential(value), mapError(err)
}

func (e engine) FindActiveOperatorMFAChallenge(ctx context.Context, operatorID int64) (identityaccess.OperatorMFAChallenge, error) {
	value, err := e.mfa.FindActiveChallenge(ctx, operatorID)
	return identityaccess.OperatorMFAChallenge(value), mapError(err)
}

func (e engine) CountOperatorMFAChallengesSince(ctx context.Context, operatorID int64, since time.Time) (int, error) {
	count, err := e.mfa.CountChallengesSince(ctx, operatorID, since)
	return count, mapError(err)
}

func (e engine) FindActiveOperatorTrustedDevice(ctx context.Context, operatorID int64, tokenHash string) (identityaccess.OperatorTrustedDevice, error) {
	value, err := e.mfa.FindActiveTrustedDevice(ctx, operatorID, tokenHash)
	return identityaccess.OperatorTrustedDevice(value), mapError(err)
}

func (e engine) ListActiveOperatorTrustedDevices(ctx context.Context, operatorID int64) ([]identityaccess.OperatorTrustedDevice, error) {
	values, err := e.mfa.ListActiveTrustedDevices(ctx, operatorID)
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]identityaccess.OperatorTrustedDevice, 0, len(values))
	for _, value := range values {
		result = append(result, identityaccess.OperatorTrustedDevice(value))
	}
	return result, nil
}

func (e engine) CreateOperatorMFACredential(ctx context.Context, credential identityaccess.OperatorMFACredential) (identityaccess.OperatorMFACredential, error) {
	value, err := e.mfa.CreateCredential(ctx, domain.OperatorMFACredential(credential))
	return identityaccess.OperatorMFACredential(value), mapError(err)
}

func (e engine) TouchOperatorMFACredential(ctx context.Context, id int64, usedAt time.Time) error {
	return mapError(e.mfa.TouchCredential(ctx, id, usedAt))
}

func (e engine) DeleteOperatorMFACredentials(ctx context.Context, operatorID int64) error {
	return mapError(e.mfa.DeleteCredentials(ctx, operatorID))
}

func (e engine) CreateOperatorMFAChallenge(ctx context.Context, challenge identityaccess.OperatorMFAChallenge) (identityaccess.OperatorMFAChallenge, error) {
	value, err := e.mfa.CreateChallenge(ctx, domain.OperatorMFAChallenge(challenge))
	return identityaccess.OperatorMFAChallenge(value), mapError(err)
}

func (e engine) ActivateOperatorMFAChallenge(ctx context.Context, id int64) error {
	return mapError(e.mfa.ActivateChallenge(ctx, id))
}

func (e engine) ConsumeOperatorMFAChallenge(ctx context.Context, id int64, consumedAt time.Time) error {
	return mapError(e.mfa.ConsumeChallenge(ctx, id, consumedAt))
}

func (e engine) CreateOperatorTrustedDevice(ctx context.Context, device identityaccess.OperatorTrustedDevice) (identityaccess.OperatorTrustedDevice, error) {
	value, err := e.mfa.CreateTrustedDevice(ctx, domain.OperatorTrustedDevice(device))
	return identityaccess.OperatorTrustedDevice(value), mapError(err)
}

func (e engine) TouchOperatorTrustedDevice(ctx context.Context, id int64, usedAt time.Time) error {
	return mapError(e.mfa.TouchTrustedDevice(ctx, id, usedAt))
}

func (e engine) RevokeOperatorTrustedDevice(ctx context.Context, id int64, revokedAt time.Time) error {
	return mapError(e.mfa.RevokeTrustedDevice(ctx, id, revokedAt))
}

func (e engine) RevokeOperatorTrustedDevices(ctx context.Context, operatorID int64, revokedAt time.Time) error {
	return mapError(e.mfa.RevokeTrustedDevices(ctx, operatorID, revokedAt))
}
