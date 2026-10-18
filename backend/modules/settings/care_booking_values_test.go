package settings

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	configModel "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

func TestCareBookingValuesServeTheRegistryDefaultsWithoutAResolver(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	values := NewCareBookingValues(nil)
	enabled, err := values.CareOfferingsEnabled(ctx)
	require.NoError(t, err)
	assert.True(t, enabled)
	authoritative, err := values.BookingsAuthoritative(ctx)
	require.NoError(t, err)
	assert.False(t, authoritative)
}

func TestCareBookingValuesResolveTheTenantSwitches(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	values := NewCareBookingValues(intakeSource{overrides: map[string]any{
		configModel.KeyEnrollmentCareOfferingsEnabled:  false,
		configModel.KeyEnrollmentBookingsAuthoritative: true,
	}})
	enabled, err := values.CareOfferingsEnabled(ctx)
	require.NoError(t, err)
	assert.False(t, enabled)
	authoritative, err := values.BookingsAuthoritative(ctx)
	require.NoError(t, err)
	assert.True(t, authoritative)
}
