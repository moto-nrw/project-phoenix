// Package students_test exercises photo-related branches of
// updateStudent and deleteStudent in api/students/api.go that the
// existing students_test.go does not reach. The diff for the
// student-photo-avatar feature added several branches to these two
// handlers:
//
//   - updateStudent: photo-consent withdrawal triggers an after-commit
//     file unlink + tenant broadcast.
//   - updateStudent: photo-consent grant stamps photo_consent_given_at
//     and photo_consent_given_by from JWT claims.
//   - updateStudent: response payload includes photo_url only when the
//     tenant has opted into the feature.
//   - deleteStudent: the photo URL is captured from the locked row and
//     the file is unlinked after the OUTER tx commits.
//
// All tests here go through tc.resource.Router() via authExec so they
// reuse the same hermetic plumbing as students_test.go (real DB, real
// service factory, testpkg.RecordingBroadcaster).
package students_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// seedPhotoFile creates a real file under public/uploads/student-photos/
// and writes a matching photo_path on the student row. Returns the on-
// disk path so callers can assert removal. This mirrors the helper in
// photo_routes_test.go but re-implementing locally keeps this file
// independent of that helper's package-internal lifecycle.
func seedPhotoFile(t *testing.T, tc *testContext, studentID int64) (storedURL, onDisk string) {
	t.Helper()

	pubDir, err := common.ResolvePublicDir()
	require.NoError(t, err, "resolve public dir")
	dir := filepath.Join(pubDir, "uploads", "student-photos")
	require.NoError(t, os.MkdirAll(dir, 0o755), "mkdir student-photos")

	tmp, err := os.CreateTemp(dir, fmt.Sprintf("apipath_%d_*.jpg", studentID))
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

// enablePhotoFeatureForTenant flips operations.student_photos_enabled
// to true for tenant 1 and registers a reset cleanup. Without this,
// the response payload from the update handler will lack photo_url and
// the test cannot assert on enabled-only fields.
func enablePhotoFeatureForTenant(t *testing.T, tc *testContext) {
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

// readPhotoPath returns the photo_path column for the given student
// (or "" when NULL). Used to assert post-tx mutations without going
// back through the API layer.
func readPhotoPath(t *testing.T, tc *testContext, studentID int64) string {
	t.Helper()
	ctx := testpkg.Ctx(t)
	var path *string
	err := tc.db.NewSelect().
		ColumnExpr("photo_path").
		Table("users.students").
		Where("id = ?", studentID).
		Scan(ctx, &path)
	require.NoError(t, err, "read photo_path")
	if path == nil {
		return ""
	}
	return *path
}

// readConsentTimestamp returns true when photo_consent_given_at is not
// NULL on the student row. Used to assert the grant transition stamps
// the audit timestamp atomically.
func readConsentTimestamp(t *testing.T, tc *testContext, studentID int64) bool {
	t.Helper()
	ctx := testpkg.Ctx(t)
	var stamped bool
	err := tc.db.NewRaw(
		`SELECT photo_consent_given_at IS NOT NULL FROM users.students WHERE id = ?`,
		studentID,
	).Scan(ctx, &stamped)
	require.NoError(t, err, "read photo_consent_given_at")
	return stamped
}

// =============================================================================
// updateStudent — photo consent withdrawal
// =============================================================================

// TestUpdateStudent_PhotoConsentWithdrawal_DeletesFile pins the
// after-commit unlink hook on the consent-withdrawal branch. Starting
// state: student has a photo + consent. PUT with
// photo_consent_given=false must:
//
//  1. Clear photo_path, photo_consent_given_at, photo_consent_given_by
//     in a single tx (no orphaned audit columns).
//  2. Trigger removeStudentPhotoFile via tenant.RegisterAfterCommit so
//     the on-disk file is gone after the response.
//  3. Emit a broadcastStudentUpdated event so SSE listeners refetch.
func TestUpdateStudent_PhotoConsentWithdrawal_DeletesFile(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)
	enablePhotoFeatureForTenant(t, tc)

	student := testpkg.CreateTestStudent(t, tc.db, "PhotoWithdraw", "Student", "PW1")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

	_, onDisk := seedPhotoFile(t, tc, student.ID)

	eventCount := len(tc.broadcaster.Events())

	body := map[string]interface{}{"photo_consent_given": false}
	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	// photo_path must be NULLed; if the handler used the stale snapshot
	// instead of the locked row we'd silently leave the path pointing
	// at a deleted file.
	assert.Equal(t, "", readPhotoPath(t, tc, student.ID),
		"consent withdrawal must NULL photo_path on the student row")
	// Audit timestamp must clear in the same tx so we don't ship a
	// row with photo_path=NULL but consent_given_at=non-NULL.
	assert.False(t, readConsentTimestamp(t, tc, student.ID),
		"consent withdrawal must NULL photo_consent_given_at")

	// File must be gone post-commit (RegisterAfterCommit ran).
	_, err := os.Stat(onDisk)
	assert.True(t, os.IsNotExist(err),
		"after-commit unlink must remove the file from disk; err=%v", err)

	// Tenant-scoped broadcast must fire so other tabs refetch.
	require.GreaterOrEqual(t, len(tc.broadcaster.Events()), eventCount+1,
		"consent withdrawal must emit at least one broadcast event")
}

// =============================================================================
// updateStudent — photo consent grant
// =============================================================================

// TestUpdateStudent_PhotoConsentGrant_StampsAuditFields verifies that
// transitioning a student from no-consent to consent_given=true through
// the update endpoint stamps photo_consent_given_at and
// photo_consent_given_by atomically — no follow-up "Speichern" round-
// trip is required to make the audit columns appear.
//
// Without the in-tx applyPhotoConsent + stampPhotoConsent invocation
// the row would carry consent_given_at=NULL even after the OK response,
// which would later trip the consent gate on photo upload.
func TestUpdateStudent_PhotoConsentGrant_StampsAuditFields(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)
	enablePhotoFeatureForTenant(t, tc)

	student := testpkg.CreateTestStudent(t, tc.db, "PhotoGrant", "Student", "PG1")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

	// Start state: no consent. Sanity check the precondition.
	require.False(t, readConsentTimestamp(t, tc, student.ID),
		"precondition: student should not have consent stamped yet")

	body := map[string]interface{}{"photo_consent_given": true}
	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	assert.True(t, readConsentTimestamp(t, tc, student.ID),
		"grant transition must stamp photo_consent_given_at in the same tx")

	// photo_consent_given_by must also be populated. We assert it via a
	// raw SELECT because applyPhotoConsent is a pure helper — only
	// stampPhotoConsent (called from the handler) writes the actor
	// column from the JWT claims.
	ctx := testpkg.Ctx(t)
	var byActor int64
	err := tc.db.NewRaw(
		`SELECT COALESCE(photo_consent_given_by, 0) FROM users.students WHERE id = ?`,
		student.ID,
	).Scan(ctx, &byActor)
	require.NoError(t, err)
	assert.NotZero(t, byActor,
		"grant transition must stamp photo_consent_given_by from JWT acting account")
}

