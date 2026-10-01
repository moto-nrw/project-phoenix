package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// defaultSchemaName is the fallback name for legacy callers that don't
// supply one. Matches the backfill string used by migration 1.15.74 so
// older rows merge cleanly into the same logical schema.
const defaultSchemaName = "Standardformular"

// FormSchemaDependencies bind schema publishing to its owners.
type FormSchemaDependencies struct {
	Records FormSchemaRecords
	// Settings backs the Heimweg-Beschränkung publish guard (#2381). Nil
	// (focused tests) skips the guard.
	Settings CollectionSettings
	Runtime  Runtime
	Logger   *slog.Logger
}

// FormSchemas publishes and reads form-schema versions. Its writes run in
// the caller's tenant transaction; TransactionalFormSchemas opens one for a
// standalone caller.
type FormSchemas struct {
	records  FormSchemaRecords
	settings CollectionSettings
	runtime  Runtime
	logger   *slog.Logger
}

// NewFormSchemas builds schema publishing. A nil logger falls back to
// slog.Default().
func NewFormSchemas(deps FormSchemaDependencies) *FormSchemas {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &FormSchemas{records: deps.Records, settings: deps.Settings, runtime: deps.Runtime, logger: logger}
}

func (s *FormSchemas) notFound(err error) bool {
	return s.runtime.NotFound != nil && s.runtime.NotFound(err)
}

// GetActive returns the tenant's active schema or ErrNoActiveSchema.
func (s *FormSchemas) GetActive(ctx context.Context) (*enrollment.FormSchema, error) {
	schema, err := s.records.ActiveSchema(ctx)
	if err != nil {
		// The owner reports a missing active schema as not-found; surface
		// it as ErrNoActiveSchema for cleaner caller error handling.
		if s.notFound(err) {
			return nil, enrollment.ErrNoActiveSchema
		}
		return nil, err
	}
	return enrollment.CopyFormSchema(schema), nil
}

// SchemaVersion returns one schema version.
func (s *FormSchemas) SchemaVersion(ctx context.Context, id int64) (*enrollment.FormSchema, error) {
	if id <= 0 {
		return nil, fmt.Errorf("schema id must be positive")
	}
	schema, err := s.records.Schema(ctx, id)
	if err != nil {
		if s.notFound(err) {
			return nil, fmt.Errorf("%w: %w", enrollment.ErrFormSchemaNotFound, err)
		}
		return nil, err
	}
	return enrollment.CopyFormSchema(schema), nil
}

// ListVersions returns every schema version of the tenant, newest first.
func (s *FormSchemas) ListVersions(ctx context.Context) ([]*enrollment.FormSchema, error) {
	values, err := s.records.SchemaVersions(ctx)
	if err != nil {
		return nil, err
	}
	var result []*enrollment.FormSchema
	for _, value := range values {
		result = append(result, enrollment.CopyFormSchema(value))
	}
	return result, nil
}

// lockNewSchemaName takes the lineage lock and refuses to overload an
// existing name. The admin should use UpdateSchema to add a new version
// instead. The "next version > 1" check is the lightweight uniqueness
// signal.
func (s *FormSchemas) lockNewSchemaName(ctx context.Context, name string) error {
	if name == "" {
		return fmt.Errorf("schema name is required")
	}
	if err := s.records.LockSchemaLineages(ctx); err != nil {
		return err
	}
	existing, err := s.records.NextSchemaVersionForName(ctx, name)
	if err != nil {
		return fmt.Errorf("check existing name: %w", err)
	}
	if existing > 1 {
		return fmt.Errorf("schema with name %q already exists; use UpdateSchema to add a new version", name)
	}
	return nil
}

// CreateSchema creates version 1 of a new named schema.
func (s *FormSchemas) CreateSchema(ctx context.Context, name string, fields []enrollment.FormField, createdBy int64, coreRequirements ...enrollment.CoreRequirements) (*enrollment.FormSchema, error) {
	if err := s.lockNewSchemaName(ctx, name); err != nil {
		return nil, err
	}
	return s.createOrVersion(ctx, name, fields, createdBy, firstCoreRequirements(coreRequirements), nil)
}

