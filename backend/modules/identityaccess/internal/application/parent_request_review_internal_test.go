package application

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

const (
	reviewGroupLeaderKey = "review.enabled"
	reviewAbsenceKey     = "operations.parent_absence_review_scope"
	reviewGroupID        = int64(71)
)

// errReviewAbsenceReadRequired stands in for the authorize sentinel the root
// binds; services/parent_request_review_internal_test.go pins its text.
var errReviewAbsenceReadRequired = errors.New("the users:read permission is required alongside users:absence")

// reviewPermissionFacts are the route-level facts the root derives from the
// permission sets these tests use; services/parent_request_review_internal_test.go
// pins that derivation.
var reviewPermissionFacts = map[string]domain.ReviewPermissions{
	"admin:*":                  {AdminWildcard: true, CanReviewExcused: true},
	"users:update":             {CanReviewExcused: true},
	"users:absence,users:read": {CanReviewExcused: true},
	"users:read":               {},
	"users:absence":            {AbsenceReadPrerequisiteUnmet: true},
}

type reviewTestSettings struct {
	scope   string
	enabled bool
	err     error
	key     string
}

func (s *reviewTestSettings) ResolveString(context.Context, string) (string, error) {
	if s.scope == "" {
		return absenceScopeInherit, s.err
	}
	return s.scope, s.err
}

func (s *reviewTestSettings) ResolveBool(_ context.Context, key string) (bool, error) {
	s.key = key
	return s.enabled, s.err
}

// reviewerHarness is a verified staff member and teacher of tenant 71 whose
// teacher profile leads the given groups.
func reviewerHarness(groups ...int64) *callerHarness {
	h := newCallerHarness(domain.Caller{
		Authenticated: true, AccountID: 81, TenantID: 71, ClaimsTenantID: 71, Scope: "tenant",
	}).withStaff()
	h.membership.teacherID, h.membership.teacherFound = 91, true
	h.structure.teacherGroups = groups
	return h
}

func newTestReview(t *testing.T, h *callerHarness, settings *reviewTestSettings) *ParentRequestReview {
	t.Helper()
	deps := ParentRequestReviewDependencies{
		Permissions: func(permissions []string) domain.ReviewPermissions {
			facts, ok := reviewPermissionFacts[strings.Join(permissions, ",")]
			require.True(t, ok, "unmapped permission set %v", permissions)
			return facts
		},
		GroupLeaderSettingKey: reviewGroupLeaderKey,
		AbsenceSettingKey:     reviewAbsenceKey,
		AbsenceReadRequired:   errReviewAbsenceReadRequired,
	}
	if settings != nil {
		deps.Settings = settings
	}
	review, err := NewParentRequestReview(h.build(t), deps)
	require.NoError(t, err)
	return review
}

func TestParentAbsenceReviewScopePreservesOtherRequestKinds(t *testing.T) {
	t.Parallel()
	for _, oldEnabled := range []bool{false, true} {
		for _, scope := range []string{absenceScopeInherit, absenceScopeAdmins, absenceScopeGroupLeaders} {
			t.Run(fmt.Sprintf("%t/%s", oldEnabled, scope), func(t *testing.T) {
				review := newTestReview(t, reviewerHarness(reviewGroupID), &reviewTestSettings{enabled: oldEnabled, scope: scope})
				wide, ids, err := review.AbsenceScope(context.Background(), []string{"users:update"})
				require.NoError(t, err)
				assert.False(t, wide)
				wantGroups := scope == absenceScopeGroupLeaders || (scope == absenceScopeInherit && oldEnabled)
				assert.Equal(t, wantGroups, slices.Contains(ids, reviewGroupID))
				wide, ids, err = review.Scope(context.Background(), []string{"users:update"})
				require.NoError(t, err)
				assert.False(t, wide)
				assert.Equal(t, oldEnabled, slices.Contains(ids, reviewGroupID))
			})
		}
	}
}

type reviewTeamCase struct {
	name        string
	portal      string
	claimTenant int64
	staffTenant int64
	permissions []string
	staff       bool
	want        bool
}

// The staff lookup is tenant-scoped: a staff record of another tenant is not
// found in the request tenant (71).
func (tc reviewTeamCase) harness() *callerHarness {
	h := reviewerHarness()
	h.caller.ClaimsTenantID, h.caller.Scope = tc.claimTenant, tc.portal
	h.membership.staffFound = tc.staff && tc.staffTenant == h.caller.TenantID
	if !tc.staff {
		// The old stub's "no staff, no error": a resolved stage without a
		// staff member.
		h.membership.staffID, h.membership.staffFound = 0, true
	}
	return h
}

