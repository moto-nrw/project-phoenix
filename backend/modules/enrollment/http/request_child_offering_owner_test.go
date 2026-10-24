package enrollmenthttp_test

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/moto-nrw/project-phoenix/api"

	"testing"

	owner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestOwnerOfferingInsertDefaultsDatesAndRollsBack(t *testing.T) {
	t.Parallel()
	db, _, tenantID, childID, offeringID := setupChildOfferingTest(t, testpkg.SetupTestDB(t))
	module := api.NewEnrollmentBookingFixtureWithin(testpkg.WithinCurrentTenant)
	selection := &owner.RequestChildOffering{RequestChildID: childID, CareOfferingID: offeringID, SelectedDays: []string{"mon"}, ManualSelectedDays: []string{"mon"}}
	failure := errors.New("injected after selection insert")
	err := testpkg.WithTenantTx(t, testpkg.Ctx(t), db, tenantID, func(ctx context.Context, tx bun.Tx) error {
		if err := testpkg.TenantCommand1(t, ctx, db, module.InsertRequestChildOffering, selection); err != nil {
			return err
		}
		require.Equal(t, owner.Date("2026-09-01"), *selection.ValidFrom)
		require.Equal(t, owner.Date("2027-08-01"), *selection.ValidUntil)
		require.Equal(t, tenantID, selection.TenantID)
		require.NotZero(t, selection.ID)
		var count int
		require.NoError(t, tx.NewRaw("SELECT COUNT(*) FROM enrollment.care_offering_bookings WHERE id = ?", selection.ID).Scan(ctx, &count))
		require.Equal(t, 1, count)
		return failure
	})
	require.ErrorIs(t, err, failure)
	var count int
	require.NoError(t, db.NewRaw("SELECT COUNT(*) FROM enrollment.care_offering_bookings WHERE id = ?", selection.ID).Scan(testpkg.Ctx(t), &count))
	require.Zero(t, count)
	from, until := owner.Date("2026-10-25"), owner.Date("2027-03-28")
	retry := &owner.RequestChildOffering{RequestChildID: childID, CareOfferingID: offeringID, ValidFrom: &from, ValidUntil: &until}
	require.NoError(t, testpkg.TenantCommand1(t, testpkg.Ctx(t), db, module.InsertRequestChildOffering, retry))
	require.Equal(t, from, *retry.ValidFrom)
	require.Equal(t, until, *retry.ValidUntil)
}

func TestOwnerOfferingReplacementRestoresDeletedRowsOnFailure(t *testing.T) {
	t.Parallel()
	db, _, tenantID, childID, offeringID := setupChildOfferingTest(t, testpkg.SetupTestDB(t))
	module := api.NewEnrollmentBookingFixtureWithin(testpkg.WithinCurrentTenant)
	ctx := testpkg.Ctx(t)
	initial := &owner.RequestChildOffering{RequestChildID: childID, CareOfferingID: offeringID, SelectedDays: []string{"mon"}}
	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, module.InsertRequestChildOffering, initial))
	// The duplicate primary key fails the bulk insert after the old selection was deleted.
	failed := []*owner.RequestChildOffering{
		{ID: initial.ID, CareOfferingID: offeringID, SelectedDays: []string{"tue"}},
		{ID: initial.ID, CareOfferingID: offeringID, SelectedDays: []string{"wed"}},
	}
	err := testpkg.TenantCommand2(t, ctx, db, module.ReplaceRequestChildOfferings, childID, failed)
	require.ErrorContains(t, err, "failed to insert replacement request child offerings")
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		restored, err := testpkg.TenantQuery1(t, ctx, db, module.RequestChildOfferingHistory, childID)
		require.NoError(t, err)
		require.Len(t, restored, 1)
		require.Equal(t, initial.ID, restored[0].ID)
		require.Equal(t, []string{"mon"}, restored[0].SelectedDays)
		return nil
	}))
	replacement := &owner.RequestChildOffering{CareOfferingID: offeringID, SelectedDays: []string{"tue"}}
	require.NoError(t, testpkg.TenantCommand2(t, ctx, db, module.ReplaceRequestChildOfferings, childID, []*owner.RequestChildOffering{replacement}))
	require.Equal(t, childID, replacement.RequestChildID)
	require.Equal(t, owner.Date("2026-09-01"), *replacement.ValidFrom)
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		rows, err := testpkg.TenantQuery1(t, ctx, db, module.RequestChildOfferingHistory, childID)
		require.NoError(t, err)
		require.Len(t, rows, 1)
		require.Equal(t, []string{"tue"}, rows[0].SelectedDays)
		return nil
	}))
	require.NoError(t, testpkg.TenantCommand2(t, ctx, db, module.ReplaceRequestChildOfferings, childID, nil))
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		rows, err := testpkg.TenantQuery1(t, ctx, db, module.RequestChildOfferingHistory, childID)
		require.NoError(t, err)
		require.Empty(t, rows)
		return nil
	}))
}

