package api

import (
	"context"
	"log/slog"
	"strings"

	projectJWT "github.com/moto-nrw/project-phoenix/auth/jwt"
	exportTransferModule "github.com/moto-nrw/project-phoenix/modules/exporttransfer"
	workforceModule "github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	timeTrackingHTTP "github.com/moto-nrw/project-phoenix/modules/workforce/inbound/timetracking"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/uptrace/bun"
)

// The time-tracking and staff administration resources live in the Workforce
// module (#2690). The root binds them to the session identity, the retained
// services behind the public Workforce contracts and the Export Transfer
// capability.

// timeTrackingIdentity derives the caller from the request's session claims.
func timeTrackingIdentity(ctx context.Context) timeTrackingHTTP.Identity {
	claims := projectJWT.ClaimsFromCtx(ctx)
	name := strings.TrimSpace(claims.FirstName + " " + claims.LastName)
	if name == "" {
		name = claims.Username
	}
	return timeTrackingHTTP.Identity{
		AccountID:   int64(claims.ID),
		Roles:       claims.Roles,
		Permissions: projectJWT.PermissionsFromCtx(ctx),
		DisplayName: name,
	}
}

// exportTransferPort adapts the Export Transfer module to the resource's port.
type exportTransferPort struct {
	module *exportTransferModule.Module
}

// newExportTransferPort returns nil when no module is wired, which the
// resource renders as "not configured".
func newExportTransferPort(module *exportTransferModule.Module) timeTrackingHTTP.ExportTransfer {
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
		ActorAccountID: request.ActorAccountID, ActorName: request.ActorName,
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
func newTimeTrackingResource(svc *services.Factory, db *bun.DB) *timeTrackingHTTP.Resource {
	capabilities := services.NewWorkforceAdminCapabilities(svc.Users, svc.StaffDocuments, svc.WorkSession, svc.StaffAbsence, svc.WorkTimeMonth,
		svc.StaffBalanceAdjust, svc.StaffMonthClose, svc.StaffOverview, svc.TimeTrackingAuditLog, svc.StaffTimeExport)
	return timeTrackingHTTP.NewResource(timeTrackingHTTP.Dependencies{
		WorkSessions:     capabilities.WorkSessions,
		StaffAbsences:    capabilities.StaffAbsences,
		Staff:            capabilities.Staff,
		WorkTimeMonth:    capabilities.WorkTimeMonth,
		StaffShifts:      svc.StaffShifts,
		Assignments:      svc.StaffAssignments,
		Calendar:         services.PlanningCalendarCapability(svc.Holidays, svc.ClosingDays),
		AccountStartDate: services.TimeTrackingAccountStartDate(svc.Settings),
		Identity:         timeTrackingIdentity,
		DB:               db,
	})
}

// newStaffAdminResource composes the workforce half of /api/staff over the
// adapted capabilities; schedules is the Workforce query the schedule views
// read from.
func newStaffAdminResource(capabilities services.WorkforceAdminCapabilities, schedules workforceModule.Query, exportTransfer *exportTransferModule.Module, db *bun.DB, logger *slog.Logger) *timeTrackingHTTP.StaffAdminResource {
	cleanup, err := workforceCompose.NewDocumentCleanup(db, nil)
	if err != nil {
		panic(err)
	}
	return timeTrackingHTTP.NewStaffAdminResource(timeTrackingHTTP.StaffAdminDependencies{
		OffboardingCleanup: cleanup,
		Staff:              capabilities.Staff,
		Documents:          capabilities.Documents,
		WorkSessions:       capabilities.WorkSessions,
		StaffAbsences:      capabilities.StaffAbsences,
		WorkTimeMonth:      capabilities.WorkTimeMonth,
		BalanceAdjustments: capabilities.BalanceAdjustments,
		MonthClosing:       capabilities.MonthClosing,
		Overview:           capabilities.Overview,
		AuditLog:           capabilities.AuditLog,
		TimeExport:         capabilities.TimeExport,
		Schedules:          schedules,
		ExportTransfer:     newExportTransferPort(exportTransfer),
		Identity:           timeTrackingIdentity,
		DB:                 db,
		Logger:             logger,
	})
}
