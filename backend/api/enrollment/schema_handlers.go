package enrollment

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// FormSchemaResponse is the wire shape returned to admin UIs. The id is
// stringified so the frontend can keep its int64-as-string convention.
type FormSchemaResponse struct {
	ID               string                      `json:"id"`
	Name             string                      `json:"name"`
	Version          int                         `json:"version"`
	IsActive         bool                        `json:"is_active"`
	Fields           []capability.FormField      `json:"fields"`
	CoreRequirements capability.CoreRequirements `json:"core_requirements"`
	LegalBlocks      []capability.FormLegalBlock `json:"legal_blocks"`
	CreatedBy        string                      `json:"created_by"`
	CreatedAt        time.Time                   `json:"created_at"`
}

func toFormSchemaResponse(s *capability.FormSchema) FormSchemaResponse {
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

func coreRequirementsValue(value capability.CoreRequirements) capability.CoreRequirements {
	if value == nil {
		return capability.CoreRequirements{}
	}
	return value
}

func legalBlocksValue(value []capability.FormLegalBlock) []capability.FormLegalBlock {
	if value == nil {
		return []capability.FormLegalBlock{}
	}
	return value
}

// PublishSchemaRequest is the wire shape POST /schema accepts. When
// Name is set, the handler routes to CreateSchema (new named schema);
// when empty it falls back to updating/creating the default
// "Standardformular" schema (legacy single-schema flow).
type PublishSchemaRequest struct {
	Name             string                       `json:"name"`
	Fields           []capability.FormField       `json:"fields"`
	CoreRequirements *capability.CoreRequirements `json:"core_requirements,omitempty"`
	LegalBlocks      *[]capability.FormLegalBlock `json:"legal_blocks,omitempty"`
}

// Bind satisfies render.Binder. Field-level validation runs in the
// service so we don't duplicate it here.
func (req *PublishSchemaRequest) Bind(_ *http.Request) error {
	if req.Fields == nil {
		req.Fields = []capability.FormField{}
	}
	return nil
}

// UpdateSchemaRequest is the wire shape PUT /schema/{id} accepts.
// Name is optional: when present and different from the lineage's current
// name, the handler renames the whole lineage AND publishes the new version
// in one transaction (a combined "rename + edit" save), so a failed publish
// rolls the rename back. When Name is absent, only the fields change and the
// name is inherited from the source schema, keeping every version of a
// logical schema under one shared name.
type UpdateSchemaRequest struct {
	Name             *string                      `json:"name,omitempty"`
	Fields           []capability.FormField       `json:"fields"`
	CoreRequirements *capability.CoreRequirements `json:"core_requirements,omitempty"`
	LegalBlocks      *[]capability.FormLegalBlock `json:"legal_blocks,omitempty"`
}

func (req *UpdateSchemaRequest) Bind(_ *http.Request) error {
	if req.Fields == nil {
		req.Fields = []capability.FormField{}
	}
	return nil
}

// RenameSchemaRequest is the wire shape PATCH /schema/{id} accepts. It
// only renames the logical schema (all versions share one name) and
// never publishes a new version or touches the form fields.
type RenameSchemaRequest struct {
	Name string `json:"name"`
}

func (req *RenameSchemaRequest) Bind(_ *http.Request) error {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return errors.New("name is required")
	}
	return nil
}

