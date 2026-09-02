package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
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
	_, ok := tenant.TransactionFromContext(ctx)
	return ok
}

func (r settingsRuntime) DB(ctx context.Context) bun.IDB {
	if raw, ok := tenant.TransactionFromContext(ctx); ok {
		if tx, valid := raw.(bun.Tx); valid {
			return tx
		}
	}
	return r.db
}

func (r settingsRuntime) LockStaffBalance(ctx context.Context, staffID int64) error {
	if staffID <= 0 {
		return errors.New("staff id is required")
	}
	tenantID := r.TenantID(ctx)
	if tenantID <= 0 {
		return errors.New("tenant id is required")
	}
	if err := r.AcquireLock(ctx, fmt.Sprintf("staff-balance:%d:%d", tenantID, staffID), false); err != nil {
		return fmt.Errorf("lock staff balance writes: %w", err)
	}
	return nil
}

func (settingsRuntime) TodayTime() time.Time { return timezone.TodayDate().UTCMidnight() }

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
	if r.unit != nil {
		ctx = tenant.WithUnitOfWork(ctx, *r.unit)
	}
	return tenant.AcquireLock(ctx, key, shared)
}

func (r settingsRuntime) AfterCommit(ctx context.Context, fn func()) {
	tenant.RegisterAfterCommit(ctx, fn)
}

type schoolSettingsStore struct {
	schools organizationtenancy.Capability
}

func newSchoolSettingsStore(schools organizationtenancy.Capability) configService.SchoolSettingsStore {
	if schools == nil {
		return nil
	}
	return schoolSettingsStore{schools: schools}
}

func (s schoolSettingsStore) FindSettings(ctx context.Context, schoolID int64) (string, error) {
	school, err := s.schools.FindSchool(ctx, schoolID)
	if err != nil {
		return "", err
	}
	return school.Settings, nil
}

func (s schoolSettingsStore) UpdateSettings(ctx context.Context, schoolID int64, update func(string) (string, error)) error {
	school, err := s.schools.FindSchoolForMutation(ctx, schoolID)
	if err != nil {
		return err
	}
	settings, err := update(school.Settings)
	if err != nil {
		return err
	}
	_, err = s.schools.UpdateSchool(ctx, organizationtenancy.UpdateSchool{
		ID: school.ID, OrganizationID: school.OrganizationID, Name: school.Name, Slug: school.Slug,
		Subdomain: school.Subdomain, Active: school.Active, Hidden: school.Hidden, Settings: settings,
		Address: school.Address, City: school.City, Zip: school.Zip, Phone: school.Phone,
		Email: school.Email, DevicePinHash: school.DevicePinHash,
	})
	return err
}
