package services

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPhaseResponseOverviewQueryBudget keeps the GET
// /enrollment/phases/{id}/responses read path flat as the live roster grows.
// Every source is a bulk read keyed by the roster IDs, so three and eight
// children must issue the same statements inside the request transaction.
func TestPhaseResponseOverviewQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	persons, err := repositories.NewPeopleDirectory(db)
	require.NoError(t, err)
	phaseService := enrollmentService.NewPhaseService(enrollmentService.PhaseServiceConfig{
		Owner:            repos.Enrollment(),
		CareOfferingRepo: repos.CareOffering,
		Responses:        newPhaseResponseSources(repos.Enrollment(), persons, repos.CarePlan()),
		DB:               db,
		Logger:           slog.Default(),
		Today:            func() timezone.Date { return timezone.NewDate(2026, 1, 15) },
	})
	phase, err := phaseService.Create(testpkg.Ctx(t), &enrollmentOwner.Phase{
		TenantID:         tenantID,
		Name:             "response-query-budget",
		Kind:             enrollmentOwner.PhaseKindSchoolYear,
		Audience:         enrollmentOwner.PhaseAudienceExistingStudents,
		ServiceStartDate: enrollmentOwner.Date(timezone.NewDate(2026, 8, 1)),
		ServiceEndDate:   enrollmentOwner.Date(timezone.NewDate(2027, 7, 31)),
		IsActive:         true,
		CareOverflowMode: enrollmentOwner.PhaseCareOverflowWaitlist,
	})
	require.NoError(t, err)

	addStudents := func(start, count int) {
		t.Helper()
		for i := start; i < start+count; i++ {
			testpkg.CreateTestStudentForTenant(t, db, tenantID, "Rücklauf", fmt.Sprintf("Budget-%d", i), "1a")
		}
	}
	addStudents(0, 3)

	counter := testpkg.CaptureQueriesForContext(t, db)
	run := func() []string {
		counter.Reset()
		err := tenant.WithinCurrentTenant(counter.Context(testpkg.Ctx(t)), func(ctx context.Context) error {
			_, err := phaseService.ResponseOverview(ctx, phase.ID)
			return err
		})
		require.NoError(t, err)
		return counter.Queries()
	}

	small := run()
	addStudents(3, 5)
	large := run()

	t.Logf("query budget: 3 children → %d statements, 8 children → %d statements", len(small), len(large))
	assert.Equal(t, len(small), len(large), "response overview queries must not grow with the roster")
	testpkg.AssertQueryBudget(t, "api.enrollment.phase_responses.list", large)
}
