package enrollment

import (
	"context"
	"fmt"
)

// PublishSchema appends a version and advances phases of the same lineage.
// Submitted requests keep their immutable version reference.
func (m *Module) PublishSchema(ctx context.Context, input FormSchema) (*FormSchema, error) {
	schema := &FormSchema{Name: input.Name, Version: 1, Fields: input.Fields,
		CoreRequirements: input.CoreRequirements, LegalBlocks: input.LegalBlocks,
		CreatedBy: input.CreatedBy, IsActive: true}
	if err := schema.Validate(); err != nil {
		return nil, fmt.Errorf("invalid schema: %w", err)
	}
	err := m.transactions.RunInTx(ctx, func(txCtx context.Context) error {
		if err := m.engine.LockSchemaLineages(txCtx); err != nil {
			return err
		}
		version, err := m.engine.NextSchemaVersionForName(txCtx, schema.Name)
		if err != nil {
			return fmt.Errorf("compute next version: %w", err)
		}
		schema.Version = version
		if err := m.engine.InsertSchemaVersion(txCtx, schema); err != nil {
			return fmt.Errorf("create schema: %w", err)
		}
		versions, err := m.engine.SchemaVersions(txCtx)
		if err != nil {
			return fmt.Errorf("list schema versions for repoint: %w", err)
		}
		oldIDs := make([]int64, 0, len(versions))
		for _, previous := range versions {
			if previous.Name == schema.Name && previous.ID != schema.ID {
				oldIDs = append(oldIDs, previous.ID)
			}
		}
		if len(oldIDs) > 0 {
			if _, err := m.engine.RepointPhaseSchemas(txCtx, oldIDs, schema.ID); err != nil {
				return fmt.Errorf("repoint phases to schema %d: %w", schema.ID, err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return schema, nil
}