// CreateSchemaWithLegal creates version 1 of a new named schema with its
// legal blocks.
func (s *FormSchemas) CreateSchemaWithLegal(ctx context.Context, name string, fields []enrollment.FormField, createdBy int64, coreRequirements enrollment.CoreRequirements, legalBlocks []enrollment.FormLegalBlock) (*enrollment.FormSchema, error) {
	if err := s.lockNewSchemaName(ctx, name); err != nil {
		return nil, err
	}
	return s.createOrVersion(ctx, name, fields, createdBy, coreRequirements, legalBlocks)
}

// lockedSource takes the lineage lock before reading the source's name: a
// concurrent rename must not move the lineage between this read and the new
// version's insert, or the new row is born under the stale name and splits
// the lineage.
func (s *FormSchemas) lockedSource(ctx context.Context, id int64) (*enrollment.FormSchema, error) {
	if id <= 0 {
		return nil, fmt.Errorf("schema id must be positive")
	}
	if err := s.records.LockSchemaLineages(ctx); err != nil {
		return nil, err
	}
	source, err := s.records.Schema(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("load source schema: %w", err)
	}
	return source, nil
}

// UpdateSchema publishes a new version of the schema selected by id.
func (s *FormSchemas) UpdateSchema(ctx context.Context, id int64, fields []enrollment.FormField, updatedBy int64, coreRequirements ...enrollment.CoreRequirements) (*enrollment.FormSchema, error) {
	source, err := s.lockedSource(ctx, id)
	if err != nil {
		return nil, err
	}
	nextCoreRequirements := source.CoreRequirements
	if len(coreRequirements) > 0 {
		nextCoreRequirements = firstCoreRequirements(coreRequirements)
	}
	return s.createOrVersion(ctx, source.Name, fields, updatedBy, nextCoreRequirements, source.LegalBlocks)
}

