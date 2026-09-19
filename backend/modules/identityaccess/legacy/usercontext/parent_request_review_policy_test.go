package usercontext

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type reviewSettingStub struct {
	scope   string
	enabled bool
	err     error
	key     string
}

func (s *reviewSettingStub) ResolveString(_ context.Context, _ string) (string, error) {
	if s.scope == "" {
		return config.ParentAbsenceReviewScopeInherit, s.err
	}
	return s.scope, s.err
}

func (s *reviewSettingStub) ResolveBool(_ context.Context, key string) (bool, error) {
	s.key = key
	return s.enabled, s.err
}

type reviewGroupStub struct {
	staff  *users.Staff
	groups []*education.Group
	err    error
	calls  int
}

func (s *reviewGroupStub) GetCurrentStaff(context.Context) (*users.Staff, error) {
	return s.staff, s.err
}

func (s *reviewGroupStub) GetMyGroups(context.Context) ([]*education.Group, error) {
	s.calls++
	return s.groups, s.err
}

func TestParentAbsenceReviewScopePreservesOtherRequestKinds(t *testing.T) {
	t.Parallel()
	for _, oldEnabled := range []bool{false, true} {
		for _, scope := range []string{config.ParentAbsenceReviewScopeInherit, config.ParentAbsenceReviewScopeAdmins, config.ParentAbsenceReviewScopeGroupLeaders} {
			t.Run(fmt.Sprintf("%t/%s", oldEnabled, scope), func(t *testing.T) {
				group := &education.Group{}
				group.ID = 71
				policy := NewParentRequestReviewPolicy(&reviewSettingStub{enabled: oldEnabled, scope: scope}, &reviewGroupStub{groups: []*education.Group{group}}, "review.enabled", config.KeyParentAbsenceReviewScope)
				wide, ids, err := policy.AbsenceScope(context.Background(), []string{"users:update"})
				require.NoError(t, err)
				assert.False(t, wide)
				wantGroups := scope == config.ParentAbsenceReviewScopeGroupLeaders || (scope == config.ParentAbsenceReviewScopeInherit && oldEnabled)
				assert.Equal(t, wantGroups, slices.Contains(ids, group.ID))
				wide, ids, err = policy.Scope(context.Background(), []string{"users:update"})
				require.NoError(t, err)
				assert.False(t, wide)
				assert.Equal(t, oldEnabled, slices.Contains(ids, group.ID))
			})
		}
	}
}

func TestParentAbsenceReviewTeamRequiresVerifiedTenantStaff(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		portal      string
		claimTenant int64
		staffTenant int64
		permissions []string
		staff       bool
		want        bool
	}{
		{"eligible", "tenant", 71, 71, []string{"users:update"}, true, true},
		{"absence and read", "org", 71, 71, []string{"users:absence", "users:read"}, true, true},
		{"read only", "tenant", 71, 71, []string{"users:read"}, true, false},
		{"not staff", "tenant", 71, 71, []string{"users:update"}, false, false},
		{"other claim tenant", "tenant", 72, 71, []string{"users:update"}, true, false},
		{"other staff tenant", "tenant", 71, 72, []string{"users:update"}, true, false},
		{"school portal", "school", 71, 71, []string{"users:update"}, true, false},
		{"parent portal", "parent", 71, 71, []string{"users:update"}, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := tenant.WithTenantID(context.Background(), 71)
			ctx = context.WithValue(ctx, jwt.CtxClaims, jwt.AppClaims{ID: 81, TenantID: tc.claimTenant, Scope: tc.portal})
			groups := &reviewGroupStub{}
			if tc.staff {
				groups.staff = &users.Staff{}
				groups.staff.ID = 91
				groups.staff.TenantID = tc.staffTenant
			}
			policy := NewParentRequestReviewPolicy(&reviewSettingStub{scope: config.ParentAbsenceReviewScopeAllStaff}, groups, "review.enabled", config.KeyParentAbsenceReviewScope)
			wide, ids, err := policy.AbsenceScope(ctx, tc.permissions)
			require.NoError(t, err)
			assert.Equal(t, tc.want, wide)
			assert.Empty(t, ids)
			assert.Zero(t, groups.calls, "team access does not depend on group assignments")
			level, err := policy.AccessLevel(ctx, tc.permissions)
			require.NoError(t, err)
			if tc.want {
				assert.Equal(t, ReviewAccessTeam, level)
			} else {
				assert.Equal(t, ReviewAccessNone, level)
			}
		})
	}
}

func TestParentAbsenceReviewScopeFailsClosed(t *testing.T) {
	t.Parallel()
	settings := &reviewSettingStub{scope: "unknown"}
	policy := NewParentRequestReviewPolicy(settings, &reviewGroupStub{}, "review.enabled", config.KeyParentAbsenceReviewScope)
	wide, ids, err := policy.AbsenceScope(context.Background(), []string{"users:update"})
	require.Error(t, err)
	assert.False(t, wide)
	assert.Empty(t, ids)
	settings.err = errors.New("settings unavailable")
	_, _, err = policy.AbsenceScope(context.Background(), []string{"users:update"})
	assert.ErrorIs(t, err, settings.err)
	_, _, err = policy.AbsenceScope(context.Background(), []string{"users:absence"})
	require.Error(t, err)
	var missing *ParentRequestReviewPolicy
	_, _, err = missing.AbsenceScope(context.Background(), []string{"users:update"})
	require.Error(t, err)
}

