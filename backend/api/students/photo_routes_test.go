// Package students_test exercises the three HTTP handlers in
// api/students/photo.go (uploadStudentPhoto, deleteStudentPhoto,
// serveStudentPhoto) end-to-end through the production
// Resource.Router(). The tests use the same hermetic pattern as
// usercontext_test.go: real database fixtures, real service factory,
// real signed JWT minted via testutil.MintTestJWT so the full
// middleware chain (Verifier → Authenticator → TenantMiddleware →
// TenantTxMiddleware) runs as it does in production.
//
// Why the helper file exists separately:
//
//   - photo_helpers_test.go is in `package students` (internal). It
//     pins the small helper functions (stampPhotoConsent,
//     ensurePhotoFeatureEnabled, removeStudentPhotoFile) without DB.
//   - this file is in `package students_test` (external). It pins
//     the HTTP handlers — including the auth gates, multipart parse,
//     consent gate, feature-disable map, filename mismatch, and
//     authenticated proxy URL.
//
// Test fixtures touch the real public/uploads/student-photos/
// directory because the handlers hard-code that path. Each test
// cleans up after itself via t.Cleanup so a failed run does not
// leave orphaned bytes around.
package students_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// init seeds JWT viper defaults so jwt.MustNewTokenAuth (called by
// students.NewResource → Router) succeeds in CI environments without
// a populated .env. Identical pattern to api/usercontext/usercontext_test.go
// and required because setupTestContext constructs the resource which
// MUST be able to verify and sign JWTs.
func init() {
	testutil.SeedTestJWTConfig()
}

// =============================================================================
// Local helpers — kept private to this file so they don't bleed into
// the existing photo_helpers_test.go (internal package) or
// test_helpers_test.go (external package, but unrelated).
// =============================================================================

// jpegBytes returns a minimal valid 1x1 JPEG. Using image/jpeg.Encode
// produces real magic bytes so common.detectContentType (which gates
// upload via http.DetectContentType) accepts the file. Hand-rolled
// "fake" bytes get rejected as application/octet-stream.
func jpegBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 255, G: 0, B: 0, A: 255})
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, nil), "encode test JPEG")
	return buf.Bytes()
}

// buildMultipart packages the given file bytes (or none) and form
// fields into a multipart body suitable for POST /api/students/{id}/photo.
// fileBytes == nil omits the file part entirely so we can exercise the
// "no file uploaded" branch of common.ParseImage.
func buildMultipart(t *testing.T, fieldName, filename string, fileBytes []byte, fields map[string]string) (body *bytes.Buffer, contentType string) {
	t.Helper()
	body = &bytes.Buffer{}
	mw := multipart.NewWriter(body)

	if fileBytes != nil {
		fw, err := mw.CreateFormFile(fieldName, filename)
		require.NoError(t, err, "create form file")
		_, err = io.Copy(fw, bytes.NewReader(fileBytes))
		require.NoError(t, err, "copy bytes into form file")
	}

	for k, v := range fields {
		require.NoError(t, mw.WriteField(k, v), "write form field %s", k)
	}

	require.NoError(t, mw.Close(), "close multipart writer")
	return body, mw.FormDataContentType()
}

// enableStudentPhotos flips operations.student_photos_enabled to true
// for tenant 1 and registers a cleanup. Without this, every upload /
// delete / serve hits the closed-fail gate and returns 403 before
// touching any other code path. We use SetValue with nil
// changedBy + nil permissions because the helper is for tests, not
// audit-bearing UI writes.
func enableStudentPhotos(t *testing.T, tc *testContext) {
	t.Helper()
	ctx := testpkg.Ctx(t)
	require.NoError(t,
		tc.services.Settings.SetValue(ctx, configModel.KeyStudentPhotosEnabled, true, nil, nil),
		"enable student_photos_enabled",
	)
	t.Cleanup(func() {
		_ = tc.services.Settings.ResetValue(ctx, configModel.KeyStudentPhotosEnabled, nil, nil)
	})
}

// adminBearer mints a bearer token for an admin claim scoped to
// tenant 1. The hub of every happy-path upload/delete/serve test
// case.
func adminBearer(t *testing.T) string {
	t.Helper()
	return testutil.MintTestJWT(t, testutil.AdminTestClaims(1))
}

