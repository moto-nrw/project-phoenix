package compose

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The personnel record family (#2690): users.staff_master_data,
// users.staff_qualifications, users.staff_financial_data, users.staff_documents
// and users.staff_document_file_cleanup.

func testDocument(staffID int64, category, stored string) workforce.StaffDocument {
	return workforce.StaffDocument{
		StaffID: staffID, Category: category, FilenameDisplay: stored + ".pdf", FilenameStored: stored,
		SizeBytes: 12, ContentType: "application/pdf", UploadedBy: staffID,
	}
}

func TestStaffMasterDataAndFinancialRowsAreTenantIsolated(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	foreignTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenantID)
	foreignCtx := testpkg.TenantContext(foreignTenantID)
	local := testpkg.CreateTestStaff(t, db, "Stammdaten", "Local")
	foreign := testpkg.CreateTestStaffForTenant(t, db, foreignTenantID, "Stammdaten", "Foreign")
	capability := buildWorkforce(t, db)

	_, err := capability.FindStaffMasterData(ctx, local.ID)
	require.ErrorIs(t, err, workforce.ErrStaffMasterDataNotFound, "an empty Stammdaten tab is a normal state")

	gender := workforce.GenderDiverse
	hours := 30.5
	created, err := capability.CreateStaffMasterData(ctx, workforce.StaffMasterData{
		StaffID: local.ID, Gender: &gender, EntryDate: "2024-08-01", ProbationEndDate: "2025-01-31", WeeklyHours: &hours,
	})
	require.NoError(t, err)
	assert.Equal(t, testpkg.Tenant(t), created.TenantID)
	assert.Equal(t, "2024-08-01", created.EntryDate)
	assert.Empty(t, created.ContractEndDate)

	foreignRow, err := capability.CreateStaffMasterData(foreignCtx, workforce.StaffMasterData{StaffID: foreign.ID})
	require.NoError(t, err)
	assert.Equal(t, foreignTenantID, foreignRow.TenantID)

	invalidGender := "unknown"
	_, err = capability.CreateStaffMasterData(ctx, workforce.StaffMasterData{StaffID: local.ID, Gender: &invalidGender})
	require.ErrorIs(t, err, workforce.ErrInvalidStaffRecord)
	assert.EqualError(t, err, "gender must be 'female', 'male', or 'diverse'")
	_, err = capability.CreateStaffMasterData(ctx, workforce.StaffMasterData{StaffID: local.ID, EntryDate: "2024-08-01", ContractEndDate: "2024-01-01"})
	require.ErrorIs(t, err, workforce.ErrInvalidStaffRecord)
	assert.EqualError(t, err, "contract_end_date must not be before entry_date")

	created.ContractEndDate = "2026-07-31"
	updated, err := capability.UpdateStaffMasterData(ctx, created)
	require.NoError(t, err)
	assert.Equal(t, "2026-07-31", updated.ContractEndDate)
	assert.Equal(t, "2024-08-01", updated.EntryDate)

	foreignRow.Phone = new("hijacked")
	_, err = capability.UpdateStaffMasterData(ctx, foreignRow)
	require.ErrorIs(t, err, workforce.ErrStaffMasterDataNotFound, "a foreign update matches no row")
	_, err = capability.FindStaffMasterData(ctx, foreign.ID)
	require.ErrorIs(t, err, workforce.ErrStaffMasterDataNotFound, "a foreign row is invisible")

	// --- financial data ---
	_, err = capability.FindStaffFinancialData(ctx, local.ID)
	require.ErrorIs(t, err, workforce.ErrStaffFinancialDataNotFound)
	financial, err := capability.CreateStaffFinancialData(ctx, workforce.StaffFinancialData{StaffID: local.ID, IBAN: new("DE02120300000000202051")})
	require.NoError(t, err)
	assert.Equal(t, testpkg.Tenant(t), financial.TenantID)
	_, err = capability.CreateStaffFinancialData(foreignCtx, workforce.StaffFinancialData{StaffID: foreign.ID, TaxID: new("12345678901")})
	require.NoError(t, err)
	financial.TaxID = new("98765432109")
	stored, err := capability.UpdateStaffFinancialData(ctx, financial)
	require.NoError(t, err)
	assert.Equal(t, "98765432109", *stored.TaxID)
	assert.Equal(t, "DE02120300000000202051", *stored.IBAN)
	_, err = capability.FindStaffFinancialData(ctx, foreign.ID)
	require.ErrorIs(t, err, workforce.ErrStaffFinancialDataNotFound)
	_, err = capability.CreateStaffFinancialData(ctx, workforce.StaffFinancialData{})
	require.ErrorIs(t, err, workforce.ErrInvalidStaffRecord)
}

