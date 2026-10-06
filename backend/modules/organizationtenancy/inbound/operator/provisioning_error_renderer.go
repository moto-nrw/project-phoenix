package operator

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

// ProvisioningErrorRenderer maps provisioning errors to HTTP responses. Each
// outcome carries its registered code (#2519).
func ProvisioningErrorRenderer(err error) render.Renderer {
	var invalidProvisioningData *organizationtenancy.InvalidProvisioningDataError
	var provisioningConflict *organizationtenancy.ProvisioningConflictError
	var organizationNotFound *organizationtenancy.OrganizationNotFoundError
	var organizationAlreadyDeleted *organizationtenancy.OrganizationAlreadyDeletedError
	var organizationNotDeleted *organizationtenancy.OrganizationNotDeletedError
	var organizationHasSchools *organizationtenancy.OrganizationHasSchoolsError
	var organizationDeleted *organizationtenancy.OrganizationDeletedError
	var schoolNotFound *organizationtenancy.SchoolNotFoundError
	var schoolInactive *organizationtenancy.SchoolInactiveError
	var schoolAlreadyDeleted *organizationtenancy.SchoolAlreadyDeletedError
	var schoolNotDeleted *organizationtenancy.SchoolNotDeletedError
	var personNotFound *organizationtenancy.PersonNotFoundError
	var personActiveSupervisors *organizationtenancy.PersonHasActiveSupervisionsError
	var identityErr *organizationtenancy.ProvisioningIdentityError

	if errors.As(err, &identityErr) && identityErr.Err != nil {
		return provisioningIdentityRenderer(identityErr)
	}
	if renderer, ok := deviceErrorRenderer(err); ok {
		return renderer
	}

	switch {
	case errors.As(err, &invalidProvisioningData):
		// The cause may carry adapter text; only its field errors go along.
		return common.OperatorInvalidInput(invalidProvisioningData.Err, "invalid input data")
	case errors.As(err, &provisioningConflict):
		return provisioningConflictRenderer(provisioningConflict)
	case errors.As(err, &organizationNotFound):
		return common.OperatorRejection(http.StatusNotFound, common.CodeProvisioningOrganizationNotFound, "Organization not found")
	case errors.As(err, &organizationAlreadyDeleted):
		return common.OperatorRejection(http.StatusConflict, common.CodeProvisioningOrganizationAlreadyDeleted, "Organization is already deleted")
	case errors.As(err, &organizationNotDeleted):
		return common.OperatorRejection(http.StatusConflict, common.CodeProvisioningOrganizationNotDeleted, "Organization is not deleted")
	case errors.As(err, &organizationHasSchools):
		return common.OperatorRejectionWithDetails(http.StatusConflict, common.CodeProvisioningOrganizationHasSchools,
			fmt.Sprintf("Organization has %d existing school(s) and cannot be deleted. Delete all schools first.", organizationHasSchools.SchoolCount),
			map[string]any{"school_count": organizationHasSchools.SchoolCount})
	case errors.As(err, &organizationDeleted):
		return common.OperatorRejection(http.StatusConflict, common.CodeProvisioningOrganizationDeleted, "Organization is deleted and cannot host schools")
	case errors.As(err, &schoolNotFound):
		return common.OperatorRejection(http.StatusNotFound, common.CodeProvisioningSchoolNotFound, "School not found")
	case errors.As(err, &schoolInactive):
		return common.OperatorRejection(http.StatusForbidden, common.CodeProvisioningSchoolInactive, "School is inactive")
	case errors.As(err, &schoolAlreadyDeleted):
		return common.OperatorRejection(http.StatusConflict, common.CodeProvisioningSchoolAlreadyDeleted, "School is already deleted")
	case errors.As(err, &schoolNotDeleted):
		return common.OperatorRejection(http.StatusConflict, common.CodeProvisioningSchoolNotDeleted, "School is not deleted")
	case errors.As(err, &personNotFound):
		return common.OperatorRejection(http.StatusNotFound, common.CodeProvisioningPersonNotFound, "Person not found")
	case errors.As(err, &personActiveSupervisors):
		return common.OperatorRejection(http.StatusConflict, common.CodeProvisioningPersonHasSupervisions, "Person has active supervisions and cannot be deleted")
	default:
		return common.OperatorInternal(internalErrorMessage)
	}
}

// provisioningIdentityRenderer maps the account and invitation outcomes the
// provisioning writes surface from Identity & Access.
func provisioningIdentityRenderer(identityErr *organizationtenancy.ProvisioningIdentityError) render.Renderer {
	switch {
	case errors.Is(identityErr.Err, organizationtenancy.ErrAccountEmailExists):
		return common.OperatorRejection(http.StatusConflict, common.CodeIdentityEmailAlreadyExists, organizationtenancy.ErrAccountEmailExists.Error())
	case errors.Is(identityErr.Err, organizationtenancy.ErrAccountUsernameExists):
		return common.OperatorRejection(http.StatusConflict, common.CodeIdentityUsernameTaken, organizationtenancy.ErrAccountUsernameExists.Error())
	case errors.Is(identityErr.Err, organizationtenancy.ErrPasswordMismatch):
		return common.OperatorInvalidField(common.CodeIdentityPasswordMismatch, "confirm_password", identityErr.Err.Error())
	case errors.Is(identityErr.Err, organizationtenancy.ErrPasswordTooWeak):
		return common.OperatorInvalidField(common.CodeIdentityPasswordTooWeak, "password", identityErr.Err.Error())
	case errors.Is(identityErr.Err, organizationtenancy.ErrInvitationNameRequired),
		identityErr.Op == organizationtenancy.OpCreateInvitation && !errors.Is(identityErr.Err, organizationtenancy.ErrIdentityStoreFailed):
		return common.OperatorInvalidRequest(identityErr.Err)
	default:
		return common.OperatorInternal(internalErrorMessage)
	}
}

