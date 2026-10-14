package workforce

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCanSeeAllSubstitutions(t *testing.T) {
	t.Parallel()

	allStaff := func(context.Context) (bool, error) { return true, nil }
	ownGroups := func(context.Context) (bool, error) { return false, nil }

	cases := []struct {
		name                            string
		scope                           SubstitutionOverviewScope
		assignmentBound, admin, isStaff bool
		want                            bool
	}{
		{name: "admin sees all under the own-groups scope", scope: ownGroups, admin: true, isStaff: true, want: true},
		{name: "staff sees own groups under the own-groups scope", scope: ownGroups, isStaff: true, want: false},
		{name: "staff sees all under the all-staff scope", scope: allStaff, isStaff: true, want: true},
		{name: "a caller without staff record never sees all under the all-staff scope", scope: allStaff, want: false},
		{name: "an assignment-bound caller never sees all, even as admin", scope: allStaff, assignmentBound: true, admin: true, isStaff: true, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := CanSeeAllSubstitutions(context.Background(), tc.scope, tc.assignmentBound, tc.admin, tc.isStaff)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestCanSeeAllSubstitutionsReportsTheScopeFailureEvenForAdmins(t *testing.T) {
	t.Parallel()

	failure := errors.New("settings unavailable")
	got, err := CanSeeAllSubstitutions(context.Background(), func(context.Context) (bool, error) { return false, failure }, false, true, true)
	require.ErrorIs(t, err, failure)
	assert.False(t, got)
}

func TestCanSeeAllSubstitutionsSkipsTheScopeForAssignmentBoundCallers(t *testing.T) {
	t.Parallel()

	got, err := CanSeeAllSubstitutions(context.Background(), func(context.Context) (bool, error) {
		t.Fatal("the scope must not be resolved for an assignment-bound caller")
		return false, nil
	}, true, true, true)
	require.NoError(t, err)
	assert.False(t, got)
}
