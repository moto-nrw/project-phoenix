package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

// These queries bootstrap authentication against global account records before
// a school/session exists. They are explicit global identity boundaries, not
// a fallback for school-scoped reads. Locks must belong to an outer command.
func findLoginAccount(ctx context.Context, runtime ports.Runtime, store ports.AccountLoginStore, id int64, forUpdate bool) (account domain.LoginAccount, found bool, stats domain.OperationStats, err error) {
	if forUpdate && !runtime.HasTransaction(ctx) {
		return account, false, stats, errors.New("identity access: transaction is required for an account lock")
	}
	err = globalIdentityRead(ctx, runtime, func(txCtx context.Context) error {
		account, found, stats, err = store.FindLoginAccount(txCtx, id, forUpdate)
		return err
	})
	return account, found, stats, err
}

func findLoginAccountByEmail(ctx context.Context, runtime ports.Runtime, store ports.AccountLoginStore, email string) (account domain.LoginAccount, found bool, stats domain.OperationStats, err error) {
	err = globalIdentityRead(ctx, runtime, func(txCtx context.Context) error {
		account, found, stats, err = store.FindLoginAccountByEmail(txCtx, email)
		return err
	})
	return account, found, stats, err
}

func globalIdentityRead(ctx context.Context, runtime ports.Runtime, read func(context.Context) error) error {
	if runtime.HasTransaction(ctx) {
		return read(ctx)
	}
	return runtime.WithAdminTx(ctx, read)
}
