package enrollment

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/uptrace/bun"
)

const formSchemaTableExpr = `enrollment.form_schemas AS "form_schema"`

// FormSchemaRepository is the bun-backed implementation of
// enrollment.FormSchemaRepository.
type FormSchemaRepository struct {
	db *bun.DB
}

func NewFormSchemaRepository(db *bun.DB) enrollment.FormSchemaRepository {
	return &FormSchemaRepository{db: db}
}

// Create inserts a new schema version.
func (r *FormSchemaRepository) Create(ctx context.Context, schema *enrollment.FormSchema) error {
	if err := schema.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	base.EnsureTenantID(ctx, schema)

	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(schema).
		ModelTableExpr(formSchemaTableExpr).
		Returning("*").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create form schema: %w", err)
	}
	return nil
}

// FindByID retrieves a schema by primary key.
func (r *FormSchemaRepository) FindByID(ctx context.Context, id int64) (*enrollment.FormSchema, error) {
	schema := new(enrollment.FormSchema)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(schema).
		ModelTableExpr(formSchemaTableExpr).
		Where(`"form_schema".id = ?`, id).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("form schema %d not found", id)
		}
		return nil, fmt.Errorf("failed to find form schema: %w", err)
	}
	return schema, nil
}

// FindActive returns the currently-active form schema for the tenant
// in context. Returns sql.ErrNoRows (wrapped) when none exists so
// callers can branch on errors.Is(err, sql.ErrNoRows). The service
// layer translates that to its own ErrNoActiveSchema sentinel.
func (r *FormSchemaRepository) FindActive(ctx context.Context) (*enrollment.FormSchema, error) {
	schema := new(enrollment.FormSchema)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(schema).
		ModelTableExpr(formSchemaTableExpr).
		Where(`"form_schema".is_active = true`).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("no active form schema for tenant: %w", err)
		}
		return nil, fmt.Errorf("failed to find active form schema: %w", err)
	}
	return schema, nil
}

// ListByTenant returns all schema versions for the tenant in context,
// ordered newest-version-first.
func (r *FormSchemaRepository) ListByTenant(ctx context.Context) ([]*enrollment.FormSchema, error) {
	var schemas []*enrollment.FormSchema
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&schemas).
		ModelTableExpr(formSchemaTableExpr).
		OrderExpr(`"form_schema".version DESC`).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list form schemas: %w", err)
	}
	return schemas, nil
}

// NextVersion returns max(version)+1 for the tenant in context, or 1 if
// no schemas exist yet. Counts across all names — used by the legacy
// single-schema publish path. New code should prefer NextVersionForName.
func (r *FormSchemaRepository) NextVersion(ctx context.Context) (int, error) {
	var maxVersion int
	err := base.GetDB(ctx, r.db).NewSelect().
		ColumnExpr(`COALESCE(MAX(version), 0)`).
		TableExpr(`enrollment.form_schemas`).
		Scan(ctx, &maxVersion)
	if err != nil {
		return 0, fmt.Errorf("failed to read max version: %w", err)
	}
	return maxVersion + 1, nil
}

// NextVersionForName returns max(version)+1 for rows with the given
// name within the tenant in context. Returns 1 when no row exists yet
// with that name — i.e. first version of a freshly created schema.
func (r *FormSchemaRepository) NextVersionForName(ctx context.Context, name string) (int, error) {
	var maxVersion int
	err := base.GetDB(ctx, r.db).NewSelect().
		ColumnExpr(`COALESCE(MAX(version), 0)`).
		TableExpr(`enrollment.form_schemas`).
		Where(`name = ?`, name).
		Scan(ctx, &maxVersion)
	if err != nil {
		return 0, fmt.Errorf("failed to read max version for name %q: %w", name, err)
	}
	return maxVersion + 1, nil
}

// DeactivatePrevious flips is_active=false on every schema for the tenant
// in context. Used by the schema service before activating a new version.
// The partial unique index uq_form_schemas_one_active_per_tenant enforces
// the at-most-one-active invariant; running this first prevents a unique
// violation when the new version is inserted with is_active=true.
func (r *FormSchemaRepository) DeactivatePrevious(ctx context.Context) error {
	_, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*enrollment.FormSchema)(nil)).
		ModelTableExpr(formSchemaTableExpr).
		Set("is_active = false").
		Where(`"form_schema".is_active = true`).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to deactivate previous schemas: %w", err)
	}
	return nil
}

// UpdateActiveFlag toggles is_active on a single schema row.
func (r *FormSchemaRepository) UpdateActiveFlag(ctx context.Context, id int64, isActive bool) error {
	res, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*enrollment.FormSchema)(nil)).
		ModelTableExpr(formSchemaTableExpr).
		Set("is_active = ?", isActive).
		Where(`"form_schema".id = ?`, id).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update is_active: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("form schema %d not found", id)
	}
	return nil
}

// DeleteByName removes every version of a logical schema. Callers must
// check phase and request references first.
func (r *FormSchemaRepository) DeleteByName(ctx context.Context, name string) error {
	res, err := base.GetDB(ctx, r.db).NewDelete().
		Model((*enrollment.FormSchema)(nil)).
		ModelTableExpr(formSchemaTableExpr).
		Where(`"form_schema".name = ?`, name).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete form schema %q: %w", name, err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("form schema %q not found", name)
	}
	return nil
}
