package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
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

// ValueSetCallback runs in the same tenant transaction as the setting
// write. Returning an error aborts the request and rolls back the update.
//
// The optional postCommit closure runs ONLY if the transaction commits
// successfully. Use it for non-transactional side effects that must not
// happen if the DB write rolls back (file-system unlinks, external API
// calls, anything we can't undo on rollback). Return nil when no post-
// commit work is needed.
//
// Why split the phases: a callback that unlinks files inside the tx will
// still leave files deleted if the surrounding tx fails to commit (lock
// timeout, pool drop, etc.) — the DB rolls back to the pre-purge state
// while the bytes on disk are gone forever. Splitting the phases lets the
// in-tx work collect work items into the post-commit closure and only run
// the destructive part when the DB write actually persisted.
type ValueSetCallback func(ctx context.Context, tenantID int64, key string, value any) (postCommit func(), err error)

// SettingsResource defines the settings API resource.
type SettingsResource struct {
	settingsService configSvc.SettingsService
	db              *bun.DB
	onValueSet      ValueSetCallback
}

// OnValueSet registers a callback that runs after a setting value change is
// validated and persisted. See ValueSetCallback for the in-tx vs post-commit
// contract.
//
// The hook always fires on PUT (`SetValue`). DELETE (`ResetValue`) replays
// the hook only for keys that need reset-specific side effects. Right now
// that is limited to operations.student_photos_enabled so unrelated
// settings keep their pre-feature reset behavior.
func (rs *SettingsResource) OnValueSet(fn ValueSetCallback) {
	rs.onValueSet = fn
}

// NewSettingsResource creates a new settings resource.
func NewSettingsResource(svc configSvc.SettingsService, db *bun.DB) *SettingsResource {
	return &SettingsResource{
		settingsService: svc,
		db:              db,
	}
}

// SettingsRouter returns a configured router for settings endpoints.
func (rs *SettingsResource) SettingsRouter() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	tokenAuth := jwt.MustNewTokenAuth()

	r.Group(func(r chi.Router) {
		r.Use(tokenAuth.Verifier())
		r.Use(jwt.Authenticator)
		r.Use(jwt.TenantMiddleware)
		withTx := tenant.TenantTxMiddleware(rs.db)

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
	})

	return r
}

// --- Request types ---

type setValueRequest struct {
	Value any `json:"value"`
}

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
			// Schedule the side-effect closure (e.g. student-photo purge,
			// cache invalidation) to run after the OUTERMOST tenant tx
			// commits — not after this WithTenantTx returns. SettingsRouter
			// wraps these handlers in TenantTxMiddleware, so the
			// inner WithTenantTx merely reuses the middleware's already-open
			// transaction; the real COMMIT happens once the handler returns.
			// Running destructive cleanup before that point would unlink
			// files / invalidate caches even if the outer commit later fails.
			tenant.RegisterAfterCommit(ctx, cb)
		}
		return nil
	})
	if err != nil {
		// Tx rolled back — the after-commit hook is dropped automatically
		// by RegisterAfterCommit: it only runs when the outermost commit
		// succeeds.
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

	claims := jwt.ClaimsFromCtx(r.Context())
	changedBy := int64(claims.ID)
	tenantID := tenant.FromContext(r.Context())

	err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		if err := rs.settingsService.ResetValue(ctx, key, &changedBy, claims.Permissions); err != nil {
			return err
		}
		// Fire the same value-changed hook PUT runs through, with the
		// registry default as the post-reset effective value. Otherwise a
		// "Zurücksetzen" click on student_photos_enabled would clear the
		// override but leave already-stored photos on disk.
		if rs.onValueSet != nil && shouldReplayResetHook(key) {
			def := configModel.GetDefinition(key)
			if def != nil {
				cb, err := rs.onValueSet(ctx, tenantID, key, def.Default)
				if err != nil {
					return err
				}
				// Defer the side-effect closure to after the OUTERMOST
				// commit (TenantTxMiddleware), same reasoning as setValue.
				tenant.RegisterAfterCommit(ctx, cb)
			}
		}
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

// --- Handler accessors for testing ---

// GetSchema returns the getSchema handler for external test access.
func (rs *SettingsResource) GetSchema() http.HandlerFunc { return rs.getSchema }

// RevealValue returns the revealValue handler for external test access.
func (rs *SettingsResource) RevealValue() http.HandlerFunc { return rs.revealValue }

// SetValue returns the setValue handler for external test access.
func (rs *SettingsResource) SetValue() http.HandlerFunc { return rs.setValue }

// ResetValue returns the resetValue handler for external test access.
func (rs *SettingsResource) ResetValue() http.HandlerFunc { return rs.resetValue }

// GetLoginImage returns the getLoginImage handler for external test access.
func (rs *SettingsResource) GetLoginImage() http.HandlerFunc { return rs.getLoginImage }

// UploadLoginImage returns the uploadLoginImage handler for external test access.
func (rs *SettingsResource) UploadLoginImage() http.HandlerFunc { return rs.uploadLoginImage }

// DeleteLoginImage returns the deleteLoginImage handler for external test access.
func (rs *SettingsResource) DeleteLoginImage() http.HandlerFunc { return rs.deleteLoginImage }

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
