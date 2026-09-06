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

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// ErrNoActiveSchema is returned by GetActive when no active form schema
// exists for the tenant. Callers should treat this as "feature not configured
// yet".
var ErrNoActiveSchema = errors.New("no active form schema for tenant")

var (
	ErrFormSchemaNotFound    = capability.ErrFormSchemaNotFound
	ErrFormSchemaHasPhases   = capability.ErrFormSchemaHasPhases
	ErrFormSchemaHasRequests = capability.ErrFormSchemaHasRequests
	// ErrFormSchemaNameExists is returned by RenameSchema when the target
	// name already identifies a different logical schema for the tenant.
	ErrFormSchemaNameExists = capability.ErrFormSchemaNameExists
)

// FormSchemaService manages form-schema versioning. GetActive feeds the public
// form renderer and admin pre-fill; CreateSchema/UpdateSchema create new
// schemas and versions.
type FormSchemaService interface {
	// GetActive returns the currently-active form schema for the
	// tenant in context, or ErrNoActiveSchema if none exists.
	GetActive(ctx context.Context) (*capability.FormSchema, error)

	// GetByID returns a specific schema version. Used to render
	// already-submitted requests against their pinned version.
	GetByID(ctx context.Context, id int64) (*capability.FormSchema, error)

	// ListVersions returns all schema versions for the tenant in
	// context, newest-first. Powers the admin "version history" view.
	ListVersions(ctx context.Context) ([]*capability.FormSchema, error)

	// CreateSchema creates a new logical schema (version 1) under the
	// given name. Use this when the admin clicks "Neues Formular" on
	// the schema list page. Names are unique per tenant by convention
	// but not by DB constraint. The service rejects an attempt to
	// create a new schema with a name that already exists; callers
	// must use UpdateSchema to add another version to an existing
	// schema instead.
	CreateSchema(ctx context.Context, name string, fields []capability.FormField, createdBy int64, coreRequirements ...capability.CoreRequirements) (*capability.FormSchema, error)
	CreateSchemaWithLegal(ctx context.Context, name string, fields []capability.FormField, createdBy int64, coreRequirements capability.CoreRequirements, legalBlocks []capability.FormLegalBlock) (*capability.FormSchema, error)

	// UpdateSchema publishes a new version of an existing schema,
	// looked up by id. The new row inherits the source row's name,
	// uses max(version)+1 for that name, and is marked active. Phases
	// using an older version of the same logical schema are repointed
	// to the new row, while previously-submitted requests keep their
	// schema reference intact.
	UpdateSchema(ctx context.Context, id int64, fields []capability.FormField, updatedBy int64, coreRequirements ...capability.CoreRequirements) (*capability.FormSchema, error)
	UpdateSchemaWithLegal(ctx context.Context, id int64, fields []capability.FormField, updatedBy int64, coreRequirements *capability.CoreRequirements, legalBlocks *[]capability.FormLegalBlock) (*capability.FormSchema, error)

	// RenameSchema renames the logical schema selected by id. All version
	// rows sharing the source's name are renamed atomically so the version
	// lineage stays intact. Renaming to a name already used by a different
	// schema returns ErrFormSchemaNameExists. The returned schema is the
	// row identified by id with its updated name.
	RenameSchema(ctx context.Context, id int64, newName string) (*capability.FormSchema, error)

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
	PublishForm(ctx context.Context, in PublishFormInput) (*capability.FormSchema, error)

	// PublishFormVersion publishes a new version of an existing schema
	// (PUT /schema/{id}). When Name is set and non-blank it renames the
	// whole lineage first, in the SAME transaction as the publish, so a
	// failed publish rolls the rename back. A rename failure beyond the
	// name-exists / not-found sentinels is wrapped in RenameStepError so
	// the caller can distinguish rename infrastructure faults from
	// publish validation errors.
	PublishFormVersion(ctx context.Context, in PublishFormVersionInput) (*capability.FormSchema, error)
}

// PublishFormInput carries the fields for the create-or-update publish
// flow that POST /schema drives.
type PublishFormInput struct {
	Name             string
	Fields           []capability.FormField
	CoreRequirements *capability.CoreRequirements
	LegalBlocks      *[]capability.FormLegalBlock
	ActorID          int64
}

