package compose

import (
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/config/sideeffects"
)

// The retained settings service and its helpers, which the legacy
// composition names through this composition instead of the Settings
// Platform's application packages (#2747).
type (
	SettingsService           = configSvc.SettingsService
	BatchSettingsService      = configSvc.BatchSettingsService
	SettingsRuntime           = configSvc.Runtime
	SchoolSettingsStore       = configSvc.SchoolSettingsStore
	SettingsSnapshot          = configSvc.SettingsSnapshot
	SettingsLookupObserver    = configSvc.SettingsLookupObserver
	SettingsChangedNotifier   = configSvc.SettingsChangedNotifier
	OperatorValueSetHook      = configSvc.OperatorValueSetHook
	TenantOperations          = configSvc.TenantOperations
	TenantOperationsRuntime   = configSvc.TenantOperationsRuntime
	PayrollStatusGetter       = configSvc.PayrollStatusGetter
	SideEffectRegistry        = sideeffects.Registry
	SideEffectFailureObserver = sideeffects.FailureObserver
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
	WithSettingsSnapshot     = configSvc.WithSettingsSnapshot
	WithSettingsRequestCache = configSvc.WithSettingsRequestCache
	ResolveBoolOrDefault     = configSvc.ResolveBoolOrDefault
	ResolveIntOrDefault      = configSvc.ResolveIntOrDefault
)
