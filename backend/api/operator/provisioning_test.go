package operator

import (
	"errors"
	"testing"

	authSvc "github.com/moto-nrw/project-phoenix/services/auth"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
	"github.com/stretchr/testify/require"
)

func TestProvisioningErrorRendererMapsInvitationValidationErrors(t *testing.T) {
	renderer := ProvisioningErrorRenderer(&authSvc.AuthError{
		Op:  "create invitation",
		Err: authSvc.ErrEmailAlreadyExists,
	})

	resp, ok := renderer.(*ErrResponse)
	require.True(t, ok)
	require.Equal(t, 409, resp.HTTPStatusCode)
	require.Equal(t, authSvc.ErrEmailAlreadyExists.Error(), resp.ErrorText)
}

func TestProvisioningErrorRendererMapsInvalidInvitationInputErrors(t *testing.T) {
	renderer := ProvisioningErrorRenderer(&authSvc.AuthError{
		Op:  "create invitation",
		Err: errors.New("invalid email address"),
	})

	resp, ok := renderer.(*ErrResponse)
	require.True(t, ok)
	require.Equal(t, 400, resp.HTTPStatusCode)
	require.Equal(t, "invalid email address", resp.ErrorText)
}

func TestProvisioningErrorRendererMapsConflictErrors(t *testing.T) {
	renderer := ProvisioningErrorRenderer(&platformSvc.ConflictError{
		Err: errors.New("school subdomain already exists"),
	})

	resp, ok := renderer.(*ErrResponse)
	require.True(t, ok)
	require.Equal(t, 409, resp.HTTPStatusCode)
	require.Equal(t, "school subdomain already exists", resp.ErrorText)
}