// ReplaceStaffQualifications rewrites the list in one unit of work: a rejected
// row or a failure after the delete leaves the previous list in place.
func TestStaffQualificationsReplaceAtomically(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Qualifikation", "Staff")
	foreignTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenantID)
	foreignCtx := testpkg.TenantContext(foreignTenantID)
	foreign := testpkg.CreateTestStaffForTenant(t, db, foreignTenantID, "Qualifikation", "Foreign")
	capability := buildWorkforce(t, db)

	stored, err := capability.ReplaceStaffQualifications(ctx, staff.ID, []workforce.StaffQualification{
		{Name: "Erste Hilfe", AcquiredOn: "2025-02-01", ExpiresOn: "2027-02-01"},
		{Name: "Schwimmschein"},
	})
	require.NoError(t, err)
	require.Len(t, stored, 2)
	assert.NotZero(t, stored[0].ID)
	assert.Equal(t, staff.ID, stored[0].StaffID)
	assert.Equal(t, testpkg.Tenant(t), stored[0].TenantID)
	_, err = capability.ReplaceStaffQualifications(foreignCtx, foreign.ID, []workforce.StaffQualification{{Name: "Fremd"}})
	require.NoError(t, err)

	listed, err := capability.ListStaffQualifications(ctx, staff.ID)
	require.NoError(t, err)
	require.Len(t, listed, 2)
	assert.Equal(t, "Erste Hilfe", listed[0].Name)
	assert.Equal(t, "2027-02-01", listed[0].ExpiresOn)
	assert.Empty(t, listed[1].AcquiredOn)
	foreignList, err := capability.ListStaffQualifications(ctx, foreign.ID)
	require.NoError(t, err)
	assert.Empty(t, foreignList, "the foreign tenant's rows are invisible")

	_, err = capability.ReplaceStaffQualifications(ctx, staff.ID, []workforce.StaffQualification{
		{Name: "Fortbildung"}, {Name: "", AcquiredOn: "2025-01-01"},
	})
	require.ErrorIs(t, err, workforce.ErrInvalidStaffRecord)
	assert.EqualError(t, err, "name is required")
	_, err = capability.ReplaceStaffQualifications(ctx, staff.ID, []workforce.StaffQualification{
		{Name: "Fortbildung", AcquiredOn: "2026-01-01", ExpiresOn: "2025-01-01"},
	})
	require.ErrorIs(t, err, workforce.ErrInvalidStaffRecord)
	assert.EqualError(t, err, "expires_on must not be before acquired_on")

	failure := errors.New("audit write failed after the replace")
	err = testpkg.WithinTenantContext(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context) error {
		replaced, err := capability.ReplaceStaffQualifications(txCtx, staff.ID, []workforce.StaffQualification{{Name: "Nur kurz"}})
		if err != nil {
			return err
		}
		require.Len(t, replaced, 1)
		return failure
	})
	require.ErrorIs(t, err, failure)

	listed, err = capability.ListStaffQualifications(ctx, staff.ID)
	require.NoError(t, err)
	require.Len(t, listed, 2, "the rolled-back replace leaves the previous list intact")
	assert.Equal(t, "Erste Hilfe", listed[0].Name)

	replaced, err := capability.ReplaceStaffQualifications(ctx, staff.ID, nil)
	require.NoError(t, err)
	assert.Empty(t, replaced)
	listed, err = capability.ListStaffQualifications(ctx, staff.ID)
	require.NoError(t, err)
	assert.Empty(t, listed)
}

