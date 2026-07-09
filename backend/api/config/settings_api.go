package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/realtime"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	enrollmentSvc "github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Login image upload constants
const (
	maxLoginImageSize = 2 * 1024 * 1024 // 2MB — advertised file limit
	// MaxBytesReader limits the entire multipart body, not just the file.
	// Add 1 KiB headroom for multipart boundaries, headers, and CRLF padding
	// so files at exactly the advertised limit are not rejected.
	maxLoginImageBody = maxLoginImageSize + 1024
	loginImageDir     = "public/uploads/login-images"

	maxLegalAGBDocumentSize = 10 * 1024 * 1024 // 10MB — legal PDFs can be longer than image assets
	maxLegalAGBDocumentBody = maxLegalAGBDocumentSize + 4096
	legalAGBDocumentDir     = "public/uploads/enrollment-legal-documents"
	legalAGBDocumentPrefix  = "/uploads/enrollment-legal-documents/"
)

var (
	errLegalAGBDocumentManagedByUpload = errors.New("AGB document URL is managed by the file upload endpoint")
	errCannotDeleteActiveLegalAGBPDF   = errors.New("AGB-PDF kann nicht entfernt werden, solange die AGB aktiv sind und als PDF angezeigt werden")
)

// errOperatorOnlyForTenant explains why a tenant admin may not read/write an
// AccessOperatorOnly setting. Surfaced as HTTP 403. The UI already hides these
// settings; this is belt-and-braces defence against direct API calls.
const errOperatorOnlyForTenant = "this setting is operator-only"

// guardTenantAccess blocks tenant-admin read/write on AccessOperatorOnly settings.
// Returns true when the handler should abort (response has already been written).
func guardTenantAccess(w http.ResponseWriter, r *http.Request, key string) bool {
	def := configModel.GetDefinition(key)
	if def == nil {
		common.RenderError(w, r, common.ErrorNotFound(fmt.Errorf("setting %q not found", key)))
		return true
	}
	if def.AccessPolicy == configModel.AccessOperatorOnly {
		common.RenderError(w, r, common.ErrorForbidden(errors.New(errOperatorOnlyForTenant)))
		return true
	}
	return false
}

func guardDirectManagedSettingWrite(w http.ResponseWriter, r *http.Request, key string) bool {
	if key != configModel.KeyEnrollmentLegalAGBDocumentURL {
		return false
	}
	common.RenderError(w, r, common.ErrorForbidden(errLegalAGBDocumentManagedByUpload))
	return true
}

// ValueSetCallback runs in the same tenant transaction as the setting
// write. Returning an error aborts the request and rolls back the update.
//
// The optional postCommit closure runs ONLY if the transaction commits
// successfully. Use it for non-transactional side effects that must not
// happen if the DB write rolls back (file-system unlinks, external API
// calls, anything we can't undo on rollback). Return nil when no post-
// commit work is needed.
type ValueSetCallback func(ctx context.Context, tenantID int64, key string, value any) (postCommit func(), err error)

// SettingsResource defines the settings API resource.
type SettingsResource struct {
	settingsService   configSvc.SettingsService
	legalDocumentRefs legalAGBDocumentReferenceRepository
	db                *bun.DB
	broadcaster       realtime.Broadcaster
	onValueSet        ValueSetCallback
}

// OnValueSet registers a callback that runs after a setting value change is
// validated and persisted. See ValueSetCallback for the in-tx vs post-commit
// contract.
func (rs *SettingsResource) OnValueSet(fn ValueSetCallback) {
	rs.onValueSet = fn
}

// scheduleSettingsBroadcast delegates to the shared cross-portal helper; see
// common.ScheduleTenantSettingsBroadcast for the SSE contract.
func (rs *SettingsResource) scheduleSettingsBroadcast(ctx context.Context, tenantID int64, key string) {
	common.ScheduleTenantSettingsBroadcast(ctx, rs.broadcaster, tenantID, key)
}

