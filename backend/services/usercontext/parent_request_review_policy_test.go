package usercontext

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/users"
)

type reviewSettingStub struct {
	enabled bool
	err     error
	key     string
}

func (s *reviewSettingStub) ResolveBool(_ context.Context, key string) (bool, error) {
	s.key = key
	return s.enabled, s.err
}

type reviewGroupStub struct {
	groups []*education.Group
	err    error
	calls  int
}

func (s *reviewGroupStub) GetMyGroups(context.Context) ([]*education.Group, error) {
	s.calls++
	return s.groups, s.err
}

func TestParentRequestReviewerPolicyAdminsKeepSchoolWideAccess(t *testing.T) {
	t.Parallel()

	groups := &reviewGroupStub{err: errors.New("must not be called")}
	policy := NewParentRequestReviewPolicy(&reviewSettingStub{err: errors.New("must not be called")}, groups, "review.enabled")

	filter, err := policy.StudentFilter(context.Background(), []string{"admin:*"})
	require.NoError(t, err)
	assert.True(t, filter(&users.Student{}))
	assert.Zero(t, groups.calls)
}

func TestParentRequestReviewerPolicyGroupLeadersAreDeniedByDefault(t *testing.T) {
	t.Parallel()

	settings := &reviewSettingStub{}
	policy := NewParentRequestReviewPolicy(settings, &reviewGroupStub{}, "review.enabled")

	filter, err := policy.StudentFilter(context.Background(), []string{"users:update"})
	require.NoError(t, err)
	assert.False(t, filter(&users.Student{}))
	assert.Equal(t, "review.enabled", settings.key)
}

func TestParentRequestReviewerPolicyEnabledGroupLeadersOnlyReachTheirGroups(t *testing.T) {
	t.Parallel()

	groupID := int64(71)
	otherGroupID := int64(72)
	group := &education.Group{}
	group.ID = groupID
	policy := NewParentRequestReviewPolicy(
		&reviewSettingStub{enabled: true},
		&reviewGroupStub{groups: []*education.Group{group}},
		"review.enabled",
	)

	filter, err := policy.StudentFilter(context.Background(), []string{"users:update"})
	require.NoError(t, err)
	assert.True(t, filter(&users.Student{GroupID: &groupID}))
	assert.False(t, filter(&users.Student{GroupID: &otherGroupID}))
	assert.False(t, filter(&users.Student{}))
	assert.False(t, filter(nil))
}

func TestParentRequestReviewerPolicyFailsClosedWhenPolicyCannotBeResolved(t *testing.T) {
	t.Parallel()

	settingErr := errors.New("settings unavailable")
	policy := NewParentRequestReviewPolicy(&reviewSettingStub{err: settingErr}, &reviewGroupStub{}, "review.enabled")
	_, err := policy.StudentFilter(context.Background(), []string{"users:update"})
	assert.ErrorIs(t, err, settingErr)

	groupsErr := errors.New("groups unavailable")
	policy = NewParentRequestReviewPolicy(
		&reviewSettingStub{enabled: true}, &reviewGroupStub{err: groupsErr}, "review.enabled",
	)
	_, err = policy.StudentFilter(context.Background(), []string{"users:update"})
	assert.ErrorIs(t, err, groupsErr)
}

// The authorize sentinel cannot be imported here (module boundary), so the
// tests pin its message; the handler-side test pins the wire code.
// #2267 A4: users:absence is a write scope that never unlocks a read surface.
// A caller holding it without users:read is refused here, exactly as the write
// gate refuses them — an empty queue would read as "no work" instead.
func TestParentRequestReviewerPolicyRefusesAbsenceWithoutRead(t *testing.T) {
	t.Parallel()

	settings := &reviewSettingStub{err: errors.New("must not be called")}
	groups := &reviewGroupStub{err: errors.New("must not be called")}
	policy := NewParentRequestReviewPolicy(settings, groups, "review.enabled")

	_, err := policy.StudentFilter(context.Background(), []string{"users:absence"})
	require.ErrorContains(t, err, "users:read permission is required")

	_, allowErr := policy.Allows(context.Background(), []string{"users:absence"}, &users.Student{})
	require.ErrorContains(t, allowErr, "users:read permission is required")

	_, levelErr := policy.AccessLevel(context.Background(), []string{"users:absence"})
	require.ErrorContains(t, levelErr, "users:read permission is required")

	assert.Zero(t, groups.calls)
}

func TestParentRequestReviewerPolicyAcceptsAbsenceWithRead(t *testing.T) {
	t.Parallel()

	settings := &reviewSettingStub{enabled: true}
	policy := NewParentRequestReviewPolicy(settings, &reviewGroupStub{}, "review.enabled")

	_, err := policy.StudentFilter(context.Background(), []string{"users:absence", "users:read"})
	require.NoError(t, err)
}

func TestParentRequestReviewerPolicyAccessLevel(t *testing.T) {
	t.Parallel()

	admin := NewParentRequestReviewPolicy(&reviewSettingStub{err: errors.New("must not be called")}, &reviewGroupStub{}, "review.enabled")
	level, err := admin.AccessLevel(context.Background(), []string{"admin:*"})
	require.NoError(t, err)
	assert.Equal(t, ReviewAccessAdmin, level)

	disabled := NewParentRequestReviewPolicy(&reviewSettingStub{}, &reviewGroupStub{}, "review.enabled")
	level, err = disabled.AccessLevel(context.Background(), []string{"users:update"})
	require.NoError(t, err)
	assert.Equal(t, ReviewAccessNone, level)

	enabled := NewParentRequestReviewPolicy(&reviewSettingStub{enabled: true}, &reviewGroupStub{}, "review.enabled")
	level, err = enabled.AccessLevel(context.Background(), []string{"users:update"})
	require.NoError(t, err)
	assert.Equal(t, ReviewAccessGroupLeader, level)
}
