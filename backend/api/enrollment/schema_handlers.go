package enrollment

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// FormSchemaResponse is the wire shape returned to admin UIs. The id is
// stringified so the frontend can keep its int64-as-string convention.
type FormSchemaResponse struct {
	ID               string                            `json:"id"`
	Name             string                            `json:"name"`
	Version          int                               `json:"version"`
	IsActive         bool                              `json:"is_active"`
	Fields           []enrollmentModels.FormField      `json:"fields"`
	CoreRequirements enrollmentModels.CoreRequirements `json:"core_requirements"`
	LegalBlocks      []enrollmentModels.FormLegalBlock `json:"legal_blocks"`
	CreatedBy        string                            `json:"created_by"`
	CreatedAt        time.Time                         `json:"created_at"`
}

func toFormSchemaResponse(s *enrollmentModels.FormSchema) FormSchemaResponse {
	return FormSchemaResponse{
		ID:               strconv.FormatInt(s.ID, 10),
		Name:             s.Name,
		Version:          s.Version,
		IsActive:         s.IsActive,
		Fields:           s.Fields,
		CoreRequirements: coreRequirementsValue(s.CoreRequirements),
		LegalBlocks:      legalBlocksValue(s.LegalBlocks),
		CreatedBy:        strconv.FormatInt(s.CreatedBy, 10),
		CreatedAt:        s.CreatedAt,
	}
}

func coreRequirementsValue(value enrollmentModels.CoreRequirements) enrollmentModels.CoreRequirements {
	if value == nil {
		return enrollmentModels.CoreRequirements{}
	}
	return value
}

func coreRequirementsOrEmpty(value *enrollmentModels.CoreRequirements) enrollmentModels.CoreRequirements {
	if value == nil {
		return enrollmentModels.CoreRequirements{}
	}
	return *value
}

func legalBlocksValue(value []enrollmentModels.FormLegalBlock) []enrollmentModels.FormLegalBlock {
	if value == nil {
		return []enrollmentModels.FormLegalBlock{}
	}
	return value
}

// PublishSchemaRequest is the wire shape POST /schema accepts. When
// Name is set, the handler routes to CreateSchema (new named schema);
// when empty it falls back to PublishVersion (legacy single-schema
// flow that writes the row under the default name).
type PublishSchemaRequest struct {
	Name             string                             `json:"name"`
	Fields           []enrollmentModels.FormField       `json:"fields"`
	CoreRequirements *enrollmentModels.CoreRequirements `json:"core_requirements,omitempty"`
	LegalBlocks      *[]enrollmentModels.FormLegalBlock `json:"legal_blocks,omitempty"`
}

// Bind satisfies render.Binder. Field-level validation runs in the
// service so we don't duplicate it here.
func (req *PublishSchemaRequest) Bind(_ *http.Request) error {
	if req.Fields == nil {
		req.Fields = []enrollmentModels.FormField{}
	}
	return nil
}

// UpdateSchemaRequest is the wire shape PUT /schema/{id} accepts.
// Only the fields are mutable — name is inherited from the source
// schema so all versions of a logical schema share the same name.
type UpdateSchemaRequest struct {
	Fields           []enrollmentModels.FormField       `json:"fields"`
	CoreRequirements *enrollmentModels.CoreRequirements `json:"core_requirements,omitempty"`
	LegalBlocks      *[]enrollmentModels.FormLegalBlock `json:"legal_blocks,omitempty"`
}

func (req *UpdateSchemaRequest) Bind(_ *http.Request) error {
	if req.Fields == nil {
		req.Fields = []enrollmentModels.FormField{}
	}
	return nil
}

// PublicFormSchemaResponse is the slim, parent-safe shape returned by
// the public schema endpoint. We deliberately drop CreatedBy and
// internal version metadata that parents don't need to render the
// form. The fields array carries everything the dynamic renderer
// consumes (label, type, options, validation).
type PublicFormSchemaResponse struct {
	ID               string                            `json:"id"`
	Version          int                               `json:"version"`
	Fields           []enrollmentModels.FormField      `json:"fields"`
	CoreRequirements enrollmentModels.CoreRequirements `json:"core_requirements"`
	LegalBlocks      []enrollmentModels.FormLegalBlock `json:"legal_blocks"`
}

