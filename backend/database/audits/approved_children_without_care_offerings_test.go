package audits_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type approvedWithoutOfferingRow struct {
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

type auditScenario struct {
	booked, missing, outOfPeriod int64
}

func TestApprovedChildrenWithoutCareOfferingsAudit(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	repos := repositories.NewFactory(db)
	phase := testpkg.CreateTestEnrollmentPhase(t, db)
	primary := seedAuditScenario(t, repos, ctx, phase)

	other := testpkg.NewTenantScope(t, db)
	otherPhase := createAuditPhase(t, repos, other.Context(), other.TenantID)
	otherMissing := createApprovedAuditChild(t, repos, other.Context(), createAuditRequest(t, repos, other.Context(), otherPhase.ID, other.TenantID).ID, "Andere")

	rows := runApprovedWithoutOfferingAudit(t, db, ctx)
	assertAuditTenantRows(t, rows, testpkg.Tenant(t), primary)
	require.Len(t, rowsForTenant(rows, other.TenantID), 1)
	assert.Equal(t, otherMissing.ID, rowsForTenant(rows, other.TenantID)[0].RequestChildID)
}

func seedAuditScenario(t *testing.T, repos *repositories.Factory, ctx context.Context, phase *enrollmentModels.Phase) auditScenario {
	t.Helper()
	request := createAuditRequest(t, repos, ctx, phase.ID, phase.TenantID)
	booked := createApprovedAuditChild(t, repos, ctx, request.ID, "Gebucht")
	missing := createApprovedAuditChild(t, repos, ctx, request.ID, "Fehlend")
	outOfPeriod := createApprovedAuditChild(t, repos, ctx, request.ID, "Ausserhalb")
	offering := &enrollmentModels.CareOffering{
		PhaseID: phase.ID, Name: "Ganztag", DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays: []string{"mon", "tue", "wed", "thu", "fri"}, IsActive: true, CountsAsCare: true,
	}
	require.NoError(t, repos.CareOffering.Create(ctx, offering))
	require.NoError(t, repos.RequestChildOffering.Create(ctx, &enrollmentModels.RequestChildOffering{RequestChildID: booked.ID, CareOfferingID: offering.ID}))
	afterPhase, afterPhaseUntil := phase.ServiceEndDate.AddDays(1), phase.ServiceEndDate.AddDays(31)
	require.NoError(t, repos.RequestChildOffering.Create(ctx, &enrollmentModels.RequestChildOffering{
		RequestChildID: outOfPeriod.ID, CareOfferingID: offering.ID, ValidFrom: &afterPhase, ValidUntil: &afterPhaseUntil,
	}))
	return auditScenario{booked.ID, missing.ID, outOfPeriod.ID}
}

func createAuditPhase(t *testing.T, repos *repositories.Factory, ctx context.Context, tenantID int64) *enrollmentModels.Phase {
	t.Helper()
	phase := &enrollmentModels.Phase{
		Name: "Other tenant audit", Kind: enrollmentModels.PhaseKindSchoolYear,
		ServiceStartDate: timezone.TodayDate().AddDays(-30), ServiceEndDate: timezone.TodayDate().AddDays(300),
		CareOverflowMode:          enrollmentModels.PhaseCareOverflowWaitlist,
		CareOfferingSelectionMode: enrollmentModels.PhaseCareOfferingSelectionOptional, IsActive: true,
	}
	phase.SetTenantID(tenantID)
	require.NoError(t, repos.Phase.Create(ctx, phase))
	return phase
}

func createAuditRequest(t *testing.T, repos *repositories.Factory, ctx context.Context, phaseID, tenantID int64) *enrollmentModels.Request {
	t.Helper()
	request := &enrollmentModels.Request{
		PhaseID:           phaseID,
		GuardianFirstName: "Anna",
		GuardianLastName:  "Audit",
		GuardianEmail:     fmt.Sprintf("offering-audit-%d@example.test", tenantID),
		ConsentFlags:      map[string]any{},
		CustomData:        map[string]any{},
		SubmissionSource:  enrollmentModels.RequestSourcePublic,
		SourceMetadata:    map[string]any{},
		StatusToken:       fmt.Sprintf("offering-audit-%d-%d", tenantID, time.Now().UnixNano()),
		SubmittedAt:       time.Now(),
	}
	require.NoError(t, repos.Request.Create(ctx, request))
	return request
}

func createApprovedAuditChild(t *testing.T, repos *repositories.Factory, ctx context.Context, requestID int64, firstName string) *enrollmentModels.RequestChild {
	t.Helper()
	child := &enrollmentModels.RequestChild{
		RequestID: requestID, FirstName: firstName, LastName: "Audit",
		DateOfBirth: timezone.NewDate(2018, 4, 15), CustomData: map[string]any{},
		Status: enrollmentModels.ChildStatusApproved, ActivationMode: enrollmentModels.ChildActivationScheduled,
	}
	require.NoError(t, repos.RequestChild.Create(ctx, child))
	return child
}

func runApprovedWithoutOfferingAudit(t *testing.T, db *bun.DB, ctx context.Context) []approvedWithoutOfferingRow {
	t.Helper()
	query, err := os.ReadFile("approved_children_without_care_offerings.sql")
	require.NoError(t, err)
	var rows []approvedWithoutOfferingRow
	require.NoError(t, db.NewRaw(string(query)).Scan(ctx, &rows))
	return rows
}

func rowsForTenant(rows []approvedWithoutOfferingRow, tenantID int64) []approvedWithoutOfferingRow {
	var tenantRows []approvedWithoutOfferingRow
	for _, row := range rows {
		if row.TenantID == tenantID {
			tenantRows = append(tenantRows, row)
		}
	}
	return tenantRows
}

func assertAuditTenantRows(t *testing.T, rows []approvedWithoutOfferingRow, tenantID int64, scenario auditScenario) {
	t.Helper()
	tenantRows := rowsForTenant(rows, tenantID)
	childIDs := make([]int64, 0, len(tenantRows))
	for _, row := range tenantRows {
		childIDs = append(childIDs, row.RequestChildID)
		assert.Equal(t, "review_optional", row.Finding)
	}
	assert.ElementsMatch(t, []int64{scenario.missing, scenario.outOfPeriod}, childIDs)
	assert.NotContains(t, childIDs, scenario.booked)
}
