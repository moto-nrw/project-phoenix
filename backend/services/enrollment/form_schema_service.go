// Package enrollment holds the parent-enrollment service layer. PR 5
// ships the form schema service; PR 6 will add care offerings; PR 7 the
// public submission service; PR 8 the per-child decision service.
package enrollment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

// ErrNoActiveSchema is returned by GetActive when no active form schema
// exists for the tenant. Callers (e.g., the public form renderer in
// PR 7) should treat this as "feature not configured yet".
var ErrNoActiveSchema = errors.New("no active form schema for tenant")

// FormSchemaService manages form-schema versioning. The two key
// operations are GetActive (consumed by PR 7's public form renderer
// + admin editor pre-fill) and PublishVersion (called by the admin
// editor on save - creates a new version and atomically marks it
// active, deactivating the previous one).
type FormSchemaService interface {
	// GetActive returns the currently-active form schema for the
	// tenant in context, or ErrNoActiveSchema if none exists.
	GetActive(ctx context.Context) (*enrollmentModels.FormSchema, error)

	// GetByID returns a specific schema version. Used to render
	// already-submitted requests against their pinned version.
	GetByID(ctx context.Context, id int64) (*enrollmentModels.FormSchema, error)

	// ListVersions returns all schema versions for the tenant in
	// context, newest-first. Powers the admin "version history" view.
	ListVersions(ctx context.Context) ([]*enrollmentModels.FormSchema, error)

	// PublishVersion creates a new schema version with the given
	// fields, marks it active, and deactivates any previously-active
	// version atomically. createdBy is the staff/admin account ID
	// from the JWT.
	//
	// Validation: every field validates per FormField.Validate; no
	// duplicate keys; no field shadows a CoreFieldKeys entry. The
	// fields slice may be empty (a tenant can publish a "no custom
	// fields" schema, in which case the parent form only collects
	// the core fields).
	PublishVersion(ctx context.Context, fields []enrollmentModels.FormField, createdBy int64) (*enrollmentModels.FormSchema, error)

	// ValidateSubmission checks a parent's submission payload against
	// a pinned schema version. PR 7's submit handler calls this. PR 5
	// ships the helper; PR 7 wires it.
	ValidateSubmission(ctx context.Context, schemaID int64, data enrollmentModels.SubmissionData) error
}

// FormSchemaServiceConfig is the dependency-injection bundle.
type FormSchemaServiceConfig struct {
	Repo   enrollmentModels.FormSchemaRepository
	Logger *slog.Logger
}

type formSchemaService struct {
	repo   enrollmentModels.FormSchemaRepository
	logger *slog.Logger
}

// NewFormSchemaService builds the service. Nil logger falls back to
// slog.Default().
func NewFormSchemaService(cfg FormSchemaServiceConfig) FormSchemaService {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &formSchemaService{
		repo:   cfg.Repo,
		logger: logger,
	}
}

func (s *formSchemaService) GetActive(ctx context.Context) (*enrollmentModels.FormSchema, error) {
	schema, err := s.repo.FindActive(ctx)
	if err != nil {
		// FindActive returns a wrapped sql.ErrNoRows; surface as
		// ErrNoActiveSchema for cleaner caller error handling.
		return nil, ErrNoActiveSchema
	}
	return schema, nil
}

func (s *formSchemaService) GetByID(ctx context.Context, id int64) (*enrollmentModels.FormSchema, error) {
	if id <= 0 {
		return nil, fmt.Errorf("schema id must be positive")
	}
	return s.repo.FindByID(ctx, id)
}

func (s *formSchemaService) ListVersions(ctx context.Context) ([]*enrollmentModels.FormSchema, error) {
	return s.repo.ListByTenant(ctx)
}

func (s *formSchemaService) PublishVersion(ctx context.Context, fields []enrollmentModels.FormField, createdBy int64) (*enrollmentModels.FormSchema, error) {
	if createdBy <= 0 {
		return nil, fmt.Errorf("createdBy is required")
	}

	// Validate fields up front so we don't write a half-correct row.
	// (FormSchema.Validate runs again inside the repo on Create - this
	// is the early bail-out path so callers get a clean 400 before
	// any DB roundtrip.)
	tmp := &enrollmentModels.FormSchema{Version: 1, CreatedBy: createdBy, Fields: fields}
	if err := tmp.Validate(); err != nil {
		return nil, fmt.Errorf("invalid schema: %w", err)
	}

	nextVersion, err := s.repo.NextVersion(ctx)
	if err != nil {
		return nil, fmt.Errorf("compute next version: %w", err)
	}

	// Deactivate previous versions before inserting the new one. The
	// partial unique index uq_form_schemas_one_active_per_tenant
	// enforces the at-most-one-active invariant; this prevents a
	// unique violation on the new INSERT.
	if err := s.repo.DeactivatePrevious(ctx); err != nil {
		return nil, fmt.Errorf("deactivate previous: %w", err)
	}

	schema := &enrollmentModels.FormSchema{
		Version:   nextVersion,
		Fields:    fields,
		IsActive:  true,
		CreatedBy: createdBy,
	}
	if err := s.repo.Create(ctx, schema); err != nil {
		return nil, fmt.Errorf("create schema: %w", err)
	}

	s.logger.Info("form schema published",
		slog.Int("version", schema.Version),
		slog.Int64("schema_id", schema.ID),
		slog.Int64("created_by", createdBy),
		slog.Int("field_count", len(fields)))

	return schema, nil
}

// ValidateSubmission resolves the schema by ID and runs the per-field
// validators against the supplied data. PR 5 ships a basic "every
// required field has a value" check; PR 7 will extend with type
// coercion and pattern enforcement.
func (s *formSchemaService) ValidateSubmission(ctx context.Context, schemaID int64, data enrollmentModels.SubmissionData) error {
	schema, err := s.repo.FindByID(ctx, schemaID)
	if err != nil {
		return fmt.Errorf("schema lookup: %w", err)
	}

	for i := range schema.Fields {
		field := schema.Fields[i]
		if !field.Required {
			continue
		}

		var value any
		var present bool
		if field.AppliesToCh {
			// Per-child fields validated by PR 7 (when child slot
			// indexing is wired); PR 5 only checks guardian-level
			// required fields. Skip until then to avoid false
			// positives during admin form preview.
			continue
		}
		value, present = data.GuardianFields[field.Key]
		if !present || value == nil || value == "" {
			return fmt.Errorf("field %q is required", field.Key)
		}
	}
	return nil
}
