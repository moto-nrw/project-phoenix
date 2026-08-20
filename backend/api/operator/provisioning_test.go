package operator

import (
	"errors"
	"testing"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	authSvc "github.com/moto-nrw/project-phoenix/services/auth"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
	"github.com/stretchr/testify/require"
)

func TestProvisioningErrorRendererMapsInvitationValidationErrors(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	renderer := ProvisioningErrorRenderer(&authSvc.AuthError{
		Op:  "create invitation",
		Err: errors.New("invalid email address"),
	})

	resp, ok := renderer.(*ErrResponse)
	require.True(t, ok)
	require.Equal(t, 400, resp.HTTPStatusCode)
	require.Equal(t, "invalid email address", resp.ErrorText)
}

func TestProvisioningErrorRendererMapsInvitationNameRequired(t *testing.T) {
	t.Parallel()

	renderer := ProvisioningErrorRenderer(&authSvc.AuthError{
		Op:  "create invitation",
		Err: authSvc.ErrInvitationNameRequired,
	})

	resp, ok := renderer.(*ErrResponse)
	require.True(t, ok)
	require.Equal(t, 400, resp.HTTPStatusCode)
	require.Equal(t, authSvc.ErrInvitationNameRequired.Error(), resp.ErrorText)
}

func TestProvisioningErrorRendererMapsPasswordMismatch(t *testing.T) {
	t.Parallel()

	renderer := ProvisioningErrorRenderer(&authSvc.AuthError{
		Op:  "accept invitation",
		Err: authSvc.ErrPasswordMismatch,
	})

	resp, ok := renderer.(*ErrResponse)
	require.True(t, ok)
	require.Equal(t, 400, resp.HTTPStatusCode)
	require.Equal(t, authSvc.ErrPasswordMismatch.Error(), resp.ErrorText)
}

func TestProvisioningErrorRendererMapsPasswordTooWeak(t *testing.T) {
	t.Parallel()

	renderer := ProvisioningErrorRenderer(&authSvc.AuthError{
		Op:  "accept invitation",
		Err: authSvc.ErrPasswordTooWeak,
	})

	resp, ok := renderer.(*ErrResponse)
	require.True(t, ok)
	require.Equal(t, 400, resp.HTTPStatusCode)
	require.Equal(t, authSvc.ErrPasswordTooWeak.Error(), resp.ErrorText)
}

func TestProvisioningErrorRendererMapsAuthErrorWithNilErr(t *testing.T) {
	t.Parallel()

	renderer := ProvisioningErrorRenderer(&authSvc.AuthError{
		Op:  "create invitation",
		Err: nil,
	})

	resp, ok := renderer.(*ErrResponse)
	require.True(t, ok)
	// When Err is nil, the authErr condition is false, falls through to generic
	require.Equal(t, 500, resp.HTTPStatusCode)
}

func TestProvisioningErrorRendererMapsAuthErrorDefault(t *testing.T) {
	t.Parallel()

	// An AuthError with a DatabaseError inner error should hit the default branch
	renderer := ProvisioningErrorRenderer(&authSvc.AuthError{
		Op:  "create invitation",
		Err: &modelBase.DatabaseError{Op: "insert", Err: errors.New("db fail")},
	})

	resp, ok := renderer.(*ErrResponse)
	require.True(t, ok)
	require.Equal(t, 500, resp.HTTPStatusCode)
	require.Equal(t, "An error occurred", resp.ErrorText)
}

func TestProvisioningErrorRendererMapsConflictErrors(t *testing.T) {
	t.Parallel()

	renderer := ProvisioningErrorRenderer(&platformSvc.ConflictError{
		Err: errors.New("school subdomain already exists"),
	})

	resp, ok := renderer.(*ErrResponse)
	require.True(t, ok)
	require.Equal(t, 409, resp.HTTPStatusCode)
	require.Equal(t, "school subdomain already exists", resp.ErrorText)
}
