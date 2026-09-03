package students

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	notificationsService "github.com/moto-nrw/project-phoenix/modules/delivery/application/notifications"
	"github.com/moto-nrw/project-phoenix/realtime"
	absenceService "github.com/moto-nrw/project-phoenix/services/absence"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	activityService "github.com/moto-nrw/project-phoenix/services/activities"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	educationService "github.com/moto-nrw/project-phoenix/services/education"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	iotSvc "github.com/moto-nrw/project-phoenix/services/iot"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	ogsGroupLiveService "github.com/moto-nrw/project-phoenix/services/ogsgrouplive"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	userContextService "github.com/moto-nrw/project-phoenix/services/usercontext"
	userService "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/uptrace/bun"
)

// Resource defines the students API resource
type Resource struct {
	ResourceConfig
}

// ResourceConfig holds all dependencies for creating a students Resource.
// Using a config struct instead of individual parameters improves maintainability.
type ResourceConfig struct {
	PersonService    userService.PersonService
	GuardianService  *userService.GuardianService
	EducationService educationService.Service
	// GradeTransitionService is required by the purge route only: it strips the
	// child's name from the transition ledger in the same transaction as the
	// delete. Optional so bare test Resources still compile; the purge handler
	// refuses rather than silently skipping the anonymization when it is nil.
	GradeTransitionService *educationService.GradeTransitionService
	UserContextService     userContextService.UserContextService
	ActiveService          activeService.Service
	IoTService             iotSvc.Service
	StaffPINAuthenticator  authService.StaffPINAuthenticator
	PickupScheduleService  scheduleService.PickupScheduleService
	PartialAbsenceService  scheduleService.PartialAbsenceService
	ArrivalScheduleService scheduleService.ArrivalScheduleService
	InstanceService        scheduleService.InstanceService
	// CareDayService gates the day-planning timetable signal on the child's
	// care plan (#1747) — without it a child assigned to a block counts as
	// "kommt heute" on every weekday, including the ones they are not booked
	// for. Optional: nil keeps the unfiltered pre-#1747 behaviour, which is
	// what bare test Resources rely on.
	CareDayService  scheduleService.CareDayService
	SchoolService   platformSvc.SchoolService
	SettingsService configService.SettingsService
	StudentService  userService.StudentService
	// ClassListEntryService supplies the class-list-only entries (#2382) the
	// "Klassenliste" export merges into the Klassenverband. Optional: nil
	// exports without entries (bare test Resources).
	ClassListEntryService  userService.ClassListEntryService
	StudentDeletionService userService.StudentDeletionService
	// CareLifecycleService backs "Betreuung beenden" (#2487) — the regular
	// exit, which is deliberately NOT a deletion.
	CareLifecycleService    userService.CareLifecycleService
	StudentAuditService     userService.StudentAuditService
	MasterDataReviewService userService.MasterDataReviewService
	CareRequestService      scheduleService.CareScheduleRequestService
	// OfferingChangeService backs the post-enrollment offering-change queue
	// (#1665).
	OfferingChangeService    enrollmentService.OfferingChangeRequestService
	PickupAdjustmentService  enrollmentService.PickupAdjustmentService
	ExcusedRequestService    absenceService.ExcusedAbsenceRequestService
	ParentRequestBulkService userService.ParentRequestBulkService
	// ParentRequestConflictService resolves a whole conflict group at once
	// (#2267). Optional: a bare test Resource answers 500 rather than
	// silently deciding requests one by one, which is the bug the group
	// exists to prevent.
	ParentRequestConflictService userService.ParentRequestConflictService
	FamilyProtectionService      userService.FamilyProtectionManager
	// RequestReviewAccess reports the caller's coarse reach over the parent
	// request queues so the empty list can explain itself. Optional: a nil
	// policy omits the field (bare test Resources).
	RequestReviewAccess     ParentRequestReviewAccess
	StudentStatusDayService *activeService.StudentStatusDayService
	AbsenceOverview         *activeService.StudentStatusDayOverviewService
	StudentHistoryService   activeService.StudentHistoryService
	OGSGroupLiveService     ogsGroupLiveService.Getter
	ActivityService         activityService.ActivityService
	EnrollmentDecision      enrollmentService.DecisionService
	EnrollmentFormSchema    enrollmentService.FormSchemaService
	// OfferingSourceResyncer re-reconciles Jahrgang-filtered offering-sourced
	// Regeltermine after a direct school_class edit, in the same transaction —
	// the same hook a grade transition uses (#2147 review round 10). Optional:
	// nil skips the resync (bare test Resources).
	OfferingSourceResyncer educationService.OfferingSourceResyncer
	// LockTemplateRecurrence takes the tenant-wide recurrence gate the resync
	// requires. It must be acquired BEFORE the student row locks (see
	// applyStudentUpdate for the ordering rationale). Required whenever
	// OfferingSourceResyncer is set.
	LockTemplateRecurrence func(ctx context.Context) error
	Broadcaster            realtime.Broadcaster
	// ParentEventEmitter wakes a child's guardians (message-independent
	// parent_child_updated SSE fan-out) after staff-side care writes, so an open
	// parents-app tab refetches the child's care state live (#1725). Optional —
	// nil is a no-op (the guardian helper guards on it), so tests that build a
	// bare Resource keep working.
	ParentEventEmitter *parentmessaging.Emitter
	AbsenceNotifier    notificationsService.AbsenceNotifier
	StudentPhotos      userService.StudentPhotoService
	StudentConsents    userService.StudentConsentService
	// StudentDocumentService backs the child's Dokumente tab (#777).
	StudentDocumentService  userService.StudentDocumentService
	ListExportService       *listexport.RendererService
	Logger                  *slog.Logger
	Now                     func() time.Time
	DB                      *bun.DB
	DevicePINFallback       string
	DeviceLastSeenDebouncer *device.LastSeenDebouncer
}