// TestUpdateStudent_PhotoConsentNoChange_NoOp pins the omitted-field
// branch: when the request does not include photo_consent_given,
// applyPhotoConsent is a no-op and the audit columns are preserved.
// This is the equivalent of a name-only edit on a student that already
// carries consent — the operation must not unstamp the original audit
// timestamp.
func TestUpdateStudent_PhotoConsentNoChange_NoOp(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)
	enablePhotoFeatureForTenant(t, tc)

	student := testpkg.CreateTestStudent(t, tc.db, "PhotoNoOp", "Student", "PN1")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

	// Stamp consent directly so we can verify it survives an unrelated
	// edit.
	ctx := testpkg.Ctx(t)
	_, err := tc.db.ExecContext(ctx,
		`UPDATE users.students
		    SET photo_consent_given_at = now(),
		        photo_consent_given_by = 1
		  WHERE id = ?`, student.ID)
	require.NoError(t, err)
	require.True(t, readConsentTimestamp(t, tc, student.ID))

	// Update an unrelated field. photo_consent_given is not present.
	body := map[string]interface{}{"first_name": "RenamedNoOp"}
	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	assert.True(t, readConsentTimestamp(t, tc, student.ID),
		"omitted photo_consent_given must not unstamp the audit timestamp")
}

// =============================================================================
// deleteStudent — photo file removal
// =============================================================================

