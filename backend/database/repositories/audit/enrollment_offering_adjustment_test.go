package audit_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/audit"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnrollmentOfferingAdjustmentRepository_ListByRequestChildID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repoFactory := repositories.NewFactory(db)
	repo := repoFactory.EnrollmentOfferingAdjustment
	ctx := testpkg.TenantContext(1)

	student := testpkg.CreateTestStudent(t, db, "Audit", "Adjustment", "1a")
	account := testpkg.CreateTestAccount(t, db, "adjustment-admin@example.test")
	phase := createAuditAdjustmentPhase(t, repoFactory)
	request := createAuditAdjustmentRequest(t, repoFactory, phase.ID)
	child := createAuditAdjustmentChild(t, repoFactory, request.ID)
	otherChild := createAuditAdjustmentChild(t, repoFactory, request.ID)
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)
	defer testpkg.CleanupTableRecords(t, db, "enrollment.phases", phase.ID)

	older := &audit.EnrollmentOfferingAdjustment{
		RequestID:      request.ID,
		RequestChildID: child.ID,
		StudentID:      student.ID,
		ActorAccountID: account.ID,
		ActorRole:      "admin",
		Reason:         "older",
		Before:         []byte(`[]`),
		After:          []byte(`[{"name":"old"}]`),
		ChangedAt:      time.Date(2026, time.June, 20, 9, 0, 0, 0, time.UTC),
	}
	newer := &audit.EnrollmentOfferingAdjustment{
		RequestID:      request.ID,
		RequestChildID: child.ID,
		StudentID:      student.ID,
		ActorAccountID: account.ID,
		ActorRole:      "admin",
		Reason:         "newer",
		Before:         []byte(`[]`),
		After:          []byte(`[{"name":"new"}]`),
		ChangedAt:      time.Date(2026, time.June, 20, 10, 0, 0, 0, time.UTC),
	}
	other := &audit.EnrollmentOfferingAdjustment{
		RequestID:      request.ID,
		RequestChildID: otherChild.ID,
		StudentID:      student.ID,
		ActorAccountID: account.ID,
		ActorRole:      "admin",
		Reason:         "other child",
		Before:         []byte(`[]`),
		After:          []byte(`[]`),
		ChangedAt:      time.Date(2026, time.June, 20, 11, 0, 0, 0, time.UTC),
	}

	require.NoError(t, repo.Create(ctx, older))
	require.NoError(t, repo.Create(ctx, newer))
	require.NoError(t, repo.Create(ctx, other))
	defer testpkg.CleanupTableRecords(t, db, "audit.enrollment_offering_adjustments", older.ID, newer.ID, other.ID)

	rows, err := repo.ListByRequestChildID(ctx, child.ID)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, newer.ID, rows[0].ID)
	assert.Equal(t, older.ID, rows[1].ID)
	assert.Equal(t, "newer", rows[0].Reason)
}

func TestEnrollmentOfferingAdjustmentRepository_ListByRequestChildID_RejectsMissingID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).EnrollmentOfferingAdjustment

	rows, err := repo.ListByRequestChildID(testpkg.TenantContext(1), 0)
	require.Error(t, err)
	assert.Nil(t, rows)
	assert.Contains(t, err.Error(), "request_child_id is required")
}

func TestEnrollmentOfferingAdjustmentRepository_ListByRequestChildID_QueryError(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	repo := repositories.NewFactory(db).EnrollmentOfferingAdjustment
	require.NoError(t, db.Close())

	rows, err := repo.ListByRequestChildID(testpkg.TenantContext(1), 999)
	require.Error(t, err)
	assert.Nil(t, rows)
	assert.Contains(t, err.Error(), "list enrollment offering adjustments")
}

func createAuditAdjustmentPhase(t *testing.T, repoFactory *repositories.Factory) *enrollmentModels.Phase {
	t.Helper()
	ctx := testpkg.TenantContext(1)
	phase := &enrollmentModels.Phase{
		Name:                      fmt.Sprintf("audit-adjustment-%d", time.Now().UnixNano()),
		Kind:                      enrollmentModels.PhaseKindSchoolYear,
		ServiceStartDate:          timezone.NewDate(2026, time.September, 1),
		ServiceEndDate:            timezone.NewDate(2027, time.July, 31),
		IsActive:                  true,
		CareOverflowMode:          enrollmentModels.PhaseCareOverflowWaitlist,
		CareOfferingSelectionMode: enrollmentModels.PhaseCareOfferingSelectionOptional,
	}
	require.NoError(t, repoFactory.Phase.Create(ctx, phase))
	return phase
}

func createAuditAdjustmentRequest(t *testing.T, repoFactory *repositories.Factory, phaseID int64) *enrollmentModels.Request {
	t.Helper()
	ctx := testpkg.TenantContext(1)
	req := &enrollmentModels.Request{
		PhaseID:           phaseID,
		GuardianFirstName: "Anna",
		GuardianLastName:  "Audit",
		GuardianEmail:     fmt.Sprintf("audit-adjustment-%d@example.test", time.Now().UnixNano()),
		ConsentFlags:      map[string]any{},
		CustomData:        map[string]any{},
		StatusToken:       fmt.Sprintf("audit-adjustment-%d", time.Now().UnixNano()),
		SubmittedAt:       time.Now().UTC(),
	}
	require.NoError(t, repoFactory.Request.Create(ctx, req))
	return req
}

func createAuditAdjustmentChild(t *testing.T, repoFactory *repositories.Factory, requestID int64) *enrollmentModels.RequestChild {
	t.Helper()
	ctx := testpkg.TenantContext(1)
	child := &enrollmentModels.RequestChild{
		RequestID:      requestID,
		FirstName:      "Lina",
		LastName:       fmt.Sprintf("Audit%d", time.Now().UnixNano()),
		DateOfBirth:    timezone.NewDate(2018, time.April, 15),
		CustomData:     map[string]any{},
		Status:         enrollmentModels.ChildStatusApproved,
		ActivationMode: enrollmentModels.ChildActivationScheduled,
	}
	require.NoError(t, repoFactory.RequestChild.Create(ctx, child))
	return child
}
