package services

import (
	"context"

	repositoryBase "github.com/moto-nrw/project-phoenix/database/repositories/base"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type settingsRuntime struct {
	db   *bun.DB
	unit *tenant.UnitOfWork
}

func newSettingsRuntime(db *bun.DB, unit *tenant.UnitOfWork) settingsRuntime {
	return settingsRuntime{db: db, unit: unit}
}

func (r settingsRuntime) TenantID(ctx context.Context) int64 { return tenant.FromContext(ctx) }

func (r settingsRuntime) HasTransaction(ctx context.Context) bool {
	return repositoryBase.NewConfigRuntime(r.db).HasTransaction(ctx)
}

func (r settingsRuntime) WithinTenant(ctx context.Context, tenantID int64, fn func(context.Context) error) error {
	if r.unit != nil {
		ctx = tenant.WithUnitOfWork(ctx, *r.unit)
	}
	return tenant.WithTenantTx(ctx, r.db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		return fn(txCtx)
	})
}

func (r settingsRuntime) WithinAdmin(ctx context.Context, fn func(context.Context) error) error {
	if r.unit != nil {
		ctx = tenant.WithUnitOfWork(ctx, *r.unit)
	}
	return tenant.WithAdminTx(ctx, r.db, func(txCtx context.Context, _ bun.Tx) error {
		return fn(txCtx)
	})
}

func (r settingsRuntime) AcquireLock(ctx context.Context, key string, shared bool) error {
	if shared {
		return repositoryBase.AcquireXactLockShared(ctx, r.db, key)
	}
	return repositoryBase.AcquireXactLock(ctx, r.db, key)
}

func (r settingsRuntime) AfterCommit(ctx context.Context, fn func()) {
	tenant.RegisterAfterCommit(ctx, fn)
}

type schoolSettingsStore struct {
	repo platformModels.SchoolRepository
}

func newSchoolSettingsStore(repo platformModels.SchoolRepository) configService.SchoolSettingsStore {
	if repo == nil {
		return nil
	}
	return schoolSettingsStore{repo: repo}
}

func (s schoolSettingsStore) FindSettings(ctx context.Context, schoolID int64) (string, error) {
	school, err := s.repo.FindByID(ctx, schoolID)
	if err != nil {
		return "", err
	}
	return school.Settings, nil
}

func (s schoolSettingsStore) UpdateSettings(ctx context.Context, schoolID int64, update func(string) (string, error)) error {
	school, err := s.repo.FindByIDForUpdate(ctx, schoolID)
	if err != nil {
		return err
	}
	settings, err := update(school.Settings)
	if err != nil {
		return err
	}
	school.Settings = settings
	return s.repo.Update(ctx, school)
}
