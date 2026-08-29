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
	"github.com/moto-nrw/project-phoenix/settings"
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

// SettingsResource defines the settings API resource.
type SettingsResource struct {
	operations *settings.Operations
	runtime    Runtime
}

func NewSettingsResource(operations *settings.Operations, runtime Runtime) *SettingsResource {
	return &SettingsResource{operations: operations, runtime: runtime}
}

func (rs *SettingsResource) OnValueSet(hook settings.ValueSetHook) {
	if rs.operations != nil {
		rs.operations.SetValueSetHook(hook)
	}
}

// SettingsRouter returns a configured router for settings endpoints.
func (rs *SettingsResource) SettingsRouter() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	if rs.runtime == nil {
		return r
	}

	rs.runtime.ProtectedTenantGroup(r, func(r chi.Router, withTx Middleware) {
		settingsWrite := rs.runtime.Require(AccessWrite)

		r.With(rs.runtime.Require(AccessRead), withTx).Get("/schema", rs.getSchema)
		// Payroll configuration status (#1417): config:manage only — the
		// same tier that writes the payroll settings; carries a per-tenant
		// staff-without-Personalnummer count (count only, no names).
		r.With(rs.runtime.Require(AccessManage), withTx).Get("/payroll-status", rs.getPayrollStatus)

		r.With(settingsWrite, withTx).Get("/values/{key}/reveal", rs.revealValue)
		r.With(settingsWrite, withTx).Put("/values/{key}", rs.setValue)
		r.With(settingsWrite, withTx).Delete("/values/{key}", rs.resetValue)

		// Login image — reads use withTx (tenant role), writes use WithAdminTx internally
		// because platform.schools requires the phoenix_admin role for UPDATE.
		// withTx is intentionally omitted on POST/DELETE to avoid conflicting role contexts.
		// GET uses settingsWrite (not ConfigRead) so write-capable roles can also read the
		// login-image metadata — the frontend depends on this GET to enable upload/delete controls.
		settingsReadOrWrite := rs.runtime.Require(AccessReadOrWrite)
		r.With(settingsReadOrWrite, withTx).Get("/login-image", rs.getLoginImage)
		r.With(settingsWrite, rs.runtime.TenantOperation()).Post("/login-image", rs.uploadLoginImage)
		r.With(settingsWrite, rs.runtime.TenantOperation()).Delete("/login-image", rs.deleteLoginImage)

		// AGB document writes manage a file-system side effect. Like login-image
		// writes, they open their own tenant tx so file cleanup only runs after
		// the DB write has committed.
		r.With(settingsWrite, rs.runtime.TenantOperation()).Post("/enrollment/legal-agb-document", rs.uploadEnrollmentLegalAGBDocument)
		r.With(settingsWrite, rs.runtime.TenantOperation()).Delete("/enrollment/legal-agb-document", rs.deleteEnrollmentLegalAGBDocument)
	})

	return r
}

// --- Request types ---

type setValueRequest struct {
	Value any `json:"value"`
}

// --- Handlers ---

func (rs *SettingsResource) getSchema(w http.ResponseWriter, r *http.Request) {
	actor := rs.runtime.Actor(r.Context())
	schema, err := rs.operations.Schema(r.Context(), actor.Permissions)
	if err != nil {
		rs.runtime.RenderError(w, r, http.StatusInternalServerError, err)
		return
	}

	rs.runtime.Respond(w, r, http.StatusOK, schema, "Schema retrieved successfully")
}

func (rs *SettingsResource) revealValue(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	actor := rs.runtime.Actor(r.Context())
	value, err := rs.operations.Reveal(r.Context(), key, actor.Permissions)
	if err != nil {
		rs.renderSettingsError(w, r, err)
		return
	}

	rs.runtime.Respond(w, r, http.StatusOK, map[string]any{"value": value}, "")
}

