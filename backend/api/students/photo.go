package students

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// authorizePhotoMutation enforces the same per-student write gate as
// updateStudent/deleteStudent: the route-level users:update permission is
// only the coarse precondition; the fine-grained "can this account modify
// THIS student" decision lives in canUpdateStudent and must run on every
// write path. Without this gate, a supervisor of group A could upload or
// delete the photo of a student in group B just because they hold the
// users:update permission. Both upload and delete share this guard so the
// authorization surface stays uniform.
func (rs *Resource) authorizePhotoMutation(w http.ResponseWriter, r *http.Request, student *users.Student) bool {
	userPermissions := jwt.PermissionsFromCtx(r.Context())
	authorized, authErr := canUpdateStudent(r.Context(), userPermissions, student, rs.UserContextService)
	if !authorized {
		renderError(w, r, ErrorForbidden(authErr))
		return false
	}
	return true
}

// Student photo storage constants. Mirrors the school-login-image flow in
// api/config/settings_api.go: SaveImage writes under publicPhotoBaseDir (a
// path relative to the resolved public/ directory) and the URL we persist
// on the row is `common.StudentPhotoStoredURLPrefix + filename` so callers
// can serve the image without knowing the on-disk layout.
const (
	maxStudentPhotoBody = 5 * 1024 * 1024 // 5 MiB — same cap as user avatars
	publicPhotoBaseDir  = "public/uploads/student-photos"
)

// User-facing and internal error strings reused across handlers. Extracted
// to constants so the same wording appears at every emission point and so
// SonarCloud's S1192 (duplicated string literal) rule does not flag the
// triple-occurrence sites.
const (
	msgPhotosFeatureDisabled = "Kinderfotos sind in dieser Schule nicht aktiviert" //nolint:staticcheck // ST1005: user-facing German message
	msgNoTenantContext       = "no tenant context"
)

// stampPhotoConsent writes the photo_consent_given_at + photo_consent_given_by
// columns on the student row, deriving the acting account from the JWT
// claims in ctx. Used by both the update flow (false→true transition in
// applyPhotoConsent) and the upload flow (`consent_acknowledged=true` on
// a row that hasn't recorded consent yet) so the audit columns are
// always populated identically — same actingID source, same `time.Now()`
// for the timestamp.
//
// Callers must check the consent gate themselves (re-stamping on a row
// that already carries consent would falsify the original audit
// timestamp; the original spec calls this out explicitly).
func stampPhotoConsent(ctx context.Context, student *users.Student) {
	actingClaims := jwt.ClaimsFromCtx(ctx)
	actingID := int64(actingClaims.ID)
	now := time.Now()
	student.PhotoConsentGivenAt = &now
	student.PhotoConsentGivenBy = &actingID
}

// ensurePhotoFeatureEnabled returns an error if the school has not opted in
// to the photo feature. Centralised so upload + delete enforce identically.
// Read failures fall closed: a settings outage must not let staff upload
// photos when the school never enabled the feature.
func (rs *Resource) ensurePhotoFeatureEnabled(ctx context.Context) error {
	if rs.SettingsService == nil {
		return errors.New("photos feature not available")
	}
	enabled, err := rs.SettingsService.ResolveBool(ctx, configModel.KeyStudentPhotosEnabled)
	if err != nil {
		return fmt.Errorf("photo feature lookup failed: %w", err)
	}
	if !enabled {
		return errors.New(msgPhotosFeatureDisabled)
	}
	return nil
}

// removeStudentPhotoFile resolves a stored photo URL back to its filesystem
// path and deletes the file. Errors are logged but never propagated — a
// missing file is fine (idempotent), and we never want to fail the surrounding
// DB write because of a stale unlink.
func removeStudentPhotoFile(storedURL string, logger *slog.Logger) {
	if storedURL == "" {
		return
	}
	path, err := common.ResolveStoredPath("public", storedURL, common.StudentPhotoStoredURLPrefix)
	if err != nil {
		if logger != nil {
			logger.Warn(
				"could not resolve student photo path for cleanup",
				"stored_url", storedURL,
				"error", err.Error(),
			)
		}
		return
	}
	common.RemoveImage(path)
}

