package services

import (
	"context"

	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
)

// WorkforceAdminCapabilities bundles the public Workforce contracts the staff
// administration and time-tracking resources consume (#2690).
type WorkforceAdminCapabilities struct {
	Staff              workforce.StaffDirectory
	Documents          workforce.StaffDocuments
	WorkSessions       workforce.WorkSessions
	StaffAbsences      workforce.StaffAbsences
	WorkTimeMonth      workforce.WorkTimeMonths
	BalanceAdjustments workforce.BalanceAdjustments
	MonthClosing       workforce.MonthClosing
	Overview           workforce.StaffOverview
	AuditLog           workforce.TimeTrackingAuditLog
	TimeExport         workforce.StaffTimeExport
}

// NewWorkforceAdminCapabilities bundles the staff directory and the
// personnel-record administration with the time-tracking contracts the
// Workforce composition built. A nil time-tracking side leaves those
// contracts nil, so a partial composition still serves the routes it has.
func NewWorkforceAdminCapabilities(people *peopleCompose.PersonDirectory, staffAdmin *workforce.StaffAdmin, timeTracking *workforceCompose.TimeTracking) WorkforceAdminCapabilities {
	capabilities := WorkforceAdminCapabilities{Staff: StaffDirectoryCapability(people, staffAdmin)}
	if staffAdmin != nil {
		capabilities.Documents = staffAdmin
	}
	if timeTracking != nil {
		capabilities.WorkSessions = timeTracking.WorkSessions
		capabilities.StaffAbsences = timeTracking.StaffAbsences
		capabilities.WorkTimeMonth = timeTracking.WorkTimeMonths
		capabilities.BalanceAdjustments = timeTracking.BalanceAdjustments
		capabilities.MonthClosing = timeTracking.MonthClosing
		capabilities.Overview = timeTracking.Overview
		capabilities.AuditLog = timeTracking.AuditLog
		capabilities.TimeExport = timeTracking.TimeExport
	}
	return capabilities
}

// TimeTrackingAccountStartDate resolves the tenant's time-account start day
// for the time-tracking config endpoint; the setting key stays next to its
// registry definition. A nil settings service leaves the start date unset.
func TimeTrackingAccountStartDate(settings settingsCompose.SettingsService) func(context.Context) (string, error) {
	if settings == nil {
		return nil
	}
	return func(ctx context.Context) (string, error) {
		return settings.ResolveString(ctx, configModels.KeyTimeTrackingAccountStartDate)
	}
}
