package test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"

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
	require.NoError(t, enrollmentOwner.New().UpdatePhase(WithTenantRuntime(t, ctx, db), phase))
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
	phase *enrollmentOwner.Phase,
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
	offering.TenantID = phase.TenantID
	InsertTestCareOffering(t, db, ctx, offering)
	bookedLink := &enrollmentModels.RequestChildOffering{
		RequestChildID: booked.ID, CareOfferingID: offering.ID,
	}
	insertAuditOfferingSelection(t, db, ctx, phase.TenantID, bookedLink)
	afterPhase, afterPhaseUntil := timezone.Date(phase.ServiceEndDate).AddDays(1), timezone.Date(phase.ServiceEndDate).AddDays(31)
	outOfPeriodLink := &enrollmentModels.RequestChildOffering{
		RequestChildID: outOfPeriod.ID, CareOfferingID: offering.ID,
		ValidFrom: (*enrollmentOwner.Date)(&afterPhase), ValidUntil: (*enrollmentOwner.Date)(&afterPhaseUntil),
	}
	insertAuditOfferingSelection(t, db, ctx, phase.TenantID, outOfPeriodLink)
	required := &enrollmentModels.CareOffering{
		PhaseID: phase.ID, Name: "Verpflichtende Betreuung", DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays: []string{"mon", "tue", "wed", "thu", "fri"}, IsActive: true, IsRequired: true, CountsAsCare: true,
	}
	required.TenantID = phase.TenantID
	InsertTestCareOffering(t, db, ctx, required)
	requiredLink := &enrollmentModels.RequestChildOffering{
		RequestChildID: requiredOnly.ID, CareOfferingID: required.ID,
	}
	insertAuditOfferingSelection(t, db, ctx, phase.TenantID, requiredLink)
	return approvedWithoutOfferingScenario{booked.ID, missing.ID, outOfPeriod.ID, requiredOnly.ID}
}

func createApprovedWithoutOfferingPhase(
	t *testing.T,
	db *bun.DB,
	ctx context.Context,
	tenantID int64,
) *enrollmentOwner.Phase {
	t.Helper()
	phase := &enrollmentOwner.Phase{
		Name: "Other tenant audit", Kind: enrollmentModels.PhaseKindSchoolYear,
		ServiceStartDate: "2025-12-06", ServiceEndDate: "2026-11-01",
		CareOverflowMode:          enrollmentModels.PhaseCareOverflowWaitlist,
		CareOfferingSelectionMode: enrollmentModels.PhaseCareOfferingSelectionOptional, IsActive: true,
	}
	phase.TenantID = tenantID
	require.NoError(t, enrollmentOwner.New().InsertPhase(WithTenantRuntime(t, ctx, db), phase))
	return phase
}

func createApprovedWithoutOfferingRequest(
	t *testing.T,
	db *bun.DB,
	ctx context.Context,
	phaseID, tenantID int64,
) *enrollmentOwner.Request {
	t.Helper()
	request := &enrollmentOwner.Request{
		PhaseID:           phaseID,
		GuardianFirstName: "Anna",
		GuardianLastName:  "Audit",
		GuardianEmail:     fmt.Sprintf("offering-audit-%d@example.test", tenantID),
		ConsentFlags:      []byte(`{}`),
		CustomData:        []byte(`{}`),
		SubmissionSource:  enrollmentModels.RequestSourcePublic,
		SourceMetadata:    []byte(`{}`),
		StatusToken:       fmt.Sprintf("offering-audit-%d-%d", tenantID, time.Now().UnixNano()),
		SubmittedAt:       time.Now(),
	}
	request.TenantID = tenantID
	require.NoError(t, enrollmentOwner.New().InsertRequest(WithTenantRuntime(t, ctx, db), request))
	return request
}

func createApprovedWithoutOfferingChild(
	t *testing.T,
	db *bun.DB,
	ctx context.Context,
	requestID int64,
	tenantID int64,
	firstName string,
) *enrollmentOwner.RequestChild {
	t.Helper()
	child := &enrollmentOwner.RequestChild{
		RequestID: requestID, FirstName: firstName, LastName: "Audit",
		DateOfBirth: "2018-04-15", CustomData: []byte(`{}`),
		Status: enrollmentModels.ChildStatusApproved, ActivationMode: enrollmentModels.ChildActivationScheduled,
	}
	child.TenantID = tenantID
	require.NoError(t, enrollmentOwner.New().InsertChild(WithTenantRuntime(t, ctx, db), child))
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