// uploadStudentPhoto handles POST /api/students/{id}/photo.
//
// Pre-conditions enforced before persisting the row:
//  1. operations.student_photos_enabled is true for this tenant
//  2. EITHER the row already carries photo_consent_given_at, OR the
//     multipart form contains `consent_acknowledged=true` — in which case
//     the handler stamps consent atomically alongside the photo path so
//     the user can tick the checkbox + upload without an intermediate
//     "Speichern" round-trip. Audit columns (_at/_by) are filled from
//     the JWT acting account just like applyPhotoConsent does.
//
// Transaction discipline: this handler is mounted WITHOUT
// TenantTxMiddleware (see api.go). The multipart parse + disk save run
// OUTSIDE any tx so the bun pool connection is not pinned for the full
// duration of a slow client's 5 MiB upload. A single short WithTenantTx
// then runs all DB work — feature gate, FindByID + auth, consent
// precondition, lock + locked re-read + writes. On a tx error we unlink
// the just-saved file so the DB and disk never disagree.
//
// On success, the previous photo file (if any) is removed via the
// after-commit hook so a commit failure can't leave the row pointing at
// a deleted file.
func (rs *Resource) uploadStudentPhoto(w http.ResponseWriter, r *http.Request) {
	id, err := common.ParseID(r)
	if err != nil {
		renderError(w, r, ErrorInvalidRequest(errors.New(common.MsgInvalidStudentID)))
		return
	}

	tenantID := tenant.FromContext(r.Context())
	if tenantID <= 0 {
		renderError(w, r, ErrorInvalidRequest(errors.New(msgNoTenantContext)))
		return
	}

	// ParseImage runs BEFORE any tenant tx so the multipart-body read
	// (potentially many seconds on a slow client at the 5 MiB cap) does
	// not pin a bun pool connection. The route-level
	// RequiresPermission(users:update) gate is the coarse precondition
	// that keeps random callers out; the fine-grained per-student auth
	// runs inside the tx below.
	uploaded, err := common.ParseImage(w, r, "photo", maxStudentPhotoBody)
	if err != nil {
		renderError(w, r, ErrorInvalidRequest(err))
		return
	}
	defer common.CloseFile(uploaded.File)

	// Read consent_acknowledged AFTER ParseImage so the multipart form is
	// fully parsed. r.FormValue is safe to call on the same multipart body.
	consentAck := r.FormValue("consent_acknowledged") == "true"

	// Filename prefix `{tenantID}_{studentID}` keeps photos for different
	// schools/students from ever colliding inside the shared dir, and makes
	// orphaned files traceable back to a row in case of out-of-band cleanup.
	prefix := fmt.Sprintf("%d_%d", tenantID, id)
	filePath, err := common.SaveImage(uploaded.File, publicPhotoBaseDir, prefix, uploaded.ContentType)
	if err != nil {
		renderError(w, r, ErrorInternalServer(err))
		return
	}

	// DB stores the raw /uploads/... URL because the serve route uses
	// ResolveStoredPath against that prefix. The JSON response transforms
	// it into the authenticated /api/students/{id}/photo/{filename} URL
	// in populatePhotoFields so the browser can render <img src=…> via
	// same-origin cookie auth (Next.js GET proxy forwards the JWT).
	newURL := common.StudentPhotoStoredURLPrefix + filepath.Base(filePath)

	// In-tx error sentinels. Each maps to the appropriate 4xx in the
	// rollback path below; errors.Is keeps the dispatch resilient to
	// future wrapping. Anything else returned from the closure (real DB
	// errors, lock failures, etc.) falls through to the default 500.
	errFeatureDisabled := errors.New("photos feature not available")
	errStudentNotFound := errors.New("student not found")
	errStudentForbidden := errors.New("you can only update students in groups you supervise")
	errConsentRequired := errors.New("Einwilligung der Eltern muss vor dem Upload bestätigt werden") //nolint:staticcheck // ST1005: user-facing German message
	errFeatureDisabledMidUpload := errors.New("photos feature disabled mid-upload")
	errConsentWithdrawnMidUpload := errors.New("photo consent withdrawn mid-upload")
	// errStudentReassignedMidUpload covers the concurrency hole where a
	// concurrent admin moved the student to a group the caller does not
	// supervise between the in-tx FindByID auth check and the locked
	// FindByIDForUpdate re-read. We re-check on the locked row to be
	// safe even though both reads happen inside the same tx.
	errStudentReassignedMidUpload := errors.New("student reassigned out of caller's scope mid-upload")

	// oldURL is captured INSIDE the tx from the FOR-UPDATE-locked row, not
	// from the pre-tx student snapshot, because a concurrent withdrawal
	// could have changed photo_path between the in-tx read and our row
	// lock. Using the locked-row value guarantees we unlink the right
	// pre-image. Declared outside the closure so the post-commit unlink
	// can read it.
	var oldURL string

	// All DB work + auth + race guards run inside this single short tx.
	// The multipart parse + disk save above already happened — if the
	// tx returns an error, the err branch unlinks the file we wrote so
	// disk and DB stay consistent.
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		// Feature gate first — closed-fail on settings outage so we
		// don't write photos for a school that never opted in.
		if err := rs.ensurePhotoFeatureEnabled(ctx); err != nil {
			return errFeatureDisabled
		}

		// Load the student inside the tx (RLS-enforced). Distinguish
		// "row genuinely missing" (sql.ErrNoRows → 404) from real
		// repository / pool / canceled-context failures (→ 500).
		student, err := rs.StudentRepo.FindByID(ctx, id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return errStudentNotFound
			}
			return err
		}

		// Per-student write gate. canUpdateStudent decides off
		// student.GroupID and only admits admins / group supervisors.
		if ok, _ := canUpdateStudent(ctx, jwt.PermissionsFromCtx(ctx), student, rs.UserContextService); !ok {
			return errStudentForbidden
		}

		// Consent precondition: the row must already carry consent OR
		// the form must acknowledge it. Without either, refuse — staff
		// cannot bypass parental consent.
		if student.PhotoConsentGivenAt == nil && !consentAck {
			return errConsentRequired
		}

		// Two concurrency holes the in-tx checks above DO NOT cover,
		// both fixed below by holding row+advisory locks before we
		// mutate state:
		//
		//  1. Consent withdrawal lost-update. If another tab withdraws
		//     photo_consent_given between our FindByID and our locked
		//     re-read, a naive Update would re-stamp the withdrawn
		//     columns from the stale read. SELECT … FOR UPDATE under
		//     the same row lock the withdrawal would take serializes
		//     us: either we win and the withdrawal blocks until we
		//     commit, or the withdrawal wins and we read the
		//     post-withdrawal state and abort.
		//
		//  2. Feature-disable race. operations.student_photos_enabled
		//     may flip to false after our in-tx ensurePhotoFeatureEnabled.
		//     The disable's PurgeAllPhotos CTE only locks rows whose
		//     photo_path is currently non-null, so a brand-new upload
		//     (NULL at CTE-eval time) is NOT serialized against the
		//     UPDATE we'd otherwise commit. LockPhotoFeature acquires
		//     a per-tenant advisory lock that BOTH this upload and the
		//     disable's PurgeAllPhotos take.
		if err := rs.StudentRepo.LockPhotoFeature(ctx); err != nil {
			return err
		}
		if recheckErr := rs.ensurePhotoFeatureEnabled(ctx); recheckErr != nil {
			return errFeatureDisabledMidUpload
		}

		// (1) Lock the student row. SELECT … FOR UPDATE serializes us
		// against any concurrent UPDATE on the same row (consent
		// withdrawal, photo delete, profile edit, …) so the values we
		// read here are the values our UPDATE will overwrite.
		fresh, err := rs.StudentRepo.FindByIDForUpdate(ctx, student.ID)
		if err != nil {
			// Concurrent delete between our FindByID above and the
			// locked re-read here is "not found" — surface as 404
			// rather than letting the bare sql.ErrNoRows fall
			// through to the default 500.
			if errors.Is(err, sql.ErrNoRows) {
				return errStudentNotFound
			}
			return err
		}

		// Re-run the per-student write gate against the LOCKED row.
		// A concurrent admin reassignment between our auth check above
		// and this lock can leave the caller without authority.
		if ok, _ := canUpdateStudent(ctx, jwt.PermissionsFromCtx(ctx), fresh, rs.UserContextService); !ok {
			return errStudentReassignedMidUpload
		}

		// Consent re-check on the locked row.
		if fresh.PhotoConsentGivenAt == nil && !consentAck {
			return errConsentWithdrawnMidUpload
		}

		// Capture the oldURL from the locked row, then apply our
		// changes on top of `fresh` (the latest committed state).
		if fresh.PhotoPath != nil {
			oldURL = *fresh.PhotoPath
		}
		fresh.PhotoPath = &newURL
		if consentAck && fresh.PhotoConsentGivenAt == nil {
			stampPhotoConsent(ctx, fresh)
		}
		if err := rs.StudentRepo.Update(ctx, fresh); err != nil {
			return err
		}

		// Defer the previous-photo unlink and the SSE broadcast until
		// AFTER this tx commits. RegisterAfterCommit drops the queued
		// hook on rollback, so the row stays consistent with the
		// on-disk state.
		capturedOldURL := oldURL
		capturedNewURL := newURL
		studentID := student.ID
		// tenantID captured into the closure so the broadcast stays
		// scoped to THIS school. broadcastStudentUpdated routes via
		// BroadcastToTenant — without a non-zero tenantID it would no-op.
		capturedTenantID := tenantID
		tenant.RegisterAfterCommit(ctx, func() {
			if capturedOldURL != "" && capturedOldURL != capturedNewURL {
				removeStudentPhotoFile(capturedOldURL, rs.Logger)
			}
			rs.broadcastStudentUpdated(capturedTenantID, studentID)
		})
		return nil
	}); err != nil {
		// Clean up the file we just wrote — the DB never accepted it.
		common.RemoveImage(filePath)
		switch {
		case errors.Is(err, errFeatureDisabled):
			render.Status(r, http.StatusForbidden)
			renderError(w, r, ErrorForbidden(errors.New(msgPhotosFeatureDisabled)))
		case errors.Is(err, errStudentNotFound):
			renderError(w, r, ErrorNotFound(err))
		case errors.Is(err, errStudentForbidden):
			renderError(w, r, ErrorForbidden(err))
		case errors.Is(err, errConsentRequired):
			renderError(w, r, ErrorInvalidRequest(err))
		case errors.Is(err, errFeatureDisabledMidUpload):
			render.Status(r, http.StatusForbidden)
			renderError(w, r, ErrorForbidden(errors.New(msgPhotosFeatureDisabled)))
		case errors.Is(err, errConsentWithdrawnMidUpload):
			// 409 Conflict: request was valid when sent but the
			// underlying consent state changed before our tx could
			// commit. Frontend re-fetches the student and re-prompts
			// for consent before retrying.
			render.Status(r, http.StatusConflict)
			renderError(w, r, ErrorInvalidRequest(errors.New("Einwilligung der Eltern wurde widerrufen — bitte erneut bestätigen"))) //nolint:staticcheck // ST1005: user-facing German message
		case errors.Is(err, errStudentReassignedMidUpload):
			// 403 Forbidden: a concurrent reassignment dropped the
			// caller out of the student's scope while we held the
			// lock.
			renderError(w, r, ErrorForbidden(errors.New("you can only update students in groups you supervise")))
		default:
			renderError(w, r, ErrorInternalServer(err))
		}
		return
	}

	// Return the SAME proxy URL the read endpoints serialize, not the raw
	// /uploads/student-photos/{filename} stored path. The DB persists the
	// raw prefix because the cleanup + ResolveStoredPath helpers key off
	// it; the JSON-facing URL goes through BuildStudentPhotoServeURL so
	// `<img src=photo_url>` actually loads (the frontend cannot fetch
	// /uploads/* directly — it only knows the authenticated proxy at
	// /api/students/{id}/photo/{filename}). Without this conversion any
	// caller that uses the POST response — e.g. the new
	// uploadStudentPhoto() helper — sees a broken image until a follow-up
	// GET reshapes the URL.
	// id (parsed from the URL at the top of the handler) is the same
	// identifier the locked-row write used — the in-tx FindByID looks
	// up the row by this id, then FindByIDForUpdate locks it. No
	// separate `student` variable in scope here since the handler no
	// longer maintains a pre-tx student snapshot.
	servedURL := common.BuildStudentPhotoServeURL(id, newURL)
	common.Respond(w, r, http.StatusOK, map[string]string{"photo_url": servedURL}, "Foto hochgeladen")
}