func TestOwnerScheduledOfferingsPreserveHistoryAndRollbackSupersession(t *testing.T) {
	t.Parallel()
	db, _, tenantID, childID, offeringID := setupChildOfferingTest(t, testpkg.SetupTestDB(t))
	module := api.NewEnrollmentBookingFixtureWithin(testpkg.WithinCurrentTenant)
	ctx := testpkg.Ctx(t)
	initial := &owner.RequestChildOffering{RequestChildID: childID, CareOfferingID: offeringID, SelectedDays: []string{"mon"}}
	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, module.InsertRequestChildOffering, initial))
	future := &owner.RequestChildOffering{CareOfferingID: offeringID, SelectedDays: []string{"tue"}}
	require.NoError(t, testpkg.TenantCommand3(t, ctx, db, module.ScheduleRequestChildOfferings, childID, "2027-01-01", []*owner.RequestChildOffering{future}))
	require.Equal(t, owner.Date("2027-01-01"), *future.ValidFrom)
	require.Equal(t, owner.Date("2027-08-01"), *future.ValidUntil)
	snapshot := func() string {
		var encoded []byte
		require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
			rows, err := testpkg.TenantQuery1(t, ctx, db, module.RequestChildOfferingHistory, childID)
			require.NoError(t, err)
			require.Len(t, rows, 2)
			encoded, err = json.Marshal(rows)
			return err
		}))
		return string(encoded)
	}
	before := snapshot()
	// Fail the workflow after the owner has deleted future rows and capped history.
	rolledBack := &owner.RequestChildOffering{CareOfferingID: offeringID, SelectedDays: []string{"wed"}}
	require.ErrorContains(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		if err := testpkg.TenantCommand3(t, ctx, db, module.ScheduleRequestChildOfferings, childID, "2026-12-01", []*owner.RequestChildOffering{rolledBack}); err != nil {
			return err
		}
		return errors.New("injected failure after scheduling")
	}), "injected failure after scheduling")
	require.JSONEq(t, before, snapshot())
	replacement := &owner.RequestChildOffering{CareOfferingID: offeringID, SelectedDays: []string{"wed"}}
	require.NoError(t, testpkg.TenantCommand3(t, ctx, db, module.ScheduleRequestChildOfferings, childID, "2026-12-01", []*owner.RequestChildOffering{replacement}))
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		old, err := testpkg.TenantQuery2(t, ctx, db, module.RequestChildOfferingsAtDate, childID, "2026-11-30")
		require.NoError(t, err)
		require.Len(t, old, 1)
		require.Equal(t, []string{"mon"}, old[0].SelectedDays)
		changed, err := testpkg.TenantQuery2(t, ctx, db, module.RequestChildOfferingsAtDate, childID, "2026-12-01")
		require.NoError(t, err)
		require.Len(t, changed, 1)
		require.Equal(t, []string{"wed"}, changed[0].SelectedDays)
		return nil
	}))
}

