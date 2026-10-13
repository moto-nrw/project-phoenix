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
	OpenAttendanceChecker     = configSvc.OpenAttendanceChecker
	TenantOperations          = configSvc.TenantOperations
	TenantOperationsRuntime   = configSvc.TenantOperationsRuntime
	PayrollStatusGetter       = configSvc.PayrollStatusGetter
	SFTPExportResolvers       = configSvc.SFTPExportResolvers
	SFTPExportKeys            = configSvc.SFTPExportKeys
	SideEffectRegistry        = sideeffects.Registry
	SideEffectFailureObserver = sideeffects.FailureObserver
)

var (
	NewSettingsService                = configSvc.NewSettingsService
	NewSettingsServiceWithHomeLayouts = configSvc.NewSettingsServiceWithHomeLayouts
	NewHomeLayoutService              = configSvc.NewHomeLayoutService
	NewTenantOperations               = configSvc.NewTenantOperations
	NewPayrollStatusService           = configSvc.NewPayrollStatusService
	NewSFTPExportSettings             = configSvc.NewSFTPExportSettings
	NewSideEffectRegistry             = sideeffects.NewRegistry
	WithSettingsSnapshot              = configSvc.WithSettingsSnapshot
	WithSettingsRequestCache          = configSvc.WithSettingsRequestCache
	ResolveBoolOrDefault              = configSvc.ResolveBoolOrDefault
	ResolveIntOrDefault               = configSvc.ResolveIntOrDefault
	ResolveStringOrDefault            = configSvc.ResolveStringOrDefault
)
