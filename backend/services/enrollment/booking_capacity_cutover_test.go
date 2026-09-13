package enrollment_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanTest "github.com/moto-nrw/project-phoenix/modules/careplan/careplantest"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentTest "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestBookingCapacityCutoverPreservesBoundariesAndTenantIsolation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	bookings := carePlanTest.NewOfferingBookings()
	projection := repositories.NewEnrollmentBookingProjection(enrollmentTest.New())
	type school struct {
		ctx                               context.Context
		phaseID, offeringID, firstChildID int64
	}
	var schools []school
	for _, name := range []string{"first", "second"} {
		t.Run(name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			ctx := testpkg.Ctx(t)
			phaseID, requestID, first := testpkg.CreateAuditAdjustmentChain(t, db)
			offering := testpkg.CreateTestCareOffering(t, db, phaseID, "Capacity boundary")
			for i, fixture := range []struct {
				status      string
				grade       *int16
				from, until *careplan.Date
			}{
				{"approved", new(int16(1)), nil, new(careplan.Date("2030-01-03"))},
				{"approved", new(int16(1)), new(careplan.Date("2030-01-02")), new(careplan.Date("2030-01-03"))},
				{"approved", nil, new(careplan.Date("2030-01-03")), new(careplan.Date("2030-01-04"))},
				{"waitlisted", new(int16(2)), new(careplan.Date("2030-01-02")), new(careplan.Date("2030-01-04"))},
				{"rejected", nil, nil, nil},
				{"withdrawn", nil, nil, nil},
			} {
				childID := first
				if i > 0 {
					childID = testpkg.CreateAuditAdjustmentChild(t, db, requestID)
				}
				_, err := db.NewRaw(`UPDATE enrollment.request_children SET status = ?, target_grade_level = ? WHERE tenant_id = ? AND id = ?`, fixture.status, fixture.grade, testpkg.Tenant(t), childID).Exec(ctx)
				require.NoError(t, err)
				require.NoError(t, bookings.RecordCareOfferingBookings(ctx, childID, []careplan.CareOfferingBooking{{CareOfferingID: offering.ID, ManualSelectedDays: []string{"mon"}, ValidFrom: fixture.from, ValidUntil: fixture.until}}))
			}
			// An adjacent interval must not count the same child twice at the switch.
			require.NoError(t, bookings.RecordCareOfferingBookings(ctx, first, []careplan.CareOfferingBooking{{CareOfferingID: offering.ID, ManualSelectedDays: []string{"wed"}, ValidFrom: new(careplan.Date("2030-01-03"))}}))
			schools = append(schools, school{ctx, phaseID, offering.ID, first})
		})
	}
	for i, own := range schools {
		foreign := schools[1-i]
		for _, window := range []struct {
			from, until enrollment.Date
			want        int
		}{
			{"2030-01-01", "2030-01-02", 1},
			{"2030-01-02", "2030-01-03", 3},
			{"2030-01-03", "2030-01-04", 3},
			{"2030-01-04", "2030-01-05", 1},
		} {
			peak, err := projection.OfferingCapacityPeak(own.ctx, own.offeringID, nil, window.from, window.until)
			require.NoError(t, err)
			require.Equal(t, window.want, peak)
		}
		peaks, err := projection.OfferingCapacityPeaks(own.ctx, []int64{own.offeringID, foreign.offeringID}, "2030-01-01", "2030-01-05")
		require.NoError(t, err)
		require.Equal(t, map[int64]int{own.offeringID: 3}, peaks)
		peak, err := projection.OfferingCapacityPeak(own.ctx, own.offeringID, []int64{own.firstChildID}, "2030-01-01", "2030-01-05")
		require.NoError(t, err)
		require.Equal(t, 2, peak)
		grades, err := projection.OfferingGradeCounts(own.ctx, []int64{own.offeringID, foreign.offeringID}, "2030-01-01", "2030-01-05")
		require.NoError(t, err)
		require.Equal(t, []*enrollment.OfferingGradeCount{
			{CareOfferingID: own.offeringID, GradeLevel: new(int16(1)), Count: 2},
			{CareOfferingID: own.offeringID, GradeLevel: new(int16(2)), Count: 1},
			{CareOfferingID: own.offeringID, Count: 1},
		}, grades)
		count, err := projection.MaterializableOfferingCount(own.ctx, own.offeringID, "2030-01-03")
		require.NoError(t, err)
		require.Equal(t, 3, count)
		state, err := projection.OfferingCatalogState(own.ctx, own.phaseID, own.firstChildID, "2030-01-03", "2030-01-05")
		require.NoError(t, err)
		require.Equal(t, peaks, state.CapacityPeaks)
		require.Len(t, state.Current, 1)
		require.Equal(t, []string{"wed"}, state.Current[0].SelectedDays)
	}
}