// UpdateSchemaWithLegal publishes a new version and replaces the core
// requirements and legal blocks that are given.
func (s *FormSchemas) UpdateSchemaWithLegal(ctx context.Context, id int64, fields []enrollment.FormField, updatedBy int64, coreRequirements *enrollment.CoreRequirements, legalBlocks *[]enrollment.FormLegalBlock) (*enrollment.FormSchema, error) {
	source, err := s.lockedSource(ctx, id)
	if err != nil {
		return nil, err
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

// RenameSchema renames the whole lineage of the schema selected by id.
func (s *FormSchemas) RenameSchema(ctx context.Context, id int64, newName string) (*enrollment.FormSchema, error) {
	schema, err := s.records.RenameSchema(ctx, id, newName)
	if err != nil {
		return nil, err
	}
	s.logger.Info("form schema renamed",
		slog.String("new_name", schema.Name),
		slog.Int64("schema_id", id))
	return enrollment.CopyFormSchema(schema), nil
}

// DeleteSchema removes every version of an unused schema lineage.
func (s *FormSchemas) DeleteSchema(ctx context.Context, id int64) error {
	name, err := s.records.DeleteUnusedSchema(ctx, id)
	if err != nil {
		return err
	}
	s.logger.Info("form schema deleted",
		slog.String("name", name),
		slog.Int64("schema_id", id))
	return nil
}

// PublishForm creates or updates a form schema (POST /schema).
func (s *FormSchemas) PublishForm(ctx context.Context, in enrollment.PublishFormInput) (*enrollment.FormSchema, error) {
	if in.Name != "" {
		return s.createNamedSchema(ctx, in.Name, in)
	}

	active, err := s.GetActive(ctx)
	if err == nil {
		return s.updateSchemaFromInput(ctx, active.ID, in)
	}
	if !errors.Is(err, enrollment.ErrNoActiveSchema) {
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

func (s *FormSchemas) createNamedSchema(ctx context.Context, name string, in enrollment.PublishFormInput) (*enrollment.FormSchema, error) {
	if in.LegalBlocks != nil {
		return s.CreateSchemaWithLegal(ctx, name, in.Fields, in.ActorID, pointerCoreRequirements(in.CoreRequirements), *in.LegalBlocks)
	}
	return s.CreateSchema(ctx, name, in.Fields, in.ActorID, pointerCoreRequirements(in.CoreRequirements))
}

func (s *FormSchemas) updateSchemaFromInput(ctx context.Context, id int64, in enrollment.PublishFormInput) (*enrollment.FormSchema, error) {
	return s.updateSchemaVersion(ctx, id, in.Fields, in.ActorID, in.CoreRequirements, in.LegalBlocks)
}

func (s *FormSchemas) updateSchemaVersion(ctx context.Context, id int64, fields []enrollment.FormField, actorID int64, coreRequirements *enrollment.CoreRequirements, legalBlocks *[]enrollment.FormLegalBlock) (*enrollment.FormSchema, error) {
	if legalBlocks != nil {
		return s.UpdateSchemaWithLegal(ctx, id, fields, actorID, coreRequirements, legalBlocks)
	}
	if coreRequirements == nil {
		return s.UpdateSchema(ctx, id, fields, actorID)
	}
	return s.UpdateSchema(ctx, id, fields, actorID, *coreRequirements)
}

// PublishFormVersion publishes a new version of an existing schema
// (PUT /schema/{id}), renaming the lineage first when a name is given.
func (s *FormSchemas) PublishFormVersion(ctx context.Context, in enrollment.PublishFormVersionInput) (*enrollment.FormSchema, error) {
	// Combined "rename + edit" save: rename the lineage first, in the same
	// transaction as the publish below, so a failed publish rolls the rename
	// back — no partial "renamed but content unchanged" state. RenameSchema
	// is a no-op when the name is unchanged; a blank name is ignored (the
	// dedicated PATCH route owns blank-name rejection).
	if in.Name != nil && strings.TrimSpace(*in.Name) != "" {
		if _, renameErr := s.RenameSchema(ctx, in.ID, *in.Name); renameErr != nil {
			return nil, enrollment.NewRenameStepError(renameErr)
		}
	}
	return s.updateSchemaVersion(ctx, in.ID, in.Fields, in.ActorID, in.CoreRequirements, in.LegalBlocks)
}

// pointerCoreRequirements dereferences an optional CoreRequirements,
// returning an empty (non-nil) matrix when the pointer is nil.
func pointerCoreRequirements(value *enrollment.CoreRequirements) enrollment.CoreRequirements {
	if value == nil {
		return enrollment.CoreRequirements{}
	}
	return *value
}

// createOrVersion is the shared internal: pick max(version)+1 for the
// name and insert a new active row. Sibling rows with the same name
// stay in place for historical submissions, but phases using any prior
// sibling version are advanced to the newly published row.
func (s *FormSchemas) createOrVersion(ctx context.Context, name string, fields []enrollment.FormField, createdBy int64, coreRequirements enrollment.CoreRequirements, legalBlocks []enrollment.FormLegalBlock) (*enrollment.FormSchema, error) {
	if createdBy <= 0 {
		return nil, fmt.Errorf("createdBy is required")
	}
	if name == "" {
		name = defaultSchemaName
	}

	normalizedLegalBlocks, err := s.normalizeSchemaLegalDocumentURLs(ctx, legalBlocks)
	if err != nil {
		return nil, err
	}

	// Validate fields up front so we don't write a half-correct row.
	tmp := &enrollment.FormSchema{Name: name, Version: 1, CreatedBy: createdBy, Fields: fields, CoreRequirements: coreRequirements, LegalBlocks: normalizedLegalBlocks}
	if err := tmp.Validate(); err != nil {
		return nil, fmt.Errorf("invalid schema: %w", err)
	}

	if err := s.ensureSingleModeGradesCollectable(ctx, fields); err != nil {
		return nil, err
	}

	schema, err := s.records.PublishSchema(ctx, enrollment.FormSchema{
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

	return enrollment.CopyFormSchema(schema), nil
}

// ensureSingleModeGradesCollectable rejects publishing a schema whose
// Heimweg-Beschränkung (single_mode_grades, #2381) the school cannot apply:
// the rule keys on each child's declared target grade level, which the form
// only collects while enrollment.collect_grade_level is on. With collection
// off the rule would silently never restrict anyone, so the publish fails
// loudly and the admin enables grade collection (or clears the rule) first.
// Mirrors the phase eligibility guard, including the per-tenant lock against
// a concurrent settings write (#1663); the settings side does not check
// schemas symmetrically — disabling grade collection later just makes the
// rule inert (children without a grade fall back to multi-select), which is
// the documented no-rule behaviour, not a bypass.
func (s *FormSchemas) ensureSingleModeGradesCollectable(ctx context.Context, fields []enrollment.FormField) error {
	if s.settings == nil || !restrictsSingleModeGrades(fields) {
		return nil
	}
	if err := s.settings.LockClassCollectionPair(ctx); err != nil {
		return fmt.Errorf("lock class-collection pair: %w", err)
	}
	collectGrade, err := s.settings.CollectGradeLevel(ctx)
	if err != nil {
		return fmt.Errorf("resolve enrollment.collect_grade_level: %w", err)
	}
	if !collectGrade {
		return fmt.Errorf("invalid schema: single_mode_grades requires the grade-level collection setting (Klassenstufen-Abfrage) to be active")
	}
	return nil
}

func restrictsSingleModeGrades(fields []enrollment.FormField) bool {
	for i := range fields {
		if len(fields[i].SingleModeGrades) > 0 {
			return true
		}
	}
	return false
}

func (s *FormSchemas) normalizeSchemaLegalDocumentURLs(ctx context.Context, blocks []enrollment.FormLegalBlock) ([]enrollment.FormLegalBlock, error) {
	if len(blocks) == 0 {
		return blocks, nil
	}

	var tenantID int64
	if s.runtime.TenantID != nil {
		tenantID = s.runtime.TenantID(ctx)
	}
	normalized := make([]enrollment.FormLegalBlock, len(blocks))
	copy(normalized, blocks)
	for i := range normalized {
		block := &normalized[i]
		if block.DisplayMode != enrollment.LegalBlockDisplayModePDF || strings.TrimSpace(block.DocumentURL) == "" {
			block.DocumentURL = ""
			continue
		}
		documentURL, ok := enrollment.NormalizeTenantLegalDocumentURL(block.DocumentURL, tenantID)
		if !ok {
			return nil, fmt.Errorf("invalid schema: legal block %q has an invalid PDF document URL", block.Key)
		}
		block.DocumentURL = documentURL
	}
	return normalized, nil
}

func firstCoreRequirements(values []enrollment.CoreRequirements) enrollment.CoreRequirements {
	if len(values) == 0 || values[0] == nil {
		return enrollment.CoreRequirements{}
	}
	return values[0]
}

// TransactionalFormSchemas keeps each lineage mutation and its phase
// repointing in the caller transaction, or opens one for standalone callers.
type TransactionalFormSchemas struct {
	*FormSchemas
	transactions Transactions
}

// NewTransactionalFormSchemas wraps schema publishing so every write runs in
// a tenant transaction.
func NewTransactionalFormSchemas(schemas *FormSchemas, transactions Transactions) *TransactionalFormSchemas {
	return &TransactionalFormSchemas{FormSchemas: schemas, transactions: transactions}
}

func runSchemaWrite(ctx context.Context, transactions Transactions, write func(context.Context) (*enrollment.FormSchema, error)) (*enrollment.FormSchema, error) {
	var result *enrollment.FormSchema
	err := transactions.RunInTx(ctx, func(txCtx context.Context) error {
		var operationErr error
		result, operationErr = write(txCtx)
		return operationErr
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// CreateSchema runs FormSchemas.CreateSchema in a tenant transaction.
func (s *TransactionalFormSchemas) CreateSchema(ctx context.Context, name string, fields []enrollment.FormField, createdBy int64, coreRequirements ...enrollment.CoreRequirements) (*enrollment.FormSchema, error) {
	return runSchemaWrite(ctx, s.transactions, func(txCtx context.Context) (*enrollment.FormSchema, error) {
		return s.FormSchemas.CreateSchema(txCtx, name, fields, createdBy, coreRequirements...)
	})
}

// CreateSchemaWithLegal runs FormSchemas.CreateSchemaWithLegal in a tenant
// transaction.
func (s *TransactionalFormSchemas) CreateSchemaWithLegal(ctx context.Context, name string, fields []enrollment.FormField, createdBy int64, coreRequirements enrollment.CoreRequirements, legalBlocks []enrollment.FormLegalBlock) (*enrollment.FormSchema, error) {
	return runSchemaWrite(ctx, s.transactions, func(txCtx context.Context) (*enrollment.FormSchema, error) {
		return s.FormSchemas.CreateSchemaWithLegal(txCtx, name, fields, createdBy, coreRequirements, legalBlocks)
	})
}

// UpdateSchema runs FormSchemas.UpdateSchema in a tenant transaction.
func (s *TransactionalFormSchemas) UpdateSchema(ctx context.Context, id int64, fields []enrollment.FormField, updatedBy int64, coreRequirements ...enrollment.CoreRequirements) (*enrollment.FormSchema, error) {
	return runSchemaWrite(ctx, s.transactions, func(txCtx context.Context) (*enrollment.FormSchema, error) {
		return s.FormSchemas.UpdateSchema(txCtx, id, fields, updatedBy, coreRequirements...)
	})
}

// UpdateSchemaWithLegal runs FormSchemas.UpdateSchemaWithLegal in a tenant
// transaction.
func (s *TransactionalFormSchemas) UpdateSchemaWithLegal(ctx context.Context, id int64, fields []enrollment.FormField, updatedBy int64, coreRequirements *enrollment.CoreRequirements, legalBlocks *[]enrollment.FormLegalBlock) (*enrollment.FormSchema, error) {
	return runSchemaWrite(ctx, s.transactions, func(txCtx context.Context) (*enrollment.FormSchema, error) {
		return s.FormSchemas.UpdateSchemaWithLegal(txCtx, id, fields, updatedBy, coreRequirements, legalBlocks)
	})
}

// RenameSchema runs FormSchemas.RenameSchema in a tenant transaction.
func (s *TransactionalFormSchemas) RenameSchema(ctx context.Context, id int64, newName string) (*enrollment.FormSchema, error) {
	return runSchemaWrite(ctx, s.transactions, func(txCtx context.Context) (*enrollment.FormSchema, error) {
		return s.FormSchemas.RenameSchema(txCtx, id, newName)
	})
}

// DeleteSchema runs FormSchemas.DeleteSchema in a tenant transaction.
func (s *TransactionalFormSchemas) DeleteSchema(ctx context.Context, id int64) error {
	return s.transactions.RunInTx(ctx, func(txCtx context.Context) error { return s.FormSchemas.DeleteSchema(txCtx, id) })
}

// PublishForm runs FormSchemas.PublishForm in a tenant transaction.
func (s *TransactionalFormSchemas) PublishForm(ctx context.Context, in enrollment.PublishFormInput) (*enrollment.FormSchema, error) {
	return runSchemaWrite(ctx, s.transactions, func(txCtx context.Context) (*enrollment.FormSchema, error) {
		return s.FormSchemas.PublishForm(txCtx, in)
	})
}

// PublishFormVersion runs FormSchemas.PublishFormVersion in a tenant
// transaction.
func (s *TransactionalFormSchemas) PublishFormVersion(ctx context.Context, in enrollment.PublishFormVersionInput) (*enrollment.FormSchema, error) {
	return runSchemaWrite(ctx, s.transactions, func(txCtx context.Context) (*enrollment.FormSchema, error) {
		return s.FormSchemas.PublishFormVersion(txCtx, in)
	})
}
