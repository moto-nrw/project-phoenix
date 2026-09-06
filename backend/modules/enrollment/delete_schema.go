package enrollment

import (
	"context"
	"errors"
	"fmt"
)

var (
	ErrFormSchemaNotFound    = errors.New("form schema not found")
	ErrFormSchemaHasPhases   = errors.New("form schema has enrollment phases")
	ErrFormSchemaHasRequests = errors.New("form schema has enrollment requests")
)

// DeleteUnusedSchema deletes a complete unused lineage under the same lock and
// transaction used by publication. Historical submissions retain their schemas.
func (m *Module) DeleteUnusedSchema(ctx context.Context, id int64) (string, error) {
	if id <= 0 {
		return "", ErrFormSchemaNotFound
	}
	var name string
	err := m.transactions.RunInTx(ctx, func(txCtx context.Context) error {
		if err := m.engine.LockSchemaLineages(txCtx); err != nil {
			return err
		}
		source, err := m.engine.Schema(txCtx, id)
		if errors.Is(err, ErrFormSchemaNotFound) {
			return ErrFormSchemaNotFound
		}
		if err != nil {
			return fmt.Errorf("load source schema: %w", err)
		}
		schemas, err := m.engine.SchemaVersions(txCtx)
		if err != nil {
			return fmt.Errorf("list schema versions: %w", err)
		}
		ids := make([]int64, 0)
		for _, schema := range schemas {
			if schema.Name == source.Name {
				ids = append(ids, schema.ID)
			}
		}
		if len(ids) == 0 {
			return ErrFormSchemaNotFound
		}
		count, err := m.engine.CountPhaseSchemaReferences(txCtx, ids)
		if err != nil {
			return fmt.Errorf("check schema phase references: %w", err)
		}
		if count > 0 {
			return ErrFormSchemaHasPhases
		}
		count, err = m.engine.CountRequestSchemaReferences(txCtx, ids)
		if err != nil {
			return fmt.Errorf("check schema request references: %w", err)
		}
		if count > 0 {
			return ErrFormSchemaHasRequests
		}
		if err := m.engine.DeleteSchemaLineage(txCtx, source.Name); err != nil {
			return fmt.Errorf("delete schema: %w", err)
		}
		name = source.Name
		return nil
	})
	if err != nil {
		return "", err
	}
	return name, nil
}
