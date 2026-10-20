package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// PublishSchema appends a version and advances phases of the same lineage.
// Submitted requests keep their immutable version reference.
func (m *Service) PublishSchema(ctx context.Context, input enrollment.FormSchema) (*enrollment.FormSchema, error) {
	schema := &enrollment.FormSchema{Name: input.Name, Version: 1, Fields: input.Fields,
		CoreRequirements: input.CoreRequirements, LegalBlocks: input.LegalBlocks,
		CreatedBy: input.CreatedBy, IsActive: true}
	if err := schema.Validate(); err != nil {
		return nil, fmt.Errorf("invalid schema: %w", err)
	}
	err := m.transactions.RunInTx(ctx, func(txCtx context.Context) error {
		if err := m.store.LockSchemaLineages(txCtx); err != nil {
			return err
		}
		version, err := m.store.NextSchemaVersionForName(txCtx, schema.Name)
		if err != nil {
			return fmt.Errorf("compute next version: %w", err)
		}
		schema.Version = version
		if err := m.store.InsertSchemaVersion(txCtx, schema); err != nil {
			return fmt.Errorf("create schema: %w", err)
		}
		return m.repointPreviousVersions(txCtx, schema)
	})
	if err != nil {
		return nil, err
	}
	return schema, nil
}

// repointPreviousVersions advances the phases on older versions of the
// schema's lineage to the newly inserted version.
func (m *Service) repointPreviousVersions(ctx context.Context, schema *enrollment.FormSchema) error {
	versions, err := m.store.SchemaVersions(ctx)
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
		if _, err := m.store.RepointPhaseSchemas(ctx, oldIDs, schema.ID); err != nil {
			return fmt.Errorf("repoint phases to schema %d: %w", schema.ID, err)
		}
	}
	return nil
}