// NewSettingsResource creates a new settings resource. broadcaster is
// optional — when supplied, the resource emits a tenant_settings_changed
// SSE event after every successful Set/Reset so other tabs (including
// cross-origin operator/tenant pairs) invalidate their settings caches.
// Same-origin tabs are already covered by the BroadcastChannel ping the
// frontend fires; this closes the cross-origin loop.
func NewSettingsResource(svc configSvc.SettingsService, db *bun.DB, broadcaster realtime.Broadcaster, legalDocumentRefs ...legalAGBDocumentReferenceRepository) *SettingsResource {
	rs := &SettingsResource{
		settingsService: svc,
		db:              db,
		broadcaster:     broadcaster,
	}
	if len(legalDocumentRefs) > 0 {
		rs.legalDocumentRefs = legalDocumentRefs[0]
	}
	return rs
}

// SettingsRouter returns a configured router for settings endpoints.
func (rs *SettingsResource) SettingsRouter() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	common.ProtectedTenantGroup(r, rs.db, func(r chi.Router, withTx common.Middleware) {

		settingsWrite := authorize.RequiresAnyPermission(permissions.ConfigUpdate, permissions.ConfigManage)

		r.With(authorize.RequiresPermission(permissions.ConfigRead), withTx).Get("/schema", rs.getSchema)
		r.With(settingsWrite, withTx).Get("/values/{key}/reveal", rs.revealValue)
		r.With(settingsWrite, withTx).Put("/values/{key}", rs.setValue)
		r.With(settingsWrite, withTx).Delete("/values/{key}", rs.resetValue)

		// Login image — reads use withTx (tenant role), writes use WithAdminTx internally
		// because platform.schools requires the phoenix_admin role for UPDATE.
		// withTx is intentionally omitted on POST/DELETE to avoid conflicting role contexts.
		// GET uses settingsWrite (not ConfigRead) so write-capable roles can also read the
		// login-image metadata — the frontend depends on this GET to enable upload/delete controls.
		settingsReadOrWrite := authorize.RequiresAnyPermission(permissions.ConfigRead, permissions.ConfigUpdate, permissions.ConfigManage)
		r.With(settingsReadOrWrite, withTx).Get("/login-image", rs.getLoginImage)
		r.With(settingsWrite).Post("/login-image", rs.uploadLoginImage)
		r.With(settingsWrite).Delete("/login-image", rs.deleteLoginImage)

		// AGB document writes manage a file-system side effect. Like login-image
		// writes, they open their own tenant tx so file cleanup only runs after
		// the DB write has committed.
		r.With(settingsWrite).Post("/enrollment/legal-agb-document", rs.uploadEnrollmentLegalAGBDocument)
		r.With(settingsWrite).Delete("/enrollment/legal-agb-document", rs.deleteEnrollmentLegalAGBDocument)
	})

	return r
}

// --- Request types ---

type setValueRequest struct {
	Value any `json:"value"`
}

// shouldReplayResetHook gates which keys re-run their OnValueSet callback
// on DELETE so reset can trigger reset-specific side effects. Limited to
// keys whose registry default carries semantic meaning (e.g. student photo
// purge on disable). Other settings keep their pre-feature reset behavior.
func shouldReplayResetHook(key string) bool {
	return key == configModel.KeyStudentPhotosEnabled
}

// --- Handlers ---

func (rs *SettingsResource) getSchema(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())

	schema, err := rs.settingsService.GetSchema(r.Context(), claims.Permissions)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, schema, "Schema retrieved successfully")
}

func (rs *SettingsResource) revealValue(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if guardTenantAccess(w, r, key) {
		return
	}

	claims := jwt.ClaimsFromCtx(r.Context())

	// Check that the user has write permission (reveal requires write, not just read).
	def := configModel.GetDefinition(key)
	if def.WritePermission != "" && !authorize.HasPermission(def.WritePermission, claims.Permissions) {
		common.RenderError(w, r, common.ErrorForbidden(fmt.Errorf("insufficient permissions for %q", key)))
		return
	}

	value, err := rs.settingsService.Resolve(r.Context(), key)
	if err != nil {
		renderSettingsError(w, r, err)
		return
	}

	common.Respond(w, r, http.StatusOK, map[string]any{"value": value}, "")
}

