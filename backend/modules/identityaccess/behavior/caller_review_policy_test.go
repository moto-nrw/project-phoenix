package behavior_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/services"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
)

// reviewsFake is a func-field double of the Identity & Access review
// capability the adapter narrows; a nil scopeFn answers "no scope". The
// review decisions themselves are pinned in the Identity & Access
// application tests.
type reviewsFake struct {
	scopeFn   func(context.Context, []string) (bool, []int64, error)
	scopeCall int
}

func (f *reviewsFake) ReviewScope(ctx context.Context, permissions []string) (bool, []int64, error) {
	f.scopeCall++
	if f.scopeFn == nil {
		return false, nil, nil
	}
	return f.scopeFn(ctx, permissions)
}

func (f *reviewsFake) AbsenceReviewScope(context.Context, []string) (bool, []int64, error) {
	return false, nil, nil
}

func (f *reviewsFake) ReviewAccessLevel(context.Context, []string) (string, error) {
	return "none", nil
}

func scopeOf(schoolWide bool, groups []int64, err error) *reviewsFake {
	return &reviewsFake{scopeFn: func(context.Context, []string) (bool, []int64, error) {
		return schoolWide, groups, err
	}}
}

// The permission facts the review policy narrows. The Identity & Access
// review tests start from these facts for the same permission sets.
func TestReviewPermissionsDerivesTheRouteLevelFacts(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		permissions                            []string
		adminWildcard, absenceUnmet, canReview bool
	}{
		{permissions: []string{"admin:*"}, adminWildcard: true, canReview: true},
		{permissions: []string{"*:*"}, adminWildcard: true, canReview: true},
		{permissions: []string{"users:update"}, canReview: true},
		{permissions: []string{"users:absence", "users:read"}, canReview: true},
		{permissions: []string{"users:read"}},
		{permissions: []string{"users:absence"}, absenceUnmet: true},
	} {
		facts := services.ReviewPermissions(tc.permissions)
		assert.Equal(t, tc.adminWildcard, facts.AdminWildcard, "admin wildcard of %v", tc.permissions)
		assert.Equal(t, tc.absenceUnmet, facts.AbsenceReadPrerequisiteUnmet, "absence prerequisite of %v", tc.permissions)
		assert.Equal(t, tc.canReview, facts.CanReviewExcused, "review permission of %v", tc.permissions)
	}
}

// The root binds this sentinel as the review policy's refusal of
// users:absence without users:read (#2267 A4); its text reaches the client.
func TestAbsenceReadRequiredNamesTheMissingPermission(t *testing.T) {
	t.Parallel()

	require.ErrorContains(t, securityruntime.ErrAbsenceReadRequired, "users:read permission is required")
}

func TestParentRequestReviewerPolicyAdminsKeepSchoolWideAccess(t *testing.T) {
	t.Parallel()

	policy := services.NewParentRequestReviewPolicy(scopeOf(true, nil, nil))

	filter, err := policy.StudentFilter(context.Background(), []string{"admin:*"})
	require.NoError(t, err)
	assert.True(t, filter(&userModels.Student{}))
	assert.False(t, filter(nil))
}

func TestParentRequestReviewerPolicyGroupLeadersAreDeniedByDefault(t *testing.T) {
	t.Parallel()

	policy := services.NewParentRequestReviewPolicy(scopeOf(false, nil, nil))

	filter, err := policy.StudentFilter(context.Background(), []string{"users:update"})
	require.NoError(t, err)
	assert.False(t, filter(&userModels.Student{}))
}

