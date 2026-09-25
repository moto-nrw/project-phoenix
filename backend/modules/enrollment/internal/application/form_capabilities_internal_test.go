package application

import (
	"context"
	"errors"
	"testing"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormCapabilities_GradeDisableMakesSchoolClassIneffective(t *testing.T) {
	t.Parallel()

	svc := NewIntake(IntakeDependencies{Settings: intakeSettingsFake{
		collectGradeLevel:    false,
		collectSchoolClass:   true,
		careOfferingsEnabled: true,
	}})

	capabilities, err := svc.formCapabilities(context.Background())

	require.NoError(t, err)
	assert.False(t, capabilities.CollectGradeLevel)
	assert.False(t, capabilities.CollectSchoolClass)
	assert.True(t, capabilities.CareOfferingsEnabled)
}

func TestFormCapabilities_ResolutionFailureIsReturned(t *testing.T) {
	t.Parallel()

	svc := NewIntake(IntakeDependencies{Settings: intakeSettingsFake{
		errFor: map[string]error{settingCareOfferingsEnabled: errors.New("repository unavailable")},
	}})

	_, err := svc.formCapabilities(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "enrollment.care_offerings_enabled")
}

func TestNormalizeSubmissionForCapabilities_StripsGradeAndClass(t *testing.T) {
	t.Parallel()

	grade := int16(3)
	class := "3a"
	req := SubmitRequest{Children: []SubmitChild{{TargetGradeLevel: &grade, TargetSchoolClass: &class}}}

	err := normalizeSubmissionForCapabilities(&req, enrollment.FormCapabilities{CollectGradeLevel: false, CareOfferingsEnabled: true})

	require.NoError(t, err)
	assert.Nil(t, req.Children[0].TargetGradeLevel)
	assert.Nil(t, req.Children[0].TargetSchoolClass)
}

// The refusal is Care Plan's shared value; the intake's public boundary marks
// it with Enrollment's value, which the handlers map.
func TestNormalizeSubmissionForCapabilities_RejectsForgedOfferings(t *testing.T) {
	t.Parallel()

	req := SubmitRequest{Children: []SubmitChild{{OfferingIDs: []int64{42}}}}

	err := normalizeSubmissionForCapabilities(&req, enrollment.FormCapabilities{CollectGradeLevel: true, CareOfferingsEnabled: false})

	assert.ErrorIs(t, err, careplan.ErrCareOfferingsDisabled)
	assert.ErrorIs(t, publicError(err), enrollment.ErrCareOfferingsDisabled)
}

func TestAllChildrenParentResolved(t *testing.T) {
	t.Parallel()

	resolved := []*RequestChild{
		{Status: enrollmentModels.ChildStatusApproved},
		{Status: enrollmentModels.ChildStatusRejected},
		{Status: enrollmentModels.ChildStatusWaitlisted},
		{Status: enrollmentModels.ChildStatusWithdrawn},
	}
	assert.True(t, childrenParentResolved(resolved))
	resolved[1].Status = enrollmentModels.ChildStatusUnderReview
	assert.False(t, childrenParentResolved(resolved))
	assert.False(t, childrenParentResolved(nil))
}

func TestEffectiveFormCapabilitiesRequiresGradeForConditionalCatalog(t *testing.T) {
	t.Parallel()

	offering := &enrollmentModels.CareOffering{
		AvailabilityRule: testGradeAvailabilityRule(enrollmentModels.AvailabilityOperatorIn, 1, 2),
	}
	got := effectiveFormCapabilities(enrollment.FormCapabilities{CareOfferingsEnabled: true}, []*enrollmentModels.CareOffering{offering})
	require.True(t, got.CollectGradeLevel)

	got = effectiveFormCapabilities(enrollment.FormCapabilities{CareOfferingsEnabled: false}, []*enrollmentModels.CareOffering{offering})
	require.False(t, got.CollectGradeLevel)
}
