package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

// OperatorMFA serves the operator MFA enrollment, e-mail challenge and
// trusted-device records (#2723). The rows are platform-wide, so every
// operation runs through RunPlatform: it joins the administrative
// transaction a caller opened for a multi-table write (the disable cascade)
// and otherwise executes on the root connection. The clock lives here, not
// in the store.
type OperatorMFA struct {
	service *Service
	store   ports.OperatorMFAStore
	now     func() time.Time
}

func NewOperatorMFA(service *Service, store ports.OperatorMFAStore) *OperatorMFA {
	if service == nil || store == nil {
		panic("identity access application: operator mfa requires the service and its store")
	}
	return &OperatorMFA{service: service, store: store, now: time.Now}
}

func (m *OperatorMFA) run(ctx context.Context, operation string, fn func(context.Context, *domain.OperationStats) error) error {
	return m.service.run(ctx, m.service.tx.RunPlatform, operation, fn)
}

func (m *OperatorMFA) FindCredential(ctx context.Context, operatorID int64) (result domain.OperatorMFACredential, err error) {
	err = m.run(ctx, "find_operator_mfa_credential", func(txCtx context.Context, stats *domain.OperationStats) error {
		if operatorID <= 0 {
			return domain.ErrOperatorMFACredentialNotFound
		}
		credential, found, queryStats, findErr := m.store.FindOperatorMFACredential(txCtx, operatorID)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return domain.ErrOperatorMFACredentialNotFound
		}
		result = credential
		return nil
	})
	return result, err
}

func (m *OperatorMFA) CreateCredential(ctx context.Context, credential domain.OperatorMFACredential) (result domain.OperatorMFACredential, err error) {
	err = m.run(ctx, "create_operator_mfa_credential", func(txCtx context.Context, stats *domain.OperationStats) error {
		if validateErr := credential.Validate(); validateErr != nil {
			return validateErr
		}
		stored, queryStats, insertErr := m.store.InsertOperatorMFACredential(txCtx, credential)
		stats.Add(queryStats)
		if insertErr != nil {
			return insertErr
		}
		result = stored
		return nil
	})
	return result, err
}

func (m *OperatorMFA) TouchCredential(ctx context.Context, id int64, usedAt time.Time) error {
	return m.run(ctx, "touch_operator_mfa_credential", func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := m.store.TouchOperatorMFACredential(txCtx, id, usedAt)
		stats.Add(queryStats)
		return err
	})
}

func (m *OperatorMFA) DeleteCredentials(ctx context.Context, operatorID int64) error {
	return m.run(ctx, "delete_operator_mfa_credentials", func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := m.store.DeleteOperatorMFACredentials(txCtx, operatorID)
		stats.Add(queryStats)
		return err
	})
}

func (m *OperatorMFA) CreateChallenge(ctx context.Context, challenge domain.OperatorMFAChallenge) (result domain.OperatorMFAChallenge, err error) {
	err = m.run(ctx, "create_operator_mfa_challenge", func(txCtx context.Context, stats *domain.OperationStats) error {
		if validateErr := challenge.Validate(); validateErr != nil {
			return validateErr
		}
		stored, queryStats, insertErr := m.store.InsertOperatorMFAChallenge(txCtx, challenge)
		stats.Add(queryStats)
		if insertErr != nil {
			return insertErr
		}
		result = stored
		return nil
	})
	return result, err
}

func (m *OperatorMFA) FindActiveChallenge(ctx context.Context, operatorID int64) (result domain.OperatorMFAChallenge, err error) {
	err = m.run(ctx, "find_active_operator_mfa_challenge", func(txCtx context.Context, stats *domain.OperationStats) error {
		challenge, found, queryStats, findErr := m.store.FindActiveOperatorMFAChallenge(txCtx, operatorID, m.now())
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return domain.ErrOperatorMFAChallengeNotFound
		}
		result = challenge
		return nil
	})
	return result, err
}

func (m *OperatorMFA) CountChallengesSince(ctx context.Context, operatorID int64, since time.Time) (result int, err error) {
	err = m.run(ctx, "count_operator_mfa_challenges", func(txCtx context.Context, stats *domain.OperationStats) error {
		count, queryStats, countErr := m.store.CountOperatorMFAChallengesSince(txCtx, operatorID, since)
		stats.Add(queryStats)
		result = count
		return countErr
	})
	return result, err
}

