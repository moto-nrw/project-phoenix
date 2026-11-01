package api

import (
	"context"
	"log/slog"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	nativeUploads "github.com/moto-nrw/project-phoenix/modules/delivery/objects"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	staffShiftsHTTP "github.com/moto-nrw/project-phoenix/modules/workforce/http/staffshifts"

	exportTransferModule "github.com/moto-nrw/project-phoenix/modules/exporttransfer"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	workforceModule "github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	absencetypesHTTP "github.com/moto-nrw/project-phoenix/modules/workforce/http/absencetypes"
	timeTrackingHTTP "github.com/moto-nrw/project-phoenix/modules/workforce/http/timetracking"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The time-tracking and staff administration resources live in the Workforce
// module (#2690). The root binds them to the session identity, the retained
// services behind the public Workforce contracts and the Export Transfer
// capability.

// timeTrackingIdentity derives the caller from the request's session claims.
func timeTrackingIdentity(ctx context.Context) timeTrackingHTTP.Identity {
	claims := sessiontoken.ClaimsFromCtx(ctx)
	return timeTrackingHTTP.Identity{
		AccountID:   int64(claims.ID),
		Roles:       claims.Roles,
		Permissions: sessiontoken.PermissionsFromCtx(ctx),
		DisplayName: claims.DisplayName(),
	}
}

// exportTransferPort adapts the Export Transfer module to the resource's port.
type exportTransferPort struct {
	module exportTransferModule.Capability
}

// newExportTransferPort returns nil when no module is wired, which the
// resource renders as "not configured".
func newExportTransferPort(module exportTransferModule.Capability) timeTrackingHTTP.ExportTransfer {
	if module == nil {
		return nil
	}
	return exportTransferPort{module: module}
}

func (p exportTransferPort) Status(ctx context.Context) (timeTrackingHTTP.ExportTransferStatus, error) {
	status, err := p.module.Status(ctx)
	if err != nil {
		return timeTrackingHTTP.ExportTransferStatus{}, err
	}
	return timeTrackingHTTP.ExportTransferStatus{
		Enabled: status.Enabled, Ready: status.Ready, Host: status.Host, Port: status.Port,
		RemoteDirectory: status.RemoteDirectory, MissingSettings: status.MissingSettings,
	}, nil
}

func (p exportTransferPort) Transfer(ctx context.Context, request timeTrackingHTTP.ExportTransferRequest) (timeTrackingHTTP.ExportTransferOutcome, error) {
	outcome, err := p.module.Transfer(ctx, exportTransferModule.Request{
		Kind: request.Kind, Format: request.Format, Filename: request.Filename, Data: request.Data,
		ActorAccountID: request.ActorAccountID, ActorName: request.ActorName, PrepareFile: request.PrepareFile,
	})
	if err != nil {
		return timeTrackingHTTP.ExportTransferOutcome{}, err
	}
	return timeTrackingHTTP.ExportTransferOutcome{
		Transferred: outcome.Transferred, Filename: outcome.Filename, ByteSize: outcome.ByteSize,
		TargetHost: outcome.TargetHost, TargetDirectory: outcome.TargetDirectory, Reason: outcome.Reason,
	}, nil
}

// newTimeTrackingResource composes the MA-facing /api/time-tracking surface.
func newTimeTrackingResource(
	users peoplePorts.PersonDirectory, documents *workforceModule.StaffAdmin, timeTracking *workforceCompose.TimeTracking,
	absenceWrites timeTrackingHTTP.OwnAbsenceWrites,
	shifts workforceModule.StaffShiftPlanning, assignments workforceModule.StaffAssignmentQuery,
	allowances workforceModule.OwnAbsenceAllowances, calendar schoolcalendar.Calendar, settings settingsCompose.SettingsService,
) *timeTrackingHTTP.Resource {
	capabilities := owners.NewWorkforceAdminCapabilities(users, documents, timeTracking)
	return timeTrackingHTTP.NewResource(timeTrackingHTTP.Dependencies{
		WorkSessions:       capabilities.WorkSessions,
		StaffAbsences:      capabilities.StaffAbsences,
		AbsenceWrites:      absenceWrites,
		Staff:              capabilities.Staff,
		WorkTimeMonth:      capabilities.WorkTimeMonth,
		StaffShifts:        shifts,
		ShiftResponses:     timeTrackingShiftResponses,
		Assignments:        assignments,
		Calendar:           PlanningCalendarCapability(calendar),
		Allowances:         allowances,
		AllowanceResponses: timeTrackingAllowanceResponses,
		AccountStartDate:   timeTrackingAccountStartDate(settings),
		Identity:           timeTrackingIdentity,
		WithinTenant: func(ctx context.Context, tenantID int64, run func(context.Context) error) error {
			id, err := tenant.NewTenantID(tenantID)
			if err != nil {
				return err
			}
			return tenant.WithinTenant(ctx, id, run)
		},
	})
}

// newStaffAdminResource composes the workforce half of /api/staff over the
// adapted capabilities; schedules is the Workforce query the schedule views
// read from and the Sonderarbeitszeiten are written through; notifyChanged
// invalidates the time-account views after such a write.
func newStaffAdminResource(capabilities owners.WorkforceAdminCapabilities, absenceWrites timeTrackingHTTP.AdminAbsenceWrites, schedules workforceModule.Capability, notifyChanged func(context.Context), exportTransfer exportTransferModule.Capability, payroll workforceModule.PayrollDispatch, db transactionruntime.Database, logger *slog.Logger) *timeTrackingHTTP.StaffAdminResource {
	cleanup, err := workforceCompose.NewDocumentCleanup(db.Pool(), nil)
	if err != nil {
		panic(err)
	}
	documentFiles, err := workforceCompose.NewStaffDocumentFileRecovery(capabilities.Documents, cleanup, nativeUploads.UploadFiles{}, logger)
	if err != nil {
		panic(err)
	}
	payrollRules, err := owners.BuildPayrollRules(db)
	if err != nil {
		panic(err)
	}
	return timeTrackingHTTP.NewStaffAdminResource(timeTrackingHTTP.StaffAdminDependencies{
		Uploads:            nativeUploads.UploadFiles{},
		OffboardingCleanup: cleanup,
		DocumentFiles:      documentFiles,
		Staff:              capabilities.Staff,
		Documents:          capabilities.Documents,
		WorkSessions:       capabilities.WorkSessions,
		StaffAbsences:      capabilities.StaffAbsences,
		AbsenceWrites:      absenceWrites,
		WorkTimeMonth:      capabilities.WorkTimeMonth,
		BalanceAdjustments: capabilities.BalanceAdjustments,
		MonthClosing:       capabilities.MonthClosing,
		Overview:           capabilities.Overview,
		AuditLog:           capabilities.AuditLog,
		TimeExport:         capabilities.TimeExport,
		Schedules:          schedules,
		TargetOverrides:    schedules,
		NotifyChanged:      notifyChanged,
		ExportTransfer:     newExportTransferPort(exportTransfer),
		PayrollDispatch:    payroll,
		PayrollRules:       payrollRules,
		Identity:           timeTrackingIdentity,
		Logger:             logger,
	})
}

func timeTrackingAllowanceResponses(accounts []workforceModule.AbsenceTypeAllowanceAccount) any {
	return absencetypesHTTP.ToOwnAllowanceResponses(accounts)
}

func timeTrackingShiftResponses(shifts []workforceModule.PlannedShift) any {
	return staffShiftsHTTP.ToShiftResponses(shifts)
}
