package parent

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentSvc "github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal/care"
)

// The Care Plan side of the guardian portal lives in
// workflows/parentportal/care (#3227). These aliases keep the
// services/parent vocabulary that api/parent names unchanged.
type (
	AttendanceReader                = care.AttendanceReader
	ChildCareOfferings              = care.ChildCareOfferings
	CareOfferingSelection           = care.CareOfferingSelection
	PendingOfferingChange           = care.PendingOfferingChange
	ChildCareSchedule               = care.ChildCareSchedule
	CareScheduleWeekday             = care.CareScheduleWeekday
	PendingCareRequest              = care.PendingCareRequest
	CareScheduleRequestCapabilities = care.CareScheduleRequestCapabilities
	TodayStatus                     = care.TodayStatus
	DayState                        = care.DayState
)

const (
	DayStateExpected   = care.DayStateExpected
	DayStateNotArrived = care.DayStateNotArrived
	DayStatePresent    = care.DayStatePresent
	DayStateLeft       = care.DayStateLeft
	DayStateAbsent     = care.DayStateAbsent
	DayStateNoCare     = care.DayStateNoCare
	DayStateUnknown    = care.DayStateUnknown

	OfferingChangesReasonNoEnrollment = care.OfferingChangesReasonNoEnrollment
	OfferingChangesReasonNoPermission = care.OfferingChangesReasonNoPermission
	OfferingChangesReasonSchoolOff    = care.OfferingChangesReasonSchoolOff
	OfferingChangesReasonPeriodOver   = care.OfferingChangesReasonPeriodOver
	OfferingChangesReasonNoTime       = care.OfferingChangesReasonNoTime
)

var (
	ErrCareRequestNotFound              = care.ErrCareRequestNotFound
	ErrCareRequestNotPending            = care.ErrCareRequestNotPending
	ErrCareRequestAlreadyPending        = care.ErrCareRequestAlreadyPending
	ErrInvalidCareRequestPayload        = care.ErrInvalidCareRequestPayload
	ErrCareRequestFieldDisabled         = care.ErrCareRequestFieldDisabled
	ErrCareRequestBookingsAuthoritative = care.ErrCareRequestBookingsAuthoritative
)

func (s *service) GetChildTodayStatus(ctx context.Context, accountID, studentID int64) (*TodayStatus, error) {
	return s.care.GetChildTodayStatus(ctx, accountID, studentID)
}

func (s *service) GetChildCareSchedule(ctx context.Context, accountID, studentID int64) (*ChildCareSchedule, error) {
	return s.care.GetChildCareSchedule(ctx, accountID, studentID)
}

func (s *service) CreateCareScheduleRequest(ctx context.Context, accountID, studentID int64, payload map[string]any) (*ChildCareSchedule, error) {
	return s.care.CreateCareScheduleRequest(ctx, accountID, studentID, payload)
}

func (s *service) EditCareScheduleRequest(
	ctx context.Context, accountID, studentID, requestID int64,
	payload map[string]any, expectedVersion string,
) (*ChildCareSchedule, error) {
	return s.care.EditCareScheduleRequest(ctx, accountID, studentID, requestID, payload, expectedVersion)
}

func (s *service) GetChildCareOfferings(ctx context.Context, accountID, studentID int64) (*ChildCareOfferings, error) {
	return s.care.GetChildCareOfferings(ctx, accountID, studentID)
}

func (s *service) GetChildOfferingCatalog(ctx context.Context, accountID, studentID int64) (*enrollmentSvc.OfferingChangeCatalog, error) {
	return s.care.GetChildOfferingCatalog(ctx, accountID, studentID)
}

func (s *service) GetChildOfferingCatalogAt(
	ctx context.Context, accountID, studentID int64, effectiveFrom timezone.Date,
) (*enrollmentSvc.OfferingChangeCatalog, error) {
	return s.care.GetChildOfferingCatalogAt(ctx, accountID, studentID, effectiveFrom)
}

func (s *service) CreateOfferingChangeRequest(
	ctx context.Context,
	accountID, studentID int64,
	selections []enrollmentSvc.OfferingChangeSelection,
	effectiveFrom timezone.Date,
	note string,
	completeWithdrawalConfirmed bool,
	recipientGuardianProfileIDs []int64,
) (*ChildCareOfferings, error) {
	return s.care.CreateOfferingChangeRequest(
		ctx, accountID, studentID, selections, effectiveFrom, note, completeWithdrawalConfirmed, recipientGuardianProfileIDs,
	)
}

func (s *service) EditOfferingChangeRequest(
	ctx context.Context,
	accountID, studentID, requestID int64,
	selections []enrollmentSvc.OfferingChangeSelection,
	effectiveFrom timezone.Date,
	note string,
	completeWithdrawalConfirmed bool,
	expectedVersion string,
) (*ChildCareOfferings, error) {
	return s.care.EditOfferingChangeRequest(
		ctx, accountID, studentID, requestID, selections, effectiveFrom, note, completeWithdrawalConfirmed, expectedVersion,
	)
}

func (s *service) GetChildCourses(ctx context.Context, accountID, studentID int64) (*enrollmentSvc.CourseCatalog, error) {
	return s.care.GetChildCourses(ctx, accountID, studentID)
}

func (s *service) RequestChildCourse(
	ctx context.Context, accountID, studentID, offeringID int64, note string,
) (*enrollmentSvc.CourseCatalog, error) {
	return s.care.RequestChildCourse(ctx, accountID, studentID, offeringID, note)
}

func (s *service) WithdrawChildCourseRequest(
	ctx context.Context, accountID, studentID, requestID int64,
) (*enrollmentSvc.CourseCatalog, error) {
	return s.care.WithdrawChildCourseRequest(ctx, accountID, studentID, requestID)
}
