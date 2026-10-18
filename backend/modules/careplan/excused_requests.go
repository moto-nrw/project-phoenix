package careplan

import (
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/excusedrequests"
)

// The student excused-absence request workflow (#3093) is declared in the
// excusedrequests contract package so consumers outside Care Plan can name
// it without the whole facade. The facade re-exports the contract under its
// own vocabulary; both spellings are the same types.

// ExcusedAbsenceRequests is the excused-absence request workflow.
type ExcusedAbsenceRequests = excusedrequests.Service

type (
	ExcusedRequestReviewItem  = excusedrequests.ReviewItem
	ExcusedRequestHistoryItem = excusedrequests.HistoryItem
	ExcusedRequestDecideInput = excusedrequests.DecideInput
	ExcusedRequestCreateInput = excusedrequests.SubmitInput
	ExcusedRequestEditInput   = excusedrequests.EditInput
	RequestCursor             = excusedrequests.Cursor
	ExcusedBulkCandidate      = excusedrequests.BulkCandidate
	ExcusedConflictCandidate  = excusedrequests.ConflictCandidate
	ExcusedConflictDecision   = excusedrequests.ConflictDecision
	ExcusedStaffValueWrite    = excusedrequests.StaffValueWrite
)

const (
	StudentStatusDaySick      = excusedrequests.StudentStatusDaySick
	StudentStatusDayExcused   = excusedrequests.StudentStatusDayExcused
	StudentStatusDayClassTrip = excusedrequests.StudentStatusDayClassTrip
	StudentStatusDayPresent   = excusedrequests.StudentStatusDayPresent
	StudentStatusSourceManual = excusedrequests.StudentStatusSourceManual
	StudentStatusSourceParent = excusedrequests.StudentStatusSourceParent

	ExcusedRequestStatusPending   = excusedrequests.StatusPending
	ExcusedRequestStatusApproved  = excusedrequests.StatusApproved
	ExcusedRequestStatusRejected  = excusedrequests.StatusRejected
	ExcusedRequestStatusWithdrawn = excusedrequests.StatusWithdrawn
	ExcusedRequestStatusDone      = excusedrequests.StatusDone
	ExcusedRequestStatusCareEnded = excusedrequests.StatusCareEnded

	BulkIneligiblePast             = excusedrequests.BulkIneligiblePast
	BulkIneligibleStale            = excusedrequests.BulkIneligibleStale
	BulkIneligibleConflict         = excusedrequests.BulkIneligibleConflict
	BulkIneligibleChildUnavailable = excusedrequests.BulkIneligibleChildUnavailable
	BulkIneligibleAccessRevoked    = excusedrequests.BulkIneligibleAccessRevoked

	ParentRequestTypeExcusedAbsence = excusedrequests.ParentRequestTypeExcusedAbsence
	ParentRequestEventSubmitted     = excusedrequests.ParentRequestEventSubmitted
	ParentRequestEventGuardianEdit  = excusedrequests.ParentRequestEventGuardianEdit
	ParentRequestEventDecided       = excusedrequests.ParentRequestEventDecided
	ParentRequestEventMarkedDone    = excusedrequests.ParentRequestEventMarkedDone
	ParentRequestEventCorrected     = excusedrequests.ParentRequestEventCorrected

	ParentMessageEventRequestCreated = excusedrequests.ParentMessageEventRequestCreated
	ParentMessageEventRequestStatus  = excusedrequests.ParentMessageEventRequestStatus
	ParentMessageActorGuardian       = excusedrequests.ParentMessageActorGuardian
	ParentMessageActorStaff          = excusedrequests.ParentMessageActorStaff
	ParentMessageRequestStatusOpen   = excusedrequests.ParentMessageRequestStatusOpen
	ParentMessageRequestStatusDone   = excusedrequests.ParentMessageRequestStatusDone
	ParentMessageRequestStatusReject = excusedrequests.ParentMessageRequestStatusReject
	ParentMessageRequestExcused      = excusedrequests.ParentMessageRequestExcused
	ParentMessageRequestSick         = excusedrequests.ParentMessageRequestSick
)