type SchemaPreviewBootstrapResponse struct {
	Schema                   *FormSchemaResponse `json:"schema"`
	AssignedPhaseCount       int                 `json:"assigned_phase_count"`
	ActiveAssignedPhaseCount int                 `json:"active_assigned_phase_count"`
}

func (rs *Resource) getSchemaPreviewBootstrap(w http.ResponseWriter, r *http.Request) {
	if rs.FormSchemaService == nil || rs.PhaseService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("schema preview endpoint not configured")))
		return
	}

	isBasePreview := r.URL.Query().Get("base") == "1"
	schemaIDParam := r.URL.Query().Get("schemaId")
	if !isBasePreview && schemaIDParam == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("schemaId is required")))
		return
	}

	var (
		schema *enrollmentModels.FormSchema
		phases []*enrollmentModels.Phase
	)
	err := rs.runInTenantTx(r, func(ctx context.Context) error {
		if !isBasePreview {
			schemaID, parseErr := strconv.ParseInt(schemaIDParam, 10, 64)
			if parseErr != nil || schemaID <= 0 {
				return errors.New("schemaId must be a positive integer")
			}
			loaded, loadErr := rs.FormSchemaService.GetByID(ctx, schemaID)
			if loadErr != nil {
				return loadErr
			}
			schema = loaded
		}
		list, listErr := rs.PhaseService.List(ctx)
		if listErr != nil {
			return listErr
		}
		phases = list
		return nil
	})
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	var schemaOut *FormSchemaResponse
	if schema != nil {
		out := toFormSchemaResponse(schema)
		schemaOut = &out
	}
	assigned, active := countAssignedPreviewPhases(phases, schema)
	common.Respond(w, r, http.StatusOK, SchemaPreviewBootstrapResponse{
		Schema:                   schemaOut,
		AssignedPhaseCount:       assigned,
		ActiveAssignedPhaseCount: active,
	}, "Schema preview bootstrap retrieved")
}

func countAssignedPreviewPhases(phases []*enrollmentModels.Phase, schema *enrollmentModels.FormSchema) (int, int) {
	assigned := 0
	active := 0
	for _, phase := range phases {
		if phase == nil {
			continue
		}
		matches := phase.FormSchemaID == nil
		if schema != nil {
			matches = phase.FormSchemaID != nil && *phase.FormSchemaID == schema.ID
		}
		if !matches {
			continue
		}
		assigned++
		if phase.IsActive {
			active++
		}
	}
	return assigned, active
}