func (rs *SettingsResource) setValue(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if guardTenantAccess(w, r, key) {
		return
	}
	if guardDirectManagedSettingWrite(w, r, key) {
		return
	}

	var req setValueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	claims := jwt.ClaimsFromCtx(r.Context())
	changedBy := int64(claims.ID)

	tenantID := tenant.FromContext(r.Context())
	err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		if err := rs.settingsService.SetValue(ctx, key, req.Value, &changedBy, claims.Permissions); err != nil {
			return err
		}
		if rs.onValueSet != nil {
			cb, err := rs.onValueSet(ctx, tenantID, key, req.Value)
			if err != nil {
				return err
			}
			// Post-commit closure runs only after the OUTERMOST tenant tx
			// commits. SettingsRouter wraps these handlers in
			// TenantTxMiddleware, so the inner WithTenantTx merely reuses
			// the middleware's already-open transaction; the real COMMIT
			// happens once the handler returns. Running destructive cleanup
			// before that point would unlink files / invalidate caches even
			// if the outer commit later fails.
			tenant.RegisterAfterCommit(ctx, cb)
		}
		rs.scheduleSettingsBroadcast(ctx, tenantID, key)
		return nil
	})
	if err != nil {
		renderSettingsError(w, r, err)
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Value updated successfully")
}

func (rs *SettingsResource) resetValue(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if guardTenantAccess(w, r, key) {
		return
	}
	if guardDirectManagedSettingWrite(w, r, key) {
		return
	}

	claims := jwt.ClaimsFromCtx(r.Context())
	changedBy := int64(claims.ID)
	tenantID := tenant.FromContext(r.Context())

	err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		if err := rs.settingsService.ResetValue(ctx, key, &changedBy, claims.Permissions); err != nil {
			return err
		}
		// Replay the OnValueSet hook for keys that need reset-specific side
		// effects (e.g. clearing photo files when student_photos_enabled is
		// reset to its default-off state). Otherwise a "Zurücksetzen" click
		// on student_photos_enabled would clear the override but leave
		// already-stored photos on disk.
		if rs.onValueSet != nil && shouldReplayResetHook(key) {
			def := configModel.GetDefinition(key)
			if def != nil {
				cb, err := rs.onValueSet(ctx, tenantID, key, def.Default)
				if err != nil {
					return err
				}
				tenant.RegisterAfterCommit(ctx, cb)
			}
		}
		rs.scheduleSettingsBroadcast(ctx, tenantID, key)
		return nil
	})
	if err != nil {
		renderSettingsError(w, r, err)
		return
	}

	common.RespondNoContent(w, r)
}

// --- Login image handlers ---

type loginImageResponse struct {
	LoginImageURL *string `json:"login_image_url"`
	CanEdit       bool    `json:"can_edit"`
}

// getLoginImage returns the current login image URL and edit permission for the tenant.
func (rs *SettingsResource) getLoginImage(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())
	if tenantID <= 0 {
		render.Status(r, http.StatusBadRequest)
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("no tenant context")))
		return
	}

	claims := jwt.ClaimsFromCtx(r.Context())

	canEdit := authorize.HasPermission(permissions.ConfigUpdate, claims.Permissions) ||
		authorize.HasPermission(permissions.ConfigManage, claims.Permissions)

	url, err := rs.settingsService.GetLoginImageURL(r.Context(), tenantID)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	resp := loginImageResponse{CanEdit: canEdit}
	if url != "" {
		resp.LoginImageURL = &url
	}

	common.Respond(w, r, http.StatusOK, resp, "")
}

// uploadLoginImage handles uploading a custom login page image for the tenant.
func (rs *SettingsResource) uploadLoginImage(w http.ResponseWriter, r *http.Request) {
	uploaded, err := common.ParseImage(w, r, "login_image", maxLoginImageBody)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	defer common.CloseFile(uploaded.File)

	tenantID := tenant.FromContext(r.Context())
	if tenantID <= 0 {
		render.Status(r, http.StatusBadRequest)
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("no tenant context")))
		return
	}

	prefix := fmt.Sprintf("%d", tenantID)
	filePath, err := common.SaveImage(uploaded.File, loginImageDir, prefix, uploaded.ContentType)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	imageURL := "/uploads/login-images/" + filepath.Base(filePath)
	oldURL, err := rs.settingsService.SetLoginImageURL(r.Context(), tenantID, imageURL)
	if err != nil {
		common.RemoveImage(filePath)
		render.Status(r, http.StatusInternalServerError)
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Clean up old file only after DB commit succeeded
	if oldURL != "" {
		if oldPath, resolveErr := common.ResolveStoredPath("public", oldURL, "/uploads/login-images/"); resolveErr == nil {
			common.RemoveImage(oldPath)
		}
	}

	common.Respond(w, r, http.StatusOK, map[string]string{"login_image_url": imageURL}, "Login image uploaded successfully")
}

