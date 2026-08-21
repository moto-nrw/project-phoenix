package enrollment

import (
	"context"
	"errors"
	"testing"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type formCapabilitySettingsStub struct {
	values map[string]bool
	errFor map[string]error
}

func (s formCapabilitySettingsStub) HasTenantOverride(context.Context, string) (bool, error) {
	return false, nil
}
func (s formCapabilitySettingsStub) ResolveBool(_ context.Context, key string) (bool, error) {
	return s.values[key], s.errFor[key]
}
func (s formCapabilitySettingsStub) ResolveString(context.Context, string) (string, error) {
	return "", nil
}
func (s formCapabilitySettingsStub) ResolveInt(context.Context, string) (int, error) {
	return 0, nil
}

func TestFormCapabilities_GradeDisableMakesSchoolClassIneffective(t *testing.T) {
	t.Parallel()

	svc := &requestService{RequestServiceConfig: RequestServiceConfig{Settings: formCapabilitySettingsStub{
		values: map[string]bool{
			configModel.KeyEnrollmentCollectGradeLevel:    false,
			configModel.KeyEnrollmentCollectSchoolClass:   true,
			configModel.KeyEnrollmentCareOfferingsEnabled: true,
		},
		errFor: map[string]error{},
	}}}

	capabilities, err := svc.FormCapabilities(context.Background())

	require.NoError(t, err)
	assert.False(t, capabilities.CollectGradeLevel)
	assert.False(t, capabilities.CollectSchoolClass)
	assert.True(t, capabilities.CareOfferingsEnabled)
}

func TestFormCapabilities_ResolutionFailureIsReturned(t *testing.T) {
	t.Parallel()

	svc := &requestService{RequestServiceConfig: RequestServiceConfig{Settings: formCapabilitySettingsStub{
		values: map[string]bool{},
		errFor: map[string]error{configModel.KeyEnrollmentCareOfferingsEnabled: errors.New("repository unavailable")},
	}}}

	_, err := svc.FormCapabilities(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), configModel.KeyEnrollmentCareOfferingsEnabled)
}

func TestNormalizeSubmissionForCapabilities_StripsGradeAndClass(t *testing.T) {
	t.Parallel()

	grade := int16(3)
	class := "3a"
	req := SubmitRequest{Children: []SubmitChild{{TargetGradeLevel: &grade, TargetSchoolClass: &class}}}

	err := normalizeSubmissionForCapabilities(&req, FormCapabilities{CollectGradeLevel: false, CareOfferingsEnabled: true})

	require.NoError(t, err)
	assert.Nil(t, req.Children[0].TargetGradeLevel)
	assert.Nil(t, req.Children[0].TargetSchoolClass)
}

func TestNormalizeSubmissionForCapabilities_RejectsForgedOfferings(t *testing.T) {
	t.Parallel()

	req := SubmitRequest{Children: []SubmitChild{{OfferingIDs: []int64{42}}}}

	err := normalizeSubmissionForCapabilities(&req, FormCapabilities{CollectGradeLevel: true, CareOfferingsEnabled: false})

	assert.ErrorIs(t, err, ErrCareOfferingsDisabled)
}

func TestAllChildrenParentResolved(t *testing.T) {
	t.Parallel()

	resolved := []*enrollmentModels.RequestChild{
		{Status: enrollmentModels.ChildStatusApproved},
		{Status: enrollmentModels.ChildStatusRejected},
		{Status: enrollmentModels.ChildStatusWaitlisted},
		{Status: enrollmentModels.ChildStatusWithdrawn},
	}
	assert.True(t, allChildrenParentResolved(resolved))
	resolved[1].Status = enrollmentModels.ChildStatusUnderReview
	assert.False(t, allChildrenParentResolved(resolved))
	assert.False(t, allChildrenParentResolved(nil))
}