// listPublicActiveSchema returns the form schema for a (tenant, phase)
// pair. The phase's pinned form_schema_id wins; if the phase has none,
// we fall back to the tenant's currently-active schema; if neither
// exists, 404 → form falls back to core fields only.
func (rs *Resource) listPublicActiveSchema(w http.ResponseWriter, r *http.Request) {
	if rs.FormSchemaService == nil || rs.PhaseRepo == nil || rs.SchoolRepo == nil || rs.db == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("public schema endpoint not wired")))
		return
	}

	slug := chi.URLParam(r, "tenantSlug")
	if slug == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("tenant slug is required")))
		return
	}
	phaseID, err := strconv.ParseInt(chi.URLParam(r, "phaseId"), 10, 64)
	if err != nil || phaseID <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("phaseId is required")))
		return
	}

	var schema *enrollmentModels.FormSchema
	schoolID, resolveErr := rs.resolvePublicTenantID(r.Context(), slug)
	if resolveErr == nil {
		resolveErr = tenant.WithTenantTx(r.Context(), rs.db, schoolID, func(txCtx context.Context, _ bun.Tx) error {
			if rs.RequestService != nil && !rs.RequestService.IsEnrollmentEnabled(txCtx) {
				return enrollmentService.ErrEnrollmentDisabled
			}
			phase, phaseErr := rs.PhaseRepo.FindByID(txCtx, phaseID)
			if phaseErr != nil {
				return errors.New("phase not found")
			}
			if phase.FormSchemaID == nil {
				// Basis phase — admin explicitly chose "nur die
				// Standardfelder". Return ErrNoActiveSchema so the
				// frontend renders core fields only; do NOT fall back
				// to whatever the tenant's currently-active schema is,
				// otherwise creating a custom form leaks its fields
				// into every Basis phase.
				return enrollmentService.ErrNoActiveSchema
			}
			s, innerErr := rs.FormSchemaService.GetByID(txCtx, *phase.FormSchemaID)
			if innerErr != nil {
				return innerErr
			}
			schema = s
			return nil
		})
	}
	if resolveErr != nil {
		renderPublicEnrollmentError(w, r, resolveErr)
		return
	}

	out := PublicFormSchemaResponse{
		ID:               strconv.FormatInt(schema.ID, 10),
		Version:          schema.Version,
		Fields:           schema.Fields,
		CoreRequirements: coreRequirementsValue(schema.CoreRequirements),
		LegalBlocks:      legalBlocksValue(schema.LegalBlocks),
	}
	common.Respond(w, r, http.StatusOK, out, "Public active form schema retrieved")
}

// PublicCaptchaConfigResponse carries the tenant-scoped Cloudflare
// Turnstile site key + enabled flag so the public form can render
// the widget. Returned by GET /enrollment/captcha-config/{tenantSlug}.
// SiteKey is intentionally surfaced — Turnstile site keys are public
// by design (they appear in the rendered widget markup).
type PublicCaptchaConfigResponse struct {
	Enabled bool   `json:"enabled"`
	SiteKey string `json:"site_key"`
}

// publicCaptchaConfig serves the captcha config for the parent form.
// Slug-gated, no JWT.
func (rs *Resource) publicCaptchaConfig(w http.ResponseWriter, r *http.Request) {
	if rs.CaptchaService == nil || rs.SchoolRepo == nil || rs.db == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("captcha config endpoint not wired")))
		return
	}

	slug := chi.URLParam(r, "tenantSlug")
	if slug == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("tenant slug is required")))
		return
	}

	out := PublicCaptchaConfigResponse{}
	schoolID, resolveErr := rs.resolvePublicTenantID(r.Context(), slug)
	if resolveErr == nil {
		resolveErr = tenant.WithTenantTx(r.Context(), rs.db, schoolID, func(txCtx context.Context, _ bun.Tx) error {
			out.Enabled = rs.CaptchaService.IsEnabled(txCtx)
			out.SiteKey = rs.CaptchaService.SiteKey(txCtx)
			return nil
		})
	}
	if resolveErr != nil {
		renderPublicEnrollmentError(w, r, resolveErr)
		return
	}

	common.Respond(w, r, http.StatusOK, out, "Captcha config retrieved")
}

// getActiveSchema returns the currently-active schema or 404 if none
// has been published yet for this tenant.
func (rs *Resource) getActiveSchema(w http.ResponseWriter, r *http.Request) {
	if rs.FormSchemaService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("form schema service not configured")))
		return
	}

	var schema *enrollmentModels.FormSchema
	err := rs.runInTenantTx(r, func(ctx context.Context) error {
		s, innerErr := rs.FormSchemaService.GetActive(ctx)
		schema = s
		return innerErr
	})
	if err != nil {
		if errors.Is(err, enrollmentService.ErrNoActiveSchema) {
			common.RenderError(w, r, common.ErrorNotFound(err))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, toFormSchemaResponse(schema), "Active form schema retrieved")
}

