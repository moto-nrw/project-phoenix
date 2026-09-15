package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/uptrace/bun"
)

type formSchemaRow struct {
	bun.BaseModel    `bun:"table:enrollment.form_schemas,alias:form_schema"`
	ID               int64                       `bun:"id,pk,autoincrement"`
	TenantID         int64                       `bun:"tenant_id,notnull"`
	CreatedAt        time.Time                   `bun:"created_at"`
	UpdatedAt        time.Time                   `bun:"updated_at"`
	Name             string                      `bun:"name"`
	Version          int                         `bun:"version"`
	Fields           []enrollment.FormField      `bun:"fields,type:jsonb"`
	CoreRequirements enrollment.CoreRequirements `bun:"core_requirements,type:jsonb"`
	LegalBlocks      []enrollment.FormLegalBlock `bun:"legal_blocks,type:jsonb"`
	IsActive         bool                        `bun:"is_active"`
	CreatedBy        int64                       `bun:"created_by"`
}

func (r *Store) Schemas(ctx context.Context, ids []int64) ([]*enrollment.FormSchema, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return []*enrollment.FormSchema{}, nil
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	var rows []formSchemaRow
	err = db.NewSelect().Model(&rows).Where("form_schema.tenant_id = ?", tenantID).Where("form_schema.id IN (?)", bun.List(ids)).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list form schemas: %w", err)
	}
	result := make([]*enrollment.FormSchema, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.value())
	}
	return result, nil
}

func (r formSchemaRow) value() *enrollment.FormSchema {
	return &enrollment.FormSchema{
		ID: r.ID, TenantID: r.TenantID, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		Name: r.Name, Version: r.Version, Fields: r.Fields, CoreRequirements: r.CoreRequirements,
		LegalBlocks: r.LegalBlocks, IsActive: r.IsActive, CreatedBy: r.CreatedBy,
	}
}

func (r *Store) Schema(ctx context.Context, id int64) (*enrollment.FormSchema, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	var row formSchemaRow
	err = db.NewSelect().Model(&row).Where("form_schema.tenant_id = ?", tenantID).Where("form_schema.id = ?", id).Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("form schema %d not found: %w: %w", id, enrollment.ErrFormSchemaNotFound, sql.ErrNoRows)
		}
		return nil, fmt.Errorf("failed to find form schema: %w", err)
	}
	return row.value(), nil
}

func (r *Store) ActiveSchema(ctx context.Context) (*enrollment.FormSchema, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	var row formSchemaRow
	err = db.NewSelect().Model(&row).Where("form_schema.tenant_id = ?", tenantID).Where("form_schema.is_active = true").Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("no active form schema for tenant: %w", err)
		}
		return nil, fmt.Errorf("failed to find active form schema: %w", err)
	}
	return row.value(), nil
}

func (r *Store) SchemaVersions(ctx context.Context) ([]*enrollment.FormSchema, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	var rows []formSchemaRow
	err = db.NewSelect().Model(&rows).Where("form_schema.tenant_id = ?", tenantID).OrderExpr("form_schema.version DESC").Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list form schemas: %w", err)
	}
	var result []*enrollment.FormSchema
	for _, row := range rows {
		result = append(result, row.value())
	}
	return result, nil
}
