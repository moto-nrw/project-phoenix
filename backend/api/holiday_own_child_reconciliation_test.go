package api

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
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
	staff, err := NewRFIDTestPeople(database)
	require.NoError(t, err)
	plans, authors := owners.NewHolidayOwnChildPlans(care, staff)
	writer, account := testpkg.CreateTestStaffWithAccount(t, db, "Ferien", "Planung")
	today := calendar.NewDate(2099, 7, 6)
	clock := time.Date(2000, 1, 1, 8, 0, 0, 0, time.UTC)
	counter := testpkg.CaptureQueriesForContext(t, db)
	ctx := counter.Context(testpkg.Ctx(t))
	inTenant := func(fn func(context.Context) error) {
		require.NoError(t, tenant.NewTransactionRunner().RunInTx(ctx, fn))
	}
	var booked []enrollment.HolidayOwnChildPlan
	var readCount int
	for _, count := range []int{3, 8} {
		for child := len(booked) / 5; child < count; child++ {
			student := testpkg.CreateTestStudent(t, db, fmt.Sprintf("Kind %d", child), "Ferien", "2b")
			for offset := range 5 {
				booked = append(booked, enrollment.HolidayOwnChildPlan{StudentID: student.ID, Day: today.AddDays(offset), StaffID: writer.ID, Time: clock, Reason: enrollment.HolidayOwnChildCareReason})
			}
		}
		counter.Reset()
		inTenant(func(ctx context.Context) error {
			resolved, err := authors.HolidayOwnChildAuthors(ctx)
			assert.Equal(t, writer.ID, resolved[account.ID])
			return err
		})
		if readCount == 0 {
			readCount = len(counter.Operation("SELECT"))
		}
		assert.Equal(t, readCount, len(counter.Operation("SELECT")), "staff resolution stays flat")
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

func TestOwnChildrenOnlyWeekRejectsAGroupChoiceWithoutAStaffAuthor(t *testing.T) {
	t.Parallel()
	f := newHolidayGuestFixture(t, "Träger Ohne Verfasser")
	lia := f.liaOfNord(t, "Lia")
	f.confirm(t, f.siblings, "confirm")
	// Both siblings belong to the host: there is no guest whose planning
	// could hide the missing author of the school's own children.
	_, err := f.db.ExecContext(t.Context(), `UPDATE enrollment.holiday_registration_children SET home_school_id = ?, guest_school_name = '' WHERE holiday_registration_id = ?`, f.nord, f.siblings)
	require.NoError(t, err)
	account := testpkg.CreateTestAccount(t, f.db, "ferien-leitung-ohne-personal")
	token := testpkg.MintTestJWT(t, sessiontoken.AppClaims{
		ID: int(account.ID), Sub: account.Email, TenantID: f.nord, Roles: []string{"admin"},
		Permissions: []string{permissions.ConfigRead, permissions.UsersCreate, permissions.UsersRead},
	})
	for range 2 {
		rec := f.assign(t, token, f.offer.Weeks[0].ID, f.group)
		require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), `"code":"enrollment.holiday_plan_author_missing"`)
	}
	assert.Empty(t, f.ownChildDays(t, lia))
	registrations := f.registration(t, token, lia)
	require.Len(t, registrations, 1)
	assert.False(t, registrations[0].Planned, "a failed group choice must not make the UI claim the child is planned")
	lead := f.staffOf(t, f.nord, permissions.ConfigRead, permissions.UsersCreate, permissions.UsersRead)
	rec := f.assign(t, lead, f.offer.Weeks[0].ID, f.group)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	transfer := decodeSchoolPortalData[enrollment.HolidayGuestTransfer](t, rec.Body.Bytes())
	assert.Equal(t, 1, transfer.OwnChildren)
	assert.Len(t, f.ownChildDays(t, lia), 5)
}
