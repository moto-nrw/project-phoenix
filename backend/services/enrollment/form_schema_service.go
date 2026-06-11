// Package enrollment holds the parent-enrollment service layer: form schemas,
// care offerings, public submissions, and per-child decisions.
package enrollment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

// ErrNoActiveSchema is returned by GetActive when no active form schema
// exists for the tenant. Callers should treat this as "feature not configured
// yet".
var ErrNoActiveSchema = errors.New("no active form schema for tenant")

var (
	ErrFormSchemaNotFound    = errors.New("form schema not found")
	ErrFormSchemaHasPhases   = errors.New("form schema has enrollment phases")
	ErrFormSchemaHasRequests = errors.New("form schema has enrollment requests")
)

// FormSchemaService manages form-schema versioning. GetActive feeds the public
// form renderer and admin pre-fill; PublishVersion creates a new active version
// for the default schema.
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
	// Deprecated: prefer CreateSchema (new schema) or UpdateSchema
	// (new version of an existing schema). PublishVersion is kept for
	// backward compatibility with the original single-schema flow.
	// it now writes the row with name="Standardformular" and does NOT
	// deactivate other names' rows, only siblings of the same name.
	PublishVersion(ctx context.Context, fields []enrollmentModels.FormField, createdBy int64, coreRequirements ...enrollmentModels.CoreRequirements) (*enrollmentModels.FormSchema, error)

	// CreateSchema creates a new logical schema (version 1) under the
	// given name. Use this when the admin clicks "Neues Formular" on
	// the schema list page. Names are unique per tenant by convention
	// but not by DB constraint. The service rejects an attempt to
	// create a new schema with a name that already exists; callers
	// must use UpdateSchema to add another version to an existing
	// schema instead.
	CreateSchema(ctx context.Context, name string, fields []enrollmentModels.FormField, createdBy int64, coreRequirements ...enrollmentModels.CoreRequirements) (*enrollmentModels.FormSchema, error)
	CreateSchemaWithLegal(ctx context.Context, name string, fields []enrollmentModels.FormField, createdBy int64, coreRequirements enrollmentModels.CoreRequirements, legalBlocks []enrollmentModels.FormLegalBlock) (*enrollmentModels.FormSchema, error)

	// UpdateSchema publishes a new version of an existing schema,
	// looked up by id. The new row inherits the source row's name,
	// uses max(version)+1 for that name, and is marked active. Phases
	// using an older version of the same logical schema are repointed
	// to the new row, while previously-submitted requests keep their
	// schema reference intact.
	UpdateSchema(ctx context.Context, id int64, fields []enrollmentModels.FormField, updatedBy int64, coreRequirements ...enrollmentModels.CoreRequirements) (*enrollmentModels.FormSchema, error)
	UpdateSchemaWithLegal(ctx context.Context, id int64, fields []enrollmentModels.FormField, updatedBy int64, coreRequirements *enrollmentModels.CoreRequirements, legalBlocks *[]enrollmentModels.FormLegalBlock) (*enrollmentModels.FormSchema, error)

	// DeleteSchema removes every version of the logical schema selected
	// by id. It refuses deletion when any version is used by a phase or
	// historical request.
	DeleteSchema(ctx context.Context, id int64) error

	// ValidateSubmission checks guardian-level custom data against a pinned
	// schema version. The parent submit flow performs the full request/child
	// validation before creating enrollment rows.
	ValidateSubmission(ctx context.Context, schemaID int64, data enrollmentModels.SubmissionData) error
}

// FormSchemaServiceConfig is the dependency-injection bundle.
type FormSchemaServiceConfig struct {
	Repo        enrollmentModels.FormSchemaRepository
	PhaseRepo   enrollmentModels.PhaseRepository
	RequestRepo enrollmentModels.RequestRepository
	Logger      *slog.Logger
}

type formSchemaService struct {
	repo        enrollmentModels.FormSchemaRepository
	phaseRepo   enrollmentModels.PhaseRepository
	requestRepo enrollmentModels.RequestRepository
	logger      *slog.Logger
}

