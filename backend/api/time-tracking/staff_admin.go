// Workforce administration under the /api/staff prefix (#2667). The staff
// directory itself moved to the school-membership module; everything about
// working time, absences, personnel records and staff documents lives here,
// next to the MA-facing time-tracking surface it shares services with. The
// URL surface is unchanged — the composition root mounts this resource and
// the membership adapter into one chi router for /staff.
package timetracking

import (
	"log/slog"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/uptrace/bun"
)

// StaffAdminResource serves the workforce half of /api/staff.
type StaffAdminResource struct {
	PersonService        usersSvc.PersonService
	StaffDocumentService usersSvc.StaffDocumentService
	WorkSessionService   activeSvc.WorkSessionService
	StaffAbsenceService  activeSvc.StaffAbsenceService
	WorkTimeMonthService activeSvc.WorkTimeMonthService
	BalanceAdjustService activeSvc.StaffBalanceAdjustmentService
	MonthCloseService    activeSvc.StaffMonthCloseService
	StaffOverviewService activeSvc.StaffOverviewService
	AuditLogService      activeSvc.TimeTrackingAuditLogService
	TimeExportService    activeSvc.StaffTimeExportService
	db                   *bun.DB
	logger               *slog.Logger
}

// NewStaffAdminResource wires the workforce resource.
func NewStaffAdminResource(
	personService usersSvc.PersonService,
	staffDocumentService usersSvc.StaffDocumentService,
	workSessionService activeSvc.WorkSessionService,
	staffAbsenceService activeSvc.StaffAbsenceService,
	workTimeMonthService activeSvc.WorkTimeMonthService,
	balanceAdjustService activeSvc.StaffBalanceAdjustmentService,
	monthCloseService activeSvc.StaffMonthCloseService,
	staffOverviewService activeSvc.StaffOverviewService,
	auditLogService activeSvc.TimeTrackingAuditLogService,
	timeExportService activeSvc.StaffTimeExportService,
	db *bun.DB,
	logger *slog.Logger,
) *StaffAdminResource {
	// Normalised once here so the handlers can log through rs.logger without
	// a nil guard at every call site.
	if logger == nil {
		logger = slog.Default()
	}
	return &StaffAdminResource{
		PersonService:        personService,
		StaffDocumentService: staffDocumentService,
		WorkSessionService:   workSessionService,
		StaffAbsenceService:  staffAbsenceService,
		WorkTimeMonthService: workTimeMonthService,
		BalanceAdjustService: balanceAdjustService,
		MonthCloseService:    monthCloseService,
		StaffOverviewService: staffOverviewService,
		AuditLogService:      auditLogService,
		TimeExportService:    timeExportService,
		db:                   db,
		logger:               logger,
	}
}

// Router builds a standalone /staff router for this resource. The composition
// root registers the workforce routes into the shared /staff router via
// RegisterStaffRoutes instead, so membership and workforce keep one prefix.
func (rs *StaffAdminResource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	common.ProtectedTenantGroup(r, rs.db, rs.RegisterStaffRoutes)
	return r
}

// RegisterStaffRoutes mounts the workforce routes on an already protected
// tenant group. Routes, permission gates and messages are identical to the
// ones api/staff served before #2667.
func (rs *StaffAdminResource) RegisterStaffRoutes(r chi.Router, withTx common.Middleware) {
	rs.registerOverviewRoutes(r, withTx)
	rs.registerPersonnelRoutes(r, withTx)
	rs.registerDocumentRoutes(r, withTx)
	rs.registerTimeTrackingRoutes(r, withTx)
	rs.registerBalanceRoutes(r, withTx)
	rs.registerCrossStaffRoutes(r, withTx)
	rs.registerAbsenceRoutes(r, withTx)
}

// registerOverviewRoutes serves the Leitungs-Dashboard (#1417 2a). The
// aggregate KPIs are users:read; the per-person Soll/Ist/Saldo table is
// deliberately stricter — working-time data about identifiable people, and
// users:read is held by everyone who may see the staff list at all.
func (rs *StaffAdminResource) registerOverviewRoutes(r chi.Router, withTx common.Middleware) {
	r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/dashboard-summary", rs.getDashboardSummary)
	r.With(common.RequiresPermission(permissions.TimeTrackingManage), withTx).Get("/time-tracking/overview", rs.getTimeTrackingOverview)
}

