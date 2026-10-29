package parent

import (
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

func renderParentCareWriteError(w http.ResponseWriter, r *http.Request, err error) bool {
	switch {
	case errors.Is(err, parentService.ErrChildNotLinked):
		// Don't reveal whether the student exists elsewhere — treat an
		// unowned child as forbidden.
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, common.CodeCareChildNotLinked))
	case errors.Is(err, parentService.ErrGuardianPermissionDenied):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, common.CodeCareGuardianPermissionDenied))
	case errors.Is(err, parentService.ErrChildCareEnded):
		// The child left the OGS. Reading stays open; every submit is refused
		// with its own code so the portal can say why instead of showing a
		// generic "keine Berechtigung" (#2487).
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, common.CodeCareChildCareEnded))
	case errors.Is(err, parentService.ErrSickNoteDisabled):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, common.CodeCareSickNoteDisabled))
	case errors.Is(err, parentService.ErrNotesDisabled):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, common.CodeCareNotesDisabled))
	case errors.Is(err, parentService.ErrLeadershipMessagesUnavailable):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, common.CodeCommunicationLeadershipMessagesUnavailable))
	case errors.Is(err, parentService.ErrInvalidMessageAudience):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	case errors.Is(err, parentService.ErrMealPlanDisabled):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, common.CodeMealsPlanDisabled))
	case errors.Is(err, parentService.ErrMealPlanWeekOutOfRange):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeMealsPlanWeekOutOfRange))
	case errors.Is(err, parentService.ErrMealRegistrationDisabled):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, common.CodeMealsRegistrationDisabled))
	case errors.Is(err, parentService.ErrMealParticipationOutOfRange):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeMealsParticipationOutOfRange))
	case errors.Is(err, parentService.ErrMealParticipationCutoff):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeMealsParticipationCutoffPassed))
	case errors.Is(err, parentService.ErrInvalidMealParticipation):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeMealsInvalidParticipation))
	case errors.Is(err, parentService.ErrPickupChangeDisabled):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, common.CodeCarePickupChangeDisabled))
	case errors.Is(err, parentService.ErrPickupChangeCutoffPassed):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeCarePickupChangeCutoffPassed))
	default:
		return false
	}
	return true
}

func renderParentMasterDataWriteError(w http.ResponseWriter, r *http.Request, err error) bool {
	switch {
	case errors.Is(err, parentService.ErrMasterDataEditDisabled):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, common.CodeCareMasterDataEditDisabled))
	case errors.Is(err, parentService.ErrMasterDataRequestDisabled):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, common.CodeCareMasterDataRequestDisabled))
	case errors.Is(err, parentService.ErrMasterDataFieldNotEditable):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, common.CodeCareMasterDataFieldNotEditable))
	case errors.Is(err, parentService.ErrMasterDataInvalidValue):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeCareMasterDataInvalidValue))
	case errors.Is(err, parentService.ErrMasterDataDuplicatePending):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeCareMasterDataDuplicatePending))
	case errors.Is(err, parentService.ErrMasterDataNoChanges):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeCareMasterDataNoChanges))
	case errors.Is(err, parentService.ErrMasterDataRequestNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeCareRequestNotFound))
	case errors.Is(err, parentService.ErrMasterDataRequestNotPending):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeCareRequestNotOpen))
	case errors.Is(err, parentService.ErrCareExceptionConflict):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeCareCareExceptionConflict))
	case errors.Is(err, parentService.ErrCareExceptionAlreadyLeft):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeCareCareExceptionAlreadyLeft))
	case errors.Is(err, parentService.ErrCareExceptionRaced):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeStudentsCareExceptionRaced))
	case errors.Is(err, parentService.ErrExcusedRequestNotFound):
		common.RenderError(w, r, common.ErrorNotFound(err))
	case errors.Is(err, parentService.ErrExcusedRequestNotPending):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeCareExcusedRequestNotPending))
	case errors.Is(err, parentService.ErrExcusedRequestOverlap):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeCareExcusedRequestOverlap))
	case errors.Is(err, parentService.ErrCareRequestNotPending):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeCareRequestNotOpen))
	case errors.Is(err, parentService.ErrCareRequestNotFound):
		common.RenderError(w, r, common.ErrorNotFound(err))
	case errors.Is(err, parentService.ErrCareRequestAlreadyPending):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeCareCareRequestAlreadyPending))
	case errors.Is(err, parentService.ErrInvalidCareRequestPayload):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeCareInvalidRequestPayload))
	case errors.Is(err, parentService.ErrCareRequestFieldDisabled):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, common.CodeCareCareRequestFieldDisabled))
	case errors.Is(err, parentService.ErrCareRequestBookingsAuthoritative):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, common.CodeCareCareRequestBookingsAuthoritative))
	case errors.Is(err, parentService.ErrNoCareException):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeCareCareExceptionNoTime))
	case errors.Is(err, parentService.ErrCareExceptionReasonRequired):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeCareCareExceptionReasonRequired))
	case errors.Is(err, parentService.ErrCareExceptionReasonTooLong):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeCareCareExceptionReasonTooLong))
	case renderParentCareExceptionDateError(w, r, err):
	default:
		return false
	}
	return true
}

