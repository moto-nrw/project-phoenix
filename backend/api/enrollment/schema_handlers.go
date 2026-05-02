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
	ID        string                       `json:"id"`
	Version   int                          `json:"version"`
	IsActive  bool                         `json:"is_active"`
	Fields    []enrollmentModels.FormField `json:"fields"`
	CreatedBy string                       `json:"created_by"`
	CreatedAt time.Time                    `json:"created_at"`
}

func toFormSchemaResponse(s *enrollmentModels.FormSchema) FormSchemaResponse {
	return FormSchemaResponse{
		ID:        strconv.FormatInt(s.ID, 10),
		Version:   s.Version,
		IsActive:  s.IsActive,
		Fields:    s.Fields,
		CreatedBy: strconv.FormatInt(s.CreatedBy, 10),
		CreatedAt: s.CreatedAt,
	}
}

// PublishSchemaRequest is the wire shape POST /schema accepts.
type PublishSchemaRequest struct {
	Fields []enrollmentModels.FormField `json:"fields"`
}

// Bind satisfies render.Binder. Field-level validation runs in the
// service so we don't duplicate it here.
func (req *PublishSchemaRequest) Bind(_ *http.Request) error {
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
	ID      string                       `json:"id"`
	Version int                          `json:"version"`
	Fields  []enrollmentModels.FormField `json:"fields"`
}

// listPublicActiveSchema returns the active form schema's custom
// fields for the resolved tenant. No JWT — same slug-gating pattern
// as listPublicCareOfferings / listPublicCalendarPeriods. The
// parent-facing enrollment form uses this so admin-defined custom
// fields render alongside the hardcoded core fields. Returns 404
// when the tenant has never published a schema (form falls back to
// core fields only).
func (rs *Resource) listPublicActiveSchema(w http.ResponseWriter, r *http.Request) {
	if rs.FormSchemaService == nil || rs.SchoolRepo == nil || rs.db == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("public schema endpoint not wired")))
		return
	}

	slug := chi.URLParam(r, "tenantSlug")
	if slug == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("tenant slug is required")))
		return
	}

	var schema *enrollmentModels.FormSchema
	resolveErr := tenant.WithAdminTx(r.Context(), rs.db, func(adminCtx context.Context, _ bun.Tx) error {
		school, schoolErr := rs.SchoolRepo.FindBySlug(adminCtx, slug)
		if schoolErr != nil || school == nil || school.IsDeleted() {
			return errors.New("tenant not found")
		}
		tenantCtx := tenant.WithTenantID(adminCtx, school.ID)
		return tenant.WithTenantTx(tenantCtx, rs.db, school.ID, func(txCtx context.Context, _ bun.Tx) error {
			s, innerErr := rs.FormSchemaService.GetActive(txCtx)
			schema = s
			return innerErr
		})
	})
	if resolveErr != nil {
		if errors.Is(resolveErr, enrollmentService.ErrNoActiveSchema) {
			common.RenderError(w, r, common.ErrorNotFound(resolveErr))
			return
		}
		common.RenderError(w, r, common.ErrorNotFound(resolveErr))
		return
	}

	out := PublicFormSchemaResponse{
		ID:      strconv.FormatInt(schema.ID, 10),
		Version: schema.Version,
		Fields:  schema.Fields,
	}
	common.Respond(w, r, http.StatusOK, out, "Public active form schema retrieved")
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

// publishSchema creates a new version with the supplied fields, marks
// it active, and deactivates any prior active version.
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
		s, innerErr := rs.FormSchemaService.PublishVersion(ctx, req.Fields, int64(claims.ID))
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
		slog.Int("version", schema.Version),
		slog.Int64("actor_account_id", int64(claims.ID)))

	common.Respond(w, r, http.StatusCreated, toFormSchemaResponse(schema), "Form schema published")
}

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
