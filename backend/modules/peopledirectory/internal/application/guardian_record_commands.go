package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

// Guardian record commands own the units of work the retained row adapters
// used to open. A workflow's ambient transaction is joined, never replaced.
// The adapters retain their savepoints, locks and owner commands.
type guardianProfileCommands struct {
	ports.GuardianProfileRepository
	transactions ports.TenantTxRunner
}

func NewGuardianProfileCommands(store ports.GuardianProfileRepository, transactions ports.TenantTxRunner) ports.GuardianProfileRepository {
	return guardianProfileCommands{store, transactions}
}

func (c guardianProfileCommands) LinkAccount(ctx context.Context, profileID, accountID int64) error {
	return c.transactions.InTenantTx(ctx, func(txCtx context.Context) error {
		return c.GuardianProfileRepository.LinkAccount(txCtx, profileID, accountID)
	})
}

type guardianPhoneCommands struct {
	ports.GuardianPhoneNumberRepository
	transactions ports.TenantTxRunner
}

func NewGuardianPhoneCommands(store ports.GuardianPhoneNumberRepository, transactions ports.TenantTxRunner) ports.GuardianPhoneNumberRepository {
	return guardianPhoneCommands{store, transactions}
}

func (c guardianPhoneCommands) SetPrimary(ctx context.Context, id, profileID int64) error {
	return c.transactions.InTenantTx(ctx, func(txCtx context.Context) error {
		return c.GuardianPhoneNumberRepository.SetPrimary(txCtx, id, profileID)
	})
}
