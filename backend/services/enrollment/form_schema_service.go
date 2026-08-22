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

	"github.com/moto-nrw/project-phoenix/models/base"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
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

	// PublishForm creates or updates a form schema (POST /schema). A
	// non-empty Name creates a new named schema (version 1). An empty
	// Name targets the tenant's active schema (legacy single-schema
	// flow): it updates the active schema when one exists, reuses an
	// existing "Standardformular" lineage, or creates the default
	// "Standardformular" schema.
	PublishForm(ctx context.Context, in PublishFormInput) (*enrollmentModels.FormSchema, error)

	// PublishFormVersion publishes a new version of an existing schema
	// (PUT /schema/{id}). When Name is set and non-blank it renames the
	// whole lineage first, in the SAME transaction as the publish, so a
	// failed publish rolls the rename back. A rename failure beyond the
	// name-exists / not-found sentinels is wrapped in RenameStepError so
	// the caller can distinguish rename infrastructure faults from
	// publish validation errors.
	PublishFormVersion(ctx context.Context, in PublishFormVersionInput) (*enrollmentModels.FormSchema, error)
}

// PublishFormInput carries the fields for the create-or-update publish
// flow that POST /schema drives.
type PublishFormInput struct {
	Name             string
	Fields           []enrollmentModels.FormField
	CoreRequirements *enrollmentModels.CoreRequirements
	LegalBlocks      *[]enrollmentModels.FormLegalBlock
	ActorID          int64
}

// PublishFormVersionInput carries the fields for the combined
// rename+publish flow that PUT /schema/{id} drives. Name is optional:
// nil or blank skips the rename step.
type PublishFormVersionInput struct {
	ID               int64
	Name             *string
	Fields           []enrollmentModels.FormField
	CoreRequirements *enrollmentModels.CoreRequirements
	LegalBlocks      *[]enrollmentModels.FormLegalBlock
	ActorID          int64
}

// RenameStepError tags a failure originating from the rename step of a
// combined rename+publish (PublishFormVersion). It unwraps so errors.Is
// still matches the rename sentinels (name-exists, not-found); a caller
// maps a bare rename infrastructure fault (lock/read/exec) to a 5xx while
// publish/validation failures keep their 400 contract.
type RenameStepError struct{ err error }

// NewRenameStepError wraps err as a rename-step failure.
func NewRenameStepError(err error) RenameStepError { return RenameStepError{err: err} }

func (e RenameStepError) Error() string { return e.err.Error() }
func (e RenameStepError) Unwrap() error { return e.err }

// FormSchemaServiceConfig is the dependency-injection bundle.
type FormSchemaServiceConfig struct {
	Repo        enrollmentModels.FormSchemaRepository
	PhaseRepo   enrollmentModels.PhaseRepository
	RequestRepo enrollmentModels.RequestRepository
	// Settings backs the Heimweg-Beschränkung publish guard (#2381). Nil
	// (some unit setups) skips the guard.
	Settings classCollectionResolver
	Logger   *slog.Logger
}