func TestParentAbsenceReviewTeamRequiresVerifiedTenantStaff(t *testing.T) {
	t.Parallel()
	for _, tc := range []reviewTeamCase{
		{"eligible", "tenant", 71, 71, []string{"users:update"}, true, true},
		{"absence and read", "org", 71, 71, []string{"users:absence", "users:read"}, true, true},
		{"read only", "tenant", 71, 71, []string{"users:read"}, true, false},
		{"not staff", "tenant", 71, 71, []string{"users:update"}, false, false},
		{"other claim tenant", "tenant", 72, 71, []string{"users:update"}, true, false},
		{"school portal", "school", 71, 71, []string{"users:update"}, true, false},
		{"parent portal", "parent", 71, 71, []string{"users:update"}, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := tc.harness()
			review := newTestReview(t, h, &reviewTestSettings{scope: absenceScopeAllStaff})
			wide, ids, err := review.AbsenceScope(context.Background(), tc.permissions)
			require.NoError(t, err)
			assert.Equal(t, tc.want, wide)
			assert.Empty(t, ids)
			assert.Zero(t, h.structure.groupCalls(), "team access does not depend on group assignments")
			level, err := review.AccessLevel(context.Background(), tc.permissions)
			require.NoError(t, err)
			if tc.want {
				assert.Equal(t, domain.ReviewAccessTeam, level)
			} else {
				assert.Equal(t, domain.ReviewAccessNone, level)
			}
		})
	}
}

// A caller whose person has no staff record in the request tenant - also a
// staff record of another tenant, which the tenant-scoped lookup never finds -
// is refused fail-closed: no team access, and the queue reports an error
// instead of an empty result.
func TestParentAbsenceReviewTeamRefusesCallerWithoutTenantStaff(t *testing.T) {
	t.Parallel()
	for _, tc := range []reviewTeamCase{
		{"not linked to staff", "tenant", 71, 0, []string{"users:update"}, true, false},
		{"other staff tenant", "tenant", 71, 72, []string{"users:update"}, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := tc.harness()
			review := newTestReview(t, h, &reviewTestSettings{scope: absenceScopeAllStaff})
			wide, ids, err := review.AbsenceScope(context.Background(), tc.permissions)
			require.ErrorIs(t, err, domain.ErrCallerNotLinkedToStaff)
			assert.False(t, wide)
			assert.Empty(t, ids)
			_, err = review.AccessLevel(context.Background(), tc.permissions)
			require.ErrorIs(t, err, domain.ErrCallerNotLinkedToStaff)
			assert.Zero(t, h.structure.groupCalls())
		})
	}
}

func TestParentAbsenceReviewScopeFailsClosed(t *testing.T) {
	t.Parallel()
	settings := &reviewTestSettings{scope: "unknown"}
	review := newTestReview(t, reviewerHarness(), settings)
	wide, ids, err := review.AbsenceScope(context.Background(), []string{"users:update"})
	require.Error(t, err)
	assert.False(t, wide)
	assert.Empty(t, ids)
	settings.err = errors.New("settings unavailable")
	_, _, err = review.AbsenceScope(context.Background(), []string{"users:update"})
	assert.ErrorIs(t, err, settings.err)
	_, _, err = review.AbsenceScope(context.Background(), []string{"users:absence"})
	require.Error(t, err)
	// The unconfigured policy (the old nil receiver) is pinned on the
	// services adapter; without settings the policy refuses as well.
	unconfigured := newTestReview(t, reviewerHarness(), nil)
	_, _, err = unconfigured.AbsenceScope(context.Background(), []string{"users:update"})
	require.Error(t, err)
	_, _, err = unconfigured.Scope(context.Background(), []string{"users:update"})
	require.Error(t, err)
}

func TestParentRequestReviewAdminsKeepSchoolWideAccess(t *testing.T) {
	t.Parallel()

	h := reviewerHarness()
	h.structure.teacherErr = errors.New("must not be called")
	review := newTestReview(t, h, &reviewTestSettings{err: errors.New("must not be called")})

	wide, ids, err := review.Scope(context.Background(), []string{"admin:*"})
	require.NoError(t, err)
	assert.True(t, wide)
	assert.Nil(t, ids)
	assert.Zero(t, h.structure.groupCalls())

	// The admin role claim counts as effective admin as well.
	h.caller.AdminRole = true
	wide, _, err = review.Scope(context.Background(), []string{"users:update"})
	require.NoError(t, err)
	assert.True(t, wide)
	assert.Zero(t, h.structure.groupCalls())
}

func TestParentRequestReviewGroupLeadersAreDeniedByDefault(t *testing.T) {
	t.Parallel()

	settings := &reviewTestSettings{}
	review := newTestReview(t, reviewerHarness(reviewGroupID), settings)

	wide, ids, err := review.Scope(context.Background(), []string{"users:update"})
	require.NoError(t, err)
	assert.False(t, wide)
	assert.Empty(t, ids)
	assert.Equal(t, reviewGroupLeaderKey, settings.key)
}