// registerPersonnelRoutes serves the Personalnummer (#1417) and the
// Stammdaten sections (#1423). Deliberately stricter than users:read:
// birthday, private address, contract terms and qualifications are HR-file
// data about identifiable people. The bank & tax section is staff:financial
// ONLY — the personnel administrators are not the Träger payroll office
// (school admins still match via the admin:* wildcard).
func (rs *StaffAdminResource) registerPersonnelRoutes(r chi.Router, withTx common.Middleware) {
	timeTracking := common.RequiresPermission(permissions.TimeTrackingManage)
	stammdaten := common.RequiresPermission(permissions.StaffStammdaten)
	financial := common.RequiresPermission(permissions.StaffFinancial)

	r.With(timeTracking, withTx).Get("/{id}/payroll-number", rs.getPayrollNumber)
	r.With(timeTracking, withTx).Put("/{id}/payroll-number", rs.updatePayrollNumber)

	r.With(common.RequiresAnyPermission(permissions.StaffStammdaten, permissions.TimeTrackingManage), withTx).Get("/{id}/stammdaten", rs.getStammdaten)
	r.With(stammdaten, withTx).Put("/{id}/stammdaten/person", rs.updateStammdatenPerson)
	r.With(stammdaten, withTx).Put("/{id}/stammdaten/kontakt", rs.updateStammdatenKontakt)
	r.With(stammdaten, withTx).Put("/{id}/stammdaten/arbeitsvertrag", rs.updateStammdatenArbeitsvertrag)
	r.With(stammdaten, withTx).Put("/{id}/stammdaten/qualifikationen", rs.updateStammdatenQualifikationen)
	r.With(financial, withTx).Get("/{id}/stammdaten/bank-steuer", rs.getStammdatenFinancial)
	r.With(financial, withTx).Put("/{id}/stammdaten/bank-steuer", rs.updateStammdatenFinancial)
	r.With(financial, withTx).Post("/{id}/stammdaten/bank-steuer/reveal", rs.revealStammdatenFinancial)
}

// registerDocumentRoutes serves the Dokumente tab (#1424). The route gate
// only proves the caller may reach the tab at all — any of the three category
// permissions. Per-category authority (AU → staff_documents:health, Lohn →
// staff:financial, rest → staff:documents) is enforced in the document
// service, including list filtering.
func (rs *StaffAdminResource) registerDocumentRoutes(r chi.Router, withTx common.Middleware) {
	gate := common.RequiresAnyPermission(permissions.StaffDocuments, permissions.StaffFinancial, permissions.StaffDocumentsHealth)
	r.With(gate, withTx).Get("/{id}/documents", rs.listStaffDocuments)
	// Upload and download each commit their own service transaction before
	// touching filesystem bytes, so failed commits cannot orphan uploads or
	// disclose a sensitive file without its access audit.
	r.With(gate).Post("/{id}/documents", rs.uploadStaffDocument)
	r.With(gate).Get("/{id}/documents/{documentId}/download", rs.downloadStaffDocument)
	r.With(gate, withTx).Delete("/{id}/documents/{documentId}", rs.deleteStaffDocument)
}

// registerTimeTrackingRoutes serves the contractual schedule, the admin
// history read and the Monatskarte (#1842).
func (rs *StaffAdminResource) registerTimeTrackingRoutes(r chi.Router, withTx common.Middleware) {
	timeTracking := common.RequiresPermission(permissions.TimeTrackingManage)

	r.With(common.RequiresAnyPermission(permissions.TimeTrackingManage, permissions.TimeTrackingOwn), withTx).Get("/{id}/schedule", rs.getSchedule)
	r.With(timeTracking, withTx).Put("/{id}/schedule", rs.updateSchedule)

	r.With(timeTracking, withTx).Get("/{id}/time-tracking/history", rs.getStaffHistory)
	r.With(timeTracking, withTx).Get("/{id}/time-tracking/month-summary", rs.getStaffMonthSummary)
	r.With(timeTracking, withTx).Get("/{id}/time-tracking/schedule-targets", rs.getStaffScheduleTargets)
	r.With(timeTracking, withTx).Get("/{id}/time-tracking/export", rs.exportStaffSessions)
}

// registerBalanceRoutes serves the Stundenkonto lifecycle (#1420): payout /
// comp-time adjustments, reset and opening balance, plus the vacation
// takeover at the moto introduction (#2132).
func (rs *StaffAdminResource) registerBalanceRoutes(r chi.Router, withTx common.Middleware) {
	timeTracking := common.RequiresPermission(permissions.TimeTrackingManage)

	r.With(timeTracking, withTx).Get("/{id}/time-tracking/adjustments", rs.listBalanceAdjustments)
	r.With(timeTracking, withTx).Post("/{id}/time-tracking/adjustments", rs.createBalanceAdjustment)
	r.With(timeTracking, withTx).Delete("/{id}/time-tracking/adjustments/{adjustmentId}", rs.deleteBalanceAdjustment)
	r.With(timeTracking, withTx).Post("/{id}/time-tracking/reset", rs.resetStaffBalance)
	r.With(timeTracking, withTx).Post("/{id}/time-tracking/opening", rs.createOpeningBalance)

	r.With(timeTracking, withTx).Post("/{id}/vacation/opening", rs.setVacationOpening)
	r.With(timeTracking, withTx).Delete("/{id}/vacation/opening", rs.deleteVacationOpening)
}