// PublicFormSchemaResponse is the slim, parent-safe shape returned by
// the public schema endpoint. We deliberately drop CreatedBy and
// internal version metadata that parents don't need to render the
// form. The fields array carries everything the dynamic renderer
// consumes (label, type, options, validation).
type PublicFormSchemaResponse struct {
	ID               string                      `json:"id"`
	Version          int                         `json:"version"`
	Fields           []capability.FormField      `json:"fields"`
	CoreRequirements capability.CoreRequirements `json:"core_requirements"`
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

	schemaID, ok := parseSchemaPreviewSchemaID(w, r)
	if !ok {
		return
	}

	var (
		schema *capability.FormSchema
		phases []*capability.Phase
	)
	err := rs.runInTenantTx(r, func(ctx context.Context) error {
		if schemaID > 0 {
			loaded, loadErr := rs.FormSchemaService.GetByID(ctx, schemaID)
			if loadErr != nil {
				return loadErr
			}
			schema = loaded
		}
		list, listErr := rs.PhaseService.List(ctx)
		phases = list
		return listErr
	})
	if err != nil {
		// A missing schema id is the caller's problem (404); everything
		// else (phase listing, DB failures) is a genuine 500 — rendering
		// those as 400 would hide internal failures behind client blame.
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, enrollmentService.ErrFormSchemaNotFound) {
			common.RenderError(w, r, common.ErrorNotFound(err))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
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

// parseSchemaPreviewSchemaID reads the preview target from the query. A
// base preview (base=1) carries no schema and returns 0; otherwise a
// positive schemaId is required. On a bad request it renders a 400 and
// returns ok=false.
func parseSchemaPreviewSchemaID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	if r.URL.Query().Get("base") == "1" {
		return 0, true
	}
	schemaIDParam := r.URL.Query().Get("schemaId")
	if schemaIDParam == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("schemaId is required")))
		return 0, false
	}
	parsed, parseErr := strconv.ParseInt(schemaIDParam, 10, 64)
	if parseErr != nil || parsed <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("schemaId must be a positive integer")))
		return 0, false
	}
	return parsed, true
}

func countAssignedPreviewPhases(phases []*capability.Phase, schema *capability.FormSchema) (int, int) {
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
	if rs.RequestService == nil || rs.SchoolService == nil || rs.db == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("public schema endpoint not wired")))
		return
	}

	slug := chi.URLParam(r, "tenantSlug")
	if slug == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("tenant slug is required")))
		return
	}
	phaseID, ok := common.ParsePositiveInt64IDWithError(w, r, "phaseId", "phaseId is required")
	if !ok {
		return
	}

	var schema *capability.FormSchema
	lateInviteToken := lateInviteTokenFromRequest(r)
	schoolID, resolveErr := rs.resolvePublicTenantID(r.Context(), slug)
	if resolveErr == nil {
		resolveErr = tenant.WithTenantTx(r.Context(), rs.db, schoolID, func(txCtx context.Context, _ bun.Tx) error {
			s, schemaErr := rs.RequestService.PublicActiveSchema(txCtx, phaseID, time.Now(), lateInviteToken)
			schema = s
			return schemaErr
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
	if rs.CaptchaService == nil || rs.SchoolService == nil || rs.db == nil {
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

	var schema *capability.FormSchema
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

	var schemas []*capability.FormSchema
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

	var schema *capability.FormSchema
	txErr := rs.runInTenantTx(r, func(ctx context.Context) error {
		s, innerErr := rs.FormSchemaService.GetByID(ctx, id)
		schema = s
		return innerErr
	})
	if txErr != nil {
		if errors.Is(txErr, sql.ErrNoRows) || errors.Is(txErr, enrollmentService.ErrFormSchemaNotFound) {
			common.RenderError(w, r, common.ErrorNotFound(txErr))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(txErr))
		return
	}

	common.Respond(w, r, http.StatusOK, toFormSchemaResponse(schema), "Form schema retrieved")
}

// publishSchema creates a schema. When the request body carries a
// name, this routes to CreateSchema (new named schema, version 1).
// When the name is omitted, it updates the active schema (or creates
// the default "Standardformular" one) for backward compatibility with
// the original single-schema flow.
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

	var schema *capability.FormSchema
	err := rs.runInTenantTx(r, func(ctx context.Context) error {
		s, publishErr := rs.FormSchemaService.PublishForm(ctx, enrollmentService.PublishFormInput{
			Name:             req.Name,
			Fields:           req.Fields,
			CoreRequirements: req.CoreRequirements,
			LegalBlocks:      req.LegalBlocks,
			ActorID:          int64(claims.ID),
		})
		schema = s
		return publishErr
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

	id, ok := common.ParsePositiveInt64IDWithError(w, r, "id", "invalid schema id")
	if !ok {
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

	var schema *capability.FormSchema
	txErr := rs.runInTenantTx(r, func(ctx context.Context) error {
		s, publishErr := rs.FormSchemaService.PublishFormVersion(ctx, enrollmentService.PublishFormVersionInput{
			ID:               id,
			Name:             req.Name,
			Fields:           req.Fields,
			CoreRequirements: req.CoreRequirements,
			LegalBlocks:      req.LegalBlocks,
			ActorID:          int64(claims.ID),
		})
		schema = s
		return publishErr
	})
	if txErr != nil {
		renderSchemaVersionError(w, r, txErr)
		return
	}

	slog.Default().Info("form schema version added",
		slog.Int64("schema_id", schema.ID),
		slog.String("name", schema.Name),
		slog.Int("version", schema.Version),
		slog.Int64("actor_account_id", int64(claims.ID)))

	common.Respond(w, r, http.StatusCreated, toFormSchemaResponse(schema), "Form schema version added")
}

// renderSchemaVersionError maps a PublishFormVersion failure to its HTTP
// status. A rename onto an existing name is a 409 with a stable code; a
// missing lineage is a 404 — same as the PATCH route, regardless of which
// step raised it. A rename failure beyond those sentinels is infrastructure
// (lock/read/exec), mapped to 5xx like the PATCH handler; publish/validation
// errors keep the 400 contract shared with POST /schema.
func renderSchemaVersionError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, enrollmentService.ErrFormSchemaNameExists):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, ErrCodeSchemaNameExists))
	case errors.Is(err, enrollmentService.ErrFormSchemaNotFound):
		common.RenderError(w, r, common.ErrorNotFound(err))
	default:
		var rf enrollmentService.RenameStepError
		if errors.As(err, &rf) {
			common.RenderError(w, r, common.ErrorInternalServer(err))
			return
		}
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	}
}

