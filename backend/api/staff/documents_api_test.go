// Router-level tests for /api/staff/{id}/documents (#1424): the route gate
// admits any of the three category permissions, the service narrows per
// category (AU → staff_documents:health, Lohn → staff:financial, rest →
// users:update), uploads are magic-number-validated, and downloads serve
// the original filename as an attachment.
package staff_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakePDF carries the %PDF magic bytes http.DetectContentType keys off.
var fakePDF = []byte("%PDF-1.4\n1 0 obj\n<<>>\nendobj\ntrailer\n<<>>\n%%EOF\n")

func fakeDOCX(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	for name, content := range map[string]string{
		"[Content_Types].xml": "<Types/>",
		"word/document.xml":   "<w:document/>",
	} {
		entry, err := writer.Create(name)
		require.NoError(t, err)
		_, err = entry.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	return buf.Bytes()
}

func fakeZIP(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	entry, err := writer.Create("not-a-word-document.txt")
	require.NoError(t, err)
	_, err = entry.Write([]byte("not a DOCX"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	return buf.Bytes()
}

type documentsAPIContext struct {
	tc      *testContext
	staffID int64
	account int64
}

func setupDocumentsAPI(t *testing.T) *documentsAPIContext {
	t.Helper()
	tc := setupTestContext(t)
	suffix := time.Now().UnixNano()

	target := testpkg.CreateTestStaff(t, tc.db, "Dokumente", fmt.Sprintf("API-%d", suffix))
	account := testpkg.CreateTestAccount(t, tc.db, fmt.Sprintf("dokumente-api-%d@example.test", suffix))
	t.Cleanup(func() {
		ctx := testpkg.Ctx(t)
		_, _ = tc.db.ExecContext(ctx, `DELETE FROM users.staff_documents WHERE staff_id = ?`, target.ID)
		_, _ = tc.db.ExecContext(ctx, `DELETE FROM audit.staff_master_data_changes WHERE staff_id = ?`, target.ID)
		testpkg.CleanupStaffFixtures(t, tc.db, target.ID)
		testpkg.CleanupAuthFixtures(t, tc.db, account.ID)
		if pubDir, err := common.ResolvePublicDir(); err == nil {
			_ = os.RemoveAll(filepath.Join(pubDir, "uploads", "staff-documents"))
		}
	})

	return &documentsAPIContext{tc: tc, staffID: target.ID, account: account.ID}
}

func (c *documentsAPIContext) token(t *testing.T, perms ...string) string {
	t.Helper()
	claims := testutil.DefaultTestClaims()
	claims.ID = int(c.account)
	claims.Permissions = perms
	return testutil.MintTestJWT(t, claims)
}

func (c *documentsAPIContext) request(t *testing.T, method, path string, body io.Reader, contentType string, perms ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, body)
	req.Header.Set("Authorization", "Bearer "+c.token(t, perms...))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	rec := httptest.NewRecorder()
	c.tc.router.ServeHTTP(rec, req)
	return rec
}

func (c *documentsAPIContext) upload(t *testing.T, category, filename string, content []byte, perms ...string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	require.NoError(t, writer.WriteField("category", category))
	part, err := writer.CreateFormFile("file", filename)
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	return c.request(t, http.MethodPost, fmt.Sprintf("/staff/%d/documents", c.staffID), &buf, writer.FormDataContentType(), perms...)
}

// uploadedDocument extracts the created document from the response envelope.
func uploadedDocument(t *testing.T, body []byte) (id int64, filename string) {
	t.Helper()
	var resp struct {
		Data struct {
			ID       int64  `json:"id"`
			Filename string `json:"filename"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &resp))
	return resp.Data.ID, resp.Data.Filename
}

// Deliberately NOT parallel: the upload path and its cleanup share one
// directory under public/uploads, so two of these tests remove each other's
// files.
func TestStaffDocumentsAPI_UploadListDownloadDelete(t *testing.T) {
	c := setupDocumentsAPI(t)
	base := fmt.Sprintf("/staff/%d/documents", c.staffID)

	rec := c.upload(t, "zeugnis", "Erste-Hilfe Zeugnis.pdf", fakePDF, "users:update")
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	docID, filename := uploadedDocument(t, rec.Body.Bytes())
	require.NotZero(t, docID)
	assert.Equal(t, "Erste-Hilfe Zeugnis.pdf", filename)
	cleanups, err := c.tc.services.StaffDocuments.ListQueuedStaffDocumentFileCleanups(testpkg.Ctx(t))
	require.NoError(t, err)
	assert.Empty(t, cleanups, "a persisted document must complete its cleanup intent")

	rec = c.request(t, http.MethodGet, base, nil, "", "users:update")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"category":"zeugnis"`)
	assert.Contains(t, rec.Body.String(), `"visible_categories"`)
	assert.NotContains(t, rec.Body.String(), `"au_bescheinigung"`,
		"users:update must not see the AU category")

	downloadPath := fmt.Sprintf("%s/%d/download", base, docID)
	rec = c.request(t, http.MethodGet, downloadPath, nil, "", "users:update")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, fakePDF, rec.Body.Bytes())
	assert.Equal(t, "application/pdf", rec.Header().Get("Content-Type"))
	assert.Contains(t, rec.Header().Get("Content-Disposition"), "attachment")
	assert.Contains(t, rec.Header().Get("Content-Disposition"), "Erste-Hilfe Zeugnis.pdf")

	rec = c.request(t, http.MethodDelete, fmt.Sprintf("%s/%d", base, docID), nil, "", "users:update")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = c.request(t, http.MethodGet, downloadPath, nil, "", "users:update")
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	// A repeat DELETE is the durable cleanup retry route: metadata stays
	// soft-deleted while the handler re-attempts removal of any orphan bytes.
	rec = c.request(t, http.MethodDelete, fmt.Sprintf("%s/%d", base, docID), nil, "", "users:update")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

// Deliberately NOT parallel: the upload path and its cleanup share one
// directory under public/uploads, so two of these tests remove each other's
// files.
func TestStaffDocumentsAPI_PermissionMatrix(t *testing.T) {
	c := setupDocumentsAPI(t)
	base := fmt.Sprintf("/staff/%d/documents", c.staffID)

	// Route gate: the staff-list tier does not open the tab.
	rec := c.request(t, http.MethodGet, base, nil, "", "users:read")
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())

	// AU-Bescheinigungen need staff_documents:health end to end.
	rec = c.upload(t, "au_bescheinigung", "au.pdf", fakePDF, "users:update")
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	rec = c.upload(t, "au_bescheinigung", "au.pdf", fakePDF, "staff_documents:health")
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	auID, _ := uploadedDocument(t, rec.Body.Bytes())

	rec = c.request(t, http.MethodGet, fmt.Sprintf("%s/%d/download", base, auID), nil, "", "users:update")
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	rec = c.request(t, http.MethodGet, fmt.Sprintf("%s/%d/download", base, auID), nil, "", "staff_documents:health")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Lohnabrechnungen ride the payroll permission; a payroll-only account
	// sees exactly that category.
	rec = c.upload(t, "lohnabrechnung", "lohn-07.pdf", fakePDF, "staff:financial")
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	rec = c.request(t, http.MethodGet, base, nil, "", "staff:financial")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"lohnabrechnung"`)
	assert.NotContains(t, rec.Body.String(), `"zeugnis"`)
	assert.NotContains(t, rec.Body.String(), `"au_bescheinigung"`)
}

// Deliberately NOT parallel: the upload path and its cleanup share one
// directory under public/uploads, so two of these tests remove each other's
// files.
func TestStaffDocumentsAPI_ScheduledCleanupRetriesQueuedOrphanWithoutStaff(t *testing.T) {
	c := setupDocumentsAPI(t)
	const orphanStaffID = int64(987654321)
	storedName := fmt.Sprintf("orphan-%d.pdf", time.Now().UnixNano())
	ctx := testpkg.Ctx(t)
	require.NoError(t, c.tc.services.StaffDocuments.QueueStaffDocumentFileCleanup(ctx, orphanStaffID, storedName))
	var retryAfter time.Time
	require.NoError(t, c.tc.db.NewRaw(`SELECT retry_after FROM users.staff_document_file_cleanup WHERE filename_stored = ?`, storedName).Scan(ctx, &retryAfter))
	assert.WithinDuration(t, time.Now().Add(5*time.Minute), retryAfter, 5*time.Second)
	cleanups, err := c.tc.services.StaffDocuments.ListQueuedStaffDocumentFileCleanups(ctx)
	require.NoError(t, err)
	assert.Empty(t, cleanups, "an in-progress upload must not be picked up by cleanup retries")
	require.NoError(t, c.tc.services.StaffDocuments.ActivateQueuedStaffDocumentFileCleanup(ctx, storedName))
	t.Cleanup(func() {
		_, _ = c.tc.db.ExecContext(context.Background(), `DELETE FROM users.staff_document_file_cleanup WHERE filename_stored = ?`, storedName)
	})

	pubDir, err := common.ResolvePublicDir()
	require.NoError(t, err)
	filePath := filepath.Join(pubDir, "uploads", "staff-documents", strconv.FormatInt(testpkg.Tenant(t), 10), storedName)
	require.NoError(t, os.MkdirAll(filepath.Dir(filePath), 0o750))
	require.NoError(t, os.WriteFile(filePath, fakePDF, 0o600))

	removed, err := c.tc.resource.CleanupOrphanedStaffDocumentFiles(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, removed)
	_, err = os.Stat(filePath)
	assert.ErrorIs(t, err, os.ErrNotExist)

	cleanups, err = c.tc.services.StaffDocuments.ListQueuedStaffDocumentFileCleanups(ctx)
	require.NoError(t, err)
	assert.Empty(t, cleanups)
}

// The cleanup pass must never delete the bytes of an upload whose metadata
// transaction is still open: that transaction has already stamped the intent
// complete and may commit right after, leaving a document row without a file.
// Deliberately NOT parallel: the upload path and its cleanup share one
// directory under public/uploads, so two of these tests remove each other's
// files.
func TestStaffDocumentsAPI_ScheduledCleanupSkipsUncommittedUpload(t *testing.T) {
	c := setupDocumentsAPI(t)
	const orphanStaffID = int64(987654322)
	storedName := fmt.Sprintf("inflight-%d.pdf", time.Now().UnixNano())
	ctx := testpkg.Ctx(t)
	require.NoError(t, c.tc.services.StaffDocuments.QueueStaffDocumentFileCleanup(ctx, orphanStaffID, storedName))
	t.Cleanup(func() {
		_, _ = c.tc.db.ExecContext(context.Background(), `DELETE FROM users.staff_document_file_cleanup WHERE filename_stored = ?`, storedName)
	})
	require.NoError(t, c.tc.services.StaffDocuments.ActivateQueuedStaffDocumentFileCleanup(ctx, storedName))

	pubDir, err := common.ResolvePublicDir()
	require.NoError(t, err)
	filePath := filepath.Join(pubDir, "uploads", "staff-documents", strconv.FormatInt(testpkg.Tenant(t), 10), storedName)
	require.NoError(t, os.MkdirAll(filepath.Dir(filePath), 0o750))
	require.NoError(t, os.WriteFile(filePath, fakePDF, 0o600))

	// Stand in for the upload's metadata transaction: it completes the intent
	// but has not committed yet, so it holds the row lock.
	uploadTx, err := c.tc.db.BeginTx(ctx, nil)
	require.NoError(t, err)
	rolledBack := false
	t.Cleanup(func() {
		if !rolledBack {
			_ = uploadTx.Rollback()
		}
	})
	_, err = uploadTx.ExecContext(ctx,
		`UPDATE users.staff_document_file_cleanup SET cleaned_at = NOW() WHERE filename_stored = ?`, storedName)
	require.NoError(t, err)

	cleanups, err := c.tc.services.StaffDocuments.ListQueuedStaffDocumentFileCleanups(ctx)
	require.NoError(t, err)
	assert.False(t, containsStoredName(cleanups, storedName),
		"an uncommitted upload's intent must not be eligible for cleanup")

	_, err = c.tc.resource.CleanupOrphanedStaffDocumentFiles(ctx)
	require.NoError(t, err)
	_, err = os.Stat(filePath)
	require.NoError(t, err, "the file of an uncommitted upload must survive the cleanup pass")

	// The upload fails after all: the intent stays queued and the pass recovers.
	require.NoError(t, uploadTx.Rollback())
	rolledBack = true

	removed, err := c.tc.resource.CleanupOrphanedStaffDocumentFiles(ctx)
	require.NoError(t, err)
	assert.Positive(t, removed)
	_, err = os.Stat(filePath)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func containsStoredName(cleanups []*users.StaffDocumentFileCleanup, storedName string) bool {
	for _, cleanup := range cleanups {
		if cleanup.FilenameStored == storedName {
			return true
		}
	}
	return false
}

// Deliberately NOT parallel: the upload path and its cleanup share one
// directory under public/uploads, so two of these tests remove each other's
// files.
func TestStaffDocumentsAPI_DirectoryRetriesOffboardedStaffDocument(t *testing.T) {
	c := setupDocumentsAPI(t)
	rec := c.upload(t, "zeugnis", "offboarded.pdf", fakePDF, "users:update")
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	docID, _ := uploadedDocument(t, rec.Body.Bytes())
	ctx := testpkg.Ctx(t)
	var storedName string
	require.NoError(t, c.tc.db.NewRaw(`SELECT filename_stored FROM users.staff_documents WHERE id = ?`, docID).Scan(ctx, &storedName))

	pubDir, err := common.ResolvePublicDir()
	require.NoError(t, err)
	filePath := filepath.Join(pubDir, "uploads", "staff-documents", strconv.FormatInt(testpkg.Tenant(t), 10), storedName)
	_, err = os.Stat(filePath)
	require.NoError(t, err)

	_, err = c.tc.db.ExecContext(ctx, `UPDATE users.staff SET deleted_at = NOW() WHERE id = ?`, c.staffID)
	require.NoError(t, err)

	rec = c.request(t, http.MethodGet, "/staff/documents-directory", nil, "", "staff_documents:health")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	_, err = os.Stat(filePath)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

// Deliberately NOT parallel: the upload path and its cleanup share one
// directory under public/uploads, so two of these tests remove each other's
// files.
func TestStaffDocumentsAPI_ScheduledCleanupRetriesOffboardedStaffDocument(t *testing.T) {
	c := setupDocumentsAPI(t)
	rec := c.upload(t, "zeugnis", "offboarded-scheduled.pdf", fakePDF, "users:update")
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	docID, _ := uploadedDocument(t, rec.Body.Bytes())

	ctx := testpkg.Ctx(t)
	var storedName string
	require.NoError(t, c.tc.db.NewRaw(`SELECT filename_stored FROM users.staff_documents WHERE id = ?`, docID).Scan(ctx, &storedName))
	pubDir, err := common.ResolvePublicDir()
	require.NoError(t, err)
	filePath := filepath.Join(pubDir, "uploads", "staff-documents", strconv.FormatInt(testpkg.Tenant(t), 10), storedName)
	_, err = os.Stat(filePath)
	require.NoError(t, err)

	// Offboarding committed, but its after-commit unlink never ran: the staff
	// row is soft-deleted while the document row stays active. No UI route
	// reaches that record, so the scheduler pass must recover the file.
	_, err = c.tc.db.ExecContext(ctx, `UPDATE users.staff SET deleted_at = NOW() WHERE id = ?`, c.staffID)
	require.NoError(t, err)

	removed, err := c.tc.resource.CleanupOrphanedStaffDocumentFiles(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, removed)
	_, err = os.Stat(filePath)
	assert.ErrorIs(t, err, os.ErrNotExist)

	var fileDeletedAt *time.Time
	require.NoError(t, c.tc.db.NewRaw(`SELECT file_deleted_at FROM users.staff_documents WHERE id = ?`, docID).Scan(ctx, &fileDeletedAt))
	require.NotNil(t, fileDeletedAt, "a removed file must be marked so the next pass skips it")

	// Second pass: nothing left to do.
	removed, err = c.tc.resource.CleanupOrphanedStaffDocumentFiles(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, removed)
}

// Deliberately NOT parallel: the upload path and its cleanup share one
// directory under public/uploads, so two of these tests remove each other's
// files.
func TestStaffDocumentsAPI_ScheduledCleanupRetriesDeletedActiveStaffDocument(t *testing.T) {
	c := setupDocumentsAPI(t)
	rec := c.upload(t, "zeugnis", "deleted.pdf", fakePDF, "users:update")
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	docID, _ := uploadedDocument(t, rec.Body.Bytes())

	ctx := testpkg.Ctx(t)
	var storedName string
	require.NoError(t, c.tc.db.NewRaw(`SELECT filename_stored FROM users.staff_documents WHERE id = ?`, docID).Scan(ctx, &storedName))
	pubDir, err := common.ResolvePublicDir()
	require.NoError(t, err)
	filePath := filepath.Join(pubDir, "uploads", "staff-documents", strconv.FormatInt(testpkg.Tenant(t), 10), storedName)
	_, err = c.tc.db.NewRaw(`UPDATE users.staff_documents SET deleted_at = NOW(), file_deleted_at = NULL WHERE id = ?`, docID).Exec(ctx)
	require.NoError(t, err)

	removed, err := c.tc.resource.CleanupOrphanedStaffDocumentFiles(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, removed)
	_, err = os.Stat(filePath)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

// Deliberately NOT parallel: the upload path and its cleanup share one
// directory under public/uploads, so two of these tests remove each other's
// files.
func TestStaffDocumentsAPI_FileValidation(t *testing.T) {
	c := setupDocumentsAPI(t)

	// Executables (and anything else off the allow-list) are rejected by
	// magic bytes.
	rec := c.upload(t, "sonstiges", "tool.exe", []byte("MZ\x90\x00executable"), "users:update")
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	// A bare ZIP is rejected even though DOCX shares its magic bytes...
	rec = c.upload(t, "sonstiges", "archiv.zip", fakeDOCX(t), "users:update")
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	// ...while a structurally valid OOXML package under a .docx name passes.
	rec = c.upload(t, "sonstiges", "vertrag.docx", fakeDOCX(t), "users:update")
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "officedocument.wordprocessingml.document")

	// A ZIP renamed to .docx is not a Word document without its OOXML parts.
	rec = c.upload(t, "sonstiges", "archiv.docx", fakeZIP(t), "users:update")
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	// Unknown categories are a 400, not a 500.
	rec = c.upload(t, "geheim", "datei.pdf", fakePDF, "users:update")
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	// Oversized uploads are refused at the advertised cap.
	big := bytes.Repeat([]byte("A"), 10*1024*1024+1)
	copy(big, "%PDF-1.4")
	rec = c.upload(t, "sonstiges", "gross.pdf", big, "users:update")
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}
