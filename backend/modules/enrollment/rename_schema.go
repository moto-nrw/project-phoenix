package enrollment

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var ErrFormSchemaNameExists = errors.New("a form schema with this name already exists")

// RenameSchema renames every version while holding the publication lock.
func (m *Module) RenameSchema(ctx context.Context, id int64, newName string) (*FormSchema, error) {
	if id <= 0 {
		return nil, ErrFormSchemaNotFound
	}
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return nil, fmt.Errorf("schema name is required")
	}
	var result *FormSchema
	err := m.transactions.RunInTx(ctx, func(txCtx context.Context) error {
		if err := m.engine.LockSchemaLineages(txCtx); err != nil {
			return err
		}
		source, err := m.engine.Schema(txCtx, id)
		if err != nil {
			return fmt.Errorf("load source schema: %w", err)
		}
		if source.Name == newName {
			result = source
			return nil
		}
		exists, err := m.engine.SchemaNameExists(txCtx, newName)
		if err != nil {
			return fmt.Errorf("check existing name: %w", err)
		}
		if exists {
			return ErrFormSchemaNameExists
		}
		if err := m.engine.RenameSchemaLineage(txCtx, source.Name, newName); err != nil {
			return fmt.Errorf("rename schema: %w", err)
		}
		source.Name = newName
		result = source
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