// PublishFormVersionInput carries the fields for the combined
// rename+publish flow that PUT /schema/{id} drives. Name is optional:
// nil or blank skips the rename step.
type PublishFormVersionInput struct {
	ID               int64
	Name             *string
	Fields           []capability.FormField
	CoreRequirements *capability.CoreRequirements
	LegalBlocks      *[]capability.FormLegalBlock
	ActorID          int64
}

// RenameStepError tags a failure originating from the rename step of a
// combined rename+publish (PublishFormVersion). It unwraps so errors.Is
// still matches the rename sentinels (name-exists, not-found); a caller
// maps a bare rename infrastructure fault (lock/read/exec) to a 5xx while
// publish/validation failures keep their 400 contract.
type RenameStepError struct{ err error }

func (e RenameStepError) Error() string { return e.err.Error() }
func (e RenameStepError) Unwrap() error { return e.err }

// FormSchemaOwner is the Enrollment capability used by schema publishing.
type FormSchemaOwner interface {
	ActiveSchema(context.Context) (*capability.FormSchema, error)
	Schema(context.Context, int64) (*capability.FormSchema, error)
	SchemaVersions(context.Context) ([]*capability.FormSchema, error)
	LockSchemaLineages(context.Context) error
	NextSchemaVersionForName(context.Context, string) (int, error)
	RenameSchema(context.Context, int64, string) (*capability.FormSchema, error)
	DeleteUnusedSchema(context.Context, int64) (string, error)
	PublishSchema(context.Context, capability.FormSchema) (*capability.FormSchema, error)
}

// FormSchemaServiceConfig is the dependency-injection bundle.
type FormSchemaServiceConfig struct {
	Owner FormSchemaOwner
	// Settings backs the Heimweg-Beschränkung publish guard (#2381). Nil
	// (some unit setups) skips the guard.
	Settings classCollectionResolver
	Logger   *slog.Logger
}

type formSchemaService struct {
	owner    FormSchemaOwner
	settings classCollectionResolver
	logger   *slog.Logger
}

// NewFormSchemaService builds the service. Nil logger falls back to
// slog.Default().
func NewFormSchemaService(cfg FormSchemaServiceConfig) FormSchemaService {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &transactionalFormSchemaService{FormSchemaService: &formSchemaService{
		owner:    cfg.Owner,
		settings: cfg.Settings,
		logger:   logger,
	}}
}

func (s *formSchemaService) GetActive(ctx context.Context) (*capability.FormSchema, error) {
	schema, err := s.owner.ActiveSchema(ctx)
	if err != nil {
		// FindActive returns a wrapped sql.ErrNoRows; surface as
		// ErrNoActiveSchema for cleaner caller error handling.
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoActiveSchema
		}
		return nil, err
	}
	return cloneSchema(schema), nil
}

func (s *formSchemaService) GetByID(ctx context.Context, id int64) (*capability.FormSchema, error) {
	if id <= 0 {
		return nil, fmt.Errorf("schema id must be positive")
	}
	schema, err := s.owner.Schema(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: %w", ErrFormSchemaNotFound, err)
		}
		return nil, err
	}
	return cloneSchema(schema), nil
}

func (s *formSchemaService) ListVersions(ctx context.Context) ([]*capability.FormSchema, error) {
	values, err := s.owner.SchemaVersions(ctx)
	if err != nil {
		return nil, err
	}
	var result []*capability.FormSchema
	for _, value := range values {
		result = append(result, cloneSchema(value))
	}
	return result, nil
}

// defaultSchemaName is the fallback name for legacy callers that don't
// supply one. Matches the backfill string used by migration 1.15.74 so
// older rows merge cleanly into the same logical schema.
const defaultSchemaName = "Standardformular"