// listSchemaVersions returns every schema version for the tenant in
// context, newest-first.
func (rs *Resource) listSchemaVersions(w http.ResponseWriter, r *http.Request) {
	if rs.FormSchemaService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("form schema service not configured")))
		return
	}

	var schemas []*enrollmentModels.FormSchema
	err := rs.runInTenantTx(r, func(ctx context.Context) error {
		s, innerErr := rs.FormSchemaService.ListVersions(ctx)
		schemas = s
		return innerErr
	})
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	out := make([]FormSchemaResponse, 0, len(schemas))
	for _, s := range schemas {
		out = append(out, toFormSchemaResponse(s))
	}
	common.Respond(w, r, http.StatusOK, out, "Form schema versions retrieved")
}

// getSchemaByID surfaces a specific version (used to render an
// already-submitted request against its pinned schema).
func (rs *Resource) getSchemaByID(w http.ResponseWriter, r *http.Request) {
	if rs.FormSchemaService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("form schema service not configured")))
		return
	}

	idParam := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil || id <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid schema id")))
		return
	}

	var schema *enrollmentModels.FormSchema
	txErr := rs.runInTenantTx(r, func(ctx context.Context) error {
		s, innerErr := rs.FormSchemaService.GetByID(ctx, id)
		schema = s
		return innerErr
	})
	if txErr != nil {
		common.RenderError(w, r, common.ErrorNotFound(txErr))
		return
	}

	common.Respond(w, r, http.StatusOK, toFormSchemaResponse(schema), "Form schema retrieved")
}

// publishSchema creates a schema. When the request body carries a
// name, this routes to CreateSchema (new named schema, version 1).
// When the name is omitted, it falls back to PublishVersion for
// backward compatibility with the original single-schema flow.
func (rs *Resource) publishSchema(w http.ResponseWriter, r *http.Request) {
	if rs.FormSchemaService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("form schema service not configured")))
		return
	}

	req := &PublishSchemaRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID <= 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(errors.New("missing actor")))
		return
	}

	var schema *enrollmentModels.FormSchema
	err := rs.runInTenantTx(r, func(ctx context.Context) error {
		updateExisting := func(id int64) (*enrollmentModels.FormSchema, error) {
			if req.LegalBlocks != nil {
				return rs.FormSchemaService.UpdateSchemaWithLegal(ctx, id, req.Fields, int64(claims.ID), req.CoreRequirements, req.LegalBlocks)
			}
			if req.CoreRequirements == nil {
				return rs.FormSchemaService.UpdateSchema(ctx, id, req.Fields, int64(claims.ID))
			}
			return rs.FormSchemaService.UpdateSchema(ctx, id, req.Fields, int64(claims.ID), *req.CoreRequirements)
		}
		if req.Name != "" {
			if req.LegalBlocks != nil {
				s, innerErr := rs.FormSchemaService.CreateSchemaWithLegal(ctx, req.Name, req.Fields, int64(claims.ID), coreRequirementsOrEmpty(req.CoreRequirements), *req.LegalBlocks)
				schema = s
				return innerErr
			}
			s, innerErr := rs.FormSchemaService.CreateSchema(ctx, req.Name, req.Fields, int64(claims.ID), coreRequirementsOrEmpty(req.CoreRequirements))
			schema = s
			return innerErr
		}
		active, innerErr := rs.FormSchemaService.GetActive(ctx)
		if errors.Is(innerErr, enrollmentService.ErrNoActiveSchema) {
			versions, listErr := rs.FormSchemaService.ListVersions(ctx)
			if listErr != nil {
				return listErr
			}
			for _, version := range versions {
				if version.Name == "Standardformular" {
					s, updateErr := updateExisting(version.ID)
					schema = s
					return updateErr
				}
			}
			if req.LegalBlocks != nil {
				s, createErr := rs.FormSchemaService.CreateSchemaWithLegal(ctx, "Standardformular", req.Fields, int64(claims.ID), coreRequirementsOrEmpty(req.CoreRequirements), *req.LegalBlocks)
				schema = s
				return createErr
			}
			s, createErr := rs.FormSchemaService.CreateSchema(ctx, "Standardformular", req.Fields, int64(claims.ID), coreRequirementsOrEmpty(req.CoreRequirements))
			schema = s
			return createErr
		}
		if innerErr != nil {
			return innerErr
		}
		s, innerErr := updateExisting(active.ID)
		schema = s
		return innerErr
	})
	if err != nil {
		// Validation errors come back as plain errors; surface as 400.
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	slog.Default().Info("form schema published",
		slog.Int64("schema_id", schema.ID),
		slog.String("name", schema.Name),
		slog.Int("version", schema.Version),
		slog.Int64("actor_account_id", int64(claims.ID)))

	common.Respond(w, r, http.StatusCreated, toFormSchemaResponse(schema), "Form schema published")
}

