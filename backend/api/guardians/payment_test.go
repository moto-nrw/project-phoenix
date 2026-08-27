package guardians_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Guardian payment endpoints (#2608). The IBAN below is the Bundesbank example
// number used across this repo's financial tests — no such account exists.
const testIBAN = "DE89370400440532013000"

const financialPerm = "guardians:financial"

// paymentContext wires the resource with the export renderer, which the shared
// setupTestContext leaves nil because no other guardian route renders files.
func paymentContext(t *testing.T) *testContext {
	t.Helper()
	ctx := setupTestContext(t)
	ctx.resource.ListExportService = listexport.NewService()
	return ctx
}

// linkGuardian creates a guardian and links it to the student.
func linkGuardian(t *testing.T, ctx *testContext, studentID int64, email string) *users.GuardianProfile {
	t.Helper()
	guardian := testpkg.CreateTestGuardianProfile(t, ctx.db, email)
	_, err := ctx.services.Guardian.LinkGuardianToStudent(testpkg.Ctx(t), usersSvc.StudentGuardianCreateRequest{
		StudentID:         studentID,
		GuardianProfileID: guardian.ID,
		RelationshipType:  "parent",
		EmergencyPriority: 1,
	})
	require.NoError(t, err)
	return guardian
}

func paymentRequest(t *testing.T, ctx *testContext, method, path string, body any, perms ...string) int {
	t.Helper()
	claims := withPerms(testutil.DefaultTestClaims(), perms...)
	req := testutil.NewAuthenticatedRequest(t, method, path, body, bearer(t, claims))
	return testutil.ExecuteRequest(ctx.resource.Router(), req).Code
}

func paymentBody(t *testing.T, ctx *testContext, method, path string, body any, perms ...string) (int, string) {
	t.Helper()
	claims := withPerms(testutil.DefaultTestClaims(), perms...)
	req := testutil.NewAuthenticatedRequest(t, method, path, body, bearer(t, claims))
	rr := testutil.ExecuteRequest(ctx.resource.Router(), req)
	return rr.Code, rr.Body.String()
}

// countAccessLogs counts data-access rows of one resource type in this test's
// tenant. Every payment read must produce exactly one.
func countAccessLogs(t *testing.T, ctx *testContext, resourceType string) int {
	t.Helper()
	count, err := ctx.db.NewSelect().
		TableExpr(`audit.data_access_log AS "log"`).
		Where(`"log".tenant_id = ?`, testpkg.Tenant(t)).
		Where(`"log".resource_type = ?`, resourceType).
		Count(testpkg.Ctx(t))
	require.NoError(t, err)
	return count
}

// TestGuardianPayment_RequiresFinancialPermission pins that users:update — the
// permission that maintains the guardian directory — unlocks none of the bank
// surface. That separation is the point of the dedicated permission.
func TestGuardianPayment_RequiresFinancialPermission(t *testing.T) {
	t.Parallel()

	ctx := paymentContext(t)
	student := testpkg.CreateTestStudent(t, ctx.db, "Payment", "Denied", "1a")
	guardian := linkGuardian(t, ctx, student.ID, "payment-denied")

	cases := []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{"read", http.MethodGet, fmt.Sprintf("/%d/payment", guardian.ID), nil},
		{"update", http.MethodPut, fmt.Sprintf("/%d/payment", guardian.ID), map[string]any{"iban": testIBAN}},
		{"reveal", http.MethodPost, fmt.Sprintf("/%d/payment/reveal", guardian.ID), map[string]any{}},
		{"set payer", http.MethodPut, fmt.Sprintf("/students/%d/payer", student.ID), map[string]any{"guardian_id": fmt.Sprintf("%d", guardian.ID)}},
		{"overview", http.MethodGet, "/payment-overview", nil},
		{"export", http.MethodPost, "/payment-overview/export", map[string]any{"format": "xlsx"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code := paymentRequest(t, ctx, tc.method, tc.path, tc.body, "users:update", "users:read")
			assert.Equal(t, http.StatusForbidden, code, "users:update must not unlock the bank surface")
		})
	}
}