func TestStaffDocumentsFollowTheLegacyRowRules(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Dokument", "Staff")
	foreignTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenantID)
	foreignCtx := testpkg.TenantContext(foreignTenantID)
	foreign := testpkg.CreateTestStaffForTenant(t, db, foreignTenantID, "Dokument", "Foreign")
	capability := buildWorkforce(t, db)

	contract, err := capability.CreateStaffDocument(ctx, testDocument(staff.ID, workforce.StaffDocumentCategoryArbeitsvertrag, "contract-1"))
	require.NoError(t, err)
	assert.NotZero(t, contract.ID)
	assert.Equal(t, testpkg.Tenant(t), contract.TenantID)
	assert.False(t, contract.CreatedAt.IsZero(), "database defaults are read back")
	payslip, err := capability.CreateStaffDocument(ctx, testDocument(staff.ID, workforce.StaffDocumentCategoryLohnabrechnung, "payslip-1"))
	require.NoError(t, err)
	foreignDoc, err := capability.CreateStaffDocument(foreignCtx, testDocument(foreign.ID, workforce.StaffDocumentCategorySonstiges, "foreign-1"))
	require.NoError(t, err)

	_, err = capability.CreateStaffDocument(ctx, testDocument(staff.ID, "diary", "invalid-1"))
	require.ErrorIs(t, err, workforce.ErrInvalidStaffRecord)
	assert.EqualError(t, err, "unknown document category")

	visible, err := capability.ListStaffDocuments(ctx, workforce.StaffDocumentFilter{
		StaffID: staff.ID, Categories: []string{workforce.StaffDocumentCategoryArbeitsvertrag, workforce.StaffDocumentCategorySonstiges},
	})
	require.NoError(t, err)
	require.Len(t, visible, 1, "the caller's visible categories narrow the list")
	assert.Equal(t, contract.ID, visible[0].ID)
	none, err := capability.ListStaffDocuments(ctx, workforce.StaffDocumentFilter{StaffID: staff.ID, Categories: []string{}})
	require.NoError(t, err)
	assert.Empty(t, none, "no visible category means no document")
	all, err := capability.ListStaffDocuments(ctx, workforce.StaffDocumentFilter{StaffID: staff.ID})
	require.NoError(t, err)
	require.Len(t, all, 2)
	assert.Equal(t, payslip.ID, all[0].ID, "newest first")

	found, err := capability.FindStaffDocument(ctx, staff.ID, contract.ID, false)
	require.NoError(t, err)
	assert.Equal(t, "contract-1", found.FilenameStored)
	_, err = capability.FindStaffDocument(ctx, foreign.ID, contract.ID, false)
	require.ErrorIs(t, err, workforce.ErrStaffDocumentNotFound, "the staff and document ids must agree")
	_, err = capability.FindStaffDocument(ctx, foreign.ID, foreignDoc.ID, false)
	require.ErrorIs(t, err, workforce.ErrStaffDocumentNotFound, "a foreign row is invisible")

	// Soft delete hides the row from ordinary reads, keeps it for the cleanup
	// passes and refuses a second delete.
	deletedAt := time.Now()
	affected, err := capability.SoftDeleteStaffDocument(ctx, contract.ID, staff.ID, deletedAt)
	require.NoError(t, err)
	assert.EqualValues(t, 1, affected)
	affected, err = capability.SoftDeleteStaffDocument(ctx, contract.ID, staff.ID, deletedAt)
	require.NoError(t, err)
	assert.Zero(t, affected)
	affected, err = capability.SoftDeleteStaffDocument(ctx, foreignDoc.ID, staff.ID, deletedAt)
	require.NoError(t, err)
	assert.Zero(t, affected, "a foreign row cannot be deleted")
	_, err = capability.FindStaffDocument(ctx, staff.ID, contract.ID, false)
	require.ErrorIs(t, err, workforce.ErrStaffDocumentNotFound)
	deleted, err := capability.FindStaffDocument(ctx, staff.ID, contract.ID, true)
	require.NoError(t, err)
	require.NotNil(t, deleted.DeletedAt)
	assert.Equal(t, staff.ID, *deleted.DeletedBy)

	pending, err := capability.ListStaffDocuments(ctx, workforce.StaffDocumentFilter{DeletedOnly: true, FilePending: true})
	require.NoError(t, err)
	require.Len(t, pending, 1)
	assert.Equal(t, contract.ID, pending[0].ID)
	allPending, err := capability.ListStaffDocuments(ctx, workforce.StaffDocumentFilter{StaffIDs: []int64{staff.ID, foreign.ID}, IncludeDeleted: true, FilePending: true})
	require.NoError(t, err)
	assert.Len(t, allPending, 2, "offboarding sees deleted and live rows of the tenant alone")

	require.NoError(t, capability.MarkStaffDocumentFileDeleted(ctx, contract.ID, time.Now()))
	pending, err = capability.ListStaffDocuments(ctx, workforce.StaffDocumentFilter{DeletedOnly: true, FilePending: true})
	require.NoError(t, err)
	assert.Empty(t, pending, "a removed file is not retried")
	byCategory, err := capability.ListStaffDocuments(ctx, workforce.StaffDocumentFilter{
		StaffID: staff.ID, Categories: []string{workforce.StaffDocumentCategoryArbeitsvertrag}, DeletedOnly: true, FilePending: true,
	})
	require.NoError(t, err)
	assert.Empty(t, byCategory)
}

