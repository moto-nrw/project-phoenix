package api

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Guardian payment endpoints (#2608). The IBAN below is the Bundesbank
// example number used across this repo's financial tests; no such account
// exists.
const testIBAN = "DE89370400440532013000"

const financialPerm = "guardians:financial"

// Access-log resource types of the payment reads, as written by the owner.
const (
	accessGuardianFinancialView   = "guardian_financial_view"
	accessGuardianFinancialReveal = "guardian_financial_reveal"
	accessGuardianPaymentOverview = "guardian_payment_overview"
	accessGuardianPaymentExport   = "guardian_payment_export"
)

func (c *guardianCompositionContext) paymentRequest(t *testing.T, method, path string, body any, perms ...string) (int, string) {
	t.Helper()
	rr := c.do(t, withPerms(testutil.DefaultTestClaims(), perms...), method, path, body)
	return rr.Code, rr.Body.String()
}

// setPayer assigns (or clears) a child's payer as a verified staff member
// holding guardians:financial: the payer route runs the same active-student
// and staff check as every relationship write.
func (c *guardianCompositionContext) setPayer(t *testing.T, studentID int64, guardianID *int64) (int, string) {
	t.Helper()
	accountID := c.staffAccount("Payer", "Setter")
	claims := guardianStaffClaims(accountID, permissions.UsersRead, permissions.GuardiansFinancial)
	body := map[string]any{"guardian_id": nil}
	if guardianID != nil {
		body["guardian_id"] = fmt.Sprintf("%d", *guardianID)
	}
	rr := c.do(t, claims, http.MethodPut, fmt.Sprintf("/students/%d/payer", studentID), body)
	return rr.Code, rr.Body.String()
}

func (c *guardianCompositionContext) requirePayerSet(t *testing.T, studentID int64, guardianID *int64) {
	t.Helper()
	code, body := c.setPayer(t, studentID, guardianID)
	require.Equal(t, http.StatusOK, code, body)
}

// assertPayer asserts exactly which guardian carries the mark for a child.
func (c *guardianCompositionContext) assertPayer(t *testing.T, studentID int64, want *int64) {
	t.Helper()
	var payers []int64
	for _, row := range c.studentGuardians(t, studentID) {
		if row.IsPayer {
			payers = append(payers, row.Guardian.ID)
		}
	}
	if want == nil {
		assert.Empty(t, payers, "no guardian may carry the payment mark")
		return
	}
	require.Len(t, payers, 1, "exactly one guardian may carry the payment mark")
	assert.Equal(t, *want, payers[0])
}

type paymentOverviewRow struct {
	StudentID  string  `json:"student_id"`
	GuardianID *string `json:"guardian_id"`
	IBANMasked string  `json:"iban_masked"`
}

func (c *guardianCompositionContext) paymentOverview(t *testing.T) []paymentOverviewRow {
	t.Helper()
	rr := c.do(t, withPerms(testutil.DefaultTestClaims(), financialPerm), http.MethodGet, "/payment-overview", nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var response struct {
		Data []paymentOverviewRow `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))
	return response.Data
}

// users:update, the permission that maintains the directory, unlocks none
// of the bank surface.
func TestGuardianComposition_PaymentRequiresFinancialPermission(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	studentID, _ := ctx.createStudent("Payment", "Denied", "1a")
	guardianID, _ := ctx.createGuardian("payment-denied")
	ctx.link(studentID, guardianID, "custom")

	cases := []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{"read", http.MethodGet, fmt.Sprintf("/%d/payment", guardianID), nil},
		{"update", http.MethodPut, fmt.Sprintf("/%d/payment", guardianID), map[string]any{"iban": testIBAN}},
		{"reveal", http.MethodPost, fmt.Sprintf("/%d/payment/reveal", guardianID), map[string]any{}},
		{"set payer", http.MethodPut, fmt.Sprintf("/students/%d/payer", studentID), map[string]any{"guardian_id": fmt.Sprintf("%d", guardianID)}},
		{"overview", http.MethodGet, "/payment-overview", nil},
		{"export", http.MethodPost, "/payment-overview/export", map[string]any{"format": "xlsx"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, _ := ctx.paymentRequest(t, tc.method, tc.path, tc.body, "users:update", "users:read")
			assert.Equal(t, http.StatusForbidden, code, "users:update must not unlock the bank surface")
		})
	}
}

// The plain users:update unlink clears the payer mark on its way out, so for
// the child's payer it must demand guardians:financial; a guardian who is
// not the payer still unlinks with users:update alone.
func TestGuardianComposition_RemovePayerNeedsFinancialPermission(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	accountID := ctx.staffAccount("Payer", "Unlinker")
	directoryClaims := guardianStaffClaims(accountID, permissions.UsersUpdate, permissions.UsersRead)
	financialClaims := guardianStaffClaims(accountID, permissions.UsersUpdate, permissions.UsersRead, permissions.GuardiansFinancial)
	studentID, _ := ctx.createStudent("Payer", "Unlink", "1a")
	payerID, _ := ctx.createGuardian("payer-unlink")
	ctx.link(studentID, payerID, "custom")
	otherID, _ := ctx.createGuardian("payer-unlink-other")
	ctx.link(studentID, otherID, "custom")
	ctx.requirePayerSet(t, studentID, &payerID)

	unlink := func(claims jwt.AppClaims, guardianID int64) (int, string) {
		rr := ctx.do(t, claims, http.MethodDelete, fmt.Sprintf("/students/%d/guardians/%d", studentID, guardianID), nil)
		return rr.Code, rr.Body.String()
	}

	code, body := unlink(directoryClaims, payerID)
	require.Equal(t, http.StatusForbidden, code, body)
	assert.Contains(t, body, "Zahler", "the refusal must say why the person cannot be removed")

	linked := ctx.studentGuardians(t, studentID)
	require.Len(t, linked, 2, "a refused unlink must leave both relationships in place")
	for _, row := range linked {
		assert.Equal(t, row.Guardian.ID == payerID, row.IsPayer, "the payer mark must be untouched")
	}

	code, body = unlink(directoryClaims, otherID)
	require.Equal(t, http.StatusOK, code, "a guardian who is not the payer unlinks with users:update alone: %s", body)
	code, body = unlink(financialClaims, payerID)
	require.Equal(t, http.StatusOK, code, "with guardians:financial the payer unlinks: %s", body)
	assert.Empty(t, ctx.studentGuardians(t, studentID))
}

// The list read never carries the full IBAN; the reveal that does is a
// separate audited request.
func TestGuardianComposition_PaymentMaskedByDefaultRevealOnDemand(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	studentID, _ := ctx.createStudent("Payment", "Masked", "1a")
	guardianID, _ := ctx.createGuardian("payment-masked")
	ctx.link(studentID, guardianID, "custom")

	code, body := ctx.paymentRequest(t, http.MethodPut, fmt.Sprintf("/%d/payment", guardianID), map[string]any{"iban": "de89 3704 0044 0532 0130 00"}, financialPerm)
	require.Equal(t, http.StatusOK, code, body)

	code, body = ctx.paymentRequest(t, http.MethodGet, fmt.Sprintf("/%d/payment", guardianID), nil, financialPerm)
	require.Equal(t, http.StatusOK, code, body)
	assert.Contains(t, body, `"iban_masked":"•••• 3000"`, "input must be normalized and stored")
	assert.NotContains(t, body, testIBAN, "the masked read must not leak the IBAN")

	code, body = ctx.paymentRequest(t, http.MethodPost, fmt.Sprintf("/%d/payment/reveal", guardianID), map[string]any{}, financialPerm)
	require.Equal(t, http.StatusOK, code, body)
	assert.Contains(t, body, fmt.Sprintf(`"iban":"%s"`, testIBAN))

	assert.Equal(t, 1, ctx.accessLogs(accessGuardianFinancialView))
	assert.Equal(t, 1, ctx.accessLogs(accessGuardianFinancialReveal))
}

func TestGuardianComposition_PaymentRejectsMalformedIBAN(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	studentID, _ := ctx.createStudent("Payment", "Invalid", "1a")
	guardianID, _ := ctx.createGuardian("payment-invalid")
	ctx.link(studentID, guardianID, "custom")

	for _, iban := range []string{
		"DE89370400440532013001", // valid shape, broken mod-97 checksum
		"DE8937040044",           // too short
		"370400440532013000",     // no country code
	} {
		code, body := ctx.paymentRequest(t, http.MethodPut, fmt.Sprintf("/%d/payment", guardianID), map[string]any{"iban": iban}, financialPerm)
		assert.Equal(t, http.StatusBadRequest, code, "IBAN %q must be refused", iban)
		assert.Contains(t, body, "Die IBAN ist nicht gültig", "IBAN %q", iban)
	}
}

func TestGuardianComposition_PaymentClearingRemovesTheStoredIBAN(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	studentID, _ := ctx.createStudent("Payment", "Cleared", "1a")
	guardianID, _ := ctx.createGuardian("payment-cleared")
	ctx.link(studentID, guardianID, "custom")

	code, _ := ctx.paymentRequest(t, http.MethodPut, fmt.Sprintf("/%d/payment", guardianID), map[string]any{"iban": testIBAN}, financialPerm)
	require.Equal(t, http.StatusOK, code)
	code, _ = ctx.paymentRequest(t, http.MethodPut, fmt.Sprintf("/%d/payment", guardianID), map[string]any{"iban": ""}, financialPerm)
	require.Equal(t, http.StatusOK, code)

	_, body := ctx.paymentRequest(t, http.MethodGet, fmt.Sprintf("/%d/payment", guardianID), nil, financialPerm)
	assert.Contains(t, body, `"iban_masked":null`)
}

func TestGuardianComposition_PaymentUnknownGuardianIsNotFound(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)

	code, _ := ctx.paymentRequest(t, http.MethodGet, "/99999/payment", nil, financialPerm)
	assert.Equal(t, http.StatusNotFound, code)
	code, body := ctx.paymentRequest(t, http.MethodGet, "/invalid/payment", nil, financialPerm)
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Contains(t, body, "konnte nicht zugeordnet werden")
}

// Assigning a second guardian moves the mark, never duplicates it.
func TestGuardianComposition_StudentPayerOnlyOnePerChild(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	studentID, _ := ctx.createStudent("Payer", "Single", "1a")
	motherID, _ := ctx.createGuardian("payer-mother")
	ctx.link(studentID, motherID, "custom")
	fatherID, _ := ctx.createGuardian("payer-father")
	ctx.link(studentID, fatherID, "custom")

	ctx.requirePayerSet(t, studentID, &motherID)
	ctx.assertPayer(t, studentID, &motherID)
	ctx.requirePayerSet(t, studentID, &fatherID)
	ctx.assertPayer(t, studentID, &fatherID)
	ctx.requirePayerSet(t, studentID, nil)
	ctx.assertPayer(t, studentID, nil)
}

func TestGuardianComposition_StudentPayerRejectsGuardianOfAnotherChild(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	studentID, _ := ctx.createStudent("Payer", "Own", "1a")
	otherID, _ := ctx.createStudent("Payer", "Other", "1b")
	strangerID, _ := ctx.createGuardian("payer-stranger")
	ctx.link(otherID, strangerID, "custom")

	code, body := ctx.setPayer(t, studentID, &strangerID)
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Contains(t, body, "nicht als erziehungsberechtigt eingetragen")
}

// guardians:financial alone does not unlock payer changes: the caller must
// also pass the active-student and verified-staff check.
func TestGuardianComposition_StudentPayerRequiresStudentWriteAccess(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	studentID, _ := ctx.createStudent("Payer", "Guarded", "1a")
	guardianID, _ := ctx.createGuardian("payer-guarded")
	ctx.link(studentID, guardianID, "custom")
	payerPath := fmt.Sprintf("/students/%d/payer", studentID)
	body := map[string]any{"guardian_id": fmt.Sprintf("%d", guardianID)}

	noStaffID := ctx.account("payer-no-staff")
	rr := ctx.do(t, guardianStaffClaims(noStaffID, permissions.UsersRead, permissions.GuardiansFinancial), http.MethodPut, payerPath, body)
	require.Equal(t, http.StatusForbidden, rr.Code, rr.Body.String())
	ctx.assertPayer(t, studentID, nil)

	ctx.requirePayerSet(t, studentID, &guardianID)
	ctx.assertPayer(t, studentID, &guardianID)

	ctx.graduate(studentID)
	code, _ := ctx.paymentRequest(t, http.MethodPut, payerPath, map[string]any{"guardian_id": nil}, "admin:*")
	require.Equal(t, http.StatusForbidden, code)
}

// The IBAN is entered once and both siblings resolve to it.
func TestGuardianComposition_SiblingsShareOneMaintainedIBAN(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	olderID, _ := ctx.createStudent("Sibling", "Older", "3a")
	youngerID, _ := ctx.createStudent("Sibling", "Younger", "1a")
	parentID, _ := ctx.createGuardian("sibling-parent")
	ctx.link(olderID, parentID, "custom")
	ctx.link(youngerID, parentID, "custom")

	code, _ := ctx.paymentRequest(t, http.MethodPut, fmt.Sprintf("/%d/payment", parentID), map[string]any{"iban": testIBAN}, financialPerm)
	require.Equal(t, http.StatusOK, code)
	for _, studentID := range []int64{olderID, youngerID} {
		ctx.requirePayerSet(t, studentID, &parentID)
	}

	seen := 0
	for _, row := range ctx.paymentOverview(t) {
		if row.StudentID != fmt.Sprintf("%d", olderID) && row.StudentID != fmt.Sprintf("%d", youngerID) {
			continue
		}
		seen++
		assert.Equal(t, "•••• 3000", row.IBANMasked, "both siblings resolve to the one maintained IBAN")
	}
	assert.Equal(t, 2, seen, "both siblings must appear in the list")
	assert.Equal(t, 1, ctx.accessLogs(accessGuardianPaymentOverview), "the overview read is access-logged once")
}

// The list shows its gaps: a child without a payer still appears.
func TestGuardianComposition_PaymentOverviewListsChildrenWithoutAPayer(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	unassignedID, _ := ctx.createStudent("Overview", "Unassigned", "2a")
	guardianID, _ := ctx.createGuardian("overview-unassigned")
	ctx.link(unassignedID, guardianID, "custom")

	var found *paymentOverviewRow
	rows := ctx.paymentOverview(t)
	for i := range rows {
		if rows[i].StudentID == fmt.Sprintf("%d", unassignedID) {
			found = &rows[i]
		}
	}
	require.NotNil(t, found, "a child without a payer must still appear")
	assert.Nil(t, found.GuardianID)
	assert.Empty(t, found.IBANMasked)
}

func TestGuardianComposition_PaymentOverviewExcludesSoftDeletedStudents(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	studentID, personID := ctx.createStudent("Overview", "Deleted", "2a")
	ctx.softDeletePerson(personID)

	for _, row := range ctx.paymentOverview(t) {
		assert.NotEqual(t, fmt.Sprintf("%d", studentID), row.StudentID)
	}
}

// The bulk export produces a file and is logged as its own event.
func TestGuardianComposition_PaymentExportRendersFileAndIsAuditedSeparately(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	studentID, _ := ctx.createStudent("Export", "Child", "1a")
	guardianID, _ := ctx.createGuardian("export-child")
	ctx.link(studentID, guardianID, "custom")
	code, _ := ctx.paymentRequest(t, http.MethodPut, fmt.Sprintf("/%d/payment", guardianID), map[string]any{"iban": testIBAN}, financialPerm)
	require.Equal(t, http.StatusOK, code)
	ctx.requirePayerSet(t, studentID, &guardianID)

	rr := ctx.do(t, withPerms(testutil.DefaultTestClaims(), financialPerm), http.MethodPost, "/payment-overview/export", map[string]any{"format": "xlsx"})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Header().Get("Content-Type"), "spreadsheetml")
	assert.Contains(t, rr.Header().Get("Content-Disposition"), "bankverbindungen.xlsx")
	assert.NotEmpty(t, rr.Body.Bytes())

	assert.Equal(t, 1, ctx.accessLogs(accessGuardianPaymentExport))
	assert.Equal(t, 0, ctx.accessLogs(accessGuardianPaymentOverview), "an export must not be logged as a plain list view")
}

// The exported file keeps the incomplete children and names what is
// missing: a child without a payer reads "Nicht zugeordnet", a payer without
// bank details "Fehlt". Dropping them would make a half-filled list look
// finished, which is exactly how a child ends up never being charged.
func TestGuardianComposition_PaymentExportNamesTheGaps(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	paidID, _ := ctx.createStudent("Export", "Paid", "1a")
	paidGuardianID, _ := ctx.createGuardian("export-paid")
	ctx.link(paidID, paidGuardianID, "custom")
	code, _ := ctx.paymentRequest(t, http.MethodPut, fmt.Sprintf("/%d/payment", paidGuardianID), map[string]any{"iban": testIBAN, "account_holder": "Sabine Export"}, financialPerm)
	require.Equal(t, http.StatusOK, code)
	ctx.requirePayerSet(t, paidID, &paidGuardianID)

	noBankID, _ := ctx.createStudent("Export", "NoBank", "2b")
	noBankGuardianID, _ := ctx.createGuardian("export-no-bank")
	ctx.link(noBankID, noBankGuardianID, "custom")
	ctx.requirePayerSet(t, noBankID, &noBankGuardianID)

	unassignedID, _ := ctx.createStudent("Export", "Unassigned", "3c")
	unassignedGuardianID, _ := ctx.createGuardian("export-unassigned")
	ctx.link(unassignedID, unassignedGuardianID, "custom")

	rr := ctx.do(t, withPerms(testutil.DefaultTestClaims(), financialPerm), http.MethodPost, "/payment-overview/export", map[string]any{"format": "xlsx"})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	strings := xlsxSharedStrings(t, rr.Body.Bytes())
	assert.Contains(t, strings, testIBAN, "the export carries the full IBAN")
	assert.Contains(t, strings, "Sabine Export")
	assert.Contains(t, strings, "Nicht zugeordnet", "a child without a payer stays in the list")
	assert.Contains(t, strings, "Fehlt", "a payer without bank details is visible as a gap")
	assert.Contains(t, strings, "3 Kinder, davon 1 mit Bankverbindung", "the subtitle says how complete the list is")
}

// xlsxSharedStrings returns the text content of the workbook's shared
// strings part.
func xlsxSharedStrings(t *testing.T, workbook []byte) string {
	t.Helper()
	archive, err := zip.NewReader(bytes.NewReader(workbook), int64(len(workbook)))
	require.NoError(t, err)
	for _, file := range archive.File {
		if file.Name != "xl/sharedStrings.xml" {
			continue
		}
		reader, err := file.Open()
		require.NoError(t, err)
		defer func() { _ = reader.Close() }()
		content, err := io.ReadAll(reader)
		require.NoError(t, err)
		return string(content)
	}
	t.Fatal("workbook has no shared strings part")
	return ""
}

// The trail records the change without becoming a second copy of the data.
func TestGuardianComposition_PaymentChangesAreAuditedWithMaskedValues(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	studentID, _ := ctx.createStudent("Audit", "Child", "1a")
	guardianID, _ := ctx.createGuardian("audit-child")
	ctx.link(studentID, guardianID, "custom")

	code, _ := ctx.paymentRequest(t, http.MethodPut, fmt.Sprintf("/%d/payment", guardianID), map[string]any{"iban": testIBAN}, financialPerm)
	require.Equal(t, http.StatusOK, code)

	rows := ctx.financialChanges(guardianID)
	require.Len(t, rows, 1)
	assert.Equal(t, "iban", rows[0].Field)
	assert.Equal(t, "•••• 3000", rows[0].NewValue)
	assert.NotContains(t, rows[0].NewValue, testIBAN, "the trail must not store the IBAN")
	assert.False(t, rows[0].HasStudentID, "a bank change is guardian-scoped")
}

// A rejected format is rejected BEFORE the rows are loaded, so no full-IBAN
// access-log entry is left behind.
func TestGuardianComposition_PaymentExportUnknownFormatLeavesNoAccessLog(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	studentID, _ := ctx.createStudent("Format", "Child", "1a")
	guardianID, _ := ctx.createGuardian("format-child")
	ctx.link(studentID, guardianID, "custom")
	code, _ := ctx.paymentRequest(t, http.MethodPut, fmt.Sprintf("/%d/payment", guardianID), map[string]any{"iban": testIBAN}, financialPerm)
	require.Equal(t, http.StatusOK, code)

	code, body := ctx.paymentRequest(t, http.MethodPost, "/payment-overview/export", map[string]any{"format": "csv"}, financialPerm)
	require.Equal(t, http.StatusBadRequest, code, body)
	assert.Equal(t, 0, ctx.accessLogs(accessGuardianPaymentExport), "a refused export must not be logged as a full-IBAN access")
}
