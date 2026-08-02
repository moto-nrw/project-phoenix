package staff

import (
	"cmp"
	"log/slog"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	authSvc "github.com/moto-nrw/project-phoenix/services/auth"
	educationSvc "github.com/moto-nrw/project-phoenix/services/education"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/uptrace/bun"
)

// Resource defines the staff API resource
type Resource struct {
	PersonService           usersSvc.PersonService
	StaffDocumentService    usersSvc.StaffDocumentService
	StaffOffboardingService usersSvc.StaffOffboardingService
	EducationService        educationSvc.Service
	AuthService             authSvc.AuthService
	WorkSessionService      activeSvc.WorkSessionService
	StaffAbsenceService     activeSvc.StaffAbsenceService
	WorkTimeMonthService    activeSvc.WorkTimeMonthService
	BalanceAdjustService    activeSvc.StaffBalanceAdjustmentService
	MonthCloseService       activeSvc.StaffMonthCloseService
	StaffOverviewService    activeSvc.StaffOverviewService
	AuditLogService         activeSvc.TimeTrackingAuditLogService
	TimeExportService       activeSvc.StaffTimeExportService
	db                      *bun.DB
	logger                  *slog.Logger
}

// NewResource creates a new staff resource
func NewResource(
	personService usersSvc.PersonService,
	staffDocumentService usersSvc.StaffDocumentService,
	staffOffboardingService usersSvc.StaffOffboardingService,
	educationService educationSvc.Service,
	authService authSvc.AuthService,
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
) *Resource {
	return &Resource{
		PersonService:           personService,
		StaffDocumentService:    staffDocumentService,
		StaffOffboardingService: staffOffboardingService,
		EducationService:        educationService,
		AuthService:             authService,
		WorkSessionService:      workSessionService,
		StaffAbsenceService:     staffAbsenceService,
		WorkTimeMonthService:    workTimeMonthService,
		BalanceAdjustService:    balanceAdjustService,
		MonthCloseService:       monthCloseService,
		StaffOverviewService:    staffOverviewService,
		AuditLogService:         auditLogService,
		TimeExportService:       timeExportService,
		db:                      db,
		logger:                  logger,
	}
}

// getLogger returns the injected logger, falling back to slog.Default()
func (rs *Resource) getLogger() *slog.Logger {
	return cmp.Or(rs.logger, slog.Default())
}