// deviceErrorRenderer maps the device outcomes; ok is false for any other
// error.
func deviceErrorRenderer(err error) (render.Renderer, bool) {
	var operatorDeviceNotFound *organizationtenancy.OperatorDeviceNotFoundError
	var deviceInUse *organizationtenancy.DeviceInUseError
	var deviceProtected *organizationtenancy.DeviceProtectedError
	var deviceTransferProtected *organizationtenancy.DeviceTransferProtectedError
	var deviceTransferBlocked *organizationtenancy.DeviceTransferBlockedError
	var deviceTransferOrganizationMismatch *organizationtenancy.DeviceTransferOrganizationMismatchError
	var deviceTransferSameSchool *organizationtenancy.DeviceTransferSameSchoolError

	switch {
	case errors.As(err, &operatorDeviceNotFound):
		return common.OperatorRejection(http.StatusNotFound, common.CodeProvisioningDeviceNotFound, "Device not found"), true
	case errors.As(err, &deviceInUse):
		return common.OperatorRejection(http.StatusConflict, common.CodeProvisioningDeviceInUse, "Device is still referenced by attendance or session records and cannot be deleted"), true
	case errors.As(err, &deviceProtected):
		return common.OperatorRejection(http.StatusForbidden, common.CodeProvisioningDeviceProtected, "This system device cannot be deleted"), true
	case errors.As(err, &deviceTransferProtected):
		return common.OperatorRejection(http.StatusForbidden, common.CodeProvisioningDeviceTransferProtected, "This system device cannot be transferred"), true
	case errors.As(err, &deviceTransferBlocked):
		switch deviceTransferBlocked.Reason {
		case organizationtenancy.DeviceTransferBlockedOnline:
			return common.OperatorRejection(http.StatusConflict, common.CodeProvisioningDeviceOnline, "Device is online and cannot be transferred"), true
		case organizationtenancy.DeviceTransferBlockedActiveSession:
			return common.OperatorRejection(http.StatusConflict, common.CodeProvisioningDeviceActiveSession, "Device has an active session and cannot be transferred"), true
		default:
			return common.OperatorInternal(internalErrorMessage), true
		}
	case errors.As(err, &deviceTransferOrganizationMismatch):
		return common.OperatorRejection(http.StatusForbidden, common.CodeProvisioningDeviceOtherOrganization, "Device can only be transferred within its organization"), true
	case errors.As(err, &deviceTransferSameSchool):
		return common.OperatorRejection(http.StatusConflict, common.CodeProvisioningDeviceSameSchool, "Device already belongs to the target school"), true
	default:
		return nil, false
	}
}

// provisioningConflictRenderer answers a collision with the code of the
// colliding value and marks the request field that holds it.
func provisioningConflictRenderer(conflict *organizationtenancy.ProvisioningConflictError) render.Renderer {
	type taken struct{ code, field string }
	outcomes := map[organizationtenancy.ProvisioningConflictKind]taken{
		organizationtenancy.ConflictOrganizationSlug: {common.CodeProvisioningOrganizationSlugTaken, "slug"},
		organizationtenancy.ConflictSchoolSubdomain:  {common.CodeProvisioningSchoolSubdomainTaken, "subdomain"},
		organizationtenancy.ConflictSchoolSlug:       {common.CodeProvisioningSchoolSlugTaken, "slug"},
		organizationtenancy.ConflictDeviceAPIKey:     {common.CodeProvisioningDeviceApiKeyTaken, "api_key"},
		organizationtenancy.ConflictDeviceID:         {common.CodeProvisioningDeviceIdTaken, "device_id"},
	}
	message := conflict.Error()
	if conflict.Err != nil {
		message = conflict.Err.Error()
	}
	if outcome, ok := outcomes[conflict.Kind]; ok {
		return common.OperatorRejectionOnField(http.StatusConflict, outcome.code, outcome.field, message)
	}
	return common.OperatorConflict(message)
}

func caregiverCapabilityProvisioningErrorRenderer(err error) render.Renderer {
	if blocked, ok := errors.AsType[caregiverCapabilityBlocked](err); ok {
		resp := common.NewCaregiverCapabilityBlockedResponse(
			http.StatusConflict,
			blocked.Error(),
			blocked.CaregiverCapabilityBlockers(),
		)
		resp.Code = common.CodeProvisioningCaregiverCapabilityBlocked
		return resp
	}
	if _, missing := errors.AsType[caregiverAccountMissing](err); missing || errors.Is(err, organizationtenancy.ErrAccountNotFound) {
		return common.OperatorRejection(http.StatusNotFound, common.CodeIdentityAccountNotFound, "Account not found")
	}
	if invalidProvisioningData, ok := errors.AsType[*organizationtenancy.InvalidProvisioningDataError](err); ok {
		return common.OperatorInvalidRequest(invalidProvisioningData.Err)
	}
	if invalid, ok := errors.AsType[caregiverRequestInvalid](err); ok {
		return common.OperatorInvalidRequest(errors.Unwrap(invalid.CaregiverRequestInvalid()))
	}
	return ProvisioningErrorRenderer(err)
}
