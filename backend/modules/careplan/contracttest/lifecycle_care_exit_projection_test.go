package contracttest_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// The lifecycle's directory reads go through the named student directory
// projection, without the retired student compatibility view (#3427). They
// must follow the live membership rather than an ID equality with the
// profile, stay inside the tenant, and hide a child as soon as any half of
// the profile/membership/care chain is missing.
func TestCareLifecycleDirectoryReadsWithoutStudentCompatibilityView(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := testpkg.Ctx(t)
	today := timezone.NewDate(2026, 9, 19)
	svc := newCareLifecycleServiceAt(t, db, today)
	testpkg.AssertStudentCompatibilityStorageAbsent(t, db)

	ended := testpkg.CreateTestStudent(t, db, "Owner", "Care", "3b")
	_, err := db.NewUpdate().TableExpr("users.student_school_memberships").
		Set("enrolled_until = ?", today.AddDays(-1)).
		Where("student_profile_id = ? AND deleted_at IS NULL", ended.ID).
		Exec(ctx)
	require.NoError(t, err)
	endedMembership := testpkg.SeparateStudentMembership(t, db, ended.ID)
	inCare := testpkg.CreateTestStudent(t, db, "Ohne", "Buchung", "3c")
	inCareMembership := testpkg.SeparateStudentMembership(t, db, inCare.ID)

	archive := func(ctx context.Context, filter careplan.EndedCareFilter) ([]careplan.EndedCare, int) {
		rows, total, err := svc.ListEnded(ctx, filter)
		require.NoError(t, err)
		return rows, total
	}
	// A child in care without a booking blocks the booking-led mode: that
	// preview reads the care population through the projection.
	blocking := func(ctx context.Context) []careplan.BookingAuthorityImpactChild {
		impact, err := svc.PreviewBookingAuthorityImpact(ctx, today)
		require.NoError(t, err)
		return impact.BlockingChildren
	}

	rows, total := archive(ctx, careplan.EndedCareFilter{Search: "3b"})
	require.Equal(t, 1, total)
	require.Len(t, rows, 1)
	require.Equal(t, ended.ID, rows[0].StudentID)
	children := blocking(ctx)
	require.Len(t, children, 1)
	require.Equal(t, strconv.FormatInt(inCare.ID, 10), children[0].StudentID)
	require.Equal(t, "3c", children[0].SchoolClass)

	testpkg.AssertStudentProjectionTenants(t, db, func(txCtx context.Context, visible bool) {
		rows, _ := archive(txCtx, careplan.EndedCareFilter{})
		require.Equal(t, visible, len(rows) == 1)
		require.Equal(t, visible, len(blocking(txCtx)) == 1)
	})

	testpkg.AssertMissingStudentProjectionStates(t, db, endedMembership, func() {
		rows, total := archive(ctx, careplan.EndedCareFilter{})
		require.Zero(t, total)
		require.Empty(t, rows)
	})
	testpkg.AssertMissingStudentProjectionStates(t, db, inCareMembership, func() {
		require.Empty(t, blocking(ctx))
	})
}
