package compose_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// TestClassHandoverReplacesAndWithdraws pins the class handover store
// (#4794): one handover per class and day, matched trimmed and lowercased; a
// second one replaces the first with its children; a withdrawal reports
// whether there was one.
func TestClassHandoverReplacesAndWithdraws(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	first := testpkg.CreateTestStudent(t, db, "Erste", "Übergabe", "3c")
	second := testpkg.CreateTestStudent(t, db, "Zweite", "Übergabe", "3c")
	staff := testpkg.CreateTestStaff(t, db, "Lehr", "Kraft")
	day := calendar.NewDate(2099, 3, 2)

	existed, err := module.ChangeClassHandover(ctx, studentpresence.ClassHandoverChange{
		SchoolClass: " 3c ", Date: day, HandedOverBy: staff.ID, AbsentStudentIDs: []int64{first.ID, first.ID},
	})
	require.NoError(t, err)
	assert.False(t, existed)

	handovers, err := module.ClassHandoversOn(ctx, day)
	require.NoError(t, err)
	require.Len(t, handovers, 1)
	assert.Equal(t, "3c", handovers[0].SchoolClass, "stored trimmed")
	assert.Equal(t, staff.ID, handovers[0].HandedOverByStaffID)
	assert.Equal(t, []int64{first.ID}, handovers[0].AbsentStudentIDs, "a child named twice is absent once")

	existed, err = module.ChangeClassHandover(ctx, studentpresence.ClassHandoverChange{
		SchoolClass: "3C", Date: day, HandedOverBy: staff.ID, AbsentStudentIDs: []int64{second.ID},
	})
	require.NoError(t, err)
	assert.True(t, existed, "the same class in another spelling")
	handovers, err = module.ClassHandoversOn(ctx, day)
	require.NoError(t, err)
	require.Len(t, handovers, 1)
	assert.Equal(t, []int64{second.ID}, handovers[0].AbsentStudentIDs)

	other, err := module.ClassHandoversOn(ctx, day.AddDays(1))
	require.NoError(t, err)
	assert.Empty(t, other, "another day has none")

	existed, err = module.ChangeClassHandover(ctx, studentpresence.ClassHandoverChange{SchoolClass: "3c", Date: day, Withdraw: true})
	require.NoError(t, err)
	assert.True(t, existed)
	existed, err = module.ChangeClassHandover(ctx, studentpresence.ClassHandoverChange{SchoolClass: "3c", Date: day, Withdraw: true})
	require.NoError(t, err)
	assert.False(t, existed)
	var absences int
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM active.class_handover_absences WHERE student_id IN (?, ?)`,
		first.ID, second.ID).Scan(ctx, &absences))
	assert.Zero(t, absences, "the absences go with the handover")

	for name, change := range map[string]studentpresence.ClassHandoverChange{
		"no class":       {SchoolClass: " ", Date: day, HandedOverBy: staff.ID},
		"no staff":       {SchoolClass: "3c", Date: day},
		"no date":        {SchoolClass: "3c", HandedOverBy: staff.ID},
		"bad child":      {SchoolClass: "3c", Date: day, HandedOverBy: staff.ID, AbsentStudentIDs: []int64{0}},
		"withdraw blank": {SchoolClass: "", Date: day, Withdraw: true},
	} {
		_, err := module.ChangeClassHandover(ctx, change)
		assert.ErrorIs(t, err, studentpresence.ErrClassHandoverInvalid, name)
	}
}

// TestClassHandoverStaysInItsSchool proves the RLS boundary: another school
// sees none of the handovers.
func TestClassHandoverStaysInItsSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	student := testpkg.CreateTestStudent(t, db, "Eigene", "Schule", "4a")
	staff := testpkg.CreateTestStaff(t, db, "Lehr", "Kraft")
	day := calendar.NewDate(2099, 3, 2)
	_, err = module.ChangeClassHandover(ctx, studentpresence.ClassHandoverChange{
		SchoolClass: "4a", Date: day, HandedOverBy: staff.ID, AbsentStudentIDs: []int64{student.ID},
	})
	require.NoError(t, err)

	t.Run("another school", func(t *testing.T) {
		otherCtx := testpkg.OwnCtx(t)
		handovers, err := module.ClassHandoversOn(otherCtx, day)
		require.NoError(t, err)
		assert.Empty(t, handovers)
		existed, err := module.ChangeClassHandover(otherCtx, studentpresence.ClassHandoverChange{SchoolClass: "4a", Date: day, Withdraw: true})
		require.NoError(t, err)
		assert.False(t, existed, "another school cannot take it back")
	})
	handovers, err := module.ClassHandoversOn(ctx, day)
	require.NoError(t, err)
	assert.Len(t, handovers, 1)
}

// TestMorningAttendanceRetentionDeletesClassHandovers proves the nightly
// retention deletes the handovers of the expired days with the morning
// attendance and counts them.
func TestMorningAttendanceRetentionDeletesClassHandovers(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	student := testpkg.CreateTestStudent(t, db, "Frist", "Übergabe", "1b")
	staff := testpkg.CreateTestStaff(t, db, "Lehr", "Kraft")
	expired := testpkg.TodayDate().AddDays(-400)
	kept := testpkg.TodayDate().AddDays(-10)
	for _, day := range []calendar.Date{expired, kept} {
		_, err := module.ChangeClassHandover(ctx, studentpresence.ClassHandoverChange{
			SchoolClass: "1b", Date: day, HandedOverBy: staff.ID, AbsentStudentIDs: []int64{student.ID},
		})
		require.NoError(t, err)
	}

	require.NoError(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		deleted, err := module.DeleteExpiredMorningAttendance(txCtx, 365)
		assert.Equal(t, 1, deleted, "the expired handover; its absence goes by cascade")
		return err
	}))
	gone, err := module.ClassHandoversOn(ctx, expired)
	require.NoError(t, err)
	assert.Empty(t, gone)
	left, err := module.ClassHandoversOn(ctx, kept)
	require.NoError(t, err)
	assert.Len(t, left, 1)
}

// A second submission must replace even an uncommitted first handover,
// including its entire absence list, without a unique-constraint error.
func TestClassHandoverConcurrentReplacement(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	first := testpkg.CreateTestStudent(t, db, "Erste", "Übergabe", "3c")
	second := testpkg.CreateTestStudent(t, db, "Zweite", "Übergabe", "3c")
	firstStaff := testpkg.CreateTestStaff(t, db, "Erste", "Lehrkraft")
	secondStaff := testpkg.CreateTestStaff(t, db, "Zweite", "Lehrkraft")
	day := calendar.NewDate(2099, 3, 2)
	type result struct {
		replaced bool
		err      error
	}
	completed := make(chan result, 1)
	err = testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
		_, changeErr := module.ChangeClassHandover(txCtx, studentpresence.ClassHandoverChange{
			SchoolClass: "3c", Date: day, HandedOverBy: firstStaff.ID, AbsentStudentIDs: []int64{first.ID},
		})
		require.NoError(t, changeErr)
		var holderPID int
		require.NoError(t, tx.NewRaw("SELECT pg_backend_pid()").Scan(txCtx, &holderPID))
		go func() {
			replaced, changeErr := module.ChangeClassHandover(ctx, studentpresence.ClassHandoverChange{
				SchoolClass: " 3C ", Date: day, HandedOverBy: secondStaff.ID, AbsentStudentIDs: []int64{second.ID},
			})
			completed <- result{replaced, changeErr}
		}()
		// Wait until the second transaction actually contends with the first.
		// Without serialization it waits at INSERT after deleting zero rows.
		require.Eventually(t, func() bool {
			var blocked bool
			queryErr := db.NewRaw(`SELECT EXISTS (
				SELECT 1 FROM pg_stat_activity
				WHERE datname = current_database() AND ? = ANY(pg_blocking_pids(pid))
			)`, holderPID).Scan(ctx, &blocked)
			require.NoError(t, queryErr)
			return blocked
		}, 5*time.Second, 10*time.Millisecond)
		return nil
	})
	require.NoError(t, err)
	select {
	case replacement := <-completed:
		require.NoError(t, replacement.err)
		assert.True(t, replacement.replaced)
	case <-ctx.Done():
		t.Fatal("concurrent handover did not finish", ctx.Err())
	}
	handovers, err := module.ClassHandoversOn(ctx, day)
	require.NoError(t, err)
	require.Len(t, handovers, 1)
	assert.Equal(t, secondStaff.ID, handovers[0].HandedOverByStaffID)
	assert.Equal(t, []int64{second.ID}, handovers[0].AbsentStudentIDs)
}