// ActivateChallenge makes a delivered code redeemable exactly once; a row
// that is no longer pending is reported, never silently accepted.
func (m *OperatorMFA) ActivateChallenge(ctx context.Context, id int64) error {
	return m.run(ctx, "activate_operator_mfa_challenge", func(txCtx context.Context, stats *domain.OperationStats) error {
		changed, queryStats, err := m.store.ActivateOperatorMFAChallenge(txCtx, id)
		stats.Add(queryStats)
		return stateChange(changed, err, domain.ErrOperatorMFAChallengeStateChanged)
	})
}

// ConsumeChallenge redeems a code exactly once: the loser of two concurrent
// verifications receives ErrOperatorMFAChallengeStateChanged and must not
// mint a session.
func (m *OperatorMFA) ConsumeChallenge(ctx context.Context, id int64, consumedAt time.Time) error {
	return m.run(ctx, "consume_operator_mfa_challenge", func(txCtx context.Context, stats *domain.OperationStats) error {
		changed, queryStats, err := m.store.ConsumeOperatorMFAChallenge(txCtx, id, consumedAt)
		stats.Add(queryStats)
		return stateChange(changed, err, domain.ErrOperatorMFAChallengeStateChanged)
	})
}

func (m *OperatorMFA) CreateTrustedDevice(ctx context.Context, device domain.OperatorTrustedDevice) (result domain.OperatorTrustedDevice, err error) {
	err = m.run(ctx, "create_operator_trusted_device", func(txCtx context.Context, stats *domain.OperationStats) error {
		if validateErr := device.Validate(); validateErr != nil {
			return validateErr
		}
		stored, queryStats, insertErr := m.store.InsertOperatorTrustedDevice(txCtx, device)
		stats.Add(queryStats)
		if insertErr != nil {
			return insertErr
		}
		result = stored
		return nil
	})
	return result, err
}

func (m *OperatorMFA) FindActiveTrustedDevice(ctx context.Context, operatorID int64, tokenHash string) (result domain.OperatorTrustedDevice, err error) {
	err = m.run(ctx, "find_active_operator_trusted_device", func(txCtx context.Context, stats *domain.OperationStats) error {
		if tokenHash == "" {
			return domain.ErrOperatorTrustedDeviceNotFound
		}
		device, found, queryStats, findErr := m.store.FindActiveOperatorTrustedDevice(txCtx, operatorID, tokenHash, m.now())
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return domain.ErrOperatorTrustedDeviceNotFound
		}
		result = device
		return nil
	})
	return result, err
}

func (m *OperatorMFA) ListActiveTrustedDevices(ctx context.Context, operatorID int64) (result []domain.OperatorTrustedDevice, err error) {
	err = m.run(ctx, "list_active_operator_trusted_devices", func(txCtx context.Context, stats *domain.OperationStats) error {
		devices, queryStats, listErr := m.store.ListActiveOperatorTrustedDevices(txCtx, operatorID, m.now())
		stats.Add(queryStats)
		if listErr != nil {
			return listErr
		}
		result = devices
		return nil
	})
	return result, err
}

func (m *OperatorMFA) TouchTrustedDevice(ctx context.Context, id int64, usedAt time.Time) error {
	return m.run(ctx, "touch_operator_trusted_device", func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := m.store.TouchOperatorTrustedDevice(txCtx, id, usedAt)
		stats.Add(queryStats)
		return err
	})
}

func (m *OperatorMFA) RevokeTrustedDevice(ctx context.Context, id int64, revokedAt time.Time) error {
	return m.run(ctx, "revoke_operator_trusted_device", func(txCtx context.Context, stats *domain.OperationStats) error {
		changed, queryStats, err := m.store.RevokeOperatorTrustedDevice(txCtx, id, revokedAt)
		stats.Add(queryStats)
		return stateChange(changed, err, domain.ErrOperatorTrustedDeviceNotFound)
	})
}

func (m *OperatorMFA) RevokeTrustedDevices(ctx context.Context, operatorID int64, revokedAt time.Time) error {
	return m.run(ctx, "revoke_operator_trusted_devices", func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := m.store.RevokeOperatorTrustedDevices(txCtx, operatorID, revokedAt)
		stats.Add(queryStats)
		return err
	})
}

func stateChange(changed bool, err error, unchanged error) error {
	if err != nil {
		return err
	}
	if !changed {
		return unchanged
	}
	return nil
}
