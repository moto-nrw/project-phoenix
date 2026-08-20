package enrollment

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

type availabilitySettingsStub struct {
	gradeMax int
	err      error
}

func (s availabilitySettingsStub) ResolveInt(context.Context, string) (int, error) {
	return s.gradeMax, s.err
}

func TestValidateAvailabilityRuleUsesTenantGradeRange(t *testing.T) {
	t.Parallel()

	service := &careOfferingService{CareOfferingServiceConfig: CareOfferingServiceConfig{
		Settings: availabilitySettingsStub{gradeMax: 4},
	}}
	offering := &enrollmentModels.CareOffering{
		AvailabilityRule: testGradeAvailabilityRule(enrollmentModels.AvailabilityOperatorIn, 2, 5),
	}
	err := service.validateAvailabilityRule(context.Background(), offering)
	require.ErrorIs(t, err, ErrCareOfferingInvalid)
	require.ErrorContains(t, err, "condition 1")
	require.ErrorContains(t, err, "range 1-4")
}

func TestEffectiveFormCapabilitiesRequiresGradeForConditionalCatalog(t *testing.T) {
	t.Parallel()

	offering := &enrollmentModels.CareOffering{
		AvailabilityRule: testGradeAvailabilityRule(enrollmentModels.AvailabilityOperatorIn, 1, 2),
	}
	got := EffectiveFormCapabilities(FormCapabilities{CareOfferingsEnabled: true}, []*enrollmentModels.CareOffering{offering})
	require.True(t, got.CollectGradeLevel)

	got = EffectiveFormCapabilities(FormCapabilities{CareOfferingsEnabled: false}, []*enrollmentModels.CareOffering{offering})
	require.False(t, got.CollectGradeLevel)
}