// NewFormSchemaService builds the service. Nil logger falls back to
// slog.Default().
func NewFormSchemaService(cfg FormSchemaServiceConfig) FormSchemaService {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &formSchemaService{
		repo:        cfg.Repo,
		phaseRepo:   cfg.PhaseRepo,
		requestRepo: cfg.RequestRepo,
		logger:      logger,
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

// defaultSchemaName is the name PublishVersion writes for legacy
// callers that don't supply one. Matches the backfill string used by
// migration 1.15.74 so older rows merge cleanly into the same logical
// schema.
const defaultSchemaName = "Standardformular"

func (s *formSchemaService) PublishVersion(ctx context.Context, fields []enrollmentModels.FormField, createdBy int64, coreRequirements ...enrollmentModels.CoreRequirements) (*enrollmentModels.FormSchema, error) {
	return s.createOrVersion(ctx, defaultSchemaName, fields, createdBy, firstCoreRequirements(coreRequirements), nil)
}

func (s *formSchemaService) CreateSchema(ctx context.Context, name string, fields []enrollmentModels.FormField, createdBy int64, coreRequirements ...enrollmentModels.CoreRequirements) (*enrollmentModels.FormSchema, error) {
	if name == "" {
		return nil, fmt.Errorf("schema name is required")
	}
	// Refuse to overload an existing name. The admin should use
	// UpdateSchema to add a new version instead. The
	// "next version > 1" check is the lightweight uniqueness signal.
	existing, err := s.repo.NextVersionForName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("check existing name: %w", err)
	}
	if existing > 1 {
		return nil, fmt.Errorf("schema with name %q already exists; use UpdateSchema to add a new version", name)
	}
	return s.createOrVersion(ctx, name, fields, createdBy, firstCoreRequirements(coreRequirements), nil)
}

func (s *formSchemaService) CreateSchemaWithLegal(ctx context.Context, name string, fields []enrollmentModels.FormField, createdBy int64, coreRequirements enrollmentModels.CoreRequirements, legalBlocks []enrollmentModels.FormLegalBlock) (*enrollmentModels.FormSchema, error) {
	if name == "" {
		return nil, fmt.Errorf("schema name is required")
	}
	existing, err := s.repo.NextVersionForName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("check existing name: %w", err)
	}
	if existing > 1 {
		return nil, fmt.Errorf("schema with name %q already exists; use UpdateSchema to add a new version", name)
	}
	return s.createOrVersion(ctx, name, fields, createdBy, coreRequirements, legalBlocks)
}

func (s *formSchemaService) UpdateSchema(ctx context.Context, id int64, fields []enrollmentModels.FormField, updatedBy int64, coreRequirements ...enrollmentModels.CoreRequirements) (*enrollmentModels.FormSchema, error) {
	if id <= 0 {
		return nil, fmt.Errorf("schema id must be positive")
	}
	source, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("load source schema: %w", err)
	}
	nextCoreRequirements := source.CoreRequirements
	if len(coreRequirements) > 0 {
		nextCoreRequirements = firstCoreRequirements(coreRequirements)
	}
	return s.createOrVersion(ctx, source.Name, fields, updatedBy, nextCoreRequirements, source.LegalBlocks)
}

func (s *formSchemaService) UpdateSchemaWithLegal(ctx context.Context, id int64, fields []enrollmentModels.FormField, updatedBy int64, coreRequirements *enrollmentModels.CoreRequirements, legalBlocks *[]enrollmentModels.FormLegalBlock) (*enrollmentModels.FormSchema, error) {
	if id <= 0 {
		return nil, fmt.Errorf("schema id must be positive")
	}
	source, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("load source schema: %w", err)
	}
	nextCoreRequirements := source.CoreRequirements
	if coreRequirements != nil {
		nextCoreRequirements = *coreRequirements
	}
	nextLegalBlocks := source.LegalBlocks
	if legalBlocks != nil {
		nextLegalBlocks = *legalBlocks
	}
	return s.createOrVersion(ctx, source.Name, fields, updatedBy, nextCoreRequirements, nextLegalBlocks)
}

func (s *formSchemaService) DeleteSchema(ctx context.Context, id int64) error {
	if id <= 0 {
		return ErrFormSchemaNotFound
	}
	if s.phaseRepo == nil || s.requestRepo == nil {
		return fmt.Errorf("schema delete dependencies not configured")
	}

	source, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return ErrFormSchemaNotFound
	}

	schemas, err := s.repo.ListByTenant(ctx)
	if err != nil {
		return fmt.Errorf("list schema versions: %w", err)
	}
	found := false
	for _, schema := range schemas {
		if schema.Name != source.Name {
			continue
		}
		found = true

		phaseUsesSchema, phaseErr := s.phaseRepo.ExistsByFormSchemaID(ctx, schema.ID)
		if phaseErr != nil {
			return fmt.Errorf("check schema phase references: %w", phaseErr)
		}
		if phaseUsesSchema {
			return ErrFormSchemaHasPhases
		}

		requestUsesSchema, requestErr := s.requestRepo.ExistsBySchemaID(ctx, schema.ID)
		if requestErr != nil {
			return fmt.Errorf("check schema request references: %w", requestErr)
		}
		if requestUsesSchema {
			return ErrFormSchemaHasRequests
		}
	}
	if !found {
		return ErrFormSchemaNotFound
	}

	if err := s.repo.DeleteByName(ctx, source.Name); err != nil {
		return fmt.Errorf("delete schema: %w", err)
	}
	s.logger.Info("form schema deleted",
		slog.String("name", source.Name),
		slog.Int64("schema_id", id))
	return nil
}

