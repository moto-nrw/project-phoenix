package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/uptrace/bun"
)

// StudentExtraFieldSchemas scans each version independently with the scalar
// row model. A malformed later version must not hide an earlier child's error.
func (r *Store) StudentExtraFieldSchemas(ctx context.Context, ids []int64) ([]enrollment.SchemaVersionResult, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return []enrollment.SchemaVersionResult{}, nil
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	query := db.NewSelect().Model((*formSchemaRow)(nil)).
		Where("form_schema.tenant_id = ?", tenantID).
		Where("form_schema.id IN (?)", bun.List(ids))
	rows, err := query.Rows(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to find form schema: %w", err)
	}
	// Iteration reports driver/close failures through rows.Err below.
	defer func() { _ = rows.Close() }()
	results := make([]enrollment.SchemaVersionResult, 0, len(ids))
	found := make(map[int64]bool, len(ids))
	for rows.Next() {
		var row formSchemaRow
		scanErr := query.DB().ScanRow(ctx, rows, &row)
		result := enrollment.SchemaVersionResult{ID: row.ID}
		if scanErr != nil {
			result.Err = fmt.Errorf("failed to find form schema: %w", scanErr)
		} else {
			result.Schema = row.value()
		}
		found[row.ID] = true
		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to find form schema: %w", err)
	}
	for _, id := range ids {
		if !found[id] {
			results = append(results, enrollment.SchemaVersionResult{ID: id,
				Err: fmt.Errorf("form schema %d not found: %w: %w", id, enrollment.ErrFormSchemaNotFound, sql.ErrNoRows)})
		}
	}
	return results, nil
}