func (s *formSchemaService) CreateSchema(ctx context.Context, name string, fields []capability.FormField, createdBy int64, coreRequirements ...capability.CoreRequirements) (*capability.FormSchema, error) {
	if name == "" {
		return nil, fmt.Errorf("schema name is required")
	}
	if err := s.owner.LockSchemaLineages(ctx); err != nil {
		return nil, err
	}
	// Refuse to overload an existing name. The admin should use
	// UpdateSchema to add a new version instead. The
	// "next version > 1" check is the lightweight uniqueness signal.
	existing, err := s.owner.NextSchemaVersionForName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("check existing name: %w", err)
	}
	if existing > 1 {
		return nil, fmt.Errorf("schema with name %q already exists; use UpdateSchema to add a new version", name)
	}
	return s.createOrVersion(ctx, name, fields, createdBy, firstCoreRequirements(coreRequirements), nil)
}

func (s *formSchemaService) CreateSchemaWithLegal(ctx context.Context, name string, fields []capability.FormField, createdBy int64, coreRequirements capability.CoreRequirements, legalBlocks []capability.FormLegalBlock) (*capability.FormSchema, error) {
	if name == "" {
		return nil, fmt.Errorf("schema name is required")
	}
	if err := s.owner.LockSchemaLineages(ctx); err != nil {
		return nil, err
	}
	existing, err := s.owner.NextSchemaVersionForName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("check existing name: %w", err)
	}
	if existing > 1 {
		return nil, fmt.Errorf("schema with name %q already exists; use UpdateSchema to add a new version", name)
	}
	return s.createOrVersion(ctx, name, fields, createdBy, coreRequirements, legalBlocks)
}

func (s *formSchemaService) UpdateSchema(ctx context.Context, id int64, fields []capability.FormField, updatedBy int64, coreRequirements ...capability.CoreRequirements) (*capability.FormSchema, error) {
	if id <= 0 {
		return nil, fmt.Errorf("schema id must be positive")
	}
	// Lock the lineage before reading its name: a concurrent rename must not
	// move the lineage between this read and the new version's insert, or the
	// new row is born under the stale name and splits the lineage.
	if err := s.owner.LockSchemaLineages(ctx); err != nil {
		return nil, err
	}
	source, err := s.owner.Schema(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("load source schema: %w", err)
	}
	nextCoreRequirements := source.CoreRequirements
	if len(coreRequirements) > 0 {
		nextCoreRequirements = firstCoreRequirements(coreRequirements)
	}
	return s.createOrVersion(ctx, source.Name, fields, updatedBy, nextCoreRequirements, source.LegalBlocks)
}

