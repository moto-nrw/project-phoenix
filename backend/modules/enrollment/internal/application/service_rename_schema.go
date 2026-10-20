package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// RenameSchema renames every version while holding the publication lock.
func (m *Service) RenameSchema(ctx context.Context, id int64, newName string) (*enrollment.FormSchema, error) {
	if id <= 0 {
		return nil, enrollment.ErrFormSchemaNotFound
	}
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return nil, enrollment.InvalidInput(enrollment.CodeSchemaNameRequired, "name", errors.New("schema name is required"))
	}
	var result *enrollment.FormSchema
	err := m.transactions.RunInTx(ctx, func(txCtx context.Context) error {
		if err := m.store.LockSchemaLineages(txCtx); err != nil {
			return err
		}
		source, err := m.store.Schema(txCtx, id)
		if err != nil {
			return fmt.Errorf("load source schema: %w", err)
		}
		if source.Name == newName {
			result = source
			return nil
		}
		exists, err := m.store.SchemaNameExists(txCtx, newName)
		if err != nil {
			return fmt.Errorf("check existing name: %w", err)
		}
		if exists {
			return enrollment.ErrFormSchemaNameExists
		}
		if err := m.store.RenameSchemaLineage(txCtx, source.Name, newName); err != nil {
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
