// Package enrollment holds the parent-enrollment service layer: form schemas,
// care offerings, public submissions, and per-child decisions.
package enrollment

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// ErrNoActiveSchema is returned by GetActive when no active form schema
// exists for the tenant. Callers should treat this as "feature not configured
// yet".
var ErrNoActiveSchema = errors.New("no active form schema for tenant")

var (
	ErrFormSchemaNotFound    = errors.New("form schema not found")
	ErrFormSchemaHasPhases   = errors.New("form schema has enrollment phases")
	ErrFormSchemaHasRequests = errors.New("form schema has enrollment requests")
	// ErrFormSchemaNameExists is returned by RenameSchema when the target
	// name already identifies a different logical schema for the tenant.
	ErrFormSchemaNameExists = errors.New("a form schema with this name already exists")
)

// formSchemaNameVersionUniqueIndex is the Postgres unique index from
// migration 1.15.74 (tenant_id, name, version). Kept as a string
// constant — the migration declares it inline — so RenameSchema can
// translate a 23505 race into ErrFormSchemaNameExists.
const formSchemaNameVersionUniqueIndex = "uq_form_schemas_tenant_name_version"

// FormSchemaService manages form-schema versioning. GetActive feeds the public
// form renderer and admin pre-fill; CreateSchema/UpdateSchema create new
// schemas and versions.
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

	// RenameSchema renames the logical schema selected by id. All version
	// rows sharing the source's name are renamed atomically so the version
	// lineage stays intact. Renaming to a name already used by a different
	// schema returns ErrFormSchemaNameExists. The returned schema is the
	// row identified by id with its updated name.
	RenameSchema(ctx context.Context, id int64, newName string) (*enrollmentModels.FormSchema, error)

	// DeleteSchema removes every version of the logical schema selected
	// by id. It refuses deletion when any version is used by a phase or
	// historical request.
	DeleteSchema(ctx context.Context, id int64) error
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
	schema, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: %w", ErrFormSchemaNotFound, err)
		}
		return nil, err
	}
	return schema, nil
}

func (s *formSchemaService) ListVersions(ctx context.Context) ([]*enrollmentModels.FormSchema, error) {
	return s.repo.ListByTenant(ctx)
}

// defaultSchemaName is the fallback name for legacy callers that don't
// supply one. Matches the backfill string used by migration 1.15.74 so
// older rows merge cleanly into the same logical schema.
const defaultSchemaName = "Standardformular"