func TestParentRequestReviewEnabledGroupLeadersOnlyReachTheirGroups(t *testing.T) {
	t.Parallel()

	review := newTestReview(t, reviewerHarness(reviewGroupID), &reviewTestSettings{enabled: true})

	wide, ids, err := review.Scope(context.Background(), []string{"users:update"})
	require.NoError(t, err)
	assert.False(t, wide)
	assert.Equal(t, []int64{reviewGroupID}, ids)
}

func TestParentRequestReviewFailsClosedWhenPolicyCannotBeResolved(t *testing.T) {
	t.Parallel()

	settingErr := errors.New("settings unavailable")
	review := newTestReview(t, reviewerHarness(), &reviewTestSettings{err: settingErr})
	_, _, err := review.Scope(context.Background(), []string{"users:update"})
	assert.ErrorIs(t, err, settingErr)

	groupsErr := errors.New("groups unavailable")
	h := reviewerHarness()
	h.structure.teacherErr = groupsErr
	review = newTestReview(t, h, &reviewTestSettings{enabled: true})
	_, _, err = review.Scope(context.Background(), []string{"users:update"})
	assert.ErrorIs(t, err, groupsErr)
}

// #2267 A4: users:absence is a write scope that never unlocks a read surface.
// A caller holding it without users:read is refused, exactly as the write
// gate refuses them - an empty queue would read as "no work" instead.
func TestParentRequestReviewRefusesAbsenceWithoutRead(t *testing.T) {
	t.Parallel()

	h := reviewerHarness()
	h.structure.teacherErr = errors.New("must not be called")
	review := newTestReview(t, h, &reviewTestSettings{err: errors.New("must not be called")})

	_, _, err := review.Scope(context.Background(), []string{"users:absence"})
	require.ErrorIs(t, err, errReviewAbsenceReadRequired)

	_, _, err = review.AbsenceScope(context.Background(), []string{"users:absence"})
	require.ErrorIs(t, err, errReviewAbsenceReadRequired)

	_, levelErr := review.AccessLevel(context.Background(), []string{"users:absence"})
	require.ErrorIs(t, levelErr, errReviewAbsenceReadRequired)

	assert.Zero(t, h.structure.groupCalls())
}

func TestParentRequestReviewAcceptsAbsenceWithRead(t *testing.T) {
	t.Parallel()

	review := newTestReview(t, reviewerHarness(), &reviewTestSettings{enabled: true})

	_, _, err := review.Scope(context.Background(), []string{"users:absence", "users:read"})
	require.NoError(t, err)
}

func TestParentRequestReviewAccessLevel(t *testing.T) {
	t.Parallel()

	admin := newTestReview(t, reviewerHarness(), &reviewTestSettings{err: errors.New("must not be called")})
	level, err := admin.AccessLevel(context.Background(), []string{"admin:*"})
	require.NoError(t, err)
	assert.Equal(t, domain.ReviewAccessAdmin, level)

	disabled := newTestReview(t, reviewerHarness(), &reviewTestSettings{})
	level, err = disabled.AccessLevel(context.Background(), []string{"users:update"})
	require.NoError(t, err)
	assert.Equal(t, domain.ReviewAccessNone, level)

	enabled := newTestReview(t, reviewerHarness(reviewGroupID), &reviewTestSettings{enabled: true})
	level, err = enabled.AccessLevel(context.Background(), []string{"users:update"})
	require.NoError(t, err)
	assert.Equal(t, domain.ReviewAccessGroupLeader, level)
}

func TestParentRequestReviewAccessLevelRequiresCurrentGroup(t *testing.T) {
	t.Parallel()

	review := newTestReview(t, reviewerHarness(), &reviewTestSettings{enabled: true})

	level, err := review.AccessLevel(context.Background(), []string{"users:update"})

	require.NoError(t, err)
	assert.Equal(t, domain.ReviewAccessNone, level)
}

func TestParentRequestReviewAccessLevelRequiresQueuePermission(t *testing.T) {
	t.Parallel()

	review := newTestReview(t, reviewerHarness(reviewGroupID), &reviewTestSettings{enabled: true})

	level, err := review.AccessLevel(context.Background(), []string{"users:read"})

	require.NoError(t, err)
	assert.Equal(t, domain.ReviewAccessNone, level)
}

// Successor of the StudentFilter case: without the queue permission the
// scope reaches no group, so no student of the group passes the filter
// (services/parent_request_review_internal_test.go pins the filter).
func TestParentRequestReviewScopeRequiresQueuePermission(t *testing.T) {
	t.Parallel()

	review := newTestReview(t, reviewerHarness(reviewGroupID), &reviewTestSettings{enabled: true})

	wide, ids, err := review.Scope(context.Background(), []string{"users:read"})

	require.NoError(t, err)
	assert.False(t, wide)
	assert.NotContains(t, ids, reviewGroupID)
}