// deleteStudentPhoto handles DELETE /api/students/{id}/photo.
//
// Clears the row's photo_path (consent metadata stays — withdrawing consent
// is a separate action through the regular update endpoint) and unlinks the
// file. Returns 200 even if no photo was set, so the frontend can call this
// idempotently when the user clicks "Foto entfernen".
func (rs *Resource) deleteStudentPhoto(w http.ResponseWriter, r *http.Request) {
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return
	}

	// Same per-student write gate as uploadStudentPhoto — see comment there.
	if !rs.authorizePhotoMutation(w, r, student) {
		return
	}

	if err := rs.ensurePhotoFeatureEnabled(r.Context()); err != nil {
		render.Status(r, http.StatusForbidden)
		renderError(w, r, ErrorForbidden(err))
		return
	}

	tenantID := tenant.FromContext(r.Context())
	if tenantID <= 0 {
		renderError(w, r, ErrorInvalidRequest(errors.New(msgNoTenantContext)))
		return
	}

	// Mirror the upload flow's locking discipline: take the per-tenant
	// advisory lock + re-read the student row FOR UPDATE inside the tx,
	// then derive oldURL from the locked row (not the pre-tx snapshot).
	// Without this, a concurrent upload that commits between
	// parseAndGetStudent and our tx would have its photo_path silently
	// clobbered by Update on the stale snapshot, while we'd unlink the
	// previous file and leave the freshly-uploaded file orphaned on disk.
	// With the lock, the upload either commits before us (we observe its
	// photo_path on the locked row and unlink THAT file when fulfilling
	// the user's delete request) or after us (it sees photo_path = NULL
	// and proceeds normally).
	// 403-mapped sentinel: a concurrent group reassignment moved the
	// student out of the caller's scope between snapshot and lock. The
	// pre-tx authorizePhotoMutation gate ran on student.GroupID from the
	// snapshot — re-running on the locked row catches the race window
	// the snapshot can't.
	errStudentReassignedMidDelete := errors.New("student reassigned out of caller's scope mid-delete")
	// 404-mapped sentinel: a concurrent delete removed the student row
	// between parseAndGetStudent above and the locked re-read here. The
	// non-racy path returns 404 for missing students, so the racy path
	// must too — bare sql.ErrNoRows would otherwise fall through to a
	// confusing 500 for a legitimate concurrent delete.
	errStudentNotFoundMidDelete := errors.New("student deleted between snapshot and lock")

	var oldURL string
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		if err := rs.StudentRepo.LockPhotoFeature(ctx); err != nil {
			return err
		}
		fresh, err := rs.StudentRepo.FindByIDForUpdate(ctx, student.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return errStudentNotFoundMidDelete
			}
			return err
		}

		// Re-run the per-student write gate against the LOCKED row —
		// see uploadStudentPhoto for the rationale (concurrent group
		// reassignment between snapshot and lock can let a supervisor
		// of the OLD group delete the photo of a now-foreign student).
		if ok, _ := canUpdateStudent(ctx, jwt.PermissionsFromCtx(ctx), fresh, rs.UserContextService); !ok {
			return errStudentReassignedMidDelete
		}

		if fresh.PhotoPath == nil || *fresh.PhotoPath == "" {
			// Idempotent no-op: nothing to clear on the locked row.
			return nil
		}
		oldURL = *fresh.PhotoPath
		fresh.PhotoPath = nil
		if err := rs.StudentRepo.Update(ctx, fresh); err != nil {
			return err
		}

		// Defer the file unlink + SSE broadcast to AFTER the outermost
		// tenant tx commits — students are routed through
		// TenantTxMiddleware, so this WithTenantTx is reusing the
		// middleware's tx and has not committed yet. If the outer commit
		// later fails, RegisterAfterCommit drops the queued hook so the
		// row stays consistent with the on-disk state.
		capturedURL := oldURL
		studentID := student.ID
		// Same tenant-scoping rationale as in uploadStudentPhoto.
		capturedTenantID := tenantID
		tenant.RegisterAfterCommit(ctx, func() {
			removeStudentPhotoFile(capturedURL, rs.Logger)
			rs.broadcastStudentUpdated(capturedTenantID, studentID)
		})
		return nil
	}); err != nil {
		// 403 the same way the pre-tx gate would — a concurrent
		// reassignment dropped the caller out of the student's scope
		// while we held the lock.
		if errors.Is(err, errStudentReassignedMidDelete) {
			renderError(w, r, ErrorForbidden(errors.New("you can only delete student photos in groups you supervise")))
			return
		}
		// 404 when a concurrent delete won the race for the row.
		if errors.Is(err, errStudentNotFoundMidDelete) {
			renderError(w, r, ErrorNotFound(errors.New("student not found")))
			return
		}
		renderError(w, r, ErrorInternalServer(err))
		return
	}

	if oldURL == "" {
		common.Respond(w, r, http.StatusOK, nil, "Kein Foto vorhanden")
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Foto entfernt")
}