func renderParentGuardianWriteError(w http.ResponseWriter, r *http.Request, err error) bool {
	switch {
	case errors.Is(err, parentService.ErrInviteDisabled):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, common.CodeCareInviteDisabled))
	case errors.Is(err, parentService.ErrRemoveDisabled):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, common.CodeCareRemoveDisabled))
	case errors.Is(err, parentService.ErrCannotRemovePrimaryGuardian):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, common.CodeCarePrimaryGuardianProtected))
	case errors.Is(err, parentService.ErrCannotRemoveStaffManagedGuardian):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, common.CodeCareStaffManagedGuardianProtected))
	case errors.Is(err, parentService.ErrCannotRemovePayerGuardian):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, common.CodeCarePayerGuardianProtected))
	case errors.Is(err, parentService.ErrCannotRemoveOwnAccess):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, common.CodeCareCannotRemoveOwnAccess))
	case errors.Is(err, parentService.ErrInviteSocialWorkerManaged):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, common.CodeCareGuardianSocialWorkerManaged))
	case errors.Is(err, parentService.ErrInviteMailNotQueued):
		common.RenderError(w, r, common.ErrorServiceUnavailableWithCode(err, common.CodeIdentityGuardianInvitationEmailNotQueued))
	case errors.Is(err, parentService.ErrGuardianManagementDisabled):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, common.CodeCareGuardianManagementDisabled))
	case errors.Is(err, parentService.ErrGuardianNotLinked):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, common.CodeCareGuardianNotLinked))
	case errors.Is(err, parentService.ErrGuardianHasOwnAccount):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, common.CodeCareGuardianHasOwnAccount))
	case errors.Is(err, parentService.ErrGuardianSharedAcrossFamilies):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, common.CodeCareGuardianSharedAcrossFamilies))
	case errors.Is(err, parentService.ErrGuardianSocialWorkerManaged):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, common.CodeCareGuardianSocialWorkerManaged))
	case errors.Is(err, parentService.ErrGuardianRoleManaged):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, common.CodeCareGuardianRoleManaged))
	case errors.Is(err, parentService.ErrGuardianEmailConflict):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeCareGuardianEmailConflict))
	case errors.Is(err, parentService.ErrGuardianNoChange):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeCareGuardianNoChange))
	case errors.Is(err, parentService.ErrGuardianContactInvalid):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeCareGuardianContactInvalid))
	case errors.Is(err, parentService.ErrGuardianRelationshipInvalid):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeCareGuardianRelationshipInvalid))
	default:
		return false
	}
	return true
}

func renderParentOfferingWriteError(w http.ResponseWriter, r *http.Request, err error) bool {
	switch {
	case errors.Is(err, careplan.ErrOfferingChangeDisabled):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, common.CodeCareOfferingChangesDisabled))
	case errors.Is(err, careplan.ErrCareOfferingsDisabled):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, common.CodeStudentsCareOfferingsDisabled))
	case errors.Is(err, careplan.ErrOfferingChangeNoEnrollment):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, common.CodeStudentsOfferingChangesNoEnrollment))
	case errors.Is(err, careplan.ErrOfferingChangeForbidden):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, common.CodeCareOfferingChangeForbidden))
	case errors.Is(err, careplan.ErrOfferingChangeCapacityFull):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeStudentsOfferingChangeCapacityFull))
	case errors.Is(err, careplan.ErrCareOfferingDayConflict):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeEnrollmentCareOfferingDayConflict))
	case errors.Is(err, careplan.ErrOfferingChangeInvalid):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeCareOfferingChangeInvalid))
	case errors.Is(err, careplan.ErrCompleteWithdrawalConfirmationRequired):
		common.RenderError(w, r, common.ErrorConflictWithDetails(
			err,
			common.CodeEnrollmentCompleteWithdrawalConfirmationRequired,
			map[string]any{"confirmation": "complete_withdrawal"},
		))
	case errors.Is(err, careplan.ErrOfferingChangeAlreadyPending):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeCareOfferingChangeAlreadyPending))
	case errors.Is(err, careplan.ErrOfferingChangeNotPending):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeCareRequestNotOpen))
	case errors.Is(err, careplan.ErrOfferingChangeNotFound):
		common.RenderError(w, r, common.ErrorNotFound(err))
	default:
		return false
	}
	return true
}

func renderParentAnnouncementWriteError(w http.ResponseWriter, r *http.Request, err error) bool {
	switch {
	case errors.Is(err, parentService.ErrAnnouncementNotFound):
		common.RenderError(w, r, common.ErrorNotFound(err))
	case errors.Is(err, parentService.ErrAnnouncementAckNotRequired):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeCareAnnouncementAckNotRequired))
	case errors.Is(err, parentService.ErrAnnouncementStale):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeCareAnnouncementStale))
	case errors.Is(err, parentService.ErrAnnouncementNotAPoll):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeCareAnnouncementNotAPoll))
	case errors.Is(err, parentService.ErrPollClosed):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeCarePollClosed))
	case errors.Is(err, parentService.ErrInvalidPollResponse):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeCareInvalidPollResponse))
	case errors.Is(err, parentService.ErrChildNotAnswerable), errors.Is(err, parentService.ErrGroupThreadNotFound):
		common.RenderError(w, r, common.ErrorNotFound(err))
	case errors.Is(err, parentService.ErrGroupThreadClosed):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeCommunicationGroupThreadClosed))
	case errors.Is(err, parentService.ErrSurveyNotOpen):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeCommunicationSurveyNotOpen))
	case errors.Is(err, parentService.ErrSurveyAlreadyAnswered):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeCommunicationSurveyAlreadyAnswered))
	case errors.Is(err, parentService.ErrNoDates),
		errors.Is(err, parentService.ErrInvalidStatus),
		errors.Is(err, parentService.ErrEmptyNote),
		errors.Is(err, parentService.ErrNoteTooLong),
		errors.Is(err, parentService.ErrEmailRequired),
		errors.Is(err, parentService.ErrInvalidSurveyAnswers),
		errors.Is(err, parentService.ErrInvalidInviteInput):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	default:
		return false
	}
	return true
}
