package timetracking_test

import (
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/workforce"
	"github.com/moto-ogs/project-phoenix/modules/workforce/internal/application/timetracking"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTargetCoverage_CountsTheOverviewsHasTarget is the payroll check of
// #6890: the count must come from the same has_target the Zeitkonten rows
// carry (#6887), not from a second rule.
func TestTargetCoverage_CountsTheOverviewsHasTarget(t *testing.T) {
	t.Parallel()

	f := newOverviewFixture(t, 2)
	for range 2 {
		staff := testpkg.CreateTestStaffForTenant(t, f.db, f.tenantID, "Ohne", "Soll")
		f.staff = append(f.staff, staff.ID)
	}

	coverage, err := testpkg.TenantQuery2(t, f.ctx, f.db, f.svc.TargetCoverage, 0, 0)
	require.NoError(t, err)

	overview, err := testpkg.TenantQuery1(t, f.ctx, f.db, f.svc.GetTimeTrackingOverview, timetracking.OverviewFilters{})
	require.NoError(t, err)
	withoutTarget := 0
	for _, row := range overview.Rows {
		if !row.HasTarget {
			withoutTarget++
		}
	}

	assert.Equal(t, overview.Year, coverage.Year)
	assert.Equal(t, overview.Month, coverage.Month)
	assert.Equal(t, len(overview.Rows), coverage.StaffTotal)
	assert.Equal(t, withoutTarget, coverage.StaffWithoutTarget)
	assert.Equal(t, 4, coverage.StaffTotal)
	assert.Equal(t, 2, coverage.StaffWithoutTarget)

	// The month rules are the overview's: no future month.
	next := f.today.AddDays(40)
	_, err = f.svc.TargetCoverage(f.ctx, next.Year(), int(next.Month()))
	require.ErrorIs(t, err, workforce.ErrOverviewInvalid)
}