func TestParentRequestReviewerPolicyAdminsKeepSchoolWideAccess(t *testing.T) {
	t.Parallel()

	groups := &reviewGroupStub{err: errors.New("must not be called")}
	policy := NewParentRequestReviewPolicy(&reviewSettingStub{err: errors.New("must not be called")}, groups, "review.enabled", config.KeyParentAbsenceReviewScope)

	filter, err := policy.StudentFilter(context.Background(), []string{"admin:*"})
	require.NoError(t, err)
	assert.True(t, filter(&users.Student{}))
	assert.Zero(t, groups.calls)
}

func TestParentRequestReviewerPolicyGroupLeadersAreDeniedByDefault(t *testing.T) {
	t.Parallel()

	settings := &reviewSettingStub{}
	policy := NewParentRequestReviewPolicy(settings, &reviewGroupStub{}, "review.enabled", config.KeyParentAbsenceReviewScope)

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
		"review.enabled", config.KeyParentAbsenceReviewScope,
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
	policy := NewParentRequestReviewPolicy(&reviewSettingStub{err: settingErr}, &reviewGroupStub{}, "review.enabled", config.KeyParentAbsenceReviewScope)
	_, err := policy.StudentFilter(context.Background(), []string{"users:update"})
	assert.ErrorIs(t, err, settingErr)

	groupsErr := errors.New("groups unavailable")
	policy = NewParentRequestReviewPolicy(
		&reviewSettingStub{enabled: true}, &reviewGroupStub{err: groupsErr}, "review.enabled", config.KeyParentAbsenceReviewScope,
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
	policy := NewParentRequestReviewPolicy(settings, groups, "review.enabled", config.KeyParentAbsenceReviewScope)

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
	policy := NewParentRequestReviewPolicy(settings, &reviewGroupStub{}, "review.enabled", config.KeyParentAbsenceReviewScope)

	_, err := policy.StudentFilter(context.Background(), []string{"users:absence", "users:read"})
	require.NoError(t, err)
}

func TestParentRequestReviewerPolicyAccessLevel(t *testing.T) {
	t.Parallel()

	admin := NewParentRequestReviewPolicy(&reviewSettingStub{err: errors.New("must not be called")}, &reviewGroupStub{}, "review.enabled", config.KeyParentAbsenceReviewScope)
	level, err := admin.AccessLevel(context.Background(), []string{"admin:*"})
	require.NoError(t, err)
	assert.Equal(t, ReviewAccessAdmin, level)

	disabled := NewParentRequestReviewPolicy(&reviewSettingStub{}, &reviewGroupStub{}, "review.enabled", config.KeyParentAbsenceReviewScope)
	level, err = disabled.AccessLevel(context.Background(), []string{"users:update"})
	require.NoError(t, err)
	assert.Equal(t, ReviewAccessNone, level)

	group := &education.Group{}
	group.ID = 71
	enabled := NewParentRequestReviewPolicy(
		&reviewSettingStub{enabled: true},
		&reviewGroupStub{groups: []*education.Group{group}},
		"review.enabled", config.KeyParentAbsenceReviewScope,
	)
	level, err = enabled.AccessLevel(context.Background(), []string{"users:update"})
	require.NoError(t, err)
	assert.Equal(t, ReviewAccessGroupLeader, level)
}

func TestParentRequestReviewerPolicyAccessLevelRequiresCurrentGroup(t *testing.T) {
	t.Parallel()

	policy := NewParentRequestReviewPolicy(
		&reviewSettingStub{enabled: true},
		&reviewGroupStub{},
		"review.enabled", config.KeyParentAbsenceReviewScope,
	)

	level, err := policy.AccessLevel(context.Background(), []string{"users:update"})

	require.NoError(t, err)
	assert.Equal(t, ReviewAccessNone, level)
}

func TestParentRequestReviewerPolicyAccessLevelRequiresQueuePermission(t *testing.T) {
	t.Parallel()

	group := &education.Group{}
	group.ID = 71
	policy := NewParentRequestReviewPolicy(
		&reviewSettingStub{enabled: true},
		&reviewGroupStub{groups: []*education.Group{group}},
		"review.enabled", config.KeyParentAbsenceReviewScope,
	)

	level, err := policy.AccessLevel(context.Background(), []string{"users:read"})

	require.NoError(t, err)
	assert.Equal(t, ReviewAccessNone, level)
}

func TestParentRequestReviewerPolicyStudentFilterRequiresQueuePermission(t *testing.T) {
	t.Parallel()

	groupID := int64(71)
	group := &education.Group{}
	group.ID = groupID
	policy := NewParentRequestReviewPolicy(
		&reviewSettingStub{enabled: true},
		&reviewGroupStub{groups: []*education.Group{group}},
		"review.enabled", config.KeyParentAbsenceReviewScope,
	)

	filter, err := policy.StudentFilter(context.Background(), []string{"users:read"})

	require.NoError(t, err)
	assert.False(t, filter(&users.Student{GroupID: &groupID}))
}