type formSchemaService struct {
	repo        enrollmentModels.FormSchemaRepository
	phaseRepo   enrollmentModels.PhaseRepository
	requestRepo enrollmentModels.RequestRepository
	settings    classCollectionResolver
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
		settings:    cfg.Settings,
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
		if base.IsUniqueViolationOn(err, formSchemaNameVersionUniqueIndex) {
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

func (s *formSchemaService) PublishForm(ctx context.Context, in PublishFormInput) (*enrollmentModels.FormSchema, error) {
	if in.Name != "" {
		return s.createNamedSchema(ctx, in.Name, in)
	}

	active, err := s.GetActive(ctx)
	if err == nil {
		return s.updateSchemaFromInput(ctx, active.ID, in)
	}
	if !errors.Is(err, ErrNoActiveSchema) {
		return nil, err
	}

	// No active schema: reuse an existing default lineage if one is around,
	// otherwise create the default "Standardformular" schema.
	versions, listErr := s.ListVersions(ctx)
	if listErr != nil {
		return nil, listErr
	}
	for _, version := range versions {
		if version.Name == defaultSchemaName {
			return s.updateSchemaFromInput(ctx, version.ID, in)
		}
	}
	return s.createNamedSchema(ctx, defaultSchemaName, in)
}

func (s *formSchemaService) createNamedSchema(ctx context.Context, name string, in PublishFormInput) (*enrollmentModels.FormSchema, error) {
	if in.LegalBlocks != nil {
		return s.CreateSchemaWithLegal(ctx, name, in.Fields, in.ActorID, pointerCoreRequirements(in.CoreRequirements), *in.LegalBlocks)
	}
	return s.CreateSchema(ctx, name, in.Fields, in.ActorID, pointerCoreRequirements(in.CoreRequirements))
}

func (s *formSchemaService) updateSchemaFromInput(ctx context.Context, id int64, in PublishFormInput) (*enrollmentModels.FormSchema, error) {
	if in.LegalBlocks != nil {
		return s.UpdateSchemaWithLegal(ctx, id, in.Fields, in.ActorID, in.CoreRequirements, in.LegalBlocks)
	}
	if in.CoreRequirements == nil {
		return s.UpdateSchema(ctx, id, in.Fields, in.ActorID)
	}
	return s.UpdateSchema(ctx, id, in.Fields, in.ActorID, *in.CoreRequirements)
}

func (s *formSchemaService) PublishFormVersion(ctx context.Context, in PublishFormVersionInput) (*enrollmentModels.FormSchema, error) {
	// Combined "rename + edit" save: rename the lineage first, in the same
	// transaction as the publish below, so a failed publish rolls the rename
	// back — no partial "renamed but content unchanged" state. RenameSchema
	// is a no-op when the name is unchanged; a blank name is ignored (the
	// dedicated PATCH route owns blank-name rejection).
	if in.Name != nil && strings.TrimSpace(*in.Name) != "" {
		if _, renameErr := s.RenameSchema(ctx, in.ID, *in.Name); renameErr != nil {
			return nil, RenameStepError{err: renameErr}
		}
	}
	if in.LegalBlocks != nil {
		return s.UpdateSchemaWithLegal(ctx, in.ID, in.Fields, in.ActorID, in.CoreRequirements, in.LegalBlocks)
	}
	if in.CoreRequirements == nil {
		return s.UpdateSchema(ctx, in.ID, in.Fields, in.ActorID)
	}
	return s.UpdateSchema(ctx, in.ID, in.Fields, in.ActorID, *in.CoreRequirements)
}

// pointerCoreRequirements dereferences an optional CoreRequirements,
// returning an empty (non-nil) matrix when the pointer is nil.
func pointerCoreRequirements(value *enrollmentModels.CoreRequirements) enrollmentModels.CoreRequirements {
	if value == nil {
		return enrollmentModels.CoreRequirements{}
	}
	return *value
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

	if err := s.ensureSingleModeGradesCollectable(ctx, fields); err != nil {
		return nil, err
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

// ensureSingleModeGradesCollectable rejects publishing a schema whose
// Heimweg-Beschränkung (single_mode_grades, #2381) the school cannot apply:
// the rule keys on each child's declared target grade level, which the form
// only collects while enrollment.collect_grade_level is on. With collection
// off the rule would silently never restrict anyone, so the publish fails
// loudly and the admin enables grade collection (or clears the rule) first.
// Mirrors ensureEligibleGradeLevelsCollectable, including the per-tenant
// lock against a concurrent settings write (#1663); the settings side does
// not check schemas symmetrically — disabling grade collection later just
// makes the rule inert (children without a grade fall back to multi-select),
// which is the documented no-rule behaviour, not a bypass.
func (s *formSchemaService) ensureSingleModeGradesCollectable(ctx context.Context, fields []enrollmentModels.FormField) error {
	if s.settings == nil {
		return nil
	}
	restricted := false
	for i := range fields {
		if len(fields[i].SingleModeGrades) > 0 {
			restricted = true
			break
		}
	}
	if !restricted {
		return nil
	}
	if locker, ok := s.settings.(interface {
		LockClassCollectionPair(context.Context) error
	}); ok {
		if err := locker.LockClassCollectionPair(ctx); err != nil {
			return fmt.Errorf("lock class-collection pair: %w", err)
		}
	}
	collectGrade, err := s.settings.ResolveBool(ctx, configModel.KeyEnrollmentCollectGradeLevel)
	if err != nil {
		return fmt.Errorf("resolve %s: %w", configModel.KeyEnrollmentCollectGradeLevel, err)
	}
	if !collectGrade {
		return fmt.Errorf("invalid schema: single_mode_grades requires the grade-level collection setting (Klassenstufen-Abfrage) to be active")
	}
	return nil
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