// TestDeleteStudent_RemovesPhotoFile pins the deleteStudent branch that
// captures the photo URL from the LOCKED row inside the tx and unlinks
// the file via tenant.RegisterAfterCommit. Without this, deleting a
// student would leave their JPEG orphaned under public/uploads/.
func TestDeleteStudent_RemovesPhotoFile(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)

	student := testpkg.CreateTestStudent(t, tc.db, "PhotoDel", "ApiPath", "PD1")
	// Seed a photo file BEFORE the delete so we can confirm it's gone
	// after the response. We don't rely on the delete handler caring
	// about the photo feature flag — the unlink branch runs whenever
	// fresh.PhotoPath is non-NULL, regardless of feature toggle state.
	_, onDisk := seedPhotoFile(t, tc, student.ID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/%d", student.ID), nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	_, err := os.Stat(onDisk)
	assert.True(t, os.IsNotExist(err),
		"deleteStudent must unlink the photo file after commit; stat err=%v", err)

	// Student row must be gone.
	ctx := testpkg.Ctx(t)
	var count int
	err = tc.db.NewRaw(
		`SELECT count(*) FROM users.students WHERE id = ?`, student.ID,
	).Scan(ctx, &count)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "student row must be deleted")
}

// TestDeleteStudent_NoPhotoSucceeds covers the other half of the
// in-tx branch: when fresh.PhotoPath is NULL the unlink is skipped
// and the delete still succeeds. This protects against a regression
// where the unlink hook fires unconditionally and the unguarded path
// resolution returns an error on empty input.
func TestDeleteStudent_NoPhotoSucceeds(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)

	student := testpkg.CreateTestStudent(t, tc.db, "DelNoPhoto", "ApiPath", "DN1")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/%d", student.ID), nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	ctx := testpkg.Ctx(t)
	var count int
	err := tc.db.NewRaw(
		`SELECT count(*) FROM users.students WHERE id = ?`, student.ID,
	).Scan(ctx, &count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

// =============================================================================
// updateStudent — feature-flag-gated response payload
// =============================================================================

// TestUpdateStudent_PhotoEnabled_ResponseIncludesPhotoURL verifies that
// when operations.student_photos_enabled is true and the student has a
// photo, the response payload includes a photo_url field. This pins
// the configService.ResolveBoolOrDefault branch in updateStudent
// (line ~1252) — without the feature flag enabled the response strips
// photo-related fields, even on a row that has a photo_path.
func TestUpdateStudent_PhotoEnabled_ResponseIncludesPhotoURL(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)
	enablePhotoFeatureForTenant(t, tc)

	student := testpkg.CreateTestStudent(t, tc.db, "PhotoResp", "Enabled", "PR1")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)
	_, _ = seedPhotoFile(t, tc, student.ID)

	body := map[string]interface{}{"first_name": "RenamedResp"}
	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
	assert.Contains(t, rr.Body.String(), "photo_url",
		"response with feature enabled + photo on row must include photo_url")

	// Cleanup — remove the on-disk file so a failed assertion above
	// doesn't leave the JPEG behind.
	// Cleanup runs before CleanupActivityFixtures (LIFO). The on-disk
	// file is registered for removal by seedPhotoFile; nothing else is
	// needed here.
}

// TestUpdateStudent_PhotoDisabled_ResponseOmitsPhotoURL asserts the
// other side of the same flag: when the tenant has not opted in, the
// response payload must not leak photo URL details — even if the row
// happens to carry an old photo_path.
func TestUpdateStudent_PhotoDisabled_ResponseOmitsPhotoURL(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)
	// Deliberately do NOT enable the feature.

	student := testpkg.CreateTestStudent(t, tc.db, "PhotoResp", "Disabled", "PR2")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)
	_, _ = seedPhotoFile(t, tc, student.ID)

	body := map[string]interface{}{"first_name": "RenamedDisabled"}
	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
	assert.NotContains(t, rr.Body.String(), `"photo_url"`,
		"response with feature disabled must not surface photo_url")

	// Cleanup runs before CleanupActivityFixtures (LIFO). The on-disk
	// file is registered for removal by seedPhotoFile; nothing else is
	// needed here.
}

// =============================================================================
// updateStudent — sentinel: 404 when student is deleted between
// snapshot and lock. We don't need to engineer a real race; we just
// invoke updateStudent with an ID for a row that was never created
// after the parseAndGetStudent gate would pass — but parseAndGetStudent
// itself rejects unknown IDs with 404, so the in-tx
// errStudentNotFoundUnderLock branch is hard to exercise without a
// concurrent delete. Skip this case — students_test.go's
// not_found_for_nonexistent_student already covers the pre-tx 404.
// =============================================================================

// noop — placeholder so context.Context import is used should we add
// a future test that needs ctx without other plumbing.
var _ context.Context
