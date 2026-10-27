package compose_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestParentConferenceListsQueryBudget guards both Elternsprechtag lists
// (#4801) against per-day and per-booking N+1: the Lehrkraft's occupancy
// and the family's offers read days, bookings and names in one batch each.
func TestParentConferenceListsQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	service := setupParentConferenceService(t, db, time.Time{})
	lehrkraft, account := createTestLehrkraft(t, db, "Budget", "Sprechtag")
	testpkg.CreateTestClassTeacher(t, db, lehrkraft.ID, "1a")
	family := testpkg.CreateTestParentGuardianChain(t, db)

	days := 0
	addDays := func(n int) {
		for range n {
			req := conferenceDay("1a")
			req.Date = portalDate(2026, 3, 4).AddDays(days)
			view := createConference(t, db, service, account.ID, req)
			other := testpkg.CreateTestParentGuardianChain(t, db)
			_, err := testpkg.TenantQuery3(t, testpkg.Ctx(t), db, service.BookParentConference, conferenceChild(other), conferenceID(t, view), "14:00")
			require.NoError(t, err, fmt.Sprintf("day %d", days))
			days++
		}
	}

	counter := testpkg.CaptureQueries(t, db)
	teacherRun := func() int {
		counter.Reset()
		require.NoError(t, testpkg.WithinTenantContext(t, calendarContext(t, account.ID), db, testpkg.Tenant(t), func(ctx context.Context) error {
			list, err := service.ListSchoolParentConferences(ctx)
			require.NoError(t, err)
			require.Len(t, list.Conferences, days)
			return nil
		}))
		return counter.Total()
	}
	familyRun := func() int {
		counter.Reset()
		require.NoError(t, testpkg.WithinTenantContext(t, testpkg.Ctx(t), db, testpkg.Tenant(t), func(ctx context.Context) error {
			offers, err := service.ListChildParentConferences(ctx, conferenceChild(family))
			require.NoError(t, err)
			require.Len(t, offers, days)
			return nil
		}))
		return counter.Total()
	}

	addDays(1)
	smallTeacher, smallFamily := teacherRun(), familyRun()
	addDays(3)
	largeTeacher, largeFamily := teacherRun(), familyRun()
	assert.Equal(t, smallTeacher, largeTeacher, "the Lehrkraft's list does not grow with its days")
	assert.Equal(t, smallFamily, largeFamily, "the family's offers do not grow with the days")

	teacherRun()
	testpkg.AssertQueryBudget(t, "services.calendar.list_school_parent_conferences", counter.Queries())
	familyRun()
	testpkg.AssertQueryBudget(t, "services.calendar.list_child_parent_conferences", counter.Queries())
}