func TestOwnerOfferingDateSelectionPreservesUpcomingAndGaps(t *testing.T) {
	t.Parallel()
	_, _, _, childID, offeringID := setupChildOfferingTest(t, testpkg.SetupTestDB(t))
	module := api.NewEnrollmentBookingFixtureWithin(testpkg.WithinCurrentTenant)
	ctx := testpkg.Ctx(t)
	start, end := owner.Date("2026-09-01"), owner.Date("2026-10-01")
	selection := &owner.RequestChildOffering{RequestChildID: childID, CareOfferingID: offeringID, ValidFrom: &start, ValidUntil: &end}
	require.NoError(t, testpkg.TenantCommand1(t, ctx, testpkg.SetupTestDB(t), module.InsertRequestChildOffering, selection))
	future := &owner.RequestChildOffering{CareOfferingID: offeringID}
	require.NoError(t, testpkg.TenantCommand3(t, ctx, testpkg.SetupTestDB(t), module.ScheduleRequestChildOfferings, childID, "2026-12-01", []*owner.RequestChildOffering{future}))
	for _, date := range []owner.Date{"2026-08-31", "2026-09-01", "2026-09-30"} {
		rows, err := testpkg.TenantQuery2(t, ctx, testpkg.SetupTestDB(t), module.RequestChildOfferingsAtDate, childID, date)
		require.NoError(t, err)
		require.Len(t, rows, 1)
		require.Equal(t, selection.ID, rows[0].ID)
	}
	for _, date := range []owner.Date{"2026-10-01", "2026-11-30", "2027-08-01"} {
		rows, err := testpkg.TenantQuery2(t, ctx, testpkg.SetupTestDB(t), module.RequestChildOfferingsAtDate, childID, date)
		require.NoError(t, err)
		require.Empty(t, rows)
	}
	rows, err := testpkg.TenantQuery2(t, ctx, testpkg.SetupTestDB(t), module.RequestChildOfferingsAtDate, childID, "2026-12-01")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, future.ID, rows[0].ID)
}

func TestOwnerApprovedOfferingSelectionsResolveStudentAndExcludeExpired(t *testing.T) {
	t.Parallel()
	db, _, _, childID, offeringID := setupChildOfferingTest(t, testpkg.SetupTestDB(t))
	module := api.NewEnrollmentBookingFixtureWithin(testpkg.WithinCurrentTenant)
	ctx := testpkg.Ctx(t)
	source, err := testpkg.TenantQuery1(t, ctx, db, module.ChildByID, childID)
	require.NoError(t, err)
	matched := testpkg.CreateTestStudent(t, db, "Matched", "Selection", "2a")
	created := testpkg.CreateTestStudent(t, db, "Created", "Selection", "3a")
	approved := &owner.RequestChild{RequestID: source.RequestID, FirstName: "Approved", LastName: "Selection", DateOfBirth: source.DateOfBirth, Status: owner.ChildStatusApproved, MatchedStudentID: &matched.ID, CreatedStudentID: &created.ID}
	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, module.InsertChild, approved))
	selection := &owner.RequestChildOffering{RequestChildID: approved.ID, CareOfferingID: offeringID}
	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, module.InsertRequestChildOffering, selection))
	pending := &owner.RequestChildOffering{RequestChildID: childID, CareOfferingID: offeringID}
	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, module.InsertRequestChildOffering, pending))
	// Future approved intervals remain relevant to roster resynchronization.
	rows, err := testpkg.TenantQuery2(t, ctx, db, module.ApprovedSelectionsForOfferings, []int64{offeringID}, "2026-08-01")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, created.ID, rows[0].StudentID)
	require.Equal(t, selection.ID, rows[0].Selection.ID)
	byStudent, err := testpkg.TenantQuery3(t, ctx, db, module.ApprovedSelectionsForStudents, []int64{created.ID}, "2026-08-01", "2026-09-01")
	require.NoError(t, err)
	require.Len(t, byStudent, 1)
	require.Equal(t, selection.ID, byStudent[0].Selection.ID)
	byStudent, err = testpkg.TenantQuery3(t, ctx, db, module.ApprovedSelectionsForStudents, []int64{matched.ID}, "2026-08-01", "2026-09-01")
	require.NoError(t, err)
	require.Empty(t, byStudent)
	byStudent, err = testpkg.TenantQuery3(t, ctx, db, module.ApprovedSelectionsForStudents, []int64{created.ID}, "2027-08-01", "2027-08-01")
	require.NoError(t, err)
	require.Empty(t, byStudent)

	rows, err = testpkg.TenantQuery2(t, ctx, db, module.ApprovedSelectionsForOfferings, []int64{offeringID}, "2027-08-01")
	require.NoError(t, err)
	require.Empty(t, rows)
}

