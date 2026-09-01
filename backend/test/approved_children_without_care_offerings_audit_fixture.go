package test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type approvedWithoutOfferingAuditRow struct {
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

type approvedWithoutOfferingScenario struct {
	booked, missing, outOfPeriod, requiredOnly int64
}

func verifyApprovedChildrenWithoutCareOfferingsAudit(t *testing.T) {
	db := SetupTestDB(t)
	ctx := Ctx(t)
	phase := CreateTestEnrollmentPhase(t, db)
	phase.CareOfferingSelectionMode = enrollmentModels.PhaseCareOfferingSelectionAtLeastOne
	updateApprovedWithoutOfferingModel(t, db, ctx, phase, "care_offering_selection_mode")
	primary := seedApprovedWithoutOfferingScenario(t, db, ctx, phase)

	other := NewTenantScope(t, db)
	otherPhase := createApprovedWithoutOfferingPhase(t, db, other.Context(), other.TenantID)
	otherRequest := createApprovedWithoutOfferingRequest(t, db, other.Context(), otherPhase.ID, other.TenantID)
	otherMissing := createApprovedWithoutOfferingChild(t, db, other.Context(), otherRequest.ID, other.TenantID, "Andere")

	rows := runApprovedWithoutOfferingAudit(t, db, ctx)
	assertApprovedWithoutOfferingTenantRows(t, rows, Tenant(t), primary)
	require.Len(t, approvedWithoutOfferingRowsForTenant(rows, other.TenantID), 1)
	assert.Equal(t, otherMissing.ID, approvedWithoutOfferingRowsForTenant(rows, other.TenantID)[0].RequestChildID)
}

func seedApprovedWithoutOfferingScenario(
	t *testing.T,
	db *bun.DB,
	ctx context.Context,
	phase *enrollmentModels.Phase,
) approvedWithoutOfferingScenario {
	t.Helper()
	request := createApprovedWithoutOfferingRequest(t, db, ctx, phase.ID, phase.TenantID)
	booked := createApprovedWithoutOfferingChild(t, db, ctx, request.ID, phase.TenantID, "Gebucht")
	missing := createApprovedWithoutOfferingChild(t, db, ctx, request.ID, phase.TenantID, "Fehlend")
	outOfPeriod := createApprovedWithoutOfferingChild(t, db, ctx, request.ID, phase.TenantID, "Ausserhalb")
	requiredOnly := createApprovedWithoutOfferingChild(t, db, ctx, request.ID, phase.TenantID, "Nur Pflichtangebot")
	offering := &enrollmentModels.CareOffering{
		PhaseID: phase.ID, Name: "Ganztag", DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays: []string{"mon", "tue", "wed", "thu", "fri"}, IsActive: true, CountsAsCare: true,
	}
	offering.SetTenantID(phase.TenantID)
	insertApprovedWithoutOfferingModel(t, db, ctx, offering)
	bookedLink := &enrollmentModels.RequestChildOffering{
		RequestChildID: booked.ID, CareOfferingID: offering.ID,
	}
	bookedLink.SetTenantID(phase.TenantID)
	insertApprovedWithoutOfferingModel(t, db, ctx, bookedLink)
	afterPhase, afterPhaseUntil := phase.ServiceEndDate.AddDays(1), phase.ServiceEndDate.AddDays(31)
	outOfPeriodLink := &enrollmentModels.RequestChildOffering{
		RequestChildID: outOfPeriod.ID, CareOfferingID: offering.ID,
		ValidFrom: &afterPhase, ValidUntil: &afterPhaseUntil,
	}
	outOfPeriodLink.SetTenantID(phase.TenantID)
	insertApprovedWithoutOfferingModel(t, db, ctx, outOfPeriodLink)
	required := &enrollmentModels.CareOffering{
		PhaseID: phase.ID, Name: "Verpflichtende Betreuung", DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays: []string{"mon", "tue", "wed", "thu", "fri"}, IsActive: true, IsRequired: true, CountsAsCare: true,
	}
	required.SetTenantID(phase.TenantID)
	insertApprovedWithoutOfferingModel(t, db, ctx, required)
	requiredLink := &enrollmentModels.RequestChildOffering{
		RequestChildID: requiredOnly.ID, CareOfferingID: required.ID,
	}
	requiredLink.SetTenantID(phase.TenantID)
	insertApprovedWithoutOfferingModel(t, db, ctx, requiredLink)
	return approvedWithoutOfferingScenario{booked.ID, missing.ID, outOfPeriod.ID, requiredOnly.ID}
}

func createApprovedWithoutOfferingPhase(
	t *testing.T,
	db *bun.DB,
	ctx context.Context,
	tenantID int64,
) *enrollmentModels.Phase {
	t.Helper()
	phase := &enrollmentModels.Phase{
		Name: "Other tenant audit", Kind: enrollmentModels.PhaseKindSchoolYear,
		ServiceStartDate: timezone.NewDate(2025, time.December, 6), ServiceEndDate: timezone.NewDate(2026, time.November, 1),
		CareOverflowMode:          enrollmentModels.PhaseCareOverflowWaitlist,
		CareOfferingSelectionMode: enrollmentModels.PhaseCareOfferingSelectionOptional, IsActive: true,
	}
	phase.SetTenantID(tenantID)
	insertApprovedWithoutOfferingModel(t, db, ctx, phase)
	return phase
}

func createApprovedWithoutOfferingRequest(
	t *testing.T,
	db *bun.DB,
	ctx context.Context,
	phaseID, tenantID int64,
) *enrollmentModels.Request {
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
	request.SetTenantID(tenantID)
	insertApprovedWithoutOfferingModel(t, db, ctx, request)
	return request
}

func createApprovedWithoutOfferingChild(
	t *testing.T,
	db *bun.DB,
	ctx context.Context,
	requestID int64,
	tenantID int64,
	firstName string,
) *enrollmentModels.RequestChild {
	t.Helper()
	child := &enrollmentModels.RequestChild{
		RequestID: requestID, FirstName: firstName, LastName: "Audit",
		DateOfBirth: timezone.NewDate(2018, 4, 15), CustomData: map[string]any{},
		Status: enrollmentModels.ChildStatusApproved, ActivationMode: enrollmentModels.ChildActivationScheduled,
	}
	child.SetTenantID(tenantID)
	insertApprovedWithoutOfferingModel(t, db, ctx, child)
	return child
}

func runApprovedWithoutOfferingAudit(t *testing.T, db *bun.DB, ctx context.Context) []approvedWithoutOfferingAuditRow {
	t.Helper()
	query, err := os.ReadFile("../database/audits/approved_children_without_care_offerings.sql")
	require.NoError(t, err)
	var rows []approvedWithoutOfferingAuditRow
	require.NoError(t, db.NewRaw(string(query)).Scan(ctx, &rows))
	return rows
}

func insertApprovedWithoutOfferingModel(t *testing.T, db *bun.DB, ctx context.Context, model any) {
	t.Helper()
	_, err := db.NewInsert().Model(model).ModelTableExpr(auditFixtureTable(model)).Exec(ctx)
	require.NoError(t, err)
}

func updateApprovedWithoutOfferingModel(t *testing.T, db *bun.DB, ctx context.Context, model any, columns ...string) {
	t.Helper()
	_, err := db.NewUpdate().Model(model).ModelTableExpr(auditFixtureUpdateTable(model)).Column(columns...).WherePK().Exec(ctx)
	require.NoError(t, err)
}

func approvedWithoutOfferingRowsForTenant(rows []approvedWithoutOfferingAuditRow, tenantID int64) []approvedWithoutOfferingAuditRow {
	var tenantRows []approvedWithoutOfferingAuditRow
	for _, row := range rows {
		if row.TenantID == tenantID {
			tenantRows = append(tenantRows, row)
		}
	}
	return tenantRows
}

func assertApprovedWithoutOfferingTenantRows(
	t *testing.T,
	rows []approvedWithoutOfferingAuditRow,
	tenantID int64,
	scenario approvedWithoutOfferingScenario,
) {
	t.Helper()
	tenantRows := approvedWithoutOfferingRowsForTenant(rows, tenantID)
	childIDs := make([]int64, 0, len(tenantRows))
	for _, row := range tenantRows {
		childIDs = append(childIDs, row.RequestChildID)
		assert.Equal(t, "missing_required", row.Finding)
	}
	assert.ElementsMatch(t, []int64{scenario.missing, scenario.outOfPeriod, scenario.requiredOnly}, childIDs)
	assert.NotContains(t, childIDs, scenario.booked)
}