// registerCrossStaffRoutes serves the school-wide feeds (#1417): the audit
// log, the payroll/evidence exports and the Monatsabschluss. Every event
// names an affected and an acting person plus free-text reasons — personal
// data end to end, so these sit behind time_tracking:manage with no
// users:read tier. Freezing the carry chain is school-wide, reopening is per
// staff member; static segments before /{id} are safe — chi prefers them over
// the wildcard.
func (rs *StaffAdminResource) registerCrossStaffRoutes(r chi.Router, withTx common.Middleware) {
	timeTracking := common.RequiresPermission(permissions.TimeTrackingManage)

	r.With(timeTracking, withTx).Get("/time-tracking/audit-log", rs.getTimeTrackingAuditLog)
	r.With(timeTracking, withTx).Get("/time-tracking/export", rs.exportTimeTracking)
	r.With(timeTracking, withTx).Get("/time-tracking/export/datev-report", rs.datevExportReport)

	r.With(timeTracking, withTx).Get("/time-tracking/month-close", rs.listMonthCloseStatus)
	r.With(timeTracking, withTx).Post("/time-tracking/month-close", rs.closeMonth)
	r.With(timeTracking, withTx).Post("/{id}/time-tracking/month-close/reopen", rs.reopenMonth)
}

// registerAbsenceRoutes serves the vacation workflow admin side, the admin
// cross-staff session corrections and the admin absence writes (#1843). The
// MA-Sicht uses /api/time-tracking/absences, which is scoped to the caller;
// these routes let vacation and time-tracking managers reach any staff member
// in the same tenant.
func (rs *StaffAdminResource) registerAbsenceRoutes(r chi.Router, withTx common.Middleware) {
	vacation := common.RequiresPermission(permissions.VacationApprove)
	timeTracking := common.RequiresPermission(permissions.TimeTrackingManage)
	either := common.RequiresAnyPermission(permissions.VacationApprove, permissions.TimeTrackingManage)

	r.With(vacation, withTx).Get("/absences/pending", rs.listPendingAbsenceRequests)
	// Anfragen-Modul, Reiter Mitarbeitende (#2433): open work list and decided
	// history in one display format, with name search and type filter.
	r.With(vacation, withTx).Get("/absences/requests", rs.listAbsenceRequests)
	r.With(vacation, withTx).Post("/absences/{absenceId}/approve", rs.approveAbsence)
	r.With(vacation, withTx).Post("/absences/{absenceId}/deny", rs.denyAbsence)
	r.With(vacation, withTx).Post("/absences/{absenceId}/question", rs.questionAbsence)

	// The vacation quota belongs to the time-tracking tier, not to the
	// personnel record: reading is vacation:approve or time_tracking:manage,
	// writing is time_tracking:manage. staff:manage is deliberately NOT on
	// this write — a holder of it can neither read the quota back nor open
	// the Abwesenheiten tab (#2906).
	r.With(either, withTx).Get("/{id}/vacation/quota", rs.getStaffVacationQuota)
	r.With(timeTracking, withTx).Put("/{id}/vacation/quota", rs.setStaffVacationQuota)

	// Audit trail of a single work session, admin-facing. The MA-side
	// /api/time-tracking/{id}/edits enforces session-staff ownership against
	// the JWT subject; here the route guarantees the session belongs to the
	// staff named in the URL instead.
	r.With(timeTracking, withTx).Get("/{id}/time-tracking/sessions/{sessionId}/edits", rs.getStaffSessionEdits)
	r.With(timeTracking, withTx).Put("/{id}/time-tracking/sessions/{sessionId}", rs.adminUpdateStaffSession)
	r.With(timeTracking, withTx).Post("/{id}/time-tracking/sessions", rs.adminCreateStaffSession)

	r.With(either, withTx).Get("/{id}/absences", rs.getStaffAbsences)

	// The comp-time preview (#2873) feeds the Saldo projection the
	// Freizeitausgleich modal shows before the create.
	r.With(timeTracking, withTx).Get("/{id}/time-tracking/comp-time-preview", rs.getCompTimeBalancePreview)
	r.With(timeTracking, withTx).Post("/{id}/absences", rs.adminCreateStaffAbsence)
	r.With(timeTracking, withTx).Delete("/{id}/absences/{absenceId}", rs.adminDeleteStaffAbsence)
}
