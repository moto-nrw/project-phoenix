package compose

import (
	configSvc "github.com/moto-nrw/project-phoenix/modules/settings/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/settings/internal/application/sideeffects"
	settingsRecords "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// The retained settings service and its helpers, which the legacy
// composition names through this composition instead of the Settings
// Platform's application packages (#2747).
type (
	SettingsService           = settingsRecords.SettingsService
	BatchSettingsService      = settingsRecords.BatchSettingsService
	SettingsRuntime           = settingsRecords.Runtime
	SchoolSettingsStore       = settingsRecords.SchoolSettingsStore
	SettingsSnapshot          = settingsRecords.SettingsSnapshot
	SettingsLookupObserver    = settingsRecords.SettingsLookupObserver
	SettingsChangedNotifier   = settingsRecords.SettingsChangedNotifier
	OperatorValueSetHook      = settingsRecords.OperatorValueSetHook
	TenantOperations          = configSvc.TenantOperations
	TenantOperationsRuntime   = settingsRecords.TenantOperationsRuntime
	PayrollStatusGetter       = settingsRecords.PayrollStatusGetter
	SideEffectRegistry        = sideeffects.Registry
	SideEffectFailureObserver = settingsRecords.SideEffectFailureObserver
	// SettingsDependencies are every collaborator of the settings service at
	// construction (ADR 0059); SettingSideEffect binds one setting key to its
	// write side effect.
	SettingsDependencies = configSvc.SettingsServiceDependencies
	SettingSideEffect    = sideeffects.Handler
)

var (
	NewSettingsService       = configSvc.NewSettingsService
	NewHomeLayoutService     = configSvc.NewHomeLayoutService
	NewTenantOperations      = configSvc.NewTenantOperations
	NewPayrollStatusService  = configSvc.NewPayrollStatusService
	NewSFTPExportSettings    = configSvc.NewSFTPExportSettings
	NewSettings              = configSvc.NewSettingsServiceWithDependencies
	NewSideEffects           = sideeffects.NewRegistryWith
	WithSettingsSnapshot     = settingsRecords.WithSettingsSnapshot
	WithSettingsRequestCache = settingsRecords.WithSettingsRequestCache
	ResolveBoolOrDefault     = configSvc.ResolveBoolOrDefault
	ResolveIntOrDefault      = configSvc.ResolveIntOrDefault
)