// deleteLoginImage removes the custom login page image for the tenant.
func (rs *SettingsResource) deleteLoginImage(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())
	if tenantID <= 0 {
		render.Status(r, http.StatusBadRequest)
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("no tenant context")))
		return
	}

	oldURL, err := rs.settingsService.ClearLoginImageURL(r.Context(), tenantID)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Clean up old file only after DB commit succeeded
	if oldURL != "" {
		if oldPath, resolveErr := common.ResolveStoredPath("public", oldURL, "/uploads/login-images/"); resolveErr == nil {
			common.RemoveImage(oldPath)
		}
	}

	common.RespondNoContent(w, r)
}

// --- Enrollment legal document handlers ---

type legalAGBDocumentResponse struct {
	DocumentURL string `json:"document_url"`
}

type enrollmentLegalAGBDeleteSettings interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
	ResolveString(ctx context.Context, key string) (string, error)
}

type legalAGBDocumentReferenceRepository interface {
	HasLegalDocumentReference(ctx context.Context, storedURL, publicURL string) (bool, error)
}

type legalAGBDocumentReferenceFunc func(context.Context, string) (bool, error)
type legalAGBDocumentPathResolver func(string, string, string) (string, error)
type legalAGBDocumentRemover func(string)

func (rs *SettingsResource) uploadEnrollmentLegalAGBDocument(w http.ResponseWriter, r *http.Request) {
	uploaded, err := common.ParsePDFWithLimits(w, r, "document", maxLegalAGBDocumentSize, maxLegalAGBDocumentBody)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	defer common.CloseFile(uploaded.File)

	tenantID := tenant.FromContext(r.Context())
	if tenantID <= 0 {
		render.Status(r, http.StatusBadRequest)
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("no tenant context")))
		return
	}

	filePath, err := common.SavePDF(uploaded.File, legalAGBDocumentDir, fmt.Sprintf("%d", tenantID))
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	documentURL := legalAGBDocumentPrefix + filepath.Base(filePath)
	claims := jwt.ClaimsFromCtx(r.Context())
	changedBy := int64(claims.ID)

	err = tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		oldURL, resolveErr := rs.settingsService.ResolveString(ctx, configModel.KeyEnrollmentLegalAGBDocumentURL)
		if resolveErr != nil {
			return resolveErr
		}
		if setErr := rs.settingsService.SetValue(ctx, configModel.KeyEnrollmentLegalAGBDocumentURL, documentURL, &changedBy, claims.Permissions); setErr != nil {
			return setErr
		}
		if cleanupErr := rs.scheduleEnrollmentLegalAGBDocumentCleanup(ctx, oldURL, documentURL); cleanupErr != nil {
			return cleanupErr
		}
		rs.scheduleSettingsBroadcast(ctx, tenantID, configModel.KeyEnrollmentLegalAGBDocumentURL)
		return nil
	})
	if err != nil {
		common.RemoveImage(filePath)
		renderSettingsError(w, r, err)
		return
	}

	common.Respond(w, r, http.StatusOK, legalAGBDocumentResponse{DocumentURL: documentURL}, "AGB document uploaded successfully")
}

func (rs *SettingsResource) deleteEnrollmentLegalAGBDocument(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())
	if tenantID <= 0 {
		render.Status(r, http.StatusBadRequest)
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("no tenant context")))
		return
	}

	claims := jwt.ClaimsFromCtx(r.Context())
	changedBy := int64(claims.ID)

	err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		if guardErr := canDeleteEnrollmentLegalAGBDocument(ctx, rs.settingsService); guardErr != nil {
			return guardErr
		}
		oldURL, resolveErr := rs.settingsService.ResolveString(ctx, configModel.KeyEnrollmentLegalAGBDocumentURL)
		if resolveErr != nil {
			return resolveErr
		}
		if resetErr := rs.settingsService.ResetValue(ctx, configModel.KeyEnrollmentLegalAGBDocumentURL, &changedBy, claims.Permissions); resetErr != nil {
			return resetErr
		}
		if cleanupErr := rs.scheduleEnrollmentLegalAGBDocumentCleanup(ctx, oldURL, ""); cleanupErr != nil {
			return cleanupErr
		}
		rs.scheduleSettingsBroadcast(ctx, tenantID, configModel.KeyEnrollmentLegalAGBDocumentURL)
		return nil
	})
	if err != nil {
		if errors.Is(err, errCannotDeleteActiveLegalAGBPDF) {
			render.Status(r, http.StatusBadRequest)
			common.RenderError(w, r, common.ErrorInvalidRequest(err))
			return
		}
		renderSettingsError(w, r, err)
		return
	}

	common.RespondNoContent(w, r)
}