func (rs *SettingsResource) setValue(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")

	var req setValueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		rs.runtime.RenderError(w, r, http.StatusBadRequest, err)
		return
	}

	err := rs.operations.SetValue(r.Context(), rs.runtime.Actor(r.Context()), key, req.Value)
	if err != nil {
		rs.renderSettingsError(w, r, err)
		return
	}

	rs.runtime.Respond(w, r, http.StatusOK, nil, "Value updated successfully")
}

func (rs *SettingsResource) resetValue(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	err := rs.operations.ResetValue(r.Context(), rs.runtime.Actor(r.Context()), key)
	if err != nil {
		rs.renderSettingsError(w, r, err)
		return
	}

	rs.runtime.RespondNoContent(w, r)
}

// --- Login image handlers ---

type loginImageResponse struct {
	LoginImageURL *string `json:"login_image_url"`
	CanEdit       bool    `json:"can_edit"`
}

// getLoginImage returns the current login image URL and edit permission for the tenant.
func (rs *SettingsResource) getLoginImage(w http.ResponseWriter, r *http.Request) {
	tenantID := rs.runtime.Actor(r.Context()).TenantID
	if tenantID <= 0 {
		rs.runtime.RenderError(w, r, http.StatusBadRequest, errors.New("no tenant context"))
		return
	}

	url, err := rs.operations.LoginImageURL(r.Context(), tenantID)
	if err != nil {
		rs.runtime.RenderError(w, r, http.StatusInternalServerError, err)
		return
	}

	resp := loginImageResponse{CanEdit: rs.runtime.CanEdit(r.Context())}
	if url != "" {
		resp.LoginImageURL = &url
	}

	rs.runtime.Respond(w, r, http.StatusOK, resp, "")
}

// uploadLoginImage handles uploading a custom login page image for the tenant.
func (rs *SettingsResource) uploadLoginImage(w http.ResponseWriter, r *http.Request) {
	uploaded, err := rs.runtime.ParseImage(w, r, "login_image", maxLoginImageBody)
	if err != nil {
		rs.runtime.RenderError(w, r, http.StatusBadRequest, err)
		return
	}
	defer func() { _ = uploaded.File.Close() }()

	tenantID := rs.runtime.Actor(r.Context()).TenantID
	if tenantID <= 0 {
		rs.runtime.RenderError(w, r, http.StatusBadRequest, errors.New("no tenant context"))
		return
	}

	prefix := fmt.Sprintf("%d", tenantID)
	filePath, err := rs.runtime.SaveImage(uploaded.File, loginImageDir, prefix, uploaded.ContentType)
	if err != nil {
		rs.runtime.RenderError(w, r, http.StatusInternalServerError, err)
		return
	}

	imageURL := "/uploads/login-images/" + filepath.Base(filePath)
	oldURL, err := rs.operations.SetLoginImageURL(r.Context(), tenantID, imageURL)
	if err != nil {
		rs.runtime.RemoveFile(filePath)
		rs.runtime.RenderError(w, r, http.StatusInternalServerError, err)
		return
	}

	// Clean up old file only after DB commit succeeded
	if oldURL != "" {
		if oldPath, resolveErr := rs.runtime.ResolveStoredPath("public", oldURL, "/uploads/login-images/"); resolveErr == nil {
			rs.runtime.RemoveFile(oldPath)
		}
	}

	rs.runtime.Respond(w, r, http.StatusOK, map[string]string{"login_image_url": imageURL}, "Login image uploaded successfully")
}

// deleteLoginImage removes the custom login page image for the tenant.
func (rs *SettingsResource) deleteLoginImage(w http.ResponseWriter, r *http.Request) {
	tenantID := rs.runtime.Actor(r.Context()).TenantID
	if tenantID <= 0 {
		rs.runtime.RenderError(w, r, http.StatusBadRequest, errors.New("no tenant context"))
		return
	}

	oldURL, err := rs.operations.ClearLoginImageURL(r.Context(), tenantID)
	if err != nil {
		rs.runtime.RenderError(w, r, http.StatusInternalServerError, err)
		return
	}

	// Clean up old file only after DB commit succeeded
	if oldURL != "" {
		if oldPath, resolveErr := rs.runtime.ResolveStoredPath("public", oldURL, "/uploads/login-images/"); resolveErr == nil {
			rs.runtime.RemoveFile(oldPath)
		}
	}

	rs.runtime.RespondNoContent(w, r)
}