// renameSchema renames a logical schema (every version row sharing the
// source's name) in place. PATCH /schema/{id} — id refers to any
// version of the schema. Unlike updateSchema this publishes no new
// version and leaves the form fields untouched.
func (rs *Resource) renameSchema(w http.ResponseWriter, r *http.Request) {
	if rs.FormSchemaService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("form schema service not configured")))
		return
	}

	id, ok := common.ParsePositiveInt64IDWithError(w, r, "id", "invalid schema id")
	if !ok {
		return
	}

	req := &RenameSchemaRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	var schema *capability.FormSchema
	txErr := rs.runInTenantTx(r, func(ctx context.Context) error {
		s, innerErr := rs.FormSchemaService.RenameSchema(ctx, id, req.Name)
		schema = s
		return innerErr
	})
	if txErr != nil {
		switch {
		case errors.Is(txErr, enrollmentService.ErrFormSchemaNameExists):
			common.RenderError(w, r, common.ErrorConflictWithCode(txErr, ErrCodeSchemaNameExists))
			return
		case errors.Is(txErr, enrollmentService.ErrFormSchemaNotFound):
			common.RenderError(w, r, common.ErrorNotFound(txErr))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(txErr))
		return
	}

	// The service already logs the rename (old + new name) via its
	// injected logger; no duplicate handler-level log here.
	common.Respond(w, r, http.StatusOK, toFormSchemaResponse(schema), "Form schema renamed")
}

func (rs *Resource) deleteSchema(w http.ResponseWriter, r *http.Request) {
	if rs.FormSchemaService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("form schema service not configured")))
		return
	}

	id, ok := common.ParsePositiveInt64IDWithError(w, r, "id", "invalid schema id")
	if !ok {
		return
	}

	err := rs.runInTenantTx(r, func(ctx context.Context) error {
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
	ErrCodeSchemaNameExists  = "enrollment.schema_name_exists"
)

// runInTenantTx wraps the request's tenant context in a tenant
// transaction so the service's repo calls hit the right RLS scope.
// Tests use a nil DB and skip the transaction wrap.
func (rs *Resource) runInTenantTx(r *http.Request, fn func(ctx context.Context) error) error {
	if rs.runInTenantTxForTest != nil {
		return rs.runInTenantTxForTest(r, fn)
	}
	ctx := r.Context()
	if rs.db == nil {
		return fn(ctx)
	}
	tenantID := tenant.FromContext(ctx)
	return tenant.WithTenantTx(ctx, rs.db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		return fn(txCtx)
	})
}
