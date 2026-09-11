package services

import (
	"context"

	configModels "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/users"
)

// WorkforceAdminCapabilities bundles the public Workforce contracts the staff
// administration and time-tracking resources consume (#2690), each served
// from its retained service.
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

// NewWorkforceAdminCapabilities adapts the retained services. The people and
// work session services are required; every other capability is left nil
// when its service is not wired, so a partial composition (a focused test
// module) still serves the routes it has services for.
func NewWorkforceAdminCapabilities(
	people users.PersonService,
	documents users.StaffDocumentService,
	sessions active.WorkSessionService,
	absences active.StaffAbsenceService,
	months active.WorkTimeMonthService,
	ledger active.StaffBalanceAdjustmentService,
	closing active.StaffMonthCloseService,
	overview active.StaffOverviewService,
	auditLog active.TimeTrackingAuditLogService,
	export active.StaffTimeExportService,
) WorkforceAdminCapabilities {
	capabilities := WorkforceAdminCapabilities{
		Staff:        StaffDirectoryCapability(people),
		WorkSessions: WorkSessionCapability(sessions, people),
	}
	if documents != nil {
		capabilities.Documents = StaffDocumentCapability(documents)
	}
	if absences != nil {
		capabilities.StaffAbsences = StaffAbsenceCapability(absences)
	}
	if months != nil {
		capabilities.WorkTimeMonth = WorkTimeMonthCapability(months)
	}
	if ledger != nil {
		capabilities.BalanceAdjustments = BalanceAdjustmentCapability(ledger)
	}
	if closing != nil {
		capabilities.MonthClosing = MonthClosingCapability(closing)
	}
	if overview != nil {
		capabilities.Overview = StaffOverviewCapability(overview)
	}
	if auditLog != nil {
		capabilities.AuditLog = TimeTrackingAuditLogCapability(auditLog)
	}
	if export != nil {
		capabilities.TimeExport = StaffTimeExportCapability(export)
	}
	return capabilities
}

// TimeTrackingAccountStartDate resolves the tenant's time-account start day
// for the time-tracking config endpoint; the setting key stays next to its
// registry definition. A nil settings service leaves the start date unset.
func TimeTrackingAccountStartDate(settings config.SettingsService) func(context.Context) (string, error) {
	if settings == nil {
		return nil
	}
	return func(ctx context.Context) (string, error) {
		return settings.ResolveString(ctx, configModels.KeyTimeTrackingAccountStartDate)
	}
}
