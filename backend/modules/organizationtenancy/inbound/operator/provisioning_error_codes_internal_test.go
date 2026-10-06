package operator

import (
	"errors"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #2519: every provisioning outcome the operator portal shows names its
// reason by a registered code, so the portal never reads the text.
func TestProvisioningErrorRendererAnswersWithCodes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"email exists", &organizationtenancy.ProvisioningIdentityError{Err: organizationtenancy.ErrAccountEmailExists}, http.StatusConflict, common.CodeIdentityEmailAlreadyExists},
		{"username exists", &organizationtenancy.ProvisioningIdentityError{Err: organizationtenancy.ErrAccountUsernameExists}, http.StatusConflict, common.CodeIdentityUsernameTaken},
		{"password mismatch", &organizationtenancy.ProvisioningIdentityError{Err: organizationtenancy.ErrPasswordMismatch}, http.StatusBadRequest, common.CodeIdentityPasswordMismatch},
		{"password too weak", &organizationtenancy.ProvisioningIdentityError{Err: organizationtenancy.ErrPasswordTooWeak}, http.StatusBadRequest, common.CodeIdentityPasswordTooWeak},
		{"organization slug", &organizationtenancy.ProvisioningConflictError{Kind: organizationtenancy.ConflictOrganizationSlug, Err: errors.New("taken")}, http.StatusConflict, common.CodeProvisioningOrganizationSlugTaken},
		{"school subdomain", &organizationtenancy.ProvisioningConflictError{Kind: organizationtenancy.ConflictSchoolSubdomain, Err: errors.New("taken")}, http.StatusConflict, common.CodeProvisioningSchoolSubdomainTaken},
		{"school slug", &organizationtenancy.ProvisioningConflictError{Kind: organizationtenancy.ConflictSchoolSlug, Err: errors.New("taken")}, http.StatusConflict, common.CodeProvisioningSchoolSlugTaken},
		{"device api key", &organizationtenancy.ProvisioningConflictError{Kind: organizationtenancy.ConflictDeviceAPIKey, Err: errors.New("taken")}, http.StatusConflict, common.CodeProvisioningDeviceApiKeyTaken},
		{"device id", &organizationtenancy.ProvisioningConflictError{Kind: organizationtenancy.ConflictDeviceID, Err: errors.New("taken")}, http.StatusConflict, common.CodeProvisioningDeviceIdTaken},
		{"organization missing", &organizationtenancy.OrganizationNotFoundError{}, http.StatusNotFound, common.CodeProvisioningOrganizationNotFound},
		{"organization already deleted", &organizationtenancy.OrganizationAlreadyDeletedError{}, http.StatusConflict, common.CodeProvisioningOrganizationAlreadyDeleted},
		{"organization not deleted", &organizationtenancy.OrganizationNotDeletedError{}, http.StatusConflict, common.CodeProvisioningOrganizationNotDeleted},
		{"organization deleted", &organizationtenancy.OrganizationDeletedError{}, http.StatusConflict, common.CodeProvisioningOrganizationDeleted},
		{"school missing", &organizationtenancy.SchoolNotFoundError{}, http.StatusNotFound, common.CodeProvisioningSchoolNotFound},
		{"school inactive", &organizationtenancy.SchoolInactiveError{}, http.StatusForbidden, common.CodeProvisioningSchoolInactive},
		{"school already deleted", &organizationtenancy.SchoolAlreadyDeletedError{}, http.StatusConflict, common.CodeProvisioningSchoolAlreadyDeleted},
		{"school not deleted", &organizationtenancy.SchoolNotDeletedError{}, http.StatusConflict, common.CodeProvisioningSchoolNotDeleted},
		{"device missing", &organizationtenancy.OperatorDeviceNotFoundError{}, http.StatusNotFound, common.CodeProvisioningDeviceNotFound},
		{"device in use", &organizationtenancy.DeviceInUseError{}, http.StatusConflict, common.CodeProvisioningDeviceInUse},
		{"device protected", &organizationtenancy.DeviceProtectedError{}, http.StatusForbidden, common.CodeProvisioningDeviceProtected},
		{"device transfer protected", &organizationtenancy.DeviceTransferProtectedError{}, http.StatusForbidden, common.CodeProvisioningDeviceTransferProtected},
		{"device online", &organizationtenancy.DeviceTransferBlockedError{Reason: organizationtenancy.DeviceTransferBlockedOnline}, http.StatusConflict, common.CodeProvisioningDeviceOnline},
		{"device session", &organizationtenancy.DeviceTransferBlockedError{Reason: organizationtenancy.DeviceTransferBlockedActiveSession}, http.StatusConflict, common.CodeProvisioningDeviceActiveSession},
		{"device other organization", &organizationtenancy.DeviceTransferOrganizationMismatchError{}, http.StatusForbidden, common.CodeProvisioningDeviceOtherOrganization},
		{"device same school", &organizationtenancy.DeviceTransferSameSchoolError{}, http.StatusConflict, common.CodeProvisioningDeviceSameSchool},
		{"person missing", &organizationtenancy.PersonNotFoundError{}, http.StatusNotFound, common.CodeProvisioningPersonNotFound},
		{"person supervises", &organizationtenancy.PersonHasActiveSupervisionsError{}, http.StatusConflict, common.CodeProvisioningPersonHasSupervisions},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			resp, ok := ProvisioningErrorRenderer(tc.err).(*common.ErrResponse)
			require.True(t, ok)
			assert.Equal(t, tc.status, resp.HTTPStatusCode)
			assert.Equal(t, tc.code, resp.Code)
		})
	}
}