func TestStaffDocumentFileCleanupIntents(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Cleanup", "Staff")
	other := testpkg.CreateTestStaff(t, db, "Cleanup", "Other")
	foreignTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenantID)
	foreignCtx := testpkg.TenantContext(foreignTenantID)
	foreign := testpkg.CreateTestStaffForTenant(t, db, foreignTenantID, "Cleanup", "Foreign")
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	capability := buildWorkforceAt(t, db, now)

	queue := func(ctx context.Context, staffID int64, name string, retryAfter time.Time) {
		require.NoError(t, capability.QueueStaffDocumentFileCleanup(ctx, workforce.StaffDocumentFileCleanup{
			StaffID: staffID, FilenameStored: name, RetryAfter: retryAfter,
		}))
	}
	queue(ctx, staff.ID, "due-1", now.Add(-time.Minute))
	queue(ctx, staff.ID, "due-1", now.Add(-time.Hour)) // same stored name: recorded once
	queue(ctx, staff.ID, "later-1", now.Add(5*time.Minute))
	queue(ctx, other.ID, "due-2", now.Add(-time.Minute))
	queue(foreignCtx, foreign.ID, "foreign-1", now.Add(-time.Minute))
	err := capability.QueueStaffDocumentFileCleanup(ctx, workforce.StaffDocumentFileCleanup{StaffID: staff.ID})
	require.ErrorIs(t, err, workforce.ErrInvalidStaffRecord)

	eligible, err := capability.ListQueuedStaffDocumentFileCleanups(ctx, 0)
	require.NoError(t, err)
	names := make([]string, 0, len(eligible))
	for _, intent := range eligible {
		names = append(names, intent.FilenameStored)
	}
	assert.ElementsMatch(t, []string{"due-1", "due-2"}, names, "only due intents of the tenant, each once")
	mine, err := capability.ListQueuedStaffDocumentFileCleanups(ctx, staff.ID)
	require.NoError(t, err)
	require.Len(t, mine, 1)
	assert.Equal(t, "due-1", mine[0].FilenameStored)

	// Activating pulls the not-yet-due intent forward; completing by name or
	// id removes intents from the eligible set exactly once.
	require.NoError(t, capability.ActivateStaffDocumentFileCleanup(ctx, "later-1"))
	eligible, err = capability.ListQueuedStaffDocumentFileCleanups(ctx, staff.ID)
	require.NoError(t, err)
	assert.Len(t, eligible, 2)
	require.NoError(t, capability.CompleteStaffDocumentFileCleanupByFilename(ctx, "later-1"))
	require.NoError(t, capability.CompleteStaffDocumentFileCleanup(ctx, mine[0].ID))
	eligible, err = capability.ListQueuedStaffDocumentFileCleanups(ctx, staff.ID)
	require.NoError(t, err)
	assert.Empty(t, eligible)
	require.NoError(t, capability.CompleteStaffDocumentFileCleanupByFilename(ctx, "foreign-1"))
	foreignEligible, err := capability.ListQueuedStaffDocumentFileCleanups(foreignCtx, 0)
	require.NoError(t, err)
	assert.Len(t, foreignEligible, 1, "a foreign complete is a no-op")
}
