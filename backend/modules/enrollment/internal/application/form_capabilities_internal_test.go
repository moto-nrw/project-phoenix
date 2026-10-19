package application

import (
	"context"
	"errors"
	"testing"

	offeringContract "github.com/moto-nrw/project-phoenix/modules/enrollment/offeringcontract"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
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

	err := normalizeSubmissionForCapabilities(&req, enrollmentNative.FormCapabilities{CollectGradeLevel: false, CareOfferingsEnabled: true})

	require.NoError(t, err)
	assert.Nil(t, req.Children[0].TargetGradeLevel)
	assert.Nil(t, req.Children[0].TargetSchoolClass)
}

// The refusal is Care Plan's shared value; the intake's public boundary marks
// it with Enrollment's value, which the handlers map.
func TestNormalizeSubmissionForCapabilities_RejectsForgedOfferings(t *testing.T) {
	t.Parallel()

	req := SubmitRequest{Children: []SubmitChild{{OfferingIDs: []int64{42}}}}

	err := normalizeSubmissionForCapabilities(&req, enrollmentNative.FormCapabilities{CollectGradeLevel: true, CareOfferingsEnabled: false})

	assert.ErrorIs(t, err, careplan.ErrCareOfferingsDisabled)
	assert.ErrorIs(t, publicError(err), enrollmentNative.ErrCareOfferingsDisabled)
}

func TestAllChildrenParentResolved(t *testing.T) {
	t.Parallel()

	resolved := []*RequestChild{
		{Status: enrollmentNative.ChildStatusApproved},
		{Status: enrollmentNative.ChildStatusRejected},
		{Status: enrollmentNative.ChildStatusWaitlisted},
		{Status: enrollmentNative.ChildStatusWithdrawn},
	}
	assert.True(t, childrenParentResolved(resolved))
	resolved[1].Status = enrollmentNative.ChildStatusUnderReview
	assert.False(t, childrenParentResolved(resolved))
	assert.False(t, childrenParentResolved(nil))
}

func TestEffectiveFormCapabilitiesRequiresGradeForConditionalCatalog(t *testing.T) {
	t.Parallel()

	offering := &offeringContract.CareOfferingRecord{
		AvailabilityRule: testGradeAvailabilityRule(offeringContract.AvailabilityOperatorIn, 1, 2),
	}
	got := effectiveFormCapabilities(enrollmentNative.FormCapabilities{CareOfferingsEnabled: true}, []*offeringContract.CareOfferingRecord{offering})
	require.True(t, got.CollectGradeLevel)

	got = effectiveFormCapabilities(enrollmentNative.FormCapabilities{CareOfferingsEnabled: false}, []*offeringContract.CareOfferingRecord{offering})
	require.False(t, got.CollectGradeLevel)
}