func TestOwnerOfferingCapacityCountsIntervalsAndExclusions(t *testing.T) {
	t.Parallel()
	_, _, _, childID, offeringID := setupChildOfferingTest(t, testpkg.SetupTestDB(t))
	module := api.NewEnrollmentBookingFixtureWithin(testpkg.WithinCurrentTenant)
	ctx := testpkg.Ctx(t)
	initial := &owner.RequestChildOffering{RequestChildID: childID, CareOfferingID: offeringID}
	require.NoError(t, testpkg.TenantCommand1(t, ctx, testpkg.SetupTestDB(t), module.InsertRequestChildOffering, initial))
	future := &owner.RequestChildOffering{CareOfferingID: offeringID}
	require.NoError(t, testpkg.TenantCommand3(t, ctx, testpkg.SetupTestDB(t), module.ScheduleRequestChildOfferings, childID, "2026-12-01", []*owner.RequestChildOffering{future}))
	grades, err := testpkg.TenantQuery3(t, ctx, testpkg.SetupTestDB(t), module.OfferingGradeCounts, []int64{offeringID}, "2026-09-01", "2027-08-01")
	require.NoError(t, err)
	require.Len(t, grades, 1)
	require.Equal(t, offeringID, grades[0].CareOfferingID)
	require.Equal(t, 1, grades[0].Count)
	child, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), module.ChildByID, childID)
	require.NoError(t, err)
	require.Equal(t, child.TargetGradeLevel, grades[0].GradeLevel)
	// Materialization counts selection rows, including future intervals.
	count, err := testpkg.TenantQuery2(t, ctx, testpkg.SetupTestDB(t), module.MaterializableOfferingCount, offeringID, "2026-09-01")
	require.NoError(t, err)
	require.Equal(t, 2, count)
	count, err = testpkg.TenantQuery2(t, ctx, testpkg.SetupTestDB(t), module.MaterializableOfferingCount, offeringID, "2026-12-01")
	require.NoError(t, err)
	require.Equal(t, 1, count)
	// Adjacent intervals of the same child consume one place, not two.
	peak, err := testpkg.TenantQuery4(t, ctx, testpkg.SetupTestDB(t), module.OfferingCapacityPeak, offeringID, nil, "2026-09-01", "2027-08-01")
	require.NoError(t, err)
	require.Equal(t, 1, peak)
	peaks, err := testpkg.TenantQuery3(t, ctx, testpkg.SetupTestDB(t), module.OfferingCapacityPeaks, []int64{offeringID}, "2026-09-01", "2027-08-01")
	require.NoError(t, err)
	require.Equal(t, map[int64]int{offeringID: peak}, peaks)
	peaks, err = testpkg.TenantQuery3(t, ctx, testpkg.SetupTestDB(t), module.OfferingCapacityPeaks, []int64{offeringID}, "2027-08-01", "2027-08-02")
	require.NoError(t, err)
	require.Empty(t, peaks)
	peak, err = testpkg.TenantQuery4(t, ctx, testpkg.SetupTestDB(t), module.OfferingCapacityPeak, offeringID, []int64{childID}, "2026-09-01", "2027-08-01")
	require.NoError(t, err)
	require.Zero(t, peak)
	peak, err = testpkg.TenantQuery4(t, ctx, testpkg.SetupTestDB(t), module.OfferingCapacityPeak, offeringID, nil, "2027-08-01", "2027-08-02")
	require.NoError(t, err)
	require.Zero(t, peak)
	_, err = testpkg.TenantQuery4(t, ctx, testpkg.SetupTestDB(t), module.OfferingCapacityPeak, offeringID, nil, "2026-09-01", "2026-09-01")
	require.EqualError(t, err, "capacity range must not be empty")
}