// createOrVersion is the shared internal: pick max(version)+1 for the
// name and insert a new active row. Sibling rows with the same name
// stay in place for historical submissions, but phases using any prior
// sibling version are advanced to the newly published row.
func (s *formSchemaService) createOrVersion(ctx context.Context, name string, fields []enrollmentModels.FormField, createdBy int64, coreRequirements enrollmentModels.CoreRequirements, legalBlocks []enrollmentModels.FormLegalBlock) (*enrollmentModels.FormSchema, error) {
	if createdBy <= 0 {
		return nil, fmt.Errorf("createdBy is required")
	}
	if name == "" {
		name = defaultSchemaName
	}

	// Validate fields up front so we don't write a half-correct row.
	tmp := &enrollmentModels.FormSchema{Name: name, Version: 1, CreatedBy: createdBy, Fields: fields, CoreRequirements: coreRequirements, LegalBlocks: legalBlocks}
	if err := tmp.Validate(); err != nil {
		return nil, fmt.Errorf("invalid schema: %w", err)
	}

	nextVersion, err := s.repo.NextVersionForName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("compute next version: %w", err)
	}

	schema := &enrollmentModels.FormSchema{
		Name:             name,
		Version:          nextVersion,
		Fields:           fields,
		CoreRequirements: tmp.CoreRequirements,
		LegalBlocks:      tmp.LegalBlocks,
		IsActive:         true,
		CreatedBy:        createdBy,
	}
	if err := s.repo.Create(ctx, schema); err != nil {
		return nil, fmt.Errorf("create schema: %w", err)
	}

	// Advance any phase still bound to a prior version of this schema name
	// onto the freshly published version, so admin edits (new required
	// fields, core requirements, ...) reach the public form without a
	// manual re-bind. A brand-new name has no prior versions, so this is a
	// no-op for CreateSchema. Failing here rolls back the publish rather
	// than silently leaving the edit invisible.
	if err := s.repointPhasesToVersion(ctx, schema); err != nil {
		return nil, err
	}

	s.logger.Info("form schema published",
		slog.String("name", schema.Name),
		slog.Int("version", schema.Version),
		slog.Int64("schema_id", schema.ID),
		slog.Int64("created_by", createdBy),
		slog.Int("field_count", len(fields)))

	return schema, nil
}

// repointPhasesToVersion moves every phase bound to an older version of
// newSchema's name onto newSchema.ID. No-op when the phase repo isn't
// wired (some unit setups) or when no sibling versions exist.
func (s *formSchemaService) repointPhasesToVersion(ctx context.Context, newSchema *enrollmentModels.FormSchema) error {
	if s.phaseRepo == nil {
		return nil
	}
	versions, err := s.repo.ListByTenant(ctx)
	if err != nil {
		return fmt.Errorf("list schema versions for repoint: %w", err)
	}
	oldIDs := make([]int64, 0, len(versions))
	for _, v := range versions {
		if v.Name == newSchema.Name && v.ID != newSchema.ID {
			oldIDs = append(oldIDs, v.ID)
		}
	}
	if len(oldIDs) == 0 {
		return nil
	}
	updated, err := s.phaseRepo.RepointFormSchema(ctx, oldIDs, newSchema.ID)
	if err != nil {
		return fmt.Errorf("repoint phases to schema %d: %w", newSchema.ID, err)
	}
	if updated > 0 {
		s.logger.Info("repointed phases to new schema version",
			slog.String("name", newSchema.Name),
			slog.Int64("new_schema_id", newSchema.ID),
			slog.Int64("phases_updated", updated))
	}
	return nil
}

func firstCoreRequirements(values []enrollmentModels.CoreRequirements) enrollmentModels.CoreRequirements {
	if len(values) == 0 || values[0] == nil {
		return enrollmentModels.CoreRequirements{}
	}
	return values[0]
}

// ValidateSubmission resolves the schema by ID and runs the legacy
// guardian-level required-field check against the supplied data. Full public
// submission validation, including child fields and structured field types,
// lives in request_service.go.
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
			// Per-child fields are validated in the parent submit flow where
			// child rows are available. This helper only receives
			// guardian-level data.
			continue
		}
		// A field hidden by its visibility condition is not shown to the
		// parent, so its required flag does not apply to this submission.
		if guardianFieldHidden(field, data.GuardianFields) {
			continue
		}
		value, present = data.GuardianFields[field.Key]
		if !present || value == nil || value == "" {
			return fmt.Errorf("field %q is required", field.Key)
		}
	}
	return nil
}

// guardianFieldHidden reports whether a guardian-level field is hidden
// by its visibility condition given the submitted guardian answers.
// Only "field"-sourced conditions are evaluable at guardian level; the
// grade_level / care_offering sources are per-child and never apply to a
// guardian-level field (the model rejects them there). Shares the scalar
// comparison logic with the full evaluator in form_visibility.go.
func guardianFieldHidden(field enrollmentModels.FormField, guardianFields map[string]any) bool {
	c := field.VisibleWhen
	if c == nil || c.Source != enrollmentModels.ConditionSourceField {
		return false
	}
	return !matchScalar(c.Operator, guardianFields[c.Field], c.Value)
}

// conditionValuesEqual compares a submitted answer against a condition
// value tolerant of the JSON round-trip (bool vs "true", number vs
// string), which is enough for boolean/select controlling fields.
func conditionValuesEqual(a, b any) bool {
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}