// TestGuardianPayment_MaskedByDefaultRevealOnDemand is the core contract: the
// list read never carries the full IBAN, and the reveal that does is a separate
// audited request.
func TestGuardianPayment_MaskedByDefaultRevealOnDemand(t *testing.T) {
	t.Parallel()

	ctx := paymentContext(t)
	student := testpkg.CreateTestStudent(t, ctx.db, "Payment", "Masked", "1a")
	guardian := linkGuardian(t, ctx, student.ID, "payment-masked")

	code := paymentRequest(t, ctx, http.MethodPut, fmt.Sprintf("/%d/payment", guardian.ID),
		map[string]any{"iban": "de89 3704 0044 0532 0130 00"}, financialPerm)
	require.Equal(t, http.StatusOK, code)

	code, body := paymentBody(t, ctx, http.MethodGet, fmt.Sprintf("/%d/payment", guardian.ID), nil, financialPerm)
	require.Equal(t, http.StatusOK, code, body)
	assert.Contains(t, body, `"iban_masked":"•••• 3000"`, "input must be normalized and stored")
	assert.NotContains(t, body, testIBAN, "the masked read must not leak the IBAN")

	code, body = paymentBody(t, ctx, http.MethodPost, fmt.Sprintf("/%d/payment/reveal", guardian.ID), map[string]any{}, financialPerm)
	require.Equal(t, http.StatusOK, code, body)
	assert.Contains(t, body, fmt.Sprintf(`"iban":"%s"`, testIBAN))

	assert.Equal(t, 1, countAccessLogs(t, ctx, auditModels.ResourceTypeGuardianFinancialView))
	assert.Equal(t, 1, countAccessLogs(t, ctx, auditModels.ResourceTypeGuardianFinancialReveal))
}

// TestGuardianPayment_RejectsMalformedIBAN pins that a typo is refused rather
// than stored: a wrong IBAN in an export becomes a wrong bank transfer.
func TestGuardianPayment_RejectsMalformedIBAN(t *testing.T) {
	t.Parallel()

	ctx := paymentContext(t)
	student := testpkg.CreateTestStudent(t, ctx.db, "Payment", "Invalid", "1a")
	guardian := linkGuardian(t, ctx, student.ID, "payment-invalid")

	for _, iban := range []string{
		"DE89370400440532013001", // valid shape, broken mod-97 checksum
		"DE8937040044",           // too short
		"370400440532013000",     // no country code
	} {
		code, body := paymentBody(t, ctx, http.MethodPut, fmt.Sprintf("/%d/payment", guardian.ID),
			map[string]any{"iban": iban}, financialPerm)
		assert.Equal(t, http.StatusBadRequest, code, "IBAN %q must be refused", iban)
		// The message is read by a school office worker, not a developer.
		assert.Contains(t, body, "Die IBAN ist nicht gültig", "IBAN %q", iban)
	}
}

// TestGuardianPayment_ClearingRemovesTheStoredIBAN pins that an empty submit
// clears the value instead of leaving the old one behind.
func TestGuardianPayment_ClearingRemovesTheStoredIBAN(t *testing.T) {
	t.Parallel()

	ctx := paymentContext(t)
	student := testpkg.CreateTestStudent(t, ctx.db, "Payment", "Cleared", "1a")
	guardian := linkGuardian(t, ctx, student.ID, "payment-cleared")

	require.Equal(t, http.StatusOK, paymentRequest(t, ctx, http.MethodPut,
		fmt.Sprintf("/%d/payment", guardian.ID), map[string]any{"iban": testIBAN}, financialPerm))
	require.Equal(t, http.StatusOK, paymentRequest(t, ctx, http.MethodPut,
		fmt.Sprintf("/%d/payment", guardian.ID), map[string]any{"iban": ""}, financialPerm))

	_, body := paymentBody(t, ctx, http.MethodGet, fmt.Sprintf("/%d/payment", guardian.ID), nil, financialPerm)
	assert.Contains(t, body, `"iban_masked":null`)
}

