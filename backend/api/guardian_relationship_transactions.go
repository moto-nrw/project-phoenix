package api

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// WithGuardianRelationshipTransactions binds the outer unit of work for the
// three owner halves of a guardian link. The root binds this boundary rather
// than letting any row adapter open or commit the shared transaction.
type guardianRelationshipCommands struct {
	ports.StudentGuardianRepository
}

func WithGuardianRelationshipTransactions(store ports.StudentGuardianRepository) ports.StudentGuardianRepository {
	return guardianRelationshipCommands{store}
}

func (c guardianRelationshipCommands) Create(ctx context.Context, relationship *peopledirectory.GuardianRelationshipSnapshot) error {
	return tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error {
		return c.StudentGuardianRepository.Create(txCtx, relationship)
	})
}

func (c guardianRelationshipCommands) Update(ctx context.Context, relationship *peopledirectory.GuardianRelationshipSnapshot) error {
	return tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error {
		return c.StudentGuardianRepository.Update(txCtx, relationship)
	})
}

func (c guardianRelationshipCommands) LinkIfNotExists(ctx context.Context, relationship *peopledirectory.GuardianRelationshipSnapshot) (linked bool, err error) {
	err = tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error {
		linked, err = c.StudentGuardianRepository.LinkIfNotExists(txCtx, relationship)
		return err
	})
	return linked, err
}

func (c guardianRelationshipCommands) UpdateColumns(ctx context.Context, relationship *peopledirectory.GuardianRelationshipSnapshot, columns ...string) (changed int64, err error) {
	err = tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error {
		changed, err = c.StudentGuardianRepository.UpdateColumns(txCtx, relationship, columns...)
		return err
	})
	return changed, err
}

func (c guardianRelationshipCommands) SetPrimary(ctx context.Context, id int64, primary bool) error {
	return tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error {
		return c.StudentGuardianRepository.SetPrimary(txCtx, id, primary)
	})
}
