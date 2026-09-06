package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/uptrace/bun/driver/pgdriver"
)

func (r *Store) InsertSchemaVersion(ctx context.Context, schema *enrollment.FormSchema) error {
	if err := schema.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return err
	}
	row := formSchemaRow{
		TenantID: tenantID, Name: schema.Name, Version: schema.Version, Fields: schema.Fields,
		CoreRequirements: schema.CoreRequirements, LegalBlocks: schema.LegalBlocks,
		IsActive: schema.IsActive, CreatedBy: schema.CreatedBy,
	}
	_, err = db.NewInsert().Model(&row).
		Value("created_at", "NOW()").Value("updated_at", "NOW()").
		Returning("*").Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create form schema: %w", err)
	}
	*schema = *row.value()
	return nil
}

func (r *Store) NextSchemaVersion(ctx context.Context) (int, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return 0, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return 0, err
	}
	var maxVersion int
	query := db.NewSelect().
		ColumnExpr(`COALESCE(MAX(version), 0)`).
		TableExpr(`enrollment.form_schemas`)

	query = query.Where("tenant_id = ?", tenantID)
	err = query.Scan(ctx, &maxVersion)
	if err != nil {
		return 0, fmt.Errorf("failed to read max version: %w", err)
	}
	return maxVersion + 1, nil
}

// NextVersionForName returns max(version)+1 for rows with the given
// name within the tenant in context. Returns 1 when no row exists yet
// with that name — i.e. first version of a freshly created schema.
func (r *Store) NextSchemaVersionForName(ctx context.Context, name string) (int, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return 0, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return 0, err
	}
	var maxVersion int
	query := db.NewSelect().
		ColumnExpr(`COALESCE(MAX(version), 0)`).
		TableExpr(`enrollment.form_schemas`).
		Where(`name = ?`, name)

	query = query.Where("tenant_id = ?", tenantID)
	err = query.Scan(ctx, &maxVersion)
	if err != nil {
		return 0, fmt.Errorf("failed to read max version for name %q: %w", name, err)
	}
	return maxVersion + 1, nil
}

// ExistsByName reports whether any version row already carries name for
// the tenant in context. RenameSchema uses it to reject a rename onto an
// existing logical schema before touching the (tenant_id, name, version)
// unique index.
func (r *Store) SchemaNameExists(ctx context.Context, name string) (bool, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return false, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return false, err
	}
	query := db.NewSelect().
		Model((*formSchemaRow)(nil)).
		ModelTableExpr(`enrollment.form_schemas AS "form_schema"`).
		Where(`"form_schema".name = ?`, name)

	query = query.Where(`"form_schema".tenant_id = ?`, tenantID)
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
func (r *Store) DeactivateSchemas(ctx context.Context) error {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return err
	}
	query := db.NewUpdate().
		Model((*formSchemaRow)(nil)).
		ModelTableExpr(`enrollment.form_schemas AS "form_schema"`).
		Set("is_active = false").
		Where(`"form_schema".is_active = true`)

	query = query.Where(`"form_schema".tenant_id = ?`, tenantID)
	_, err = query.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to deactivate previous schemas: %w", err)
	}
	return nil
}

// UpdateActiveFlag toggles is_active on a single schema row.
func (r *Store) SetSchemaActive(ctx context.Context, id int64, isActive bool) error {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return err
	}
	query := db.NewUpdate().
		Model((*formSchemaRow)(nil)).
		ModelTableExpr(`enrollment.form_schemas AS "form_schema"`).
		Set("is_active = ?", isActive).
		Where(`"form_schema".id = ?`, id)

	query = query.Where(`"form_schema".tenant_id = ?`, tenantID)
	res, err := query.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update is_active: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
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
func (r *Store) RenameSchemaLineage(ctx context.Context, oldName, newName string) error {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return err
	}
	query := db.NewUpdate().
		Model((*formSchemaRow)(nil)).
		ModelTableExpr(`enrollment.form_schemas AS "form_schema"`).
		Set("name = ?", newName).
		Set("updated_at = NOW()").
		Where(`"form_schema".name = ?`, oldName)

	query = query.Where(`"form_schema".tenant_id = ?`, tenantID)
	res, err := query.Exec(ctx)
	if err != nil {
		var postgresError pgdriver.Error
		if errors.As(err, &postgresError) && postgresError.IntegrityViolation() && postgresError.Field('n') == "uq_form_schemas_tenant_name_version" {
			return enrollment.ErrFormSchemaNameExists
		}
		return fmt.Errorf("failed to rename form schema %q to %q: %w", oldName, newName, err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
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
func (r *Store) LockSchemaLineages(ctx context.Context) error {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("enrollment.form_schema_lineage:%d", tenantID)
	if _, err := db.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", key); err != nil {
		return fmt.Errorf("failed to lock form schema lineages: %w", err)
	}
	return nil
}

// DeleteByName removes every version of a logical schema. Callers must
// check phase and request references first.
func (r *Store) DeleteSchemaLineage(ctx context.Context, name string) error {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return err
	}
	query := db.NewDelete().
		Model((*formSchemaRow)(nil)).
		ModelTableExpr(`enrollment.form_schemas AS "form_schema"`).
		Where(`"form_schema".name = ?`, name)

	query = query.Where(`"form_schema".tenant_id = ?`, tenantID)
	res, err := query.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete form schema %q: %w", name, err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("form schema %q not found", name)
	}
	return nil
}

// HasLegalDocumentReference reports whether any saved form-schema legal block
// still links to a stored AGB document. Callers pass both accepted URL shapes:
// the stored upload URL and the public route URL rendered into legal_blocks.
func (r *Store) SchemaReferencesLegalDocument(ctx context.Context, storedURL, publicURL string) (bool, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return false, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return false, err
	}

	var referenced bool
	err = db.NewRaw(`
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