func (s *formSchemaService) UpdateSchemaWithLegal(ctx context.Context, id int64, fields []capability.FormField, updatedBy int64, coreRequirements *capability.CoreRequirements, legalBlocks *[]capability.FormLegalBlock) (*capability.FormSchema, error) {
	if id <= 0 {
		return nil, fmt.Errorf("schema id must be positive")
	}
	// Lock the lineage before reading its name (see UpdateSchema).
	if err := s.owner.LockSchemaLineages(ctx); err != nil {
		return nil, err
	}
	source, err := s.owner.Schema(ctx, id)
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

func (s *formSchemaService) RenameSchema(ctx context.Context, id int64, newName string) (*capability.FormSchema, error) {
	schema, err := s.owner.RenameSchema(ctx, id, newName)
	if err != nil {
		return nil, err
	}
	s.logger.Info("form schema renamed",
		slog.String("new_name", schema.Name),
		slog.Int64("schema_id", id))
	return cloneSchema(schema), nil
}

func (s *formSchemaService) DeleteSchema(ctx context.Context, id int64) error {
	name, err := s.owner.DeleteUnusedSchema(ctx, id)
	if err != nil {
		return err
	}
	s.logger.Info("form schema deleted",
		slog.String("name", name),
		slog.Int64("schema_id", id))
	return nil
}

func (s *formSchemaService) PublishForm(ctx context.Context, in PublishFormInput) (*capability.FormSchema, error) {
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

func (s *formSchemaService) createNamedSchema(ctx context.Context, name string, in PublishFormInput) (*capability.FormSchema, error) {
	if in.LegalBlocks != nil {
		return s.CreateSchemaWithLegal(ctx, name, in.Fields, in.ActorID, pointerCoreRequirements(in.CoreRequirements), *in.LegalBlocks)
	}
	return s.CreateSchema(ctx, name, in.Fields, in.ActorID, pointerCoreRequirements(in.CoreRequirements))
}

func (s *formSchemaService) updateSchemaFromInput(ctx context.Context, id int64, in PublishFormInput) (*capability.FormSchema, error) {
	if in.LegalBlocks != nil {
		return s.UpdateSchemaWithLegal(ctx, id, in.Fields, in.ActorID, in.CoreRequirements, in.LegalBlocks)
	}
	if in.CoreRequirements == nil {
		return s.UpdateSchema(ctx, id, in.Fields, in.ActorID)
	}
	return s.UpdateSchema(ctx, id, in.Fields, in.ActorID, *in.CoreRequirements)
}

func (s *formSchemaService) PublishFormVersion(ctx context.Context, in PublishFormVersionInput) (*capability.FormSchema, error) {
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
func pointerCoreRequirements(value *capability.CoreRequirements) capability.CoreRequirements {
	if value == nil {
		return capability.CoreRequirements{}
	}
	return *value
}

// createOrVersion is the shared internal: pick max(version)+1 for the
// name and insert a new active row. Sibling rows with the same name
// stay in place for historical submissions, but phases using any prior
// sibling version are advanced to the newly published row.
func (s *formSchemaService) createOrVersion(ctx context.Context, name string, fields []capability.FormField, createdBy int64, coreRequirements capability.CoreRequirements, legalBlocks []capability.FormLegalBlock) (*capability.FormSchema, error) {
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
	tmp := &capability.FormSchema{Name: name, Version: 1, CreatedBy: createdBy, Fields: fields, CoreRequirements: coreRequirements, LegalBlocks: legalBlocks}
	tmp.LegalBlocks = normalizedLegalBlocks
	if err := tmp.Validate(); err != nil {
		return nil, fmt.Errorf("invalid schema: %w", err)
	}

	if err := s.ensureSingleModeGradesCollectable(ctx, fields); err != nil {
		return nil, err
	}

	schema, err := s.owner.PublishSchema(ctx, capability.FormSchema{
		Name: tmp.Name, Fields: tmp.Fields, CoreRequirements: tmp.CoreRequirements,
		LegalBlocks: tmp.LegalBlocks, CreatedBy: tmp.CreatedBy,
	})
	if err != nil {
		return nil, err
	}

	s.logger.Info("form schema published",
		slog.String("name", schema.Name),
		slog.Int("version", schema.Version),
		slog.Int64("schema_id", schema.ID),
		slog.Int64("created_by", createdBy),
		slog.Int("field_count", len(fields)))

	return cloneSchema(schema), nil
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
func (s *formSchemaService) ensureSingleModeGradesCollectable(ctx context.Context, fields []capability.FormField) error {
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

func normalizeSchemaLegalDocumentURLs(ctx context.Context, blocks []capability.FormLegalBlock) ([]capability.FormLegalBlock, error) {
	if len(blocks) == 0 {
		return blocks, nil
	}

	tenantID := tenant.FromContext(ctx)
	normalized := make([]capability.FormLegalBlock, len(blocks))
	copy(normalized, blocks)
	for i := range normalized {
		block := &normalized[i]
		if block.DisplayMode != capability.LegalBlockDisplayModePDF {
			block.DocumentURL = ""
			continue
		}
		if strings.TrimSpace(block.DocumentURL) == "" {
			block.DocumentURL = ""
			continue
		}
		documentURL, ok := capability.NormalizeTenantLegalDocumentURL(block.DocumentURL, tenantID)
		if !ok {
			return nil, fmt.Errorf("invalid schema: legal block %q has an invalid PDF document URL", block.Key)
		}
		block.DocumentURL = documentURL
	}
	return normalized, nil
}

func firstCoreRequirements(values []capability.CoreRequirements) capability.CoreRequirements {
	if len(values) == 0 || values[0] == nil {
		return capability.CoreRequirements{}
	}
	return values[0]
}

// conditionValuesEqual compares a submitted answer against a condition
// value tolerant of the JSON round-trip (bool vs "true", number vs
// string), which is enough for boolean/select controlling fields.
func conditionValuesEqual(a, b any) bool {
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

func cloneSchema(value *capability.FormSchema) *capability.FormSchema {
	if value == nil {
		return nil
	}
	schema := *value
	return &schema
}