// staffWithUsersUpdate returns the JWT claims for a staff member who
// holds users:update + users:read but who is NOT an admin and is NOT
// (by default) a group supervisor. Since #2329 the handler-level
// canUpdateStudent gate admits this caller for every child of the tenant.
func staffWithUsersUpdate(tb testing.TB, accountID int64) jwt.AppClaims {
	return jwt.AppClaims{
		ID:          int(accountID),
		Sub:         "staff@example.com",
		Username:    "staff",
		FirstName:   "Staff",
		LastName:    "Member",
		Roles:       []string{"user"},
		Permissions: []string{"users:read", "users:update"},
		TenantID:    testpkg.RebaseTenantID(tb, 1),
	}
}

// stampPhotoConsentRow flips photo_consent_given_at on the given
// student row directly via SQL. Tests that exercise the "consent
// already on row" branch use this to avoid running through the
// unrelated PUT /privacy-consent handler.
func stampPhotoConsentRow(t *testing.T, tc *testContext, studentID int64) {
	t.Helper()
	ctx := testpkg.Ctx(t)
	_, err := tc.db.ExecContext(ctx,
		`UPDATE users.students
		    SET photo_consent_given_at = now(),
		        photo_consent_given_by = 1
		  WHERE id = ?`,
		studentID,
	)
	require.NoError(t, err, "stamp photo consent")
}

func graduateStudent(t *testing.T, tc *testContext, studentID int64) {
	t.Helper()
	_, err := tc.db.ExecContext(testpkg.Ctx(t), `
		UPDATE users.students SET status = ? WHERE id = ?
	`, users.StudentStatusAlumnus, studentID)
	require.NoError(t, err)
}

// readStudentPhotoPath returns the current photo_path column for the
// student row, or empty string if NULL. Used to assert that
// upload/delete actually mutated the DB.
func readStudentPhotoPath(t *testing.T, tc *testContext, studentID int64) string {
	t.Helper()
	ctx := testpkg.Ctx(t)
	var path *string
	err := tc.db.NewSelect().
		ColumnExpr("photo_path").
		Table("users.students").
		Where("id = ?", studentID).
		Scan(ctx, &path)
	require.NoError(t, err, "read student photo_path")
	if path == nil {
		return ""
	}
	return *path
}

// seedStudentWithPhoto creates a real photo file under
// public/uploads/student-photos and writes the URL on the student's
// photo_path column. Returns the on-disk path so tests can verify
// the file is removed after a successful delete.
func seedStudentWithPhoto(t *testing.T, tc *testContext, studentID int64) (storedURL, onDisk string) {
	t.Helper()
	pubDir, err := common.ResolvePublicDir()
	require.NoError(t, err, "resolve public dir")

	dir := filepath.Join(pubDir, "uploads", "student-photos")
	require.NoError(t, os.MkdirAll(dir, 0o755), "mkdir student-photos")

	tmp, err := os.CreateTemp(dir, fmt.Sprintf("seed_%d_*.jpg", studentID))
	require.NoError(t, err, "create seed photo")
	require.NoError(t, tmp.Close())

	storedURL = common.StudentPhotoStoredURLPrefix + filepath.Base(tmp.Name())
	onDisk = tmp.Name()

	ctx := testpkg.Ctx(t)
	_, err = tc.db.ExecContext(ctx,
		`UPDATE users.students
		    SET photo_path = ?,
		        photo_consent_given_at = now(),
		        photo_consent_given_by = 1
		  WHERE id = ?`,
		storedURL, studentID,
	)
	require.NoError(t, err, "set photo_path")

	t.Cleanup(func() { _ = os.Remove(onDisk) })
	return storedURL, onDisk
}

