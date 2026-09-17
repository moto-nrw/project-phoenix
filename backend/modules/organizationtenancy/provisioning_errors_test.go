package organizationtenancy_test

import (
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/stretchr/testify/assert"
)

func TestProvisioningErrorMessages(t *testing.T) {
	t.Parallel()

	cause := errors.New("name is required")
	cases := []struct {
		err  error
		want string
	}{
		{&organizationtenancy.InvalidProvisioningDataError{Err: cause}, "invalid data: name is required"},
		{&organizationtenancy.ProvisioningConflictError{Err: cause}, "conflict: name is required"},
		{&organizationtenancy.OrganizationNotFoundError{OrganizationID: 333}, "organization with ID 333 not found"},
		{&organizationtenancy.OrganizationAlreadyDeletedError{OrganizationID: 101}, "organization with ID 101 is already soft-deleted"},
		{&organizationtenancy.OrganizationNotDeletedError{OrganizationID: 202}, "organization with ID 202 is not soft-deleted"},
		{&organizationtenancy.OrganizationDeletedError{OrganizationID: 404}, "organization with ID 404 is soft-deleted and cannot host schools"},
		{&organizationtenancy.SchoolNotFoundError{SchoolID: 444}, "school with ID 444 not found"},
		{&organizationtenancy.SchoolInactiveError{SchoolID: 99}, "school with ID 99 is inactive"},
		{&organizationtenancy.SchoolAlreadyDeletedError{SchoolID: 88}, "school with ID 88 is already soft-deleted"},
		{&organizationtenancy.SchoolNotDeletedError{SchoolID: 77}, "school with ID 77 is not soft-deleted"},
		{&organizationtenancy.OperatorDeviceNotFoundError{DeviceID: 555}, "device with ID 555 not found"},
		{&organizationtenancy.DeviceInUseError{DeviceID: 66}, "device with ID 66 is still in use and cannot be deleted"},
		{&organizationtenancy.DeviceProtectedError{DeviceID: 55, Reason: "web-manual device"}, "device with ID 55 is protected: web-manual device"},
		{&organizationtenancy.DeviceTransferProtectedError{DeviceID: 11, Reason: "protected"}, "device with ID 11 cannot be transferred: protected"},
		{
			&organizationtenancy.DeviceTransferBlockedError{DeviceID: 12, Reason: organizationtenancy.DeviceTransferBlockedOnline},
			"device with ID 12 cannot be transferred: device_online",
		},
		{
			&organizationtenancy.DeviceTransferOrganizationMismatchError{SourceSchoolID: 13, TargetSchoolID: 14},
			"schools 13 and 14 belong to different organizations",
		},
		{&organizationtenancy.DeviceTransferSameSchoolError{SchoolID: 15}, "device already belongs to school 15"},
		{&organizationtenancy.PersonNotFoundError{PersonID: 42}, "person with ID 42 not found"},
		{
			&organizationtenancy.PersonHasActiveSupervisionsError{PersonID: 42, Count: 3},
			"person with ID 42 has 3 active supervision(s) and cannot be deleted",
		},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, tc.err.Error())
	}
}

func TestProvisioningWrappersUnwrap(t *testing.T) {
	t.Parallel()

	cause := errors.New("cause")
	assert.ErrorIs(t, &organizationtenancy.InvalidProvisioningDataError{Err: cause}, cause)
	assert.ErrorIs(t, &organizationtenancy.ProvisioningConflictError{Err: cause}, cause)
}
