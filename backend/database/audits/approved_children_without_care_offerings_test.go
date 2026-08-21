package audits_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestApprovedChildrenWithoutCareOfferingsAudit(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	repos := repositories.NewFactory(db)
	phase := testpkg.CreateTestEnrollmentPhase(t, db)
	request := &enrollmentModels.Request{
		PhaseID:           phase.ID,
		GuardianFirstName: "Anna",
		GuardianLastName:  "Audit",
		GuardianEmail:     fmt.Sprintf("offering-audit-%d@example.test", testpkg.Tenant(t)),
		ConsentFlags:      map[string]any{},
		CustomData:        map[string]any{},
		SubmissionSource:  enrollmentModels.RequestSourcePublic,
		SourceMetadata:    map[string]any{},
		StatusToken:       fmt.Sprintf("offering-audit-%d-%d", testpkg.Tenant(t), time.Now().UnixNano()),
		SubmittedAt:       time.Now(),
	}
	require.NoError(t, repos.Request.Create(ctx, request))

	createApprovedChild := func(firstName string) *enrollmentModels.RequestChild {
		child := &enrollmentModels.RequestChild{
			RequestID:      request.ID,
			FirstName:      firstName,
			LastName:       "Audit",
			DateOfBirth:    timezone.NewDate(2018, 4, 15),
			CustomData:     map[string]any{},
			Status:         enrollmentModels.ChildStatusApproved,
			ActivationMode: enrollmentModels.ChildActivationScheduled,
		}
		require.NoError(t, repos.RequestChild.Create(ctx, child))
		return child
	}
	booked := createApprovedChild("Gebucht")
	missing := createApprovedChild("Fehlend")
	outOfPeriod := createApprovedChild("Ausserhalb")
	offering := testpkg.CreateTestCareOffering(t, db, phase.ID, "Ganztag")
	require.NoError(t, repos.RequestChildOffering.Create(ctx, &enrollmentModels.RequestChildOffering{
		RequestChildID: booked.ID,
		CareOfferingID: offering.ID,
	}))
	afterPhase := phase.ServiceEndDate.AddDays(1)
	afterPhaseUntil := afterPhase.AddDays(30)
	require.NoError(t, repos.RequestChildOffering.Create(ctx, &enrollmentModels.RequestChildOffering{
		RequestChildID: outOfPeriod.ID,
		CareOfferingID: offering.ID,
		ValidFrom:      &afterPhase,
		ValidUntil:     &afterPhaseUntil,
	}))

	query, err := os.ReadFile("approved_children_without_care_offerings.sql")
	require.NoError(t, err)
	var rows []struct {
		TenantID                  int64      `bun:"tenant_id"`
		SchoolName                string     `bun:"school_name"`
		PhaseID                   int64      `bun:"phase_id"`
		PhaseName                 string     `bun:"phase_name"`
		CareOfferingSelectionMode string     `bun:"care_offering_selection_mode"`
		RequestID                 int64      `bun:"request_id"`
		RequestChildID            int64      `bun:"request_child_id"`
		CreatedStudentID          *int64     `bun:"created_student_id"`
		ReviewedAt                *time.Time `bun:"reviewed_at"`
		Finding                   string     `bun:"finding"`
	}
	require.NoError(t, db.NewRaw(string(query)).Scan(ctx, &rows))

	var tenantRows []int64
	for _, row := range rows {
		if row.TenantID == testpkg.Tenant(t) {
			tenantRows = append(tenantRows, row.RequestChildID)
			assert.Equal(t, "review_optional", row.Finding)
		}
	}
	assert.ElementsMatch(t, []int64{missing.ID, outOfPeriod.ID}, tenantRows)
	assert.NotContains(t, tenantRows, booked.ID)
}
