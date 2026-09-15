package compose

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAllowanceCalendarDays(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	capability := buildWorkforce(t, db)
	ctx := testpkg.Ctx(t)
	absenceType, err := capability.CreateAbsenceType(ctx, workforce.CreateAbsenceType{Name: "Calendar"})
	require.NoError(t, err)
	for _, tc := range []struct {
		name, from, to           string
		half, startHalf, endHalf bool
		want                     float64
	}{
		{name: "one boundary counted once", from: "2026-01-05", to: "2026-01-05", startHalf: true, endHalf: true, want: 0.5},
		{name: "weekend boundaries do not count", from: "2026-01-03", to: "2026-01-04", half: true, want: 0},
		{name: "explicit first half", from: "2026-01-05", to: "2026-01-06", startHalf: true, want: 1.5},
		{name: "clipped half stays in its original year", from: "2025-12-31", to: "2026-01-02", startHalf: true, endHalf: true, want: 1.5},
		{name: "DST weekend", from: "2026-03-27", to: "2026-03-30", want: 2},
		{name: "legacy half flags across DST", from: "2026-03-27", to: "2026-03-30", half: true, want: 1},
		{name: "outside year", from: "2025-12-01", to: "2025-12-31", want: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			staff := testpkg.CreateTestStaff(t, db, "Calendar", "Coverage")
			_, err := capability.CreateStaffAbsence(ctx, workforce.StaffAbsence{
				StaffID: staff.ID, CreatedBy: staff.ID, AbsenceTypeID: &absenceType.ID,
				AbsenceType: workforce.AbsenceTypeOther, Status: workforce.AbsenceStatusApproved,
				DateStart: tc.from, DateEnd: tc.to, HalfDay: tc.half, StartHalfDay: tc.startHalf, EndHalfDay: tc.endHalf,
			})
			require.NoError(t, err)
			summary, err := capability.AllowanceSummary(ctx, staff.ID, absenceType.ID, 2026)
			require.NoError(t, err)
			assert.Equal(t, tc.want, summary.TakenDays)
		})
	}
}
