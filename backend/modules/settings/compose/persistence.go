package compose

import (
	"context"

	configRepo "github.com/moto-nrw/project-phoenix/modules/settings/internal/adapters/configpostgres"
	configModel "github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/uptrace/bun"
)

// PersistenceRuntime supplies the ambient transaction the Settings Postgres
// adapter runs in; tenant scoping is left to row-level security.
type PersistenceRuntime interface {
	DB(context.Context) (bun.IDB, error)
}

// NewSettingValueRepository builds the tenant setting value store over the
// caller's settings runtime (ADR 0049).
func NewSettingValueRepository(runtime PersistenceRuntime) configModel.SettingValueRepository {
	return configRepo.NewSettingValueRepository(runtime)
}

// NewSettingAuditRepository builds the setting change trail over the caller's
// settings runtime.
func NewSettingAuditRepository(runtime PersistenceRuntime) configModel.SettingAuditRepository {
	return configRepo.NewSettingAuditRepository(runtime)
}

// NewTraegerSettingRepository builds the Träger level store (ADR 0086).
func NewTraegerSettingRepository(runtime PersistenceRuntime) configModel.TraegerSettingRepository {
	return configRepo.NewTraegerSettingRepository(runtime)
}

// NewHomeLayoutRepository builds the store behind the personal start page
// (#2875). Callers pass the same tenant-aware runtime as the settings
// repositories, so its rows take part in the settings tenant transaction and
// stay behind RLS.
func NewHomeLayoutRepository(runtime PersistenceRuntime) configModel.HomeLayoutRepository {
	return configRepo.NewHomeLayoutRepository(runtime)
}

// NewStoredOverrideReader binds the CLI query to its caller's ambient runtime.
func NewStoredOverrideReader(runtime PersistenceRuntime) configRepo.StoredOverrideReader {
	return configRepo.NewStoredOverrideReader(runtime)
}

// NewSchoolSetupRepository builds the wizard's Settings-owned persistence.
func NewSchoolSetupRepository[S configRepo.SchoolSetupState](runtime PersistenceRuntime) *configRepo.SchoolSetupRepository[S] {
	return configRepo.NewSchoolSetupRepository[S](runtime)
}

// NewStaffOnboardingRepository builds the staff checklist's Settings-owned persistence.
func NewStaffOnboardingRepository[S configRepo.StaffOnboardingState](runtime PersistenceRuntime) *configRepo.StaffOnboardingRepository[S] {
	return configRepo.NewStaffOnboardingRepository[S](runtime)
}