// removeUploadedPhotoFile parses the JSON body returned by
// uploadStudentPhoto, extracts photo_url, walks it back to the
// on-disk path and unlinks the file so each happy-path test cleans
// up after itself. The handler keeps the file (it's the new state),
// so this helper is the test's responsibility.
func removeUploadedPhotoFile(t *testing.T, body []byte) {
	t.Helper()
	var resp struct {
		Data struct {
			PhotoURL string `json:"photo_url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return
	}
	if resp.Data.PhotoURL == "" {
		return
	}
	// /api/students/{id}/photo/{filename} → grab the trailing
	// filename and unlink under the canonical /uploads prefix.
	parts := strings.Split(resp.Data.PhotoURL, "/")
	if len(parts) == 0 {
		return
	}
	filename := parts[len(parts)-1]
	if filename == "" {
		return
	}
	pubDir, err := common.ResolvePublicDir()
	if err != nil {
		return
	}
	_ = os.Remove(filepath.Join(pubDir, "uploads", "student-photos", filename))
}

// =============================================================================
// uploadStudentPhoto — POST /api/students/{id}/photo
// =============================================================================

// TestUploadStudentPhoto_HappyPath_ConsentOnRow uploads a JPEG for a
// student whose row already carries photo_consent_given_at. The
// handler must persist photo_path, return 200 with a /api/students/...
// proxy URL (not the raw /uploads/... path), and emit a tenant
// broadcast so SSE listeners refresh. Without the consent on the row
// the handler would refuse — that branch is exercised in a separate
// test below.
func TestUploadStudentPhoto_HappyPath_ConsentOnRow(t *testing.T) {
	tc := setupTestContext(t)
	enableStudentPhotos(t, tc)

	student := testpkg.CreateTestStudent(t, tc.db, "PhotoUpload", "Happy", "PU1")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)
	stampPhotoConsentRow(t, tc, student.ID)

	body, contentType := buildMultipart(t, "photo", "test.jpg", jpegBytes(t), nil)
	req, err := http.NewRequest("POST", fmt.Sprintf("/%d/photo", student.ID), body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+adminBearer(t))

	rr := testutil.ExecuteRequest(tc.resource.Router(), req)
	t.Cleanup(func() { removeUploadedPhotoFile(t, rr.Body.Bytes()) })

	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
	assert.Contains(t, rr.Body.String(), "photo_url")
	assert.Contains(t, rr.Body.String(), fmt.Sprintf("/api/students/%d/photo/", student.ID),
		"response must use the authenticated proxy URL, not the raw /uploads/ path")

	stored := readStudentPhotoPath(t, tc, student.ID)
	assert.True(t, strings.HasPrefix(stored, common.StudentPhotoStoredURLPrefix),
		"DB must store the canonical /uploads/student-photos/ prefix, got %q", stored)

	// Tenant-scoped broadcast emitted so SSE clients re-fetch the
	// student. The RecordingBroadcaster captures every event; we
	// just need to see at least one.
	assert.NotEmpty(t, tc.broadcaster.Events(), "expected at least one broadcast event")
}

// TestUploadStudentPhoto_HappyPath_ConsentAcknowledged exercises the
// other happy-path branch: the student has NEVER had consent
// recorded, but the multipart form contains consent_acknowledged=true.
// The handler must atomically stamp consent and write photo_path so
// the user does not need an intermediate PUT round-trip.
func TestUploadStudentPhoto_HappyPath_ConsentAcknowledged(t *testing.T) {
	tc := setupTestContext(t)
	enableStudentPhotos(t, tc)

	student := testpkg.CreateTestStudent(t, tc.db, "PhotoUpload", "Ack", "PU2")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

	body, contentType := buildMultipart(t, "photo", "test.jpg", jpegBytes(t),
		map[string]string{"consent_acknowledged": "true"})
	req, _ := http.NewRequest("POST", fmt.Sprintf("/%d/photo", student.ID), body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+adminBearer(t))

	rr := testutil.ExecuteRequest(tc.resource.Router(), req)
	t.Cleanup(func() { removeUploadedPhotoFile(t, rr.Body.Bytes()) })

	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	// Assert the consent audit columns got stamped in the same tx.
	ctx := testpkg.Ctx(t)
	var s users.Student
	err := tc.db.NewSelect().Model(&s).Where("id = ?", student.ID).Scan(ctx)
	require.NoError(t, err)
	require.NotNil(t, s.PhotoConsentGivenAt, "consent timestamp must be stamped")
	require.NotNil(t, s.PhotoConsentGivenBy, "consent actor must be stamped")
}

// TestUploadStudentPhoto_InvalidStudentID — non-numeric URL param.
// The handler returns 400 before any DB work; useful regression
// against a future "lenient" parser silently coercing to 0.
func TestUploadStudentPhoto_InvalidStudentID(t *testing.T) {
	tc := setupTestContext(t)
	enableStudentPhotos(t, tc)

	body, contentType := buildMultipart(t, "photo", "test.jpg", jpegBytes(t), nil)
	req, _ := http.NewRequest("POST", "/not-a-number/photo", body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+adminBearer(t))

	rr := testutil.ExecuteRequest(tc.resource.Router(), req)
	assert.Equal(t, http.StatusBadRequest, rr.Code, "Body: %s", rr.Body.String())
}

// TestUploadStudentPhoto_NoFile — multipart present but no `photo`
// field. common.ParseImage maps this to "no file uploaded" and the
// handler returns 400. Without this assertion a future refactor
// could silently accept zero-byte files.
func TestUploadStudentPhoto_NoFile(t *testing.T) {
	tc := setupTestContext(t)
	enableStudentPhotos(t, tc)

	student := testpkg.CreateTestStudent(t, tc.db, "PhotoUpload", "NoFile", "PU3")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

	body, contentType := buildMultipart(t, "photo", "", nil, nil) // nil bytes → no file part
	req, _ := http.NewRequest("POST", fmt.Sprintf("/%d/photo", student.ID), body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+adminBearer(t))

	rr := testutil.ExecuteRequest(tc.resource.Router(), req)
	assert.Equal(t, http.StatusBadRequest, rr.Code, "Body: %s", rr.Body.String())
}

// TestUploadStudentPhoto_InvalidImageBytes — file part contains
// arbitrary bytes that http.DetectContentType resolves to
// application/octet-stream, not an allowed image MIME. The handler
// must reject with 400 before any disk write.
func TestUploadStudentPhoto_InvalidImageBytes(t *testing.T) {
	tc := setupTestContext(t)
	enableStudentPhotos(t, tc)

	student := testpkg.CreateTestStudent(t, tc.db, "PhotoUpload", "BadBytes", "PU4")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

	body, contentType := buildMultipart(t, "photo", "evil.exe",
		[]byte("definitely not an image — random ASCII bytes"), nil)
	req, _ := http.NewRequest("POST", fmt.Sprintf("/%d/photo", student.ID), body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+adminBearer(t))

	rr := testutil.ExecuteRequest(tc.resource.Router(), req)
	assert.Equal(t, http.StatusBadRequest, rr.Code, "Body: %s", rr.Body.String())
}

// TestUploadStudentPhoto_MissingConsent — student has no
// photo_consent_given_at AND form lacks consent_acknowledged. The
// handler must refuse with 400 + the German error message — staff
// cannot bypass parental consent. The file gets unlinked so disk
// stays clean.
func TestUploadStudentPhoto_MissingConsent(t *testing.T) {
	tc := setupTestContext(t)
	enableStudentPhotos(t, tc)

	student := testpkg.CreateTestStudent(t, tc.db, "PhotoUpload", "NoConsent", "PU5")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

	body, contentType := buildMultipart(t, "photo", "test.jpg", jpegBytes(t), nil)
	req, _ := http.NewRequest("POST", fmt.Sprintf("/%d/photo", student.ID), body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+adminBearer(t))

	rr := testutil.ExecuteRequest(tc.resource.Router(), req)
	assert.Equal(t, http.StatusBadRequest, rr.Code, "Body: %s", rr.Body.String())
	assert.Contains(t, rr.Body.String(), "Einwilligung",
		"German error message must surface so the OGS UI can display it verbatim")

	assert.Equal(t, "", readStudentPhotoPath(t, tc, student.ID),
		"refused upload must not persist a photo_path")
}

// TestUploadStudentPhoto_FeatureDisabled — operations.student_photos_enabled
// is false (the registry default). The handler must return 403 with
// the German error verbatim. Critical: a school that never opted in
// MUST NOT receive photo writes even from an admin.
func TestUploadStudentPhoto_FeatureDisabled(t *testing.T) {
	tc := setupTestContext(t)
	// Deliberately do NOT call enableStudentPhotos.

	student := testpkg.CreateTestStudent(t, tc.db, "PhotoUpload", "FeatureOff", "PU6")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)
	stampPhotoConsentRow(t, tc, student.ID) // make sure consent is NOT the failing gate

	body, contentType := buildMultipart(t, "photo", "test.jpg", jpegBytes(t), nil)
	req, _ := http.NewRequest("POST", fmt.Sprintf("/%d/photo", student.ID), body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+adminBearer(t))

	rr := testutil.ExecuteRequest(tc.resource.Router(), req)
	assert.Equal(t, http.StatusForbidden, rr.Code, "Body: %s", rr.Body.String())
	assert.Contains(t, rr.Body.String(), "nicht aktiviert",
		"feature-disabled response must use the canonical German message")
}

// TestUploadStudentPhoto_StaffOutsideGroup — a staff member holding
// users:update but no supervision of the child's group. Since #2329 the
// per-student write gate (canUpdateStudent) asks only for a staff record, so
// this upload must succeed exactly like an admin's.
func TestUploadStudentPhoto_StaffOutsideGroup(t *testing.T) {
	tc := setupTestContext(t)
	enableStudentPhotos(t, tc)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "ForeignGroup")
	student := testpkg.CreateTestStudent(t, tc.db, "PhotoUpload", "ForeignStu", "PU7")
	testpkg.AssignStudentToGroup(t, tc.db, student.ID, group.ID)
	stampPhotoConsentRow(t, tc, student.ID)

	staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "NonSup", "Staff")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID, student.ID, staff.ID)

	claims := staffWithUsersUpdate(t, account.ID)
	token := testutil.MintTestJWT(t, claims)

	body, contentType := buildMultipart(t, "photo", "test.jpg", jpegBytes(t), nil)
	req, _ := http.NewRequest("POST", fmt.Sprintf("/%d/photo", student.ID), body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := testutil.ExecuteRequest(tc.resource.Router(), req)
	t.Cleanup(func() { removeUploadedPhotoFile(t, rr.Body.Bytes()) })

	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
	assert.NotEqual(t, "", readStudentPhotoPath(t, tc, student.ID),
		"a successful upload must persist a photo_path")
}

// TestUploadStudentPhoto_StudentNotFound — non-existent ID inside
// the tenant. Handler must return 404 (not 500). A legitimate caller
// hitting a stale URL must see a clean error.
func TestUploadStudentPhoto_StudentNotFound(t *testing.T) {
	tc := setupTestContext(t)
	enableStudentPhotos(t, tc)

	body, contentType := buildMultipart(t, "photo", "test.jpg", jpegBytes(t), nil)
	req, _ := http.NewRequest("POST", "/9999999/photo", body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+adminBearer(t))

	rr := testutil.ExecuteRequest(tc.resource.Router(), req)
	assert.Equal(t, http.StatusNotFound, rr.Code, "Body: %s", rr.Body.String())
}

func TestUploadStudentPhoto_GraduatedStudentNotFound(t *testing.T) {
	tc := setupTestContext(t)
	enableStudentPhotos(t, tc)

	student := testpkg.CreateTestStudent(t, tc.db, "PhotoUpload", "Graduate", "PU7")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)
	graduateStudent(t, tc, student.ID)

	body, contentType := buildMultipart(t, "photo", "test.jpg", jpegBytes(t), map[string]string{"consent_acknowledged": "true"})
	req, _ := http.NewRequest("POST", fmt.Sprintf("/%d/photo", student.ID), body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+adminBearer(t))
	rr := testutil.ExecuteRequest(tc.resource.Router(), req)

	assert.Equal(t, http.StatusNotFound, rr.Code, "Body: %s", rr.Body.String())
	assert.Empty(t, readStudentPhotoPath(t, tc, student.ID))
}

// TestUploadStudentPhoto_ReplacesExistingPhoto — student already has
// a photo_path. A successful upload must overwrite the row, return
// 200, and queue the previous file for after-commit unlink. We do
// NOT assert the old file is gone (that is racy with the after-commit
// goroutine) — only that the row points at a NEW filename.
func TestUploadStudentPhoto_ReplacesExistingPhoto(t *testing.T) {
	tc := setupTestContext(t)
	enableStudentPhotos(t, tc)

	student := testpkg.CreateTestStudent(t, tc.db, "PhotoUpload", "Replace", "PU8")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)
	oldStored, oldDisk := seedStudentWithPhoto(t, tc, student.ID)

	body, contentType := buildMultipart(t, "photo", "test.jpg", jpegBytes(t), nil)
	req, _ := http.NewRequest("POST", fmt.Sprintf("/%d/photo", student.ID), body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+adminBearer(t))

	rr := testutil.ExecuteRequest(tc.resource.Router(), req)
	t.Cleanup(func() { removeUploadedPhotoFile(t, rr.Body.Bytes()) })
	t.Cleanup(func() { _ = os.Remove(oldDisk) })

	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	newStored := readStudentPhotoPath(t, tc, student.ID)
	assert.NotEqual(t, oldStored, newStored, "photo_path must be replaced with a new filename")
	assert.True(t, strings.HasPrefix(newStored, common.StudentPhotoStoredURLPrefix))
}

// =============================================================================
// deleteStudentPhoto — DELETE /api/students/{id}/photo
// =============================================================================

// TestDeleteStudentPhoto_HappyPath — student has a photo, admin
// deletes, row's photo_path goes NULL, response is 200 "Foto entfernt",
// and a broadcast is emitted.
func TestDeleteStudentPhoto_HappyPath(t *testing.T) {
	tc := setupTestContext(t)
	enableStudentPhotos(t, tc)

	student := testpkg.CreateTestStudent(t, tc.db, "PhotoDel", "Happy", "PD1")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)
	_, _ = seedStudentWithPhoto(t, tc, student.ID)

	tc.broadcaster.Reset() // reset before action

	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/%d/photo", student.ID), nil)
	req.Header.Set("Authorization", "Bearer "+adminBearer(t))
	rr := testutil.ExecuteRequest(tc.resource.Router(), req)

	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
	assert.Contains(t, rr.Body.String(), "Foto entfernt")
	assert.Equal(t, "", readStudentPhotoPath(t, tc, student.ID),
		"successful delete must NULL out photo_path")
	assert.NotEmpty(t, tc.broadcaster.Events(), "delete must emit a tenant broadcast")
}

// TestDeleteStudentPhoto_Idempotent — student has no photo yet. The
// handler must still return 200 with "Kein Foto vorhanden" so the
// frontend can fire DELETE blindly when the user clicks "Foto
// entfernen" without checking state first.
func TestDeleteStudentPhoto_Idempotent(t *testing.T) {
	tc := setupTestContext(t)
	enableStudentPhotos(t, tc)

	student := testpkg.CreateTestStudent(t, tc.db, "PhotoDel", "NoPhoto", "PD2")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/%d/photo", student.ID), nil)
	req.Header.Set("Authorization", "Bearer "+adminBearer(t))
	rr := testutil.ExecuteRequest(tc.resource.Router(), req)

	assert.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
	assert.Contains(t, rr.Body.String(), "Kein Foto vorhanden")
}

// TestDeleteStudentPhoto_FeatureDisabled — feature is off; the
// handler must reject even DELETE with 403. Without this gate, a
// school that disabled photos but still has rows pointing at files
// could not be administered consistently.
func TestDeleteStudentPhoto_FeatureDisabled(t *testing.T) {
	tc := setupTestContext(t)
	// Feature DELIBERATELY left disabled.

	student := testpkg.CreateTestStudent(t, tc.db, "PhotoDel", "FeatureOff", "PD3")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)
	_, _ = seedStudentWithPhoto(t, tc, student.ID)

	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/%d/photo", student.ID), nil)
	req.Header.Set("Authorization", "Bearer "+adminBearer(t))
	rr := testutil.ExecuteRequest(tc.resource.Router(), req)

	assert.Equal(t, http.StatusForbidden, rr.Code, "Body: %s", rr.Body.String())
}

// TestDeleteStudentPhoto_StaffOutsideGroup — same per-student write gate as
// upload: a staff member with users:update deletes any child's photo since
// #2329, supervision of the group no longer participates.
func TestDeleteStudentPhoto_StaffOutsideGroup(t *testing.T) {
	tc := setupTestContext(t)
	enableStudentPhotos(t, tc)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "ForeignGroupDel")
	student := testpkg.CreateTestStudent(t, tc.db, "PhotoDel", "Foreign", "PD4")
	testpkg.AssignStudentToGroup(t, tc.db, student.ID, group.ID)
	_, _ = seedStudentWithPhoto(t, tc, student.ID)

	staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "DelNonSup", "Staff")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID, student.ID, staff.ID)

	token := testutil.MintTestJWT(t, staffWithUsersUpdate(t, account.ID))

	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/%d/photo", student.ID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := testutil.ExecuteRequest(tc.resource.Router(), req)

	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
	assert.Equal(t, "", readStudentPhotoPath(t, tc, student.ID),
		"a successful delete must clear photo_path")
}

// TestDeleteStudentPhoto_StudentNotFound — non-existent ID. The
// pre-tx parseAndGetStudent maps to 404 before any tx work.
func TestDeleteStudentPhoto_StudentNotFound(t *testing.T) {
	tc := setupTestContext(t)
	enableStudentPhotos(t, tc)

	req, _ := http.NewRequest("DELETE", "/9999998/photo", nil)
	req.Header.Set("Authorization", "Bearer "+adminBearer(t))
	rr := testutil.ExecuteRequest(tc.resource.Router(), req)

	assert.Equal(t, http.StatusNotFound, rr.Code, "Body: %s", rr.Body.String())
}

func TestDeleteStudentPhoto_GraduatedStudentNotFound(t *testing.T) {
	tc := setupTestContext(t)
	enableStudentPhotos(t, tc)

	student := testpkg.CreateTestStudent(t, tc.db, "PhotoDel", "Graduate", "PD7")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)
	storedURL, diskPath := seedStudentWithPhoto(t, tc, student.ID)
	t.Cleanup(func() { _ = os.Remove(diskPath) })
	graduateStudent(t, tc, student.ID)

	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/%d/photo", student.ID), nil)
	req.Header.Set("Authorization", "Bearer "+adminBearer(t))
	rr := testutil.ExecuteRequest(tc.resource.Router(), req)

	assert.Equal(t, http.StatusNotFound, rr.Code, "Body: %s", rr.Body.String())
	assert.Equal(t, storedURL, readStudentPhotoPath(t, tc, student.ID))
}

// =============================================================================
// serveStudentPhoto — GET /api/students/{id}/photo/{filename}
// =============================================================================

// TestServeStudentPhoto_HappyPath — admin fetching a real seeded
// photo. Response is 200, Cache-Control is "private, no-cache" (so
// feature/consent/delete take effect on the next image load), and
// the body contains the on-disk bytes.
func TestServeStudentPhoto_HappyPath(t *testing.T) {
	tc := setupTestContext(t)
	enableStudentPhotos(t, tc)

	student := testpkg.CreateTestStudent(t, tc.db, "PhotoServe", "Happy", "PS1")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)
	storedURL, _ := seedStudentWithPhoto(t, tc, student.ID)
	filename := filepath.Base(storedURL)

	req, _ := http.NewRequest("GET", fmt.Sprintf("/%d/photo/%s", student.ID, filename), nil)
	req.Header.Set("Authorization", "Bearer "+adminBearer(t))
	rr := testutil.ExecuteRequest(tc.resource.Router(), req)

	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
	assert.Equal(t, "private, no-cache", rr.Header().Get("Cache-Control"),
		"cache header must force revalidation so disable/withdrawal take effect on next load")
}

// TestServeStudentPhoto_InvalidStudentID — non-numeric URL param.
// Returns 400 before any DB or filesystem work.
func TestServeStudentPhoto_InvalidStudentID(t *testing.T) {
	tc := setupTestContext(t)
	enableStudentPhotos(t, tc)

	req, _ := http.NewRequest("GET", "/abc/photo/foo.jpg", nil)
	req.Header.Set("Authorization", "Bearer "+adminBearer(t))
	rr := testutil.ExecuteRequest(tc.resource.Router(), req)

	assert.Equal(t, http.StatusBadRequest, rr.Code, "Body: %s", rr.Body.String())
}

// TestServeStudentPhoto_FeatureDisabled — feature off; even an
// admin with a previously-cached URL must be rejected. The byte
// boundary must enforce the gate, not just the JSON serializer.
func TestServeStudentPhoto_FeatureDisabled(t *testing.T) {
	tc := setupTestContext(t)
	// Photos enabled NOT called.

	student := testpkg.CreateTestStudent(t, tc.db, "PhotoServe", "FeatureOff", "PS2")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)
	// Seed photo BEFORE flipping (or rather: never flip on). Test
	// simulates "URL was cached when feature was on, then admin
	// turned it off".
	storedURL, _ := seedStudentWithPhoto(t, tc, student.ID)
	filename := filepath.Base(storedURL)

	req, _ := http.NewRequest("GET", fmt.Sprintf("/%d/photo/%s", student.ID, filename), nil)
	req.Header.Set("Authorization", "Bearer "+adminBearer(t))
	rr := testutil.ExecuteRequest(tc.resource.Router(), req)

	assert.Equal(t, http.StatusForbidden, rr.Code, "Body: %s", rr.Body.String())
}

// TestServeStudentPhoto_NoPhoto — student exists, feature is on, but
// photo_path is NULL. Returns 404 with the German "kein Foto
// hinterlegt" message.
func TestServeStudentPhoto_NoPhoto(t *testing.T) {
	tc := setupTestContext(t)
	enableStudentPhotos(t, tc)

	student := testpkg.CreateTestStudent(t, tc.db, "PhotoServe", "NoPhoto", "PS3")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

	req, _ := http.NewRequest("GET", fmt.Sprintf("/%d/photo/anything.jpg", student.ID), nil)
	req.Header.Set("Authorization", "Bearer "+adminBearer(t))
	rr := testutil.ExecuteRequest(tc.resource.Router(), req)

	assert.Equal(t, http.StatusNotFound, rr.Code, "Body: %s", rr.Body.String())
}

// TestServeStudentPhoto_FilenameMismatch — student has photo X.jpg
// stored, but the URL asks for Y.jpg. The handler must refuse with
// 403 EVEN IF Y.jpg exists on disk: serving any file other than the
// row's current photo would let a leaked filename outlive a delete
// or replacement.
func TestServeStudentPhoto_FilenameMismatch(t *testing.T) {
	tc := setupTestContext(t)
	enableStudentPhotos(t, tc)

	student := testpkg.CreateTestStudent(t, tc.db, "PhotoServe", "Mismatch", "PS4")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)
	_, _ = seedStudentWithPhoto(t, tc, student.ID)

	req, _ := http.NewRequest("GET", fmt.Sprintf("/%d/photo/wrong-filename.jpg", student.ID), nil)
	req.Header.Set("Authorization", "Bearer "+adminBearer(t))
	rr := testutil.ExecuteRequest(tc.resource.Router(), req)

	assert.Equal(t, http.StatusForbidden, rr.Code, "Body: %s", rr.Body.String())
	assert.Contains(t, rr.Body.String(), "gehört nicht zu diesem Kind",
		"German error must surface so the UI can show why the load failed")
}

// TestServeStudentPhoto_ForbiddenWithoutStaffRecord — the photo bytes follow
// CanReadStudent, so since #2329 every verified staff member may fetch them and
// the surviving refusal is an account without a staff record (guest, guardian)
// that merely holds users:read. Mirrors the policy used by getStudent and
// protects against leaked photo URLs.
func TestServeStudentPhoto_ForbiddenWithoutStaffRecord(t *testing.T) {
	tc := setupTestContext(t)
	enableStudentPhotos(t, tc)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "ForeignServeGroup")
	student := testpkg.CreateTestStudent(t, tc.db, "PhotoServe", "Foreign", "PS5")
	testpkg.AssignStudentToGroup(t, tc.db, student.ID, group.ID)
	storedURL, _ := seedStudentWithPhoto(t, tc, student.ID)
	filename := filepath.Base(storedURL)

	guest := testpkg.CreateTestAccount(t, tc.db, "photo-serve-guest@example.com")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID, student.ID, guest.ID)

	claims := jwt.AppClaims{
		ID:          int(guest.ID),
		Sub:         "noread@example.com",
		Username:    "noread",
		Roles:       []string{"user"},
		Permissions: []string{"users:read"},
		TenantID:    testpkg.Tenant(t),
	}
	token := testutil.MintTestJWT(t, claims)

	req, _ := http.NewRequest("GET", fmt.Sprintf("/%d/photo/%s", student.ID, filename), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := testutil.ExecuteRequest(tc.resource.Router(), req)

	assert.Equal(t, http.StatusForbidden, rr.Code, "Body: %s", rr.Body.String())
}

// TestServeStudentPhoto_AllowedForStaffOutsideGroup — the counterpart: a staff
// member who does not supervise the child's group fetches the photo normally.
func TestServeStudentPhoto_AllowedForStaffOutsideGroup(t *testing.T) {
	tc := setupTestContext(t)
	enableStudentPhotos(t, tc)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "ForeignServeGroupOK")
	student := testpkg.CreateTestStudent(t, tc.db, "PhotoServe", "Allowed", "PS6")
	testpkg.AssignStudentToGroup(t, tc.db, student.ID, group.ID)
	storedURL, _ := seedStudentWithPhoto(t, tc, student.ID)
	filename := filepath.Base(storedURL)

	staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "ServeNonSup", "Staff")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID, student.ID, staff.ID)

	claims := jwt.AppClaims{
		ID:          int(account.ID),
		Sub:         "staffread@example.com",
		Username:    "staffread",
		Roles:       []string{"user"},
		Permissions: []string{"users:read"},
		TenantID:    testpkg.Tenant(t),
	}
	token := testutil.MintTestJWT(t, claims)

	req, _ := http.NewRequest("GET", fmt.Sprintf("/%d/photo/%s", student.ID, filename), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := testutil.ExecuteRequest(tc.resource.Router(), req)

	assert.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
}

// TestServeStudentPhoto_StudentNotFound — non-existent ID returns 404
// (not 500). The DB lookup distinguishes sql.ErrNoRows from real
// infrastructure failures so monitoring stays clean.
func TestServeStudentPhoto_StudentNotFound(t *testing.T) {
	tc := setupTestContext(t)
	enableStudentPhotos(t, tc)

	req, _ := http.NewRequest("GET", "/9999997/photo/anything.jpg", nil)
	req.Header.Set("Authorization", "Bearer "+adminBearer(t))
	rr := testutil.ExecuteRequest(tc.resource.Router(), req)

	assert.Equal(t, http.StatusNotFound, rr.Code, "Body: %s", rr.Body.String())
}

func TestServeStudentPhoto_GraduatedStudentNotFound(t *testing.T) {
	tc := setupTestContext(t)
	enableStudentPhotos(t, tc)

	student := testpkg.CreateTestStudent(t, tc.db, "PhotoServe", "Graduate", "PS7")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)
	storedURL, diskPath := seedStudentWithPhoto(t, tc, student.ID)
	t.Cleanup(func() { _ = os.Remove(diskPath) })
	graduateStudent(t, tc, student.ID)

	req, _ := http.NewRequest("GET", common.BuildStudentPhotoServeURL(student.ID, storedURL), nil)
	req.Header.Set("Authorization", "Bearer "+adminBearer(t))
	rr := testutil.ExecuteRequest(tc.resource.Router(), req)

	assert.Equal(t, http.StatusNotFound, rr.Code, "Body: %s", rr.Body.String())
}
