//go:build testhooks

package careplanhttp

import (
	"context"

	"github.com/go-chi/render"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The unit suites of this directory are external tests since #6914, so a test
// binary that also imports the root no longer compiles the root a second time
// against this package's test variant. They reach the private helpers they pin
// through these exports, which exist only in testhooks builds.

const AccidentNotRecordedForTest = accidentNotRecorded

var (
	AccidentReportDocumentForTest           = accidentReportDocument
	AccidentReportErrorRendererForTest      = accidentReportErrorRenderer
	AccidentReportVersionsForTest           = accidentReportVersions
	ApplyBusDaysForTest                     = applyBusDays
	ApplyDeparturePlanForTest               = applyDeparturePlan
	ApplyStudentFieldUpdatesForTest         = applyStudentFieldUpdates
	AbsenceOnlyUpdateFieldsForTest          = absenceOnlyUpdateFields
	BuildPickupDataResponseForTest          = buildPickupDataResponse
	CareExitErrorRendererForTest            = careExitErrorRenderer
	CareRequestProofReportForTest           = careRequestProofReport
	CompanionConflictsConfirmedForTest      = companionConflictsConfirmed
	CompanionPlanErrorRendererForTest       = companionPlanErrorRenderer
	ErrCompanionsFingerprintRequiredForTest = errCompanionsFingerprintRequired
	MapExceptionToResponseForTest           = mapExceptionToResponse
	MapScheduleToResponseForTest            = mapScheduleToResponse
	NormalizeDeparturePlanForAuditForTest   = normalizeDeparturePlanForAudit
	OfferingDecisionErrorRendererForTest    = offeringDecisionErrorRenderer
	ParseTimeOnlyForTest                    = parseTimeOnly
	PickupResetErrorRendererForTest         = pickupResetErrorRenderer
	PickupScheduleDateRangeForTest          = pickupScheduleDateRange
	ReconcilePhotoConsentRequestForTest     = reconcilePhotoConsentRequest
	RenderOfferingDecisionErrorForTest      = renderOfferingDecisionError
	RenderPickupAdjustmentErrorForTest      = renderPickupAdjustmentError
	ResolveConflictErrorRendererForTest     = resolveConflictErrorRenderer
	StudentAuditBeforeImageForTest          = studentAuditBeforeImage
	TrimPtrToNilForTest                     = trimPtrToNil
)

// AccidentReportFactsForTest is the student and school data an accident
// report prints.
type AccidentReportFactsForTest = accidentReportFacts

// TenantSettingKeysForTest are the setting keys and values this resource
// reads, for the suite that pins them to the Settings Platform's names.
var TenantSettingKeysForTest = struct {
	AttendanceLogEnabled, AttendanceVisibleDays, RoomDetailVisibleDays, PrivacyConsentRetentionDays string
	EnrollmentBookingsAuthoritative, CareDefaultArrivalTime, CareDefaultPickupTime                  string
	ParentRequestReasonPolicy, SessionEndTime, StudentAbsenceEditScope, PresenceMode                string
	StudentPhotosEnabled, ClassArrivalExceptionEditors, FeedbackEnabled                             string
	StudentAbsenceEditScopeAllStaff, ClassArrivalExceptionEditorsAllStaff                           string
	SchoolPeriodCount                                                                               int
	SchoolPeriodEnd                                                                                 func(int) string
}{
	AttendanceLogEnabled: settingAttendanceLogEnabled, AttendanceVisibleDays: settingAttendanceVisibleDays,
	RoomDetailVisibleDays: settingRoomDetailVisibleDays, PrivacyConsentRetentionDays: settingPrivacyConsentRetentionDays,
	EnrollmentBookingsAuthoritative: settingEnrollmentBookingsAuthoritative,
	CareDefaultArrivalTime:          settingCareDefaultArrivalTime, CareDefaultPickupTime: settingCareDefaultPickupTime,
	ParentRequestReasonPolicy: settingParentRequestReasonPolicy, SessionEndTime: settingSessionEndTime,
	StudentAbsenceEditScope: settingStudentAbsenceEditScope, PresenceMode: settingPresenceMode,
	StudentPhotosEnabled: settingStudentPhotosEnabled, ClassArrivalExceptionEditors: settingClassArrivalExceptionEditors,
	FeedbackEnabled:                 settingFeedbackEnabled,
	StudentAbsenceEditScopeAllStaff: studentAbsenceEditScopeAllStaff, ClassArrivalExceptionEditorsAllStaff: classArrivalExceptionEditorsAllStaff,
	SchoolPeriodCount: schoolPeriodCount, SchoolPeriodEnd: schoolPeriodEndSetting,
}

// IsAbsenceOnlyForTest reports whether the update touches only absence fields.
func (req *UpdateStudentRequest) IsAbsenceOnlyForTest() bool { return req.isAbsenceOnly() }

// TouchesCompanionsForTest reports whether the update edits the companions.
func (req *UpdateStudentRequest) TouchesCompanionsForTest() bool { return req.touchesCompanions() }

// DateForTest is the calendar date Bind parsed from Date.
func (r *ResetOfferingPickupRequest) DateForTest() calendar.Date { return r.date }

// BroadcastStudentUpdatedForTest publishes the student update event.
func (rs *Resource) BroadcastStudentUpdatedForTest(tenantID, studentID int64) {
	rs.broadcastStudentUpdated(tenantID, studentID)
}

// ScheduleStudentUpdateWakesForTest schedules the wakes a student update
// triggers.
func (rs *Resource) ScheduleStudentUpdateWakesForTest(ctx context.Context, tenantID, studentID int64, req *UpdateStudentRequest, companionsChanged bool, reportedStatus string, reportDate calendar.Date) error {
	return rs.scheduleStudentUpdateWakes(ctx, tenantID, studentID, req, companionsChanged, reportedStatus, reportDate)
}

// UpdateStudentTxErrorRendererForTest maps a student update transaction error
// to its response.
func (rs *Resource) UpdateStudentTxErrorRendererForTest(err error) render.Renderer {
	return rs.updateStudentTxErrorRenderer(err)
}