// TestStudentPayer_OnlyOnePerChild pins the invariant the partial unique index
// enforces: assigning a second guardian moves the mark, never duplicates it.
func TestStudentPayer_OnlyOnePerChild(t *testing.T) {
	t.Parallel()

	ctx := paymentContext(t)
	student := testpkg.CreateTestStudent(t, ctx.db, "Payer", "Single", "1a")
	mother := linkGuardian(t, ctx, student.ID, "payer-mother")
	father := linkGuardian(t, ctx, student.ID, "payer-father")

	payerPath := fmt.Sprintf("/students/%d/payer", student.ID)

	require.Equal(t, http.StatusOK, paymentRequest(t, ctx, http.MethodPut, payerPath,
		map[string]any{"guardian_id": fmt.Sprintf("%d", mother.ID)}, financialPerm))
	assertPayer(t, ctx, student.ID, &mother.ID)

	require.Equal(t, http.StatusOK, paymentRequest(t, ctx, http.MethodPut, payerPath,
		map[string]any{"guardian_id": fmt.Sprintf("%d", father.ID)}, financialPerm))
	assertPayer(t, ctx, student.ID, &father.ID)

	require.Equal(t, http.StatusOK, paymentRequest(t, ctx, http.MethodPut, payerPath,
		map[string]any{"guardian_id": nil}, financialPerm))
	assertPayer(t, ctx, student.ID, nil)
}

// assertPayer asserts exactly which guardian carries the mark for a child.
func assertPayer(t *testing.T, ctx *testContext, studentID int64, want *int64) {
	t.Helper()
	relationships, err := ctx.services.Guardian.GetStudentGuardians(testpkg.Ctx(t), studentID)
	require.NoError(t, err)

	var payers []int64
	for _, rel := range relationships {
		if rel.Relationship.IsPayer {
			payers = append(payers, rel.Relationship.GuardianProfileID)
		}
	}
	if want == nil {
		assert.Empty(t, payers, "no guardian may carry the payment mark")
		return
	}
	require.Len(t, payers, 1, "exactly one guardian may carry the payment mark")
	assert.Equal(t, *want, payers[0])
}

// TestStudentPayer_RejectsGuardianOfAnotherChild pins that the payer must be a
// guardian of THIS child — otherwise the export would name a stranger.
func TestStudentPayer_RejectsGuardianOfAnotherChild(t *testing.T) {
	t.Parallel()

	ctx := paymentContext(t)
	student := testpkg.CreateTestStudent(t, ctx.db, "Payer", "Own", "1a")
	other := testpkg.CreateTestStudent(t, ctx.db, "Payer", "Other", "1b")
	stranger := linkGuardian(t, ctx, other.ID, "payer-stranger")

	code, body := paymentBody(t, ctx, http.MethodPut, fmt.Sprintf("/students/%d/payer", student.ID),
		map[string]any{"guardian_id": fmt.Sprintf("%d", stranger.ID)}, financialPerm)
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Contains(t, body, "nicht als erziehungsberechtigt eingetragen")
}

