package enrollmenthttp_test

import (
	"encoding/json"
	"fmt"
	"testing"

	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"

	"github.com/moto-ogs/project-phoenix/api"
	enrollment "github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestEnrollmentExpiryRouteGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	f := newEnrollmentGrowthFixture(t)
	env := f.env
	ctx := testpkg.Ctx(t)
	f.resource.PhaseExpiryService = NewTestPhaseExpiryWarnings(newPhaseExpiryProjection(t, env.db))
	router := f.router(t)
	counter := testpkg.CaptureQueries(t, env.db)
	today := calendar.TodayDate()
	end := today.AddDays(7)
	start := today.AddDays(-100)
	ids := []string{}
	add := func(n int) {
		for range n {
			phase := *env.phase
			phase.ID = 0
			phase.Name = fmt.Sprintf("Expiring %02d", len(ids))
			phase.ServiceStartDate = enrollment.Date(start)
			phase.ServiceEndDate = enrollment.Date(end)
			require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, env.repos.Enrollment().InsertPhase, &phase))
			request := makeOwnerRequest(phase.ID, uniqueToken("expiry-growth"), fmt.Sprintf("expiry%d@example.test", len(ids)))
			require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, env.repos.Enrollment().InsertRequest, request))
			student := testpkg.CreateTestStudent(t, env.db, "Expiry", fmt.Sprintf("Child%02d", len(ids)), "2a")
			require.NoError(t, newExpiryStudentFixture(t, env.db).SetEnrollmentWindowByID(ctx, student.ID, start, "active"))
			child := makeChild(request.ID, "Expiry", "Child")
			child.Status = enrollment.ChildStatusApproved
			child.CreatedStudentID = &student.ID
			require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, env.repos.Enrollment().InsertChild, child))
			offering := makeOffering(phase.ID, uniqueOfferingName("expiry-growth"))
			offering.AvailableDays = []string{"mon", "tue", "wed", "thu", "fri"}
			require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, newCareOfferingFixtures(env.reposCare.CarePlan).Create, offering))
			until := end.AddDays(1)
			require.NoError(t, api.NewEnrollmentBookingFixtureWithin(testpkg.WithinCurrentTenant).InsertRequestChildOffering(ctx, &enrollment.RequestChildOffering{RequestChildID: child.ID, CareOfferingID: offering.ID, ValidFrom: offeringDatePointer(start), ValidUntil: offeringDatePointer(until), SelectedDays: offering.AvailableDays}))
			ids = append(ids, fmt.Sprint(phase.ID))
		}
	}
	run := func(n int) testpkg.QueryMeasurement {
		raw, q := growthGET(t, router, counter, f.token, "/phases/expiry-warnings")
		var rows []struct {
			ID         string `json:"source_phase_id"`
			Affected   int    `json:"affected_children"`
			Unresolved int    `json:"unresolved_children"`
			State      string `json:"state"`
		}
		require.NoError(t, json.Unmarshal(raw, &rows))
		require.Len(t, rows, n)
		for i, row := range rows {
			require.Equal(t, ids[i], row.ID)
			require.Equal(t, 1, row.Affected)
			require.Equal(t, 1, row.Unresolved)
			require.Equal(t, enrollment.PhaseExpiryStateMissingSuccessor, row.State)
		}
		return q
	}
	add(3)
	small := run(3)
	add(5)
	large := run(8)
	require.Equal(t, len(small.Statements), len(large.Statements))
	t.Logf("expiry N3=%d N8=%d", len(small.Statements), len(large.Statements))
	querybudgets.AssertQueryBudget(t, "api.enrollment.growth.expiry", large)
}