// PendingExcusedRequestStatuses are the statuses of the excused requests
// still waiting for a decision.
func PendingExcusedRequestStatuses() []string {
	return []string{ExcusedRequestStatusPending}
}

// DecidedExcusedRequestStatuses are the statuses of the decided excused
// requests a review history lists: approved, rejected and withdrawn.
func DecidedExcusedRequestStatuses() []string {
	return []string{ExcusedRequestStatusApproved, ExcusedRequestStatusRejected, ExcusedRequestStatusWithdrawn}
}

// ApprovedAbsenceLiveFlags is what an approved excused request that covers
// today does to the child's live flags, mirroring the direct parent path.
type ApprovedAbsenceLiveFlags struct {
	// Sick is the live sick flag after the approval. Its start is the
	// approval time when the flag is raised and cleared otherwise.
	Sick bool
	// ClearExcused clears the live excused flag and its start.
	ClearExcused bool
}

// LiveFlagsForApprovedAbsence decides the live flags for an approved absence
// of the given status: a sick day raises the live sick flag and clears the
// excused flag; any other absence clears the live sick flag and leaves the
// excused flag alone.
func LiveFlagsForApprovedAbsence(status string) ApprovedAbsenceLiveFlags {
	sick := status == StudentStatusDaySick
	return ApprovedAbsenceLiveFlags{Sick: sick, ClearExcused: sick}
}

var (
	ErrExcusedRequestForbidden             = excusedrequests.ErrExcusedRequestForbidden
	ErrExcusedRequestGuardianAccessRevoked = excusedrequests.ErrExcusedRequestGuardianAccessRevoked
	ErrExcusedRequestOverlap               = excusedrequests.ErrExcusedRequestOverlap
	ErrExcusedRequestStatusConflict        = excusedrequests.ErrExcusedRequestStatusConflict
	ErrExcusedRequestNoDates               = excusedrequests.ErrExcusedRequestNoDates
	ErrExcusedRequestEmptyNote             = excusedrequests.ErrExcusedRequestEmptyNote
	ErrExcusedRequestNoteTooLong           = excusedrequests.ErrExcusedRequestNoteTooLong
	ErrAbsenceRequestInvalidStatus         = excusedrequests.ErrAbsenceRequestInvalidStatus
	ErrExcusedRequestRejectReasonRequired  = excusedrequests.ErrExcusedRequestRejectReasonRequired
	ErrExcusedRequestRejectReasonTooLong   = excusedrequests.ErrExcusedRequestRejectReasonTooLong
	ErrParentRequestStale                  = excusedrequests.ErrParentRequestStale
	ErrParentRequestReasonRequired         = excusedrequests.ErrParentRequestReasonRequired
	ErrParentRequestPast                   = excusedrequests.ErrParentRequestPast
	ErrParentRequestNotPast                = excusedrequests.ErrParentRequestNotPast
	ErrParentRequestNotDecided             = excusedrequests.ErrParentRequestNotDecided
	ErrParentRequestDecisionRace           = excusedrequests.ErrParentRequestDecisionRace
	ErrParentRequestCorrectionUnsupported  = excusedrequests.ErrParentRequestCorrectionUnsupported
	ErrStaffValueUnsupported               = excusedrequests.ErrStaffValueUnsupported
)

// StudentStatusDayStatusesExcept lists the persisted statuses other than the
// given one.
func StudentStatusDayStatusesExcept(status string) []string {
	return excusedrequests.StudentStatusDayStatusesExcept(status)
}

// ParentRequestVersion is the optimistic-concurrency token of one request row.
func ParentRequestVersion(updatedAt time.Time) string {
	return excusedrequests.ParentRequestVersion(updatedAt)
}
