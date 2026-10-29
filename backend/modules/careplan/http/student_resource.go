package careplanhttp

import (
	context "context"
	slog "log/slog"
	http "net/http"
	time "time"

	chi "github.com/go-chi/chi/v5"
	common "github.com/moto-nrw/project-phoenix/api/common"
	permissions "github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	careplan "github.com/moto-nrw/project-phoenix/modules/careplan"
	carerequests "github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	excusedrequests "github.com/moto-nrw/project-phoenix/modules/careplan/excusedrequests"
	masterdatarequests "github.com/moto-nrw/project-phoenix/modules/careplan/masterdatarequests"
	parentrequests "github.com/moto-nrw/project-phoenix/modules/careplan/parentrequests"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	realtime "github.com/moto-nrw/project-phoenix/realtime"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type ResourceConfig struct {
	ArrivalScheduleService       careplan.ArrivalScheduleService
	Broadcaster                  realtime.Broadcaster
	CareResumption               CareResumption
	CareExitRestoration          CareExitRestoration
	CareLifecycleService         careplan.CareLifecycle
	CareRequestReviews           careplan.CareScheduleReviewQuery
	CareRequestService           carerequests.Decisions
	CompanionService             careplan.StudentCompanions
	DocumentStorage              StudentDocumentStorage
	ExcusedRequestService        excusedrequests.Service
	LockTemplateRecurrence       func(ctx context.Context) error
	Logger                       *slog.Logger
	MasterDataReviewService      masterdatarequests.Decisions
	Now                          func() time.Time
	OfferingChangeService        careplan.OfferingChangeRequests
	OfferingPickupTimes          careplan.OfferingPickupTimes
	OfferingSourceResyncer       OfferingSourceResyncer
	ParentEventEmitter           GuardianWake
	ParentRequestBulkService     parentrequests.BulkApprover
	ParentRequestConflictService parentrequests.ConflictResolver
	PartialAbsenceService        careplan.PartialAbsenceService
	PeopleDirectory              peopleModule.Capability
	Persons                      PersonRecords
	PickupAdjustmentService      careplan.PickupAdjustments
	PickupScheduleService        careplan.PickupScheduleService
	PickupSignatureProofs        PickupSignatureProofs
	Reports                      ReportRenderer
	SettingsService              TenantSettings
	StudentAuditService          StudentChangeHistory
	StudentConsents              StudentConsentCapability
	StudentDocumentService       careplan.StudentDocuments
	StudentPhotos                StudentPhotoLifecycle
	UserContextService           CallerContext
	WeekdayPickupNotes           WeekdayPickupNoteReplacer
	RespondCreatedStudent        func(http.ResponseWriter, *http.Request, *peopleModule.StudentState, *peopleModule.Person)
	RespondUpdatedStudent        func(http.ResponseWriter, *http.Request, int64, *peopleModule.Person, bool, bool, time.Time)
	PersistStudentStatusHistory  func(context.Context, *peopleModule.StudentState, *bool, *bool, bool, bool, time.Time, *string) error
	NotifyAbsenceReported        func(context.Context, int64, []int64, string, []calendar.Date, bool, int64) error
	IsPartialAbsenceConflict     func(error) bool
	WriteCareDecision            func(http.ResponseWriter, *http.Request, *careplan.CareScheduleReviewItem)
	WriteCareDetail              func(http.ResponseWriter, *http.Request, *carerequests.HistoryItem)
	WriteMasterDataDecision      func(http.ResponseWriter, *http.Request, *masterdatarequests.ReviewItem)
	WriteExcusedDecision         func(http.ResponseWriter, *http.Request, *excusedrequests.ReviewItem)
	OfferingDiffLabel            func(string, []string) string
}

type Resource struct{ ResourceConfig }

func NewResource(cfg ResourceConfig) *Resource {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Resource{ResourceConfig: cfg}
}
func (rs *Resource) Register(r chi.Router, withTx common.Middleware) {
	r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Post("/master-data-change-requests/{requestId}/decide", rs.decideMasterDataChangeRequest)
	r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Post("/care-schedule-change-requests/{requestId}/decide", rs.decideCareScheduleChangeRequest)
	r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Get("/care-schedule-change-requests/{requestId}", rs.getCareScheduleChangeRequest)
	r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Get("/care-schedule-change-requests/{requestId}/proof", rs.getCareScheduleChangeRequestProof)
	r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Post("/offering-change-requests/{requestId}/preview", rs.previewOfferingChangeRequest)
	r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Post("/offering-change-requests/{requestId}/decide", rs.decideOfferingChangeRequest)
	r.With(common.RequiresAnyPermission(permissions.UsersUpdate, permissions.UsersAbsence), withTx).Post("/excused-absence-requests/{requestId}/decide", rs.decideExcusedAbsenceRequest)
	r.With(common.RequiresAnyPermission(permissions.UsersUpdate, permissions.UsersAbsence), withTx).Post("/change-requests/bulk-approve", rs.bulkApproveParentRequests)
	r.With(common.RequiresAnyPermission(permissions.UsersUpdate, permissions.UsersAbsence), withTx).
		Post("/change-requests/conflicts/resolve", rs.resolveRequestConflict)
	r.With(common.RequiresAnyPermission(permissions.UsersUpdate, permissions.UsersAbsence), withTx).
		Post("/change-requests/{kind}/{requestId}/mark-done", rs.markParentRequestDone)
	r.With(common.RequiresAnyPermission(permissions.UsersUpdate, permissions.UsersAbsence), withTx).
		Post("/change-requests/{kind}/{requestId}/correct", rs.correctParentRequestDecision)
	r.With(common.RequiresPermission(permissions.UsersCreate), withTx).Post("/", rs.createStudent)
	r.With(common.RequiresAnyPermission(permissions.UsersUpdate, permissions.UsersAbsence), withTx).Put("/{id}", rs.updateStudent)
	r.With(common.RequiresPermission(permissions.UsersDelete), withTx).Post("/care-end/preview", rs.previewCareExit)
	r.With(common.RequiresPermission(permissions.UsersDelete), withTx).Post("/care-end", rs.confirmCareExit)
	r.With(common.RequiresPermission(permissions.UsersDelete), withTx).Post("/care-end/cancel", rs.cancelCareExit)
	r.With(common.RequiresPermission(permissions.UsersDelete), withTx).Get("/care-withdrawals", rs.listCareWithdrawals)
	r.With(common.RequiresAnyPermission(permissions.UsersUpdate, permissions.UsersAbsence), withTx).Get("/{id}/care-withdrawals", rs.listStudentResolvedCareWithdrawals)
	r.With(common.RequiresPermission(permissions.UsersDelete), withTx).Post("/care-withdrawals/{completionId}/care-end/preview", rs.previewWithdrawalCareEnd)
	r.With(common.RequiresPermission(permissions.UsersDelete), withTx).Post("/care-withdrawals/{completionId}/care-end", rs.confirmWithdrawalCareEnd)
	r.With(common.RequiresPermission(permissions.UsersDelete), withTx).Post("/{id}/care-end/resume", rs.resumeCare)
	r.With(common.RequiresPermission(permissions.UsersDelete), withTx).Get("/ended-care", rs.listEndedCare)
	r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/companions", rs.getStudentCompanions)
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
	r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Put("/{id}/departure-exceptions/{date}", rs.putStudentDepartureException)
	r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Delete("/{id}/departure-exceptions/{date}", rs.deleteStudentDepartureException)
	r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Post("/{id}/partial-absences", rs.createStudentPartialAbsence)
	r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Put("/{id}/partial-absences/{partialAbsenceId}", rs.updateStudentPartialAbsence)
	r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Delete("/{id}/partial-absences/{partialAbsenceId}", rs.deleteStudentPartialAbsence)
	r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Post("/{id}/pickup-notes", rs.createStudentPickupNote)
	r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Put("/{id}/pickup-notes", rs.replaceStudentWeekdayPickupNotes)
	r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Put("/{id}/pickup-notes/{noteId}", rs.updateStudentPickupNote)
	r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Delete("/{id}/pickup-notes/{noteId}", rs.deleteStudentPickupNote)
	r.With(common.RequiresPermission(permissions.UsersRead), withTx).Post("/pickup-times/bulk", rs.getBulkPickupTimes)
	rs.registerArrivalRoutes(r, withTx)
	rs.registerDocumentRoutes(r, withTx)
}

func (rs *Resource) registerDocumentRoutes(r chi.Router, withTx common.Middleware) {
	studentDocumentsGate := common.RequiresAnyPermission(permissions.UsersUpdate, permissions.StudentDocumentsHealth, permissions.StudentDocumentsLegal)
	r.With(studentDocumentsGate, withTx).Get("/{id}/documents", rs.listStudentDocuments)
	r.With(studentDocumentsGate).Post("/{id}/documents", rs.uploadStudentDocument)
	r.With(studentDocumentsGate).Get("/{id}/documents/{documentId}/download", rs.downloadStudentDocument)
	r.With(studentDocumentsGate, withTx).Delete("/{id}/documents/{documentId}", rs.deleteStudentDocument)
}

func (rs *Resource) registerArrivalRoutes(r chi.Router, withTx common.Middleware) {
	r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/arrival-settings", rs.getArrivalSettings)
	r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/arrival-schedules", rs.getStudentArrivalSchedules)
	r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Put("/{id}/arrival-schedules", rs.updateStudentArrivalSchedules)
	r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Post("/{id}/arrival-exceptions", rs.createStudentArrivalException)
	r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Put("/{id}/arrival-exceptions/{exceptionId}", rs.updateStudentArrivalException)
	r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Delete("/{id}/arrival-exceptions/{exceptionId}", rs.deleteStudentArrivalException)
	r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Post("/{id}/arrival-notes", rs.createStudentArrivalNote)
	r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Put("/{id}/arrival-notes/{noteId}", rs.updateStudentArrivalNote)
	r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Delete("/{id}/arrival-notes/{noteId}", rs.deleteStudentArrivalNote)
	r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Post("/arrival-schedules/bulk", rs.bulkUpsertArrivalSchedules)
	r.With(common.RequiresPermission(permissions.UsersRead), withTx).Post("/arrival-schedules/status", rs.getBulkArrivalScheduleStatus)
	r.With(common.RequiresPermission(permissions.UsersRead), withTx).Post("/arrival-times/bulk", rs.getBulkArrivalTimes)
	r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/class-arrival-times/{schoolClass}", rs.getClassArrivalTimes)
	r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/class-arrival-exceptions/{schoolClass}", rs.getClassArrivalExceptions)
	r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Put("/class-arrival-exceptions/{schoolClass}/{date}", rs.putClassArrivalException)
	r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Delete("/class-arrival-exceptions/{schoolClass}/{date}", rs.deleteClassArrivalException)
}