// TestStudentPayer_SiblingsShareOneMaintainedIBAN is the sibling requirement
// from the issue: the IBAN is entered once and both children resolve to it.
func TestStudentPayer_SiblingsShareOneMaintainedIBAN(t *testing.T) {
	t.Parallel()

	ctx := paymentContext(t)
	older := testpkg.CreateTestStudent(t, ctx.db, "Sibling", "Older", "3a")
	younger := testpkg.CreateTestStudent(t, ctx.db, "Sibling", "Younger", "1a")

	parent := linkGuardian(t, ctx, older.ID, "sibling-parent")
	_, err := ctx.services.Guardian.LinkGuardianToStudent(testpkg.Ctx(t), usersSvc.StudentGuardianCreateRequest{
		StudentID:         younger.ID,
		GuardianProfileID: parent.ID,
		RelationshipType:  "parent",
		EmergencyPriority: 1,
	})
	require.NoError(t, err)

	require.Equal(t, http.StatusOK, paymentRequest(t, ctx, http.MethodPut,
		fmt.Sprintf("/%d/payment", parent.ID), map[string]any{"iban": testIBAN}, financialPerm))
	for _, studentID := range []int64{older.ID, younger.ID} {
		require.Equal(t, http.StatusOK, paymentRequest(t, ctx, http.MethodPut,
			fmt.Sprintf("/students/%d/payer", studentID),
			map[string]any{"guardian_id": fmt.Sprintf("%d", parent.ID)}, financialPerm))
	}

	rows, err := ctx.services.Guardian.ListPaymentOverview(testpkg.Ctx(t), 1, "test")
	require.NoError(t, err)

	seen := 0
	for _, row := range rows {
		if row.StudentID != older.ID && row.StudentID != younger.ID {
			continue
		}
		seen++
		assert.Equal(t, "•••• 3000", row.IBANMasked, "both siblings resolve to the one maintained IBAN")
	}
	assert.Equal(t, 2, seen, "both siblings must appear in the list")
}

// TestPaymentOverview_ListsChildrenWithoutAPayer pins that the list shows its
// gaps. A bank list that silently omits unassigned children reads as complete
// while missing exactly the rows that need work.
func TestPaymentOverview_ListsChildrenWithoutAPayer(t *testing.T) {
	t.Parallel()

	ctx := paymentContext(t)
	unassigned := testpkg.CreateTestStudent(t, ctx.db, "Overview", "Unassigned", "2a")
	linkGuardian(t, ctx, unassigned.ID, "overview-unassigned")

	rows, err := ctx.services.Guardian.ListPaymentOverview(testpkg.Ctx(t), 1, "test")
	require.NoError(t, err)

	var found *usersSvc.GuardianPaymentRow
	for i := range rows {
		if rows[i].StudentID == unassigned.ID {
			found = &rows[i]
		}
	}
	require.NotNil(t, found, "a child without a payer must still appear")
	assert.Nil(t, found.GuardianProfileID)
	assert.False(t, found.HasIBAN())
}

// TestPaymentOverview_ExcludesSoftDeletedStudents prevents former students
// from appearing in a payment export merely because their student row remains.
func TestPaymentOverview_ExcludesSoftDeletedStudents(t *testing.T) {
	t.Parallel()

	ctx := paymentContext(t)
	student := testpkg.CreateTestStudent(t, ctx.db, "Overview", "Deleted", "2a")
	_, err := ctx.db.NewUpdate().
		TableExpr(`users.persons AS "person"`).
		Set("deleted_at = NOW()").
		Where(`"person".id = ?`, student.PersonID).
		Exec(testpkg.Ctx(t))
	require.NoError(t, err)

	rows, err := ctx.services.Guardian.ListPaymentOverview(testpkg.Ctx(t), 1, "test")
	require.NoError(t, err)
	for _, row := range rows {
		assert.NotEqual(t, student.ID, row.StudentID)
	}
}