func (s *formSchemaService) CreateSchema(ctx context.Context, name string, fields []enrollmentModels.FormField, createdBy int64, coreRequirements ...enrollmentModels.CoreRequirements) (*enrollmentModels.FormSchema, error) {
	if name == "" {
		return nil, fmt.Errorf("schema name is required")
	}
	if err := s.repo.LockLineages(ctx); err != nil {
		return nil, err
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
	if err := s.repo.LockLineages(ctx); err != nil {
		return nil, err
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
	// Lock the lineage before reading its name: a concurrent rename must not
	// move the lineage between this read and the new version's insert, or the
	// new row is born under the stale name and splits the lineage.
	if err := s.repo.LockLineages(ctx); err != nil {
		return nil, err
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
	// Lock the lineage before reading its name (see UpdateSchema).
	if err := s.repo.LockLineages(ctx); err != nil {
		return nil, err
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

func (s *formSchemaService) RenameSchema(ctx context.Context, id int64, newName string) (*enrollmentModels.FormSchema, error) {
	if id <= 0 {
		return nil, ErrFormSchemaNotFound
	}
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return nil, fmt.Errorf("schema name is required")
	}

	// Serialize against concurrent publishes of the same lineage: hold the
	// lock across the load, the collision check, and the rename so a publish
	// can't insert a new version under the old name mid-rename.
	if err := s.repo.LockLineages(ctx); err != nil {
		return nil, err
	}

	source, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: %w", ErrFormSchemaNotFound, err)
		}
		return nil, fmt.Errorf("load source schema: %w", err)
	}

	// Renaming to the current name is a no-op; return the source unchanged
	// so callers don't trip the collision check against the schema itself.
	if source.Name == newName {
		return source, nil
	}

	// Reject collisions with another logical schema before we touch the
	// unique index. The same-name early return above means any hit here is
	// a different lineage.
	exists, err := s.repo.ExistsByName(ctx, newName)
	if err != nil {
		return nil, fmt.Errorf("check existing name: %w", err)
	}
	if exists {
		return nil, ErrFormSchemaNameExists
	}

	oldName := source.Name
	if err := s.repo.RenameByName(ctx, oldName, newName); err != nil {
		// Race-safe fallback: if a concurrent rename claimed newName
		// between the check above and this update, the unique index
		// (tenant_id, name, version) raises 23505. Translate it to the
		// sentinel so the handler still returns 409, not a generic 400.
		if isUniqueViolationOn(err, formSchemaNameVersionUniqueIndex) {
			return nil, ErrFormSchemaNameExists
		}
		return nil, fmt.Errorf("rename schema: %w", err)
	}

	s.logger.Info("form schema renamed",
		slog.String("old_name", oldName),
		slog.String("new_name", newName),
		slog.Int64("schema_id", id))

	source.Name = newName
	return source, nil
}

func (s *formSchemaService) DeleteSchema(ctx context.Context, id int64) error {
	if id <= 0 {
		return ErrFormSchemaNotFound
	}
	if s.phaseRepo == nil || s.requestRepo == nil {
		return fmt.Errorf("schema delete dependencies not configured")
	}

	// Serialize against a concurrent publish: deleting a lineage while a new
	// version is being inserted under the same name would otherwise leave an
	// orphan row behind.
	if err := s.repo.LockLineages(ctx); err != nil {
		return err
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

	normalizedLegalBlocks, err := normalizeSchemaLegalDocumentURLs(ctx, legalBlocks)
	if err != nil {
		return nil, err
	}

	// Validate fields up front so we don't write a half-correct row.
	tmp := &enrollmentModels.FormSchema{Name: name, Version: 1, CreatedBy: createdBy, Fields: fields, CoreRequirements: coreRequirements, LegalBlocks: legalBlocks}
	tmp.LegalBlocks = normalizedLegalBlocks
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

func normalizeSchemaLegalDocumentURLs(ctx context.Context, blocks []enrollmentModels.FormLegalBlock) ([]enrollmentModels.FormLegalBlock, error) {
	if len(blocks) == 0 {
		return blocks, nil
	}

	tenantID := tenant.FromContext(ctx)
	normalized := make([]enrollmentModels.FormLegalBlock, len(blocks))
	copy(normalized, blocks)
	for i := range normalized {
		block := &normalized[i]
		if block.DisplayMode != enrollmentModels.LegalBlockDisplayModePDF {
			block.DocumentURL = ""
			continue
		}
		if strings.TrimSpace(block.DocumentURL) == "" {
			block.DocumentURL = ""
			continue
		}
		documentURL, ok := enrollmentModels.NormalizeTenantLegalDocumentURL(block.DocumentURL, tenantID)
		if !ok {
			return nil, fmt.Errorf("invalid schema: legal block %q has an invalid PDF document URL", block.Key)
		}
		block.DocumentURL = documentURL
	}
	return normalized, nil
}

func firstCoreRequirements(values []enrollmentModels.CoreRequirements) enrollmentModels.CoreRequirements {
	if len(values) == 0 || values[0] == nil {
		return enrollmentModels.CoreRequirements{}
	}
	return values[0]
}

// conditionValuesEqual compares a submitted answer against a condition
// value tolerant of the JSON round-trip (bool vs "true", number vs
// string), which is enough for boolean/select controlling fields.
func conditionValuesEqual(a, b any) bool {
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}
