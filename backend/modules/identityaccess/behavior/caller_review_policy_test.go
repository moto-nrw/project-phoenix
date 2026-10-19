package behavior_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		facts := owners.ReviewPermissions(tc.permissions)
		assert.Equal(t, tc.adminWildcard, facts.AdminWildcard, "admin wildcard of %v", tc.permissions)
		assert.Equal(t, tc.absenceUnmet, facts.AbsenceReadPrerequisiteUnmet, "absence prerequisite of %v", tc.permissions)
		assert.Equal(t, tc.canReview, facts.CanReviewExcused, "review permission of %v", tc.permissions)
	}
}

// errAbsenceReadRequired stands in for the Security Runtime refusal of
// users:absence without users:read (#2267 A4), whose text the owner's own
// tests pin; the policy must pass it through unchanged.
var errAbsenceReadRequired = errors.New("users:read permission is required")

func TestParentRequestReviewerPolicyAdminsKeepSchoolWideAccess(t *testing.T) {
	t.Parallel()

	policy := owners.NewParentRequestReviewPolicy(scopeOf(true, nil, nil))

	filter, err := policy.StudentFilter(context.Background(), []string{"admin:*"})
	require.NoError(t, err)
	assert.True(t, filter(api.ParentRequestReviewStudent(nil)))
	assert.False(t, filter(nil))
}

func TestParentRequestReviewerPolicyGroupLeadersAreDeniedByDefault(t *testing.T) {
	t.Parallel()

	policy := owners.NewParentRequestReviewPolicy(scopeOf(false, nil, nil))

	filter, err := policy.StudentFilter(context.Background(), []string{"users:update"})
	require.NoError(t, err)
	assert.False(t, filter(api.ParentRequestReviewStudent(nil)))
}

func TestParentRequestReviewerPolicyEnabledGroupLeadersOnlyReachTheirGroups(t *testing.T) {
	t.Parallel()

	groupID := int64(71)
	otherGroupID := int64(72)
	policy := owners.NewParentRequestReviewPolicy(scopeOf(false, []int64{groupID}, nil))

	filter, err := policy.StudentFilter(context.Background(), []string{"users:update"})
	require.NoError(t, err)
	assert.True(t, filter(api.ParentRequestReviewStudent(&groupID)))
	assert.False(t, filter(api.ParentRequestReviewStudent(&otherGroupID)))
	assert.False(t, filter(api.ParentRequestReviewStudent(nil)))
	assert.False(t, filter(nil))

	allowed, err := policy.Allows(context.Background(), []string{"users:update"}, api.ParentRequestReviewStudent(&groupID))
	require.NoError(t, err)
	assert.True(t, allowed)
	allowed, err = policy.Allows(context.Background(), []string{"users:update"}, api.ParentRequestReviewStudent(&otherGroupID))
	require.NoError(t, err)
	assert.False(t, allowed)
}

func TestParentRequestReviewerPolicyFailsClosedWhenPolicyCannotBeResolved(t *testing.T) {
	t.Parallel()

	for _, scopeErr := range []error{errors.New("settings unavailable"), errors.New("groups unavailable")} {
		policy := owners.NewParentRequestReviewPolicy(scopeOf(false, nil, scopeErr))

		filter, err := policy.StudentFilter(context.Background(), []string{"users:update"})
		require.ErrorIs(t, err, scopeErr)
		assert.Nil(t, filter)

		allowed, err := policy.Allows(context.Background(), []string{"users:update"}, api.ParentRequestReviewStudent(nil))
		require.ErrorIs(t, err, scopeErr)
		assert.False(t, allowed)
	}
}

// #2267 A4: users:absence is a write scope that never unlocks a read surface.
// The refusal the review capability reports reaches the filter and the
// single decision unchanged.
func TestParentRequestReviewerPolicyRefusesAbsenceWithoutRead(t *testing.T) {
	t.Parallel()

	policy := owners.NewParentRequestReviewPolicy(scopeOf(false, nil, errAbsenceReadRequired))

	_, err := policy.StudentFilter(context.Background(), []string{"users:absence"})
	require.ErrorContains(t, err, "users:read permission is required")

	_, allowErr := policy.Allows(context.Background(), []string{"users:absence"}, api.ParentRequestReviewStudent(nil))
	require.ErrorContains(t, allowErr, "users:read permission is required")
}

// Resolving once per request: one filter, one scope evaluation.
func TestParentRequestReviewerPolicyStudentFilterResolvesTheScopeOnce(t *testing.T) {
	t.Parallel()

	groupID := int64(71)
	reviews := scopeOf(false, []int64{groupID}, nil)
	filter, err := owners.NewParentRequestReviewPolicy(reviews).StudentFilter(context.Background(), []string{"users:update"})
	require.NoError(t, err)

	filter(api.ParentRequestReviewStudent(&groupID))
	filter(api.ParentRequestReviewStudent(nil))
	assert.Equal(t, 1, reviews.scopeCall)
}

func TestParentRequestReviewerPolicyStudentFilterRequiresQueuePermission(t *testing.T) {
	t.Parallel()

	groupID := int64(71)
	// users:read alone reaches no group (pinned in the Identity & Access
	// review tests); the filter then admits no student of the group.
	policy := owners.NewParentRequestReviewPolicy(scopeOf(false, []int64{}, nil))

	filter, err := policy.StudentFilter(context.Background(), []string{"users:read"})

	require.NoError(t, err)
	assert.False(t, filter(api.ParentRequestReviewStudent(&groupID)))
}

func TestParentRequestReviewerPolicyWithoutReviewsIsNotConfigured(t *testing.T) {
	t.Parallel()

	var missing *owners.ParentRequestReviewPolicy
	for name, policy := range map[string]*owners.ParentRequestReviewPolicy{
		"nil policy":  missing,
		"nil reviews": owners.NewParentRequestReviewPolicy(nil),
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
			_, err = policy.Allows(context.Background(), []string{"users:update"}, api.ParentRequestReviewStudent(nil))
			require.Error(t, err)
		})
	}
}
