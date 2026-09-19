package operator

import (
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/common"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/stretchr/testify/require"
)

func TestProvisioningErrorRendererMapsInvitationValidationErrors(t *testing.T) {
	t.Parallel()

	renderer := ProvisioningErrorRenderer(&organizationtenancy.ProvisioningIdentityError{
		Op:  "create invitation",
		Err: organizationtenancy.ErrAccountEmailExists,
	})

	resp, ok := renderer.(*common.OperatorErrResponse)
	require.True(t, ok)
	require.Equal(t, 409, resp.HTTPStatusCode)
	require.Equal(t, organizationtenancy.ErrAccountEmailExists.Error(), resp.ErrorText)
}

func TestProvisioningErrorRendererMapsInvalidInvitationInputErrors(t *testing.T) {
	t.Parallel()

	renderer := ProvisioningErrorRenderer(&organizationtenancy.ProvisioningIdentityError{
		Op:  "create invitation",
		Err: errors.New("invalid email address"),
	})

	resp, ok := renderer.(*common.OperatorErrResponse)
	require.True(t, ok)
	require.Equal(t, 400, resp.HTTPStatusCode)
	require.Equal(t, "invalid email address", resp.ErrorText)
}

func TestProvisioningErrorRendererMapsInvitationNameRequired(t *testing.T) {
	t.Parallel()

	renderer := ProvisioningErrorRenderer(&organizationtenancy.ProvisioningIdentityError{
		Op:  "create invitation",
		Err: organizationtenancy.ErrInvitationNameRequired,
	})

	resp, ok := renderer.(*common.OperatorErrResponse)
	require.True(t, ok)
	require.Equal(t, 400, resp.HTTPStatusCode)
	require.Equal(t, organizationtenancy.ErrInvitationNameRequired.Error(), resp.ErrorText)
}

func TestProvisioningErrorRendererMapsPasswordMismatch(t *testing.T) {
	t.Parallel()

	renderer := ProvisioningErrorRenderer(&organizationtenancy.ProvisioningIdentityError{
		Op:  "accept invitation",
		Err: organizationtenancy.ErrPasswordMismatch,
	})

	resp, ok := renderer.(*common.OperatorErrResponse)
	require.True(t, ok)
	require.Equal(t, 400, resp.HTTPStatusCode)
	require.Equal(t, organizationtenancy.ErrPasswordMismatch.Error(), resp.ErrorText)
}

func TestProvisioningErrorRendererMapsPasswordTooWeak(t *testing.T) {
	t.Parallel()

	renderer := ProvisioningErrorRenderer(&organizationtenancy.ProvisioningIdentityError{
		Op:  "accept invitation",
		Err: organizationtenancy.ErrPasswordTooWeak,
	})

	resp, ok := renderer.(*common.OperatorErrResponse)
	require.True(t, ok)
	require.Equal(t, 400, resp.HTTPStatusCode)
	require.Equal(t, organizationtenancy.ErrPasswordTooWeak.Error(), resp.ErrorText)
}

func TestProvisioningErrorRendererMapsAuthErrorWithNilErr(t *testing.T) {
	t.Parallel()

	renderer := ProvisioningErrorRenderer(&organizationtenancy.ProvisioningIdentityError{
		Op:  "create invitation",
		Err: nil,
	})

	resp, ok := renderer.(*common.OperatorErrResponse)
	require.True(t, ok)
	// When Err is nil, the authErr condition is false, falls through to generic
	require.Equal(t, 500, resp.HTTPStatusCode)
}

func TestProvisioningErrorRendererMapsAuthErrorDefault(t *testing.T) {
	t.Parallel()

	// An AuthError with a DatabaseError inner error should hit the default branch
	renderer := ProvisioningErrorRenderer(&organizationtenancy.ProvisioningIdentityError{
		Op:  "create invitation",
		Err: &modelBase.DatabaseError{Op: "insert", Err: errors.New("db fail")},
	})

	resp, ok := renderer.(*common.OperatorErrResponse)
	require.True(t, ok)
	require.Equal(t, 500, resp.HTTPStatusCode)
	require.Equal(t, "An error occurred", resp.ErrorText)
}

func TestProvisioningErrorRendererMapsConflictErrors(t *testing.T) {
	t.Parallel()

	renderer := ProvisioningErrorRenderer(&organizationtenancy.ProvisioningConflictError{
		Err: errors.New("school subdomain already exists"),
	})

	resp, ok := renderer.(*common.OperatorErrResponse)
	require.True(t, ok)
	require.Equal(t, 409, resp.HTTPStatusCode)
	require.Equal(t, "school subdomain already exists", resp.ErrorText)
}

func TestProvisioningErrorRendererMapsProvisioningConflictErrors(t *testing.T) {
	t.Parallel()

	renderer := ProvisioningErrorRenderer(&organizationtenancy.ProvisioningConflictError{
		Err: errors.New("school subdomain already exists"),
	})

	resp, ok := renderer.(*common.OperatorErrResponse)
	require.True(t, ok)
	require.Equal(t, 409, resp.HTTPStatusCode)
	require.Equal(t, "school subdomain already exists", resp.ErrorText)
}