// --- Enrollment legal document handlers ---

type legalAGBDocumentResponse struct {
	DocumentURL string `json:"document_url"`
}

type legalAGBDocumentReferenceFunc func(context.Context, string) (bool, error)
type legalAGBDocumentPathResolver func(string, string, string) (string, error)
type legalAGBDocumentRemover func(string)

func (rs *SettingsResource) uploadEnrollmentLegalAGBDocument(w http.ResponseWriter, r *http.Request) {
	uploaded, err := rs.runtime.ParsePDF(w, r, "document", maxLegalAGBDocumentSize, maxLegalAGBDocumentBody)
	if err != nil {
		rs.runtime.RenderError(w, r, http.StatusBadRequest, err)
		return
	}
	defer func() { _ = uploaded.File.Close() }()

	actor := rs.runtime.Actor(r.Context())
	tenantID := actor.TenantID
	if tenantID <= 0 {
		rs.runtime.RenderError(w, r, http.StatusBadRequest, errors.New("no tenant context"))
		return
	}

	filePath, err := rs.runtime.SavePDF(uploaded.File, legalAGBDocumentDir, fmt.Sprintf("%d", tenantID))
	if err != nil {
		rs.runtime.RenderError(w, r, http.StatusInternalServerError, err)
		return
	}

	documentURL := legalAGBDocumentPrefix + filepath.Base(filePath)
	err = rs.operations.SetLegalDocument(r.Context(), actor, documentURL, rs.prepareEnrollmentLegalAGBDocumentCleanup)
	if err != nil {
		rs.runtime.RemoveFile(filePath)
		rs.renderSettingsError(w, r, err)
		return
	}

	rs.runtime.Respond(w, r, http.StatusOK, legalAGBDocumentResponse{DocumentURL: documentURL}, "AGB document uploaded successfully")
}

func (rs *SettingsResource) deleteEnrollmentLegalAGBDocument(w http.ResponseWriter, r *http.Request) {
	actor := rs.runtime.Actor(r.Context())
	tenantID := actor.TenantID
	if tenantID <= 0 {
		rs.runtime.RenderError(w, r, http.StatusBadRequest, errors.New("no tenant context"))
		return
	}

	err := rs.operations.DeleteLegalDocument(r.Context(), actor, rs.prepareEnrollmentLegalAGBDocumentCleanup)
	if err != nil {
		rs.renderSettingsError(w, r, err)
		return
	}

	rs.runtime.RespondNoContent(w, r)
}

func (rs *SettingsResource) prepareEnrollmentLegalAGBDocumentCleanup(ctx context.Context, tenantID int64, oldURL, newURL string) (func(), error) {
	return prepareEnrollmentLegalAGBDocumentCleanup(
		ctx,
		tenantID,
		oldURL,
		newURL,
		rs.runtime.LegalDocumentReferenced,
		rs.runtime.ResolveStoredPath,
		rs.runtime.RemoveFile,
	)
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

// --- Error rendering ---

func (rs *SettingsResource) renderSettingsError(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusInternalServerError
	switch settings.ClassifyError(err) {
	case settings.ErrorNotFound:
		status = http.StatusNotFound
	case settings.ErrorInvalid:
		status = http.StatusBadRequest
	case settings.ErrorForbidden:
		status = http.StatusForbidden
	}
	// All resources are constructed with a runtime; nil is accepted only by the
	// constructor smoke test, which never invokes a handler.
	rs.runtime.RenderError(w, r, status, err)
}