// updateSchema publishes a new version of an existing named schema.
// PUT /schema/{id} — id refers to any prior version of the schema;
// the new row inherits its name and is written with version+1.
func (rs *Resource) updateSchema(w http.ResponseWriter, r *http.Request) {
	if rs.FormSchemaService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("form schema service not configured")))
		return
	}

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid schema id")))
		return
	}

	req := &UpdateSchemaRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID <= 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(errors.New("missing actor")))
		return
	}

	var schema *enrollmentModels.FormSchema
	txErr := rs.runInTenantTx(r, func(ctx context.Context) error {
		var (
			s        *enrollmentModels.FormSchema
			innerErr error
		)
		if req.LegalBlocks != nil {
			s, innerErr = rs.FormSchemaService.UpdateSchemaWithLegal(ctx, id, req.Fields, int64(claims.ID), req.CoreRequirements, req.LegalBlocks)
		} else if req.CoreRequirements == nil {
			s, innerErr = rs.FormSchemaService.UpdateSchema(ctx, id, req.Fields, int64(claims.ID))
		} else {
			s, innerErr = rs.FormSchemaService.UpdateSchema(ctx, id, req.Fields, int64(claims.ID), *req.CoreRequirements)
		}
		schema = s
		return innerErr
	})
	if txErr != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(txErr))
		return
	}

	slog.Default().Info("form schema version added",
		slog.Int64("schema_id", schema.ID),
		slog.String("name", schema.Name),
		slog.Int("version", schema.Version),
		slog.Int64("actor_account_id", int64(claims.ID)))

	common.Respond(w, r, http.StatusCreated, toFormSchemaResponse(schema), "Form schema version added")
}

func (rs *Resource) deleteSchema(w http.ResponseWriter, r *http.Request) {
	if rs.FormSchemaService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("form schema service not configured")))
		return
	}

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid schema id")))
		return
	}

	err = rs.runInTenantTx(r, func(ctx context.Context) error {
		return rs.FormSchemaService.DeleteSchema(ctx, id)
	})
	if err != nil {
		switch {
		case errors.Is(err, enrollmentService.ErrFormSchemaHasPhases):
			common.RenderError(w, r, common.ErrorConflictWithCode(err, ErrCodeSchemaHasPhases))
			return
		case errors.Is(err, enrollmentService.ErrFormSchemaHasRequests):
			common.RenderError(w, r, common.ErrorConflictWithCode(err, ErrCodeSchemaHasRequests))
			return
		case errors.Is(err, enrollmentService.ErrFormSchemaNotFound):
			common.RenderError(w, r, common.ErrorNotFound(err))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.RespondNoContent(w, r)
}

const (
	ErrCodeSchemaHasPhases   = "enrollment.schema_has_phases"
	ErrCodeSchemaHasRequests = "enrollment.schema_has_requests"
)

// runInTenantTx wraps the request's tenant context in a tenant
// transaction so the service's repo calls hit the right RLS scope.
// Tests use a nil DB and skip the transaction wrap.
func (rs *Resource) runInTenantTx(r *http.Request, fn func(ctx context.Context) error) error {
	ctx := r.Context()
	if rs.db == nil {
		return fn(ctx)
	}
	tenantID := tenant.FromContext(ctx)
	return tenant.WithTenantTx(ctx, rs.db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		return fn(txCtx)
	})
}