// NewResource creates a new students resource from the provided configuration.
func NewResource(cfg ResourceConfig) *Resource {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Resource{ResourceConfig: cfg}
}

func (rs *Resource) todayDate() timezone.Date {
	return timezone.DateFromTime(rs.Now())
}

// Router returns a configured router for student endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Protected routes that require authentication and permissions
	common.ProtectedTenantGroup(r, rs.DB, func(r chi.Router, withTx common.Middleware) {

		// Routes requiring users:read permission
		r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/", rs.listStudents)
		// Aggregated OGS-group live projection (#2056). Gated on users:read
		// like the former roster endpoint; the group-derived sections (room
		// status, transfers, tracking indicators) additionally require
		// groups:read and degrade to empty inside the service when it is
		// missing — mirroring the permission split of the replaced single
		// endpoints instead of failing the whole roster.
		r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/ogs-group-live", rs.getOGSGroupLive)
		// Navigation only exposes groups scoped by the service. It remains
		// authenticated-only so legacy caregiver sessions and staff with
		// users:read retain their personal-group navigation; groups:read only
		// controls whether the service includes further tenant groups.
		r.With(withTx).Get("/ogs-group-navigation",
			common.Fetch(func(ctx context.Context) ([]ogsGroupLiveService.Group, error) {
				if rs.OGSGroupLiveService == nil {
					return nil, errors.New("OGS group live service is not configured")
				}
				return rs.OGSGroupLiveService.ListGroups(ctx)
			}, common.ErrorInternalServer, "OGS group navigation retrieved successfully"),
		)
		r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/school-classes", rs.listSchoolClasses)
		r.With(common.RequiresPermission(permissions.UsersRead), withTx).Post("/export", rs.exportStudents)
		r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}", rs.getStudent)
		r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/in-group-room", rs.getStudentInGroupRoom)
		r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/current-location", rs.getStudentCurrentLocation)
		r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/current-visit", rs.getStudentCurrentVisit)
		r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/visit-history", rs.getStudentVisitHistory)
		// Group day log ("Tagesauswertung", #1456). Static paths take
		// precedence over /{id} in chi; gate + scope enforced in the handlers.
		r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/day-log", rs.getStudentsDayLog)
		r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/day-log/export", rs.exportStudentsDayLog)
		r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/attendance-history", rs.getStudentAttendanceHistory)
		r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/attendance-history/export", rs.exportStudentAttendanceHistory)
		// Planned absence days. users:read like every other read here — an
		// absence writer always holds it, because users:absence is a write scope
		// on top of the children a caller may see and grants nothing on its own
		// (authorize.CanManageStudentAbsence). Widening this route instead would
		// have let a caller without any read permission pull absence data out of
		// the tenant while the child's list entry and detail page stayed closed
		// to them either way.
		r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/status-days", rs.getStudentStatusDays)
		// Absence overview (#2288): forward-looking status-day list across the
		// children of every group the caller may see. Static path takes
		// precedence over /{id}/status-days in chi.
		r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/status-days", rs.getStudentStatusDaysOverview)
		r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/enrollment-extra-fields", rs.getStudentEnrollmentExtraFields)
		// Per-child change history (issue #1455). Full access (admin / group
		// supervisor) is enforced inside the handler.
		r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/change-history", rs.getStudentChangeHistory)

		// Parent Stammdaten change-request decision (Track B). Requests can
		// contain parent-submitted name, birthday, and departure-plan changes.
		// Gated on users:update — the same permission as editing a child directly
		// (PUT /{id}) — because deciding a request is that same write. The service
		// additionally scopes the decision per child (admin or the child's group
		// supervisor), so a supervisor decides only their own group's requests.
		// Reading the queue itself goes through the aggregated list below.
		// Static paths take precedence over the /{id} param route in chi.
		r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Post("/master-data-change-requests/{requestId}/decide", rs.decideMasterDataChangeRequest)

		// Parent care-schedule change-request decision (#1803). Decisions rewrite
		// the child's permanent weekly plan, so they share the users:update gate +
		// per-child write scope of the master-data queue — both are decided in the
		// same Anfragen module.
		r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Post("/care-schedule-change-requests/{requestId}/decide", rs.decideCareScheduleChangeRequest)

		// Post-enrollment offering change requests (#1665). Approving one moves
		// the child between activity groups on a chosen date, so it shares the
		// users:update gate of the queues it sits next to in the Anfragen module.
		r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Post("/offering-change-requests/{requestId}/preview", rs.previewOfferingChangeRequest)
		r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Post("/offering-change-requests/{requestId}/decide", rs.decideOfferingChangeRequest)

		// Excused-absence approval requests (#1845): staff decision. Deciding one
		// writes status days, so it is gated like the staff-side absence actions:
		// users:update OR users:absence at the route, with the per-child absence
		// gate deciding inside the service (#2232).
		r.With(common.RequiresAnyPermission(permissions.UsersUpdate, permissions.UsersAbsence), withTx).Post("/excused-absence-requests/{requestId}/decide", rs.decideExcusedAbsenceRequest)

		// Combined pending count across the review queues, driving the
		// Anfragen sidebar badge. Same gate + per-child scope as the
		// queues it summarizes (the count sums their scoped lists), including
		// the excused queue's users:absence path — a caller who only holds that
		// permission counts excused requests and nothing else.
		r.With(common.RequiresAnyPermission(permissions.UsersUpdate, permissions.UsersAbsence), withTx).Get("/change-requests/pending-count", rs.pendingChangeRequestCount)

		// Effective parent-request capability for shared navigation. This route
		// is authenticated-only because callers with config:manage, users:delete
		// or vacation:approve may open another part of the Anfragen module without
		// holding the two permissions required by the aggregated parent queue.
		r.With(withTx).Get("/change-requests/access", rs.changeRequestAccess)

		// Aggregated Eltern request list (#2432): all four queues as ONE list
		// (open or history) with search, filters and keyset pagination. Same
		// route gate as the badge; inside, an absence-only caller is narrowed
		// to the excused queue — the only one whose per-type routes accept
		// users:absence.
		r.With(common.RequiresAnyPermission(permissions.UsersUpdate, permissions.UsersAbsence), withTx).Get("/change-requests", rs.listAggregatedChangeRequests)
		r.With(common.RequiresAnyPermission(permissions.UsersUpdate, permissions.UsersAbsence), withTx).Post("/change-requests/bulk-approve", rs.bulkApproveParentRequests)
		// Gemeinsames Ergebnis festlegen (#2267): decide a whole conflict
		// group in one transaction. Same route gate as the list; the per-kind
		// permission and the per-child scope are re-checked inside the
		// coordinator and the domain services.
		r.With(common.RequiresAnyPermission(permissions.UsersUpdate, permissions.UsersAbsence), withTx).
			Post("/change-requests/conflicts/resolve", rs.resolveRequestConflict)
		// Als erledigt markieren (#2267): the third verdict for a request whose
		// days have all passed. Same route gate as the list; the per-child
		// scope and the per-kind gate are re-checked inside the service.
		r.With(common.RequiresAnyPermission(permissions.UsersUpdate, permissions.UsersAbsence), withTx).
			Post("/change-requests/{kind}/{requestId}/mark-done", rs.markParentRequestDone)
		// Entscheidung korrigieren (#2267): rewrite a decision staff already
		// took. The old decision stays in the ledger.
		r.With(common.RequiresAnyPermission(permissions.UsersUpdate, permissions.UsersAbsence), withTx).
			Post("/change-requests/{kind}/{requestId}/correct", rs.correctParentRequestDecision)
		r.With(common.RequiresPermission(permissions.ConfigManage), withTx).Get("/{id}/family-protection", rs.getFamilyProtection)
		r.With(common.RequiresPermission(permissions.ConfigManage), withTx).Put("/{id}/family-protection", rs.setFamilyProtection)

		// Routes requiring users:create permission
		r.With(common.RequiresPermission(permissions.UsersCreate), withTx).Post("/", rs.createStudent)

		// Routes requiring users:update permission. The absence surfaces accept
		// users:absence as well — it is the permission a school without fixed
		// groups grants for exactly these writes (#2232). The coarse route gate
		// only decides who may knock; per-child authority is decided in the
		// handlers (authorizeStudentUpdate / canManageStudentAbsence).
		//
		// PUT /{id} carries both the Krankmeldung and every Stammdaten field, so
		// it re-checks the permission itself: the full write needs users:update,
		// and a users:absence holder gets nothing but a payload of sick/excused
		// past it, no matter which groups they supervise.
		r.With(common.RequiresAnyPermission(permissions.UsersUpdate, permissions.UsersAbsence), withTx).Put("/{id}", rs.updateStudent)
		r.With(common.RequiresAnyPermission(permissions.UsersUpdate, permissions.UsersAbsence), withTx).Post("/status-days/bulk", rs.bulkCreateStudentStatusDays)
		r.With(common.RequiresAnyPermission(permissions.UsersUpdate, permissions.UsersAbsence), withTx).Post("/{id}/status-days", rs.createStudentStatusDays)
		r.With(common.RequiresAnyPermission(permissions.UsersUpdate, permissions.UsersAbsence), withTx).Delete("/{id}/status-days/{statusDayId}", rs.deleteStudentStatusDay)

		// "Betreuung beenden" (#2487). Behind users:delete like the permanent
		// deletion — ending a care relationship and reading why it ended are
		// the two halves of one sensitive decision — but a deliberately
		// separate surface: a regular exit is not a data deletion.
		// Static paths are registered before /{id} so chi never routes
		// "care-end" into the id parameter.
		r.With(common.RequiresPermission(permissions.UsersDelete), withTx).Post("/care-end/preview", rs.previewCareExit)
		r.With(common.RequiresPermission(permissions.UsersDelete), withTx).Post("/care-end", rs.confirmCareExit)
		r.With(common.RequiresPermission(permissions.UsersDelete), withTx).Post("/care-end/cancel", rs.cancelCareExit)
		r.With(common.RequiresPermission(permissions.UsersDelete), withTx).Get("/care-withdrawals", rs.listCareWithdrawals)
		r.With(common.RequiresPermission(permissions.UsersDelete), withTx).Post("/care-withdrawals/{completionId}/care-end/preview", rs.previewWithdrawalCareEnd)
		r.With(common.RequiresPermission(permissions.UsersDelete), withTx).Post("/care-withdrawals/{completionId}/care-end", rs.confirmWithdrawalCareEnd)
		r.With(common.RequiresPermission(permissions.UsersDelete), withTx).Get("/care-withdrawals/{completionId}/deletion-impact", rs.previewWithdrawalDeletion)
		r.With(common.RequiresPermission(permissions.UsersDelete), withTx).Delete("/care-withdrawals/{completionId}", rs.deleteWithdrawalStudent)
		r.With(common.RequiresPermission(permissions.UsersDelete), withTx).Post("/{id}/care-end/resume", rs.resumeCare)
		r.With(common.RequiresPermission(permissions.UsersDelete), withTx).Get("/ended-care", rs.listEndedCare)

		// Routes requiring users:delete permission
		r.With(common.RequiresPermission(permissions.UsersDelete), withTx).Delete("/{id}", rs.deleteStudent)
		r.With(common.RequiresPermission(permissions.UsersDelete), withTx).Get("/{id}/delete-impact", rs.getStudentDeleteImpact)
		// Hard-deletes a child that a grade transition graduated. Separate from
		// the route above because the alumnus gate that route relies on is
		// exactly what this one has to bypass — see purgeGraduatedStudent.
		r.With(common.RequiresPermission(permissions.UsersDelete), withTx).Delete("/{id}/purge", rs.purgeGraduatedStudent)

		// Privacy consent routes
		r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/privacy-consent", rs.getStudentPrivacyConsent)
		r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Put("/{id}/privacy-consent", rs.updateStudentPrivacyConsent)

		// Departure companions ("läuft mit" / Laufgemeinschaft), read-only.
		// There is deliberately no PUT here: a link is only legal on a weekday
		// the child's departure plan allows "Anderes Kind", so writing it
		// separately from that plan would allow the two to contradict each
		// other. Companions are written through PUT /students/{id}.
		r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/companions", rs.getStudentCompanions)

		// Pickup schedule routes (full access required - checked in handlers)
		r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/pickup-schedules", rs.getStudentPickupSchedules)
		r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/partial-absences", rs.getStudentPartialAbsences)
		r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Put("/{id}/pickup-schedules", rs.updateStudentPickupSchedules)
		r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Post("/{id}/pickup-schedules/preview", rs.previewStudentPickupAdjustment)
		r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Post("/{id}/pickup-schedules/apply", rs.applyStudentPickupAdjustment)
		r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Post("/{id}/pickup-schedules/reset-offering", rs.resetStudentPickupToOffering)
		r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Post("/pickup-schedules/bulk", rs.bulkUpsertPickupSchedules)
		r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Post("/{id}/pickup-exceptions", rs.createStudentPickupException)
		r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Put("/{id}/pickup-exceptions/{exceptionId}", rs.updateStudentPickupException)
		r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Delete("/{id}/pickup-exceptions/{exceptionId}", rs.deleteStudentPickupException)
		r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Post("/{id}/partial-absences", rs.createStudentPartialAbsence)
		r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Put("/{id}/partial-absences/{partialAbsenceId}", rs.updateStudentPartialAbsence)
		r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Delete("/{id}/partial-absences/{partialAbsenceId}", rs.deleteStudentPartialAbsence)

		// Pickup note routes (full access required - checked in handlers)
		r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Post("/{id}/pickup-notes", rs.createStudentPickupNote)
		r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Put("/{id}/pickup-notes/{noteId}", rs.updateStudentPickupNote)
		r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Delete("/{id}/pickup-notes/{noteId}", rs.deleteStudentPickupNote)

		// Bulk pickup times endpoint (returns pickup times for multiple students)
		r.With(common.RequiresPermission(permissions.UsersRead), withTx).Post("/pickup-times/bulk", rs.getBulkPickupTimes)

		// Arrival schedule routes (full access required - checked in handlers)
		r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/arrival-settings", rs.getArrivalSettings)
		r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/arrival-schedules", rs.getStudentArrivalSchedules)
		r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Put("/{id}/arrival-schedules", rs.updateStudentArrivalSchedules)
		r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Post("/{id}/arrival-exceptions", rs.createStudentArrivalException)
		r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Put("/{id}/arrival-exceptions/{exceptionId}", rs.updateStudentArrivalException)
		r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Delete("/{id}/arrival-exceptions/{exceptionId}", rs.deleteStudentArrivalException)

		// Arrival note routes (full access required - checked in handlers)
		r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Post("/{id}/arrival-notes", rs.createStudentArrivalNote)
		r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Put("/{id}/arrival-notes/{noteId}", rs.updateStudentArrivalNote)
		r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Delete("/{id}/arrival-notes/{noteId}", rs.deleteStudentArrivalNote)

		// Bulk arrival schedule and time endpoints
		r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Post("/arrival-schedules/bulk", rs.bulkUpsertArrivalSchedules)
		r.With(common.RequiresPermission(permissions.UsersRead), withTx).Post("/arrival-schedules/status", rs.getBulkArrivalScheduleStatus)
		r.With(common.RequiresPermission(permissions.UsersRead), withTx).Post("/arrival-times/bulk", rs.getBulkArrivalTimes)
		r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/class-arrival-times/{schoolClass}", rs.getClassArrivalTimes)

		// Class-wide arrival day exceptions (#2962). Writes additionally run
		// through operations.class_arrival_exception_editors in the handler.
		r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/class-arrival-exceptions/{schoolClass}", rs.getClassArrivalExceptions)
		r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Put("/class-arrival-exceptions/{schoolClass}/{date}", rs.putClassArrivalException)
		r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Delete("/class-arrival-exceptions/{schoolClass}/{date}", rs.deleteClassArrivalException)

		// Web-based school check-in/out. Mode-agnostic (writes attendance only).
		// The users:checkin permission is the gate; any verified staff member may
		// toggle any student (#2329).
		r.With(common.RequiresPermission(permissions.UsersCheckin), withTx, common.RequireWebAttendanceEnabled(rs.SettingsService)).Post("/{id}/school-checkin", rs.schoolCheckinHandler)
		// Batch variant (#2359): one explicit action for a selected set of
		// children. Same gate as the single endpoint; static path takes
		// precedence over /{id} in chi.
		r.With(common.RequiresPermission(permissions.UsersCheckin), withTx, common.RequireWebAttendanceEnabled(rs.SettingsService)).Post("/school-checkin/batch", rs.schoolCheckinBatchHandler)

		// Student photo (Datenverwaltung). upload + delete: users:update;
		// serve: users:read. Feature gate + consent enforced in photo.go.
		// upload + serve skip withTx so a slow body / file stream doesn't
		// pin a bun pool connection. The handlers open their own short tx.
		r.With(common.RequiresPermission(permissions.UsersUpdate)).Post("/{id}/photo", rs.uploadStudentPhoto)
		r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Delete("/{id}/photo", rs.deleteStudentPhoto)
		r.With(common.RequiresPermission(permissions.UsersRead)).Get("/{id}/photo/{filename}", rs.serveStudentPhoto)

		// Child documents (#777). The gate only proves the caller may reach
		// the tab at all. Per-category authority (health → student_documents:health,
		// Sorgerecht → student_documents:legal, the rest → users:update) is
		// decided in the document service, which also filters the list down to
		// the categories the caller may see.
		//
		// upload + download skip withTx so a slow 10 MB body or file stream
		// doesn't pin a bun pool connection; those handlers open their own
		// short transactions.
		studentDocumentsGate := common.RequiresAnyPermission(
			permissions.UsersUpdate,
			permissions.StudentDocumentsHealth,
			permissions.StudentDocumentsLegal,
		)
		r.With(studentDocumentsGate, withTx).Get("/{id}/documents", rs.listStudentDocuments)
		r.With(studentDocumentsGate).Post("/{id}/documents", rs.uploadStudentDocument)
		r.With(studentDocumentsGate).Get("/{id}/documents/{documentId}/download", rs.downloadStudentDocument)
		r.With(studentDocumentsGate, withTx).Delete("/{id}/documents/{documentId}", rs.deleteStudentDocument)
	})

	// Device-authenticated routes for RFID devices.
	// DeviceAuthenticator validates API key + PIN and sets tenant context,
	// then TenantTxMiddleware wraps each handler in a tenant-scoped transaction
	// (SET LOCAL ROLE phoenix_tenant + set_config) so RLS is enforced.
	r.Group(func(r chi.Router) {
		r.Use(device.DeviceAuthenticatorWithDebouncer(rs.IoTService, rs.SchoolService, rs.StaffPINAuthenticator, nil, rs.DevicePINFallback, rs.DeviceLastSeenDebouncer))
		r.Use(common.TenantTxMiddleware)

		// RFID tag assignment endpoint
		r.Post("/{id}/rfid", rs.assignRFIDTag)
		r.Delete("/{id}/rfid", rs.unassignRFIDTag)
	})

	return r
}