// Router returns a configured router for staff endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Protected routes that require authentication and permissions
	common.ProtectedTenantGroup(r, rs.db, func(r chi.Router, withTx common.Middleware) {

		// Leitungs-Dashboard KPIs (#1417 2a). Aggregate only, no per-person
		// working-time data, hence users:read.
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/dashboard-summary", rs.getDashboardSummary)
		// Per-person Soll/Ist/Saldo/Resturlaub across all staff. Deliberately
		// stricter than the issue's users:read: this is working-time data about
		// identifiable people, and users:read is held by everyone who may see
		// the staff list at all.
		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Get("/time-tracking/overview", rs.getTimeTrackingOverview)

		// Staff profile reads are also needed by absence management and the
		// section-specific Stammdaten workflows below.
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/", rs.listStaff)
		r.With(authorize.RequiresPermission(permissions.StaffFinancial), withTx).Get("/financial-profile/{id}", rs.getFinancialProfile)
		r.With(authorize.RequiresAnyPermission(permissions.UsersUpdate, permissions.StaffFinancial, permissions.StaffDocumentsHealth), withTx).Get("/documents-profile/{id}", rs.getDocumentProfile)
		r.With(authorize.RequiresAnyPermission(permissions.UsersRead, permissions.UsersUpdate, permissions.TimeTrackingManage), withTx).Get("/{id}", rs.getStaff)

		// Other staff reads require users:read permission.
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/avatar", rs.serveStaffAvatar)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/groups", rs.getStaffGroups)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/substitutions", rs.getStaffSubstitutions)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/available", rs.getAvailableStaff)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/available-for-substitution", rs.getAvailableForSubstitution)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/by-role", rs.getStaffByRole)

		// Write operations require users:create, users:update, or users:delete permission
		r.With(authorize.RequiresPermission(permissions.UsersCreate), withTx).Post("/", rs.createStaff)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Put("/{id}", rs.updateStaff)
		r.With(authorize.RequiresPermission(permissions.UsersDelete), withTx).Delete("/{id}", rs.deleteStaff)

		// Personnel number (payroll identifier, #1417): time_tracking:manage
		// like every payroll-relevant per-person write of this issue —
		// deliberately not the users:read/update tier of the directory.
		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Get("/{id}/payroll-number", rs.getPayrollNumber)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Put("/{id}/payroll-number", rs.updatePayrollNumber)

		// Stammdaten (#1423). Deliberately stricter than the issue's
		// users:read: birthday, private address, contract terms and
		// qualifications are HR-file data about identifiable people, and
		// users:read is held by everyone who may see the staff list at all
		// (same reasoning as /time-tracking/overview and /payroll-number).
		// Readable only for those who maintain staff (users:update) or hold
		// the management view (time_tracking:manage). The bank & tax section
		// is staff:financial ONLY — the directory maintainers are not the
		// Träger payroll office (school admins still match via the admin:*
		// wildcard).
		r.With(authorize.RequiresAnyPermission(permissions.UsersUpdate, permissions.TimeTrackingManage), withTx).Get("/{id}/stammdaten", rs.getStammdaten)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Put("/{id}/stammdaten/person", rs.updateStammdatenPerson)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Put("/{id}/stammdaten/kontakt", rs.updateStammdatenKontakt)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Put("/{id}/stammdaten/arbeitsvertrag", rs.updateStammdatenArbeitsvertrag)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Put("/{id}/stammdaten/qualifikationen", rs.updateStammdatenQualifikationen)
		r.With(authorize.RequiresPermission(permissions.StaffFinancial), withTx).Get("/{id}/stammdaten/bank-steuer", rs.getStammdatenFinancial)
		r.With(authorize.RequiresPermission(permissions.StaffFinancial), withTx).Put("/{id}/stammdaten/bank-steuer", rs.updateStammdatenFinancial)
		r.With(authorize.RequiresPermission(permissions.StaffFinancial), withTx).Post("/{id}/stammdaten/bank-steuer/reveal", rs.revealStammdatenFinancial)

		// Dokumente tab (#1424). The route gate only proves the caller may
		// reach the tab at all — any of the three category permissions.
		// Per-category authority (AU → staff_documents:health, Lohn →
		// staff:financial, rest → users:update) is enforced in the document
		// service, including list filtering, so a payroll-only account sees
		// exactly the Lohnabrechnung category and nothing else.
		documentsGate := authorize.RequiresAnyPermission(permissions.UsersUpdate, permissions.StaffFinancial, permissions.StaffDocumentsHealth)
		r.With(documentsGate, withTx).Get("/{id}/documents", rs.listStaffDocuments)
		r.With(documentsGate, withTx).Post("/{id}/documents", rs.uploadStaffDocument)
		r.With(documentsGate, withTx).Get("/{id}/documents/{documentId}/download", rs.downloadStaffDocument)
		r.With(documentsGate, withTx).Delete("/{id}/documents/{documentId}", rs.deleteStaffDocument)

		// Work schedule endpoints expose contractual target hours.
		r.With(authorize.RequiresAnyPermission(permissions.TimeTrackingManage, permissions.TimeTrackingOwn), withTx).Get("/{id}/schedule", rs.getSchedule)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Put("/{id}/schedule", rs.updateSchedule)

		// Time tracking history for a specific staff member (admin read)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Get("/{id}/time-tracking/history", rs.getStaffHistory)

		// Monatskarte (#1842)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Get("/{id}/time-tracking/month-summary", rs.getStaffMonthSummary)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Get("/{id}/time-tracking/schedule-targets", rs.getStaffScheduleTargets)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Get("/{id}/time-tracking/export", rs.exportStaffSessions)

		// Stundenkonto lifecycle (#1420): payout / comp-time adjustments + reset
		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Get("/{id}/time-tracking/adjustments", rs.listBalanceAdjustments)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Post("/{id}/time-tracking/adjustments", rs.createBalanceAdjustment)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Delete("/{id}/time-tracking/adjustments/{adjustmentId}", rs.deleteBalanceAdjustment)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Post("/{id}/time-tracking/reset", rs.resetStaffBalance)

		// Monatsabschluss (#1417): freezing the carry chain is school-wide,
		// reopening is per staff member. Static segments before /{id} are
		// safe — chi prefers them over the wildcard.
		// Cross-staff audit feed (#1417). Every event names an affected and an
		// acting person plus free-text reasons — personal data end to end, so
		// the whole feed sits behind time_tracking:manage with no users:read tier.
		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Get("/time-tracking/audit-log", rs.getTimeTrackingAuditLog)

		// Cross-staff payroll/evidence export (#1417 2b). A bulk export of
		// per-person working-time data — time_tracking:manage only, and the
		// service writes a GDPR access-audit row per download.
		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Get("/time-tracking/export", rs.exportTimeTracking)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Get("/time-tracking/export/datev-report", rs.datevExportReport)

		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Get("/time-tracking/month-close", rs.listMonthCloseStatus)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Post("/time-tracking/month-close", rs.closeMonth)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Post("/{id}/time-tracking/month-close/reopen", rs.reopenMonth)

		// Vacation workflow admin-side (Tranche 4)
		r.With(authorize.RequiresPermission(permissions.VacationApprove), withTx).Get("/absences/pending", rs.listPendingAbsenceRequests)
		r.With(authorize.RequiresPermission(permissions.VacationApprove), withTx).Post("/absences/{absenceId}/approve", rs.approveAbsence)
		r.With(authorize.RequiresPermission(permissions.VacationApprove), withTx).Post("/absences/{absenceId}/deny", rs.denyAbsence)
		r.With(authorize.RequiresPermission(permissions.VacationApprove), withTx).Post("/absences/{absenceId}/question", rs.questionAbsence)
		r.With(authorize.RequiresAnyPermission(permissions.VacationApprove, permissions.TimeTrackingManage), withTx).Get("/{id}/vacation/quota", rs.getStaffVacationQuota)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Put("/{id}/vacation/quota", rs.setStaffVacationQuota)
		// Audit trail of a single work session, admin-facing. The MA-side
		// /api/time-tracking/{id}/edits enforces session-staff ownership
		// against the JWT subject; here the route guarantees the session
		// belongs to the staff named in the URL instead.
		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Get("/{id}/time-tracking/sessions/{sessionId}/edits", rs.getStaffSessionEdits)

		// Admin cross-staff corrections. time_tracking:manage is the gate.
		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Put("/{id}/time-tracking/sessions/{sessionId}", rs.adminUpdateStaffSession)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Post("/{id}/time-tracking/sessions", rs.adminCreateStaffSession)

		// Absences for a specific staff member (manager read). The MA-Sicht uses
		// /api/time-tracking/absences which is scoped to the caller; this route
		// lets vacation and time-tracking managers see any staff member in the
		// same tenant.
		r.With(authorize.RequiresAnyPermission(permissions.VacationApprove, permissions.TimeTrackingManage), withTx).Get("/{id}/absences", rs.getStaffAbsences)

		// Admin absence writes (#1843): file or delete an absence on a staff
		// member's behalf; sick reports cascade into the plans in the same tx.
		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Post("/{id}/absences", rs.adminCreateStaffAbsence)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Delete("/{id}/absences/{absenceId}", rs.adminDeleteStaffAbsence)

		// PIN management endpoints - staff can manage their own PIN
		r.With(withTx).Get("/pin", rs.getPINStatus)
		r.With(withTx).Put("/pin", rs.updatePIN)
	})

	return r
}