// serveStudentPhoto handles GET /api/students/{id}/photo/{filename}.
//
// Authenticated route — staff with users:read whose per-student access
// passes authorize.CanReadStudent are allowed to fetch the photo. The
// route-level users:read permission is the coarse gate; the fine-grained
// "can this account read THIS student" decision (admin / all_staff scope
// / group-supervisor membership) lives in CanReadStudent and is mirrored
// from getStudent + every other student GET, so the photo bytes follow
// the same gdpr.student_data_scope policy as the rest of the row.
// Without this gate, a leaked photo URL would let a non-supervising
// staff member (or worse, a guardian holding users:read) fetch a child's
// avatar in a tenant that has scope=group_supervisors_only.
//
// Re-checks operations.student_photos_enabled here even though the JSON
// response also suppresses photo_url when off. A client that cached a
// photo URL before the feature was disabled would otherwise still be
// able to fetch the bytes — the gate must apply at the byte boundary
// too, not just at JSON serialization.
//
// Cache header is `private, no-cache`: the browser MAY keep bytes locally
// but MUST revalidate with this handler before reusing them, so feature
// disable / consent withdrawal / photo deletion all take effect on the
// next image load instead of remaining viewable until a max-age window
// elapses. ServeContent still returns 304 when the file is unchanged
// after our auth checks pass, so revalidation is cheap.
//
// Transaction discipline: this handler is mounted WITHOUT
// TenantTxMiddleware (see api.go). All DB work — student lookup, setting
// resolution, filename match, path resolution — runs inside a short
// WithTenantTx below. ServeImage (which streams bytes to the client and
// can block on slow connections) runs AFTER that tx commits and releases
// the bun pool connection. A photo-heavy page rendering N avatars must
// not pin N connections for the full duration of N file streams.
func (rs *Resource) serveStudentPhoto(w http.ResponseWriter, r *http.Request) {
	filename := chi.URLParam(r, "filename")
	if filename == "" {
		renderError(w, r, ErrorInvalidRequest(errors.New("filename required")))
		return
	}

	id, err := common.ParseID(r)
	if err != nil {
		renderError(w, r, ErrorInvalidRequest(errors.New(common.MsgInvalidStudentID)))
		return
	}

	tenantID := tenant.FromContext(r.Context())
	if tenantID <= 0 {
		renderError(w, r, ErrorInvalidRequest(errors.New(msgNoTenantContext)))
		return
	}

	// Outcome of the pre-stream tx: the on-disk path to serve, or one of
	// the typed sentinel errors below. Declared outside the closure so
	// ServeImage (post-commit) can read it without a second DB hit.
	var resolvedPath string

	errFeatureDisabled := errors.New("photos feature not available")
	errStudentNotFound := errors.New("student not found")
	errStudentForbidden := errors.New("dieses Foto ist für diesen Account nicht freigegeben")
	errFilenameMismatch := errors.New("dieses Foto gehört nicht zu diesem Kind")
	errNoPhoto := errors.New("kein Foto hinterlegt")
	errPathResolution := errors.New("could not resolve stored photo path")

	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		if err := rs.ensurePhotoFeatureEnabled(ctx); err != nil {
			return errFeatureDisabled
		}
		student, err := rs.StudentRepo.FindByID(ctx, id)
		if err != nil {
			// Distinguish "row genuinely missing" (sql.ErrNoRows
			// bubbles through DatabaseError.Unwrap) from real
			// repository / pool / canceled-context failures. A
			// blanket 404 mapping would silently degrade
			// avatar-heavy pages into "no photo" placeholders during
			// real outages — exactly the operational hiding flagged
			// in review. Surface infrastructure errors as 500 so
			// they show up in logs and dashboards instead.
			if errors.Is(err, sql.ErrNoRows) {
				return errStudentNotFound
			}
			return err
		}

		// Per-student read gate. Mirrors the predicate used by getStudent
		// and the other read endpoints — runs INSIDE the tenant tx so the
		// settings lookup honours RLS and so a feature/consent flip mid
		// flight doesn't sneak past the check. Returned BEFORE the
		// filename comparison so a non-authorised caller cannot probe
		// whether a given filename matches a student's current photo
		// (which would otherwise leak existence even with a 403).
		if !authorize.CanReadStudent(
			ctx,
			jwt.PermissionsFromCtx(ctx),
			student,
			rs.UserContextService,
			rs.SettingsService,
			rs.Logger,
		) {
			return errStudentForbidden
		}

		if student.PhotoPath == nil || *student.PhotoPath == "" {
			return errNoPhoto
		}
		// Constant-equality check — never serve a filename that doesn't
		// match the row's current photo, even if the file exists on disk.
		if filepath.Base(*student.PhotoPath) != filename {
			return errFilenameMismatch
		}
		path, err := common.ResolveStoredPath("public", *student.PhotoPath, common.StudentPhotoStoredURLPrefix)
		if err != nil {
			return errPathResolution
		}
		resolvedPath = path
		return nil
	}); err != nil {
		switch {
		case errors.Is(err, errFeatureDisabled):
			render.Status(r, http.StatusForbidden)
			renderError(w, r, ErrorForbidden(err))
		case errors.Is(err, errStudentNotFound):
			renderError(w, r, ErrorNotFound(err))
		case errors.Is(err, errStudentForbidden):
			renderError(w, r, ErrorForbidden(err))
		case errors.Is(err, errNoPhoto):
			renderError(w, r, ErrorNotFound(err))
		case errors.Is(err, errFilenameMismatch):
			renderError(w, r, ErrorForbidden(err))
		case errors.Is(err, errPathResolution):
			// Mirror the previous handler's mapping — a bad stored path
			// is treated as an authorization-style failure (403) rather
			// than a 500, since it usually means the path traversal
			// guard in ResolveStoredPath rejected an attempted escape.
			renderError(w, r, ErrorForbidden(err))
		default:
			renderError(w, r, ErrorInternalServer(err))
		}
		return
	}

	// Tx is committed and the connection is back in the pool. From here
	// on, only filesystem I/O and the response stream run.
	common.ServeImage(w, r, filepath.Dir(resolvedPath), filepath.Base(resolvedPath), "private, no-cache")
}

// Compile-time guard: the photo handlers reference Resource fields, ensuring
// the route wiring below stays in sync if Resource ever drops a dependency.
var _ = (*users.Student)(nil)