func TestProvisioningErrorRendererCarriesSchoolCount(t *testing.T) {
	t.Parallel()

	resp, ok := ProvisioningErrorRenderer(&organizationtenancy.OrganizationHasSchoolsError{SchoolCount: 3}).(*common.ErrResponse)
	require.True(t, ok)
	assert.Equal(t, http.StatusConflict, resp.HTTPStatusCode)
	assert.Equal(t, common.CodeProvisioningOrganizationHasSchools, resp.Code)
	assert.Equal(t, map[string]any{"school_count": 3}, resp.Details)
}

// The adapter text of invalid data stays out of the answer; its field
// errors still mark the field.
func TestProvisioningErrorRendererHidesInvalidDataText(t *testing.T) {
	t.Parallel()

	resp, ok := ProvisioningErrorRenderer(&organizationtenancy.InvalidProvisioningDataError{Err: errors.New("pq: secret detail")}).(*common.ErrResponse)
	require.True(t, ok)
	assert.Equal(t, http.StatusBadRequest, resp.HTTPStatusCode)
	assert.Equal(t, "invalid input data", resp.ErrorText)
	assert.Empty(t, resp.Errors)
}

func TestCaregiverCapabilityRendererAnswersWithCodes(t *testing.T) {
	t.Parallel()

	blocked, ok := caregiverCapabilityProvisioningErrorRenderer(fakeCaregiverBlockedError{blockers: []string{"group_assignments"}}).(*common.CaregiverCapabilityBlockedResponse)
	require.True(t, ok)
	assert.Equal(t, http.StatusConflict, blocked.HTTPStatusCode)
	assert.Equal(t, common.CodeProvisioningCaregiverCapabilityBlocked, blocked.Code)
	assert.Equal(t, []string{"group_assignments"}, blocked.Blockers)

	missing, ok := caregiverCapabilityProvisioningErrorRenderer(organizationtenancy.ErrAccountNotFound).(*common.ErrResponse)
	require.True(t, ok)
	assert.Equal(t, http.StatusNotFound, missing.HTTPStatusCode)
	assert.Equal(t, common.CodeIdentityAccountNotFound, missing.Code)
}
