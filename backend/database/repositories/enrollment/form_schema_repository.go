package enrollment

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/tenant"
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
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(schema).
		ModelTableExpr(formSchemaTableExpr).
		Where(`"form_schema".id = ?`, id)
	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where(`"form_schema".tenant_id = ?`, tenantID)
	}
	err := query.Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("form schema %d not found: %w", id, sql.ErrNoRows)
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
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(schema).
		ModelTableExpr(formSchemaTableExpr).
		Where(`"form_schema".is_active = true`)
	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where(`"form_schema".tenant_id = ?`, tenantID)
	}
	err := query.Scan(ctx)
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
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&schemas).
		ModelTableExpr(formSchemaTableExpr).
		OrderExpr(`"form_schema".version DESC`)
	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where(`"form_schema".tenant_id = ?`, tenantID)
	}
	err := query.Scan(ctx)
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
	query := base.GetDB(ctx, r.db).NewSelect().
		ColumnExpr(`COALESCE(MAX(version), 0)`).
		TableExpr(`enrollment.form_schemas`)
	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}
	err := query.Scan(ctx, &maxVersion)
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
	query := base.GetDB(ctx, r.db).NewSelect().
		ColumnExpr(`COALESCE(MAX(version), 0)`).
		TableExpr(`enrollment.form_schemas`).
		Where(`name = ?`, name)
	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}
	err := query.Scan(ctx, &maxVersion)
	if err != nil {
		return 0, fmt.Errorf("failed to read max version for name %q: %w", name, err)
	}
	return maxVersion + 1, nil
}

// ExistsByName reports whether any version row already carries name for
// the tenant in context. RenameSchema uses it to reject a rename onto an
// existing logical schema before touching the (tenant_id, name, version)
// unique index.
func (r *FormSchemaRepository) ExistsByName(ctx context.Context, name string) (bool, error) {
	query := base.GetDB(ctx, r.db).NewSelect().
		Model((*enrollment.FormSchema)(nil)).
		ModelTableExpr(formSchemaTableExpr).
		Where(`"form_schema".name = ?`, name)
	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where(`"form_schema".tenant_id = ?`, tenantID)
	}
	exists, err := query.Exists(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to check form schema name %q: %w", name, err)
	}
	return exists, nil
}

// DeactivatePrevious flips is_active=false on every schema for the tenant
// in context. Used by the schema service before activating a new version.
// The partial unique index uq_form_schemas_one_active_per_tenant enforces
// the at-most-one-active invariant; running this first prevents a unique
// violation when the new version is inserted with is_active=true.
func (r *FormSchemaRepository) DeactivatePrevious(ctx context.Context) error {
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*enrollment.FormSchema)(nil)).
		ModelTableExpr(formSchemaTableExpr).
		Set("is_active = false").
		Where(`"form_schema".is_active = true`)
	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where(`"form_schema".tenant_id = ?`, tenantID)
	}
	_, err := query.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to deactivate previous schemas: %w", err)
	}
	return nil
}

// UpdateActiveFlag toggles is_active on a single schema row.
func (r *FormSchemaRepository) UpdateActiveFlag(ctx context.Context, id int64, isActive bool) error {
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*enrollment.FormSchema)(nil)).
		ModelTableExpr(formSchemaTableExpr).
		Set("is_active = ?", isActive).
		Where(`"form_schema".id = ?`, id)
	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where(`"form_schema".tenant_id = ?`, tenantID)
	}
	res, err := query.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update is_active: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("form schema %d not found", id)
	}
	return nil
}

// RenameByName updates the name on every version row of a logical
// schema for the tenant in context, keeping the whole version lineage
// under one shared name. The service guarantees newName doesn't collide
// with another lineage before calling this, so the
// (tenant_id, name, version) unique index is never violated.
func (r *FormSchemaRepository) RenameByName(ctx context.Context, oldName, newName string) error {
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*enrollment.FormSchema)(nil)).
		ModelTableExpr(formSchemaTableExpr).
		Set("name = ?", newName).
		Set("updated_at = NOW()").
		Where(`"form_schema".name = ?`, oldName)
	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where(`"form_schema".tenant_id = ?`, tenantID)
	}
	res, err := query.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to rename form schema %q to %q: %w", oldName, newName, err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("form schema %q not found", oldName)
	}
	return nil
}

// LockLineages takes the per-tenant, transaction-scoped advisory lock that
// serializes form-schema lineage mutations (publish, rename, delete) against
// one another. Without it a rename and a concurrent version-publish can split
// a lineage: the publish reads the pre-rename name and inserts a new row under
// it while the rename moves the existing rows to the new name, leaving the
// lineage's name no longer shared (migration 1.15.74's whole-lineage
// invariant). Callers must already be inside the request's tenant transaction
// (runInTenantTx / WithTenantTx); the lock releases automatically at
// COMMIT/ROLLBACK. The key is namespaced per tenant so it never collides with
// other advisory locks.
func (r *FormSchemaRepository) LockLineages(ctx context.Context) error {
	key := fmt.Sprintf("enrollment.form_schema_lineage:%d", tenant.FromContext(ctx))
	if err := base.AcquireXactLock(ctx, r.db, key); err != nil {
		return fmt.Errorf("failed to lock form schema lineages: %w", err)
	}
	return nil
}

// DeleteByName removes every version of a logical schema. Callers must
// check phase and request references first.
func (r *FormSchemaRepository) DeleteByName(ctx context.Context, name string) error {
	query := base.GetDB(ctx, r.db).NewDelete().
		Model((*enrollment.FormSchema)(nil)).
		ModelTableExpr(formSchemaTableExpr).
		Where(`"form_schema".name = ?`, name)
	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where(`"form_schema".tenant_id = ?`, tenantID)
	}
	res, err := query.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete form schema %q: %w", name, err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("form schema %q not found", name)
	}
	return nil
}

// HasLegalDocumentReference reports whether any saved form-schema legal block
// still links to a stored AGB document. Callers pass both accepted URL shapes:
// the stored upload URL and the public route URL rendered into legal_blocks.
func (r *FormSchemaRepository) HasLegalDocumentReference(ctx context.Context, storedURL, publicURL string) (bool, error) {
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return false, errors.New("tenant context is required")
	}

	var referenced bool
	err := base.GetDB(ctx, r.db).NewRaw(`
		SELECT EXISTS (
			SELECT 1
			FROM enrollment.form_schemas AS "form_schema"
			CROSS JOIN LATERAL jsonb_array_elements(
				CASE
					WHEN jsonb_typeof("form_schema".legal_blocks) = 'array'
					THEN "form_schema".legal_blocks
					ELSE '[]'::jsonb
				END
			) AS block(elem)
			WHERE "form_schema".tenant_id = ?
				AND (
					strpos(COALESCE(block.elem->>'text', ''), ?) > 0
					OR strpos(COALESCE(block.elem->>'text', ''), ?) > 0
					OR COALESCE(block.elem->>'document_url', '') = ?
					OR COALESCE(block.elem->>'document_url', '') = ?
				)
		)
	`, tenantID, storedURL, publicURL, storedURL, publicURL).Scan(ctx, &referenced)
	if err != nil {
		return false, fmt.Errorf("check legal document references: %w", err)
	}
	return referenced, nil
}