func canDeleteEnrollmentLegalAGBDocument(ctx context.Context, settingsService enrollmentLegalAGBDeleteSettings) error {
	termsEnabled, err := settingsService.ResolveBool(ctx, configModel.KeyEnrollmentLegalTermsEnabled)
	if err != nil {
		return err
	}
	displayMode, err := settingsService.ResolveString(ctx, configModel.KeyEnrollmentLegalAGBDisplayMode)
	if err != nil {
		return err
	}
	if termsEnabled && displayMode == configModel.EnrollmentLegalAGBDisplayModePDF {
		return errCannotDeleteActiveLegalAGBPDF
	}
	return nil
}

func (rs *SettingsResource) scheduleEnrollmentLegalAGBDocumentCleanup(ctx context.Context, oldURL, newURL string) error {
	cleanup, err := prepareEnrollmentLegalAGBDocumentCleanup(
		ctx,
		tenant.FromContext(ctx),
		oldURL,
		newURL,
		rs.enrollmentLegalAGBDocumentReferenced,
		common.ResolveStoredPath,
		common.RemoveImage,
	)
	if err != nil {
		return err
	}
	tenant.RegisterAfterCommit(ctx, cleanup)
	return nil
}

func prepareEnrollmentLegalAGBDocumentCleanup(
	ctx context.Context,
	tenantID int64,
	oldURL string,
	newURL string,
	isReferenced legalAGBDocumentReferenceFunc,
	resolvePath legalAGBDocumentPathResolver,
	removeFile legalAGBDocumentRemover,
) (func(), error) {
	if oldURL == "" || oldURL == newURL {
		return nil, nil
	}
	if !legalAGBDocumentBelongsToTenant(oldURL, tenantID) {
		return nil, nil
	}
	oldPath, err := resolvePath("public", oldURL, legalAGBDocumentPrefix)
	if err != nil {
		return nil, nil
	}
	referenced, err := isReferenced(ctx, oldURL)
	if err != nil {
		return nil, err
	}
	if referenced {
		return nil, nil
	}
	return func() {
		removeFile(oldPath)
	}, nil
}

func legalAGBDocumentBelongsToTenant(storedURL string, tenantID int64) bool {
	if tenantID <= 0 || !strings.HasPrefix(storedURL, legalAGBDocumentPrefix) {
		return false
	}
	filename := strings.TrimPrefix(storedURL, legalAGBDocumentPrefix)
	return strings.HasPrefix(filename, fmt.Sprintf("%d_", tenantID))
}

func (rs *SettingsResource) enrollmentLegalAGBDocumentReferenced(ctx context.Context, storedURL string) (bool, error) {
	if rs.legalDocumentRefs == nil {
		return false, errors.New("legal document reference repository is not configured")
	}
	publicURL := enrollmentSvc.PublicEnrollmentLegalDocumentURL(storedURL)

	referenced, err := rs.legalDocumentRefs.HasLegalDocumentReference(ctx, storedURL, publicURL)
	if err != nil {
		return false, fmt.Errorf("check AGB document references: %w", err)
	}
	return referenced, nil
}

// --- Error rendering ---

func renderSettingsError(w http.ResponseWriter, r *http.Request, err error) {
	var settingsErr *configSvc.SettingsError
	if !errors.As(err, &settingsErr) {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	inner := settingsErr.Unwrap()

	var defNotFound *configSvc.DefinitionNotFoundError
	var invalidValue *configSvc.InvalidValueError
	var permDenied *configSvc.PermissionDeniedError

	switch {
	case errors.As(inner, &defNotFound):
		common.RenderError(w, r, common.ErrorNotFound(err))
	case errors.As(inner, &invalidValue):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	case errors.As(inner, &permDenied):
		common.RenderError(w, r, common.ErrorForbidden(err))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}
