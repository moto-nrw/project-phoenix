package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Guardian record commands own the units of work the retained row adapters
// used to open. A workflow's ambient transaction is joined, never replaced.
// The adapters retain their savepoints, locks and owner commands.
type guardianProfileCommands struct {
	ports.GuardianProfileRepository
}

func NewGuardianProfileCommands(store ports.GuardianProfileRepository) ports.GuardianProfileRepository {
	return guardianProfileCommands{store}
}

func (c guardianProfileCommands) LinkAccount(ctx context.Context, profileID, accountID int64) error {
	return tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error {
		return c.GuardianProfileRepository.LinkAccount(txCtx, profileID, accountID)
	})
}

type guardianPhoneCommands struct {
	ports.GuardianPhoneNumberRepository
}

func NewGuardianPhoneCommands(store ports.GuardianPhoneNumberRepository) ports.GuardianPhoneNumberRepository {
	return guardianPhoneCommands{store}
}

func (c guardianPhoneCommands) SetPrimary(ctx context.Context, id, profileID int64) error {
	return tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error {
		return c.GuardianPhoneNumberRepository.SetPrimary(txCtx, id, profileID)
	})
}
