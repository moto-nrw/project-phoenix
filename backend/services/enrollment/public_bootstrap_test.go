package enrollment_test

import (
	"errors"
	"testing"
	"time"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestService_LoadPublicFormBootstrap_AssemblesPhaseSchemaCapabilities(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()

	ctx := testpkg.TenantContext(1)
	data, err := env.svc.LoadPublicFormBootstrap(ctx, env.phaseID, time.Now(), "")
	require.NoError(t, err)
	require.NotNil(t, data)

	require.NotNil(t, data.Phase)
	assert.Equal(t, env.phaseID, data.Phase.ID)
	assert.True(t, data.Capabilities.CareOfferingsEnabled)
	// No offerings were seeded, so the assembled catalog is empty.
	assert.Empty(t, data.Offerings)
}

func TestRequestService_LoadPublicFormBootstrap_CapabilityFailureIsStageError(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()

	env.settings.boolErrors[configModel.KeyEnrollmentCollectGradeLevel] = errors.New("boom")

	ctx := testpkg.TenantContext(1)
	_, err := env.svc.LoadPublicFormBootstrap(ctx, env.phaseID, time.Now(), "")
	require.Error(t, err)

	var stageErr *enrollmentService.BootstrapStageError
	require.True(t, errors.As(err, &stageErr))
	assert.Equal(t, enrollmentService.BootstrapStageCapabilities, stageErr.Stage)
}

func TestRequestService_LoadManualEnrollmentBootstrap_CapabilityFailureIsRaw(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()

	env.settings.boolErrors[configModel.KeyEnrollmentCollectGradeLevel] = errors.New("boom")

	ctx := testpkg.TenantContext(1)
	_, err := env.svc.LoadManualEnrollmentBootstrap(ctx, env.phaseID)
	require.Error(t, err)

	// The admin flow does not wrap stage errors — the raw error surfaces so
	// the handler's own switch maps it to a 500.
	var stageErr *enrollmentService.BootstrapStageError
	assert.False(t, errors.As(err, &stageErr))
}