// TestPaymentExport_RendersFileAndIsAuditedSeparately pins that the bulk export
// produces a file and is logged as its own event — viewing masked rows and
// downloading every full IBAN are materially different accesses.
func TestPaymentExport_RendersFileAndIsAuditedSeparately(t *testing.T) {
	t.Parallel()

	ctx := paymentContext(t)
	student := testpkg.CreateTestStudent(t, ctx.db, "Export", "Child", "1a")
	guardian := linkGuardian(t, ctx, student.ID, "export-child")
	require.Equal(t, http.StatusOK, paymentRequest(t, ctx, http.MethodPut,
		fmt.Sprintf("/%d/payment", guardian.ID), map[string]any{"iban": testIBAN}, financialPerm))
	require.Equal(t, http.StatusOK, paymentRequest(t, ctx, http.MethodPut,
		fmt.Sprintf("/students/%d/payer", student.ID),
		map[string]any{"guardian_id": fmt.Sprintf("%d", guardian.ID)}, financialPerm))

	claims := withPerms(testutil.DefaultTestClaims(), financialPerm)
	req := testutil.NewAuthenticatedRequest(t, http.MethodPost, "/payment-overview/export",
		map[string]any{"format": "xlsx"}, bearer(t, claims))
	rr := testutil.ExecuteRequest(ctx.resource.Router(), req)

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Header().Get("Content-Type"), "spreadsheetml")
	// safeFilename lowercases the rendered filename.
	assert.Contains(t, rr.Header().Get("Content-Disposition"), "bankverbindungen.xlsx")
	assert.NotEmpty(t, rr.Body.Bytes())

	assert.Equal(t, 1, countAccessLogs(t, ctx, auditModels.ResourceTypeGuardianPaymentExport))
	assert.Equal(t, 0, countAccessLogs(t, ctx, auditModels.ResourceTypeGuardianPaymentOverview),
		"an export must not be logged as a plain list view")
}

// TestGuardianPayment_ChangesAreAuditedWithMaskedValues pins that the trail
// records the change without becoming a second copy of the bank data.
func TestGuardianPayment_ChangesAreAuditedWithMaskedValues(t *testing.T) {
	t.Parallel()

	ctx := paymentContext(t)
	student := testpkg.CreateTestStudent(t, ctx.db, "Audit", "Child", "1a")
	guardian := linkGuardian(t, ctx, student.ID, "audit-child")

	require.Equal(t, http.StatusOK, paymentRequest(t, ctx, http.MethodPut,
		fmt.Sprintf("/%d/payment", guardian.ID), map[string]any{"iban": testIBAN}, financialPerm))

	var rows []auditModels.GuardianFinancialChange
	err := ctx.db.NewSelect().
		Model(&rows).
		ModelTableExpr(`audit.guardian_financial_changes AS "guardian_financial_change"`).
		Where(`"guardian_financial_change".tenant_id = ?`, testpkg.Tenant(t)).
		Where(`"guardian_financial_change".guardian_profile_id = ?`, guardian.ID).
		Scan(testpkg.Ctx(t))
	require.NoError(t, err)

	require.Len(t, rows, 1)
	assert.Equal(t, auditModels.GuardianPaymentFieldIBAN, rows[0].FieldName)
	assert.Equal(t, "•••• 3000", rows[0].NewValue)
	assert.NotContains(t, rows[0].NewValue, testIBAN, "the trail must not store the IBAN")
	assert.Nil(t, rows[0].StudentID, "a bank change is guardian-scoped")
}

// TestPaymentExport_UnknownFormatLeavesNoAccessLog pins that a rejected format
// is rejected BEFORE the rows are loaded. The row load writes the
// full-IBAN export access-log entry, so an export that never produced a file
// must not leave that entry behind.
func TestPaymentExport_UnknownFormatLeavesNoAccessLog(t *testing.T) {
	t.Parallel()

	ctx := paymentContext(t)
	student := testpkg.CreateTestStudent(t, ctx.db, "Format", "Child", "1a")
	guardian := linkGuardian(t, ctx, student.ID, "format-child")
	require.Equal(t, http.StatusOK, paymentRequest(t, ctx, http.MethodPut,
		fmt.Sprintf("/%d/payment", guardian.ID), map[string]any{"iban": testIBAN}, financialPerm))

	code, body := paymentBody(t, ctx, http.MethodPost, "/payment-overview/export",
		map[string]any{"format": "csv"}, financialPerm)

	require.Equal(t, http.StatusBadRequest, code, body)
	assert.Equal(t, 0, countAccessLogs(t, ctx, auditModels.ResourceTypeGuardianPaymentExport),
		"a refused export must not be logged as a full-IBAN access")
}
