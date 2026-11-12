package api

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/auth/authorize/permissions"
	"github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-ogs/project-phoenix/roots/owners"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOwnChildPlanBindingBatchesWrites(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	database := testpkg.NewDatabase(t, db)
	people := owners.MustNewPeopleDirectory(database)
	care, err := owners.NewUnobservedCarePlan(database, owners.MustNewUnobservedTimetable(database), people)
	require.NoError(t, err)
	plans := owners.NewHolidayOwnChildPlans(care)
	today := calendar.NewDate(2099, 7, 6)
	clock := time.Date(2000, 1, 1, 8, 0, 0, 0, time.UTC)
	counter := testpkg.CaptureQueriesForContext(t, db)
	ctx := counter.Context(testpkg.Ctx(t))
	inTenant := func(fn func(context.Context) error) {
		require.NoError(t, tenant.NewTransactionRunner().RunInTx(ctx, fn))
	}
	var booked []enrollment.HolidayOwnChildPlan
	for _, count := range []int{3, 8} {
		for child := len(booked) / 5; child < count; child++ {
			student := testpkg.CreateTestStudent(t, db, fmt.Sprintf("Kind %d", child), "Ferien", "2b")
			for offset := range 5 {
				booked = append(booked, enrollment.HolidayOwnChildPlan{StudentID: student.ID, Day: today.AddDays(offset), Time: clock, Reason: enrollment.HolidayOwnChildCareReason})
			}
		}
		counter.Reset()
		inTenant(func(ctx context.Context) error {
			return plans.ApplyHolidayOwnChildPlans(ctx, enrollment.HolidayOwnChildPlanChanges{Arrivals: booked, Pickups: booked})
		})
		assert.Equal(t, 2, len(counter.Operation("INSERT")), "N%d: one insert per exception table", count)
		var removals enrollment.HolidayOwnChildPlanChanges
		inTenant(func(ctx context.Context) error {
			arrivals, err := plans.HolidayOwnChildArrivals(ctx, today)
			if err != nil {
				return err
			}
			pickups, err := plans.HolidayOwnChildPickups(ctx, today)
			if err != nil {
				return err
			}
			require.Len(t, arrivals, count*5)
			require.Len(t, pickups, count*5)
			for _, row := range arrivals {
				removals.DeleteArrivals = append(removals.DeleteArrivals, row.ID)
			}
			for _, row := range pickups {
				removals.DeletePickups = append(removals.DeletePickups, row.ID)
			}
			return nil
		})
		counter.Reset()
		inTenant(func(ctx context.Context) error { return plans.ApplyHolidayOwnChildPlans(ctx, removals) })
		assert.Equal(t, 2, len(counter.Operation("DELETE")), "N%d: one delete per exception table", count)
	}
}

// The booking is the author of an own child's planned day (#6730): an
// account without a staff record at the hosting school, such as the
// Träger-Büro, picks the group and the days are planned all the same.
func TestOwnChildDaysArePlannedWhenAnAccountWithoutStaffRecordPicksTheGroup(t *testing.T) {
	t.Parallel()
	f := newHolidayGuestFixture(t, "Träger Ohne Verfasser")
	lia := f.liaOfNord(t, "Lia")
	f.confirm(t, f.siblings, "confirm")
	account := testpkg.CreateTestAccount(t, f.db, "ferien-leitung-ohne-personal")
	token := testpkg.MintTestJWT(t, sessiontoken.AppClaims{
		ID: int(account.ID), Sub: account.Email, TenantID: f.nord, Roles: []string{"admin"},
		Permissions: []string{permissions.ConfigRead, permissions.UsersCreate, permissions.UsersRead},
	})
	rec := f.assign(t, token, f.offer.Weeks[0].ID, f.group)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	transfer := decodeSchoolPortalData[enrollment.HolidayGuestTransfer](t, rec.Body.Bytes())
	assert.Equal(t, 1, transfer.OwnChildren)
	assert.Len(t, f.ownChildDays(t, lia), 5)
	var sources []string
	require.NoError(t, f.db.NewRaw(`SELECT DISTINCT source || ':' || COALESCE(created_by::text, '') FROM schedule.student_pickup_exceptions WHERE student_id = ?`, lia).
		Scan(t.Context(), &sources))
	assert.Equal(t, []string{"holiday_booking:"}, sources, "the booking is the author, no staff member")
}