func TestParentRequestReviewerPolicyEnabledGroupLeadersOnlyReachTheirGroups(t *testing.T) {
	t.Parallel()

	groupID := int64(71)
	otherGroupID := int64(72)
	policy := services.NewParentRequestReviewPolicy(scopeOf(false, []int64{groupID}, nil))

	filter, err := policy.StudentFilter(context.Background(), []string{"users:update"})
	require.NoError(t, err)
	assert.True(t, filter(&userModels.Student{GroupID: &groupID}))
	assert.False(t, filter(&userModels.Student{GroupID: &otherGroupID}))
	assert.False(t, filter(&userModels.Student{}))
	assert.False(t, filter(nil))

	allowed, err := policy.Allows(context.Background(), []string{"users:update"}, &userModels.Student{GroupID: &groupID})
	require.NoError(t, err)
	assert.True(t, allowed)
	allowed, err = policy.Allows(context.Background(), []string{"users:update"}, &userModels.Student{GroupID: &otherGroupID})
	require.NoError(t, err)
	assert.False(t, allowed)
}

func TestParentRequestReviewerPolicyFailsClosedWhenPolicyCannotBeResolved(t *testing.T) {
	t.Parallel()

	for _, scopeErr := range []error{errors.New("settings unavailable"), errors.New("groups unavailable")} {
		policy := services.NewParentRequestReviewPolicy(scopeOf(false, nil, scopeErr))

		filter, err := policy.StudentFilter(context.Background(), []string{"users:update"})
		require.ErrorIs(t, err, scopeErr)
		assert.Nil(t, filter)

		allowed, err := policy.Allows(context.Background(), []string{"users:update"}, &userModels.Student{})
		require.ErrorIs(t, err, scopeErr)
		assert.False(t, allowed)
	}
}

// #2267 A4: users:absence is a write scope that never unlocks a read surface.
// The refusal the review capability reports reaches the filter and the
// single decision unchanged.
func TestParentRequestReviewerPolicyRefusesAbsenceWithoutRead(t *testing.T) {
	t.Parallel()

	policy := services.NewParentRequestReviewPolicy(scopeOf(false, nil, securityruntime.ErrAbsenceReadRequired))

	_, err := policy.StudentFilter(context.Background(), []string{"users:absence"})
	require.ErrorContains(t, err, "users:read permission is required")

	_, allowErr := policy.Allows(context.Background(), []string{"users:absence"}, &userModels.Student{})
	require.ErrorContains(t, allowErr, "users:read permission is required")
}

// Resolving once per request: one filter, one scope evaluation.
func TestParentRequestReviewerPolicyStudentFilterResolvesTheScopeOnce(t *testing.T) {
	t.Parallel()

	groupID := int64(71)
	reviews := scopeOf(false, []int64{groupID}, nil)
	filter, err := services.NewParentRequestReviewPolicy(reviews).StudentFilter(context.Background(), []string{"users:update"})
	require.NoError(t, err)

	filter(&userModels.Student{GroupID: &groupID})
	filter(&userModels.Student{})
	assert.Equal(t, 1, reviews.scopeCall)
}

func TestParentRequestReviewerPolicyStudentFilterRequiresQueuePermission(t *testing.T) {
	t.Parallel()

	groupID := int64(71)
	// users:read alone reaches no group (pinned in the Identity & Access
	// review tests); the filter then admits no student of the group.
	policy := services.NewParentRequestReviewPolicy(scopeOf(false, []int64{}, nil))

	filter, err := policy.StudentFilter(context.Background(), []string{"users:read"})

	require.NoError(t, err)
	assert.False(t, filter(&userModels.Student{GroupID: &groupID}))
}

func TestParentRequestReviewerPolicyWithoutReviewsIsNotConfigured(t *testing.T) {
	t.Parallel()

	var missing *services.ParentRequestReviewPolicy
	for name, policy := range map[string]*services.ParentRequestReviewPolicy{
		"nil policy":  missing,
		"nil reviews": services.NewParentRequestReviewPolicy(nil),
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := policy.AbsenceScope(context.Background(), []string{"users:update"})
			require.Error(t, err)
			_, _, err = policy.Scope(context.Background(), []string{"users:update"})
			require.Error(t, err)
			_, err = policy.AccessLevel(context.Background(), []string{"users:update"})
			require.Error(t, err)
			_, err = policy.StudentFilter(context.Background(), []string{"users:update"})
			require.Error(t, err)
			_, err = policy.Allows(context.Background(), []string{"users:update"}, &userModels.Student{})
			require.Error(t, err)
		})
	}
}
