package enrollment

import "context"

// Schemas loads pinned versions in one tenant-scoped query.
func (m *Module) Schemas(ctx context.Context, ids []int64) ([]*FormSchema, error) {
	var schemas []*FormSchema
	err := m.transactions.RunInTx(ctx, func(txCtx context.Context) error {
		var queryErr error
		schemas, queryErr = m.engine.Schemas(txCtx, ids)
		return queryErr
	})
	return schemas, err
}

func (m *Module) InsertSchemaVersion(ctx context.Context, schema *FormSchema) error {
	return m.transactions.RunInTx(ctx, func(txCtx context.Context) error {
		return m.engine.InsertSchemaVersion(txCtx, schema)
	})
}

func (m *Module) NextSchemaVersion(ctx context.Context) (int, error) {
	var result int
	err := m.transactions.RunInTx(ctx, func(txCtx context.Context) error {
		var operationErr error
		result, operationErr = m.engine.NextSchemaVersion(txCtx)
		return operationErr
	})
	return result, err
}

func (m *Module) NextSchemaVersionForName(ctx context.Context, name string) (int, error) {
	var result int
	err := m.transactions.RunInTx(ctx, func(txCtx context.Context) error {
		var operationErr error
		result, operationErr = m.engine.NextSchemaVersionForName(txCtx, name)
		return operationErr
	})
	return result, err
}

func (m *Module) SchemaNameExists(ctx context.Context, name string) (bool, error) {
	var result bool
	err := m.transactions.RunInTx(ctx, func(txCtx context.Context) error {
		var operationErr error
		result, operationErr = m.engine.SchemaNameExists(txCtx, name)
		return operationErr
	})
	return result, err
}

func (m *Module) DeactivateSchemas(ctx context.Context) error {
	return m.transactions.RunInTx(ctx, func(txCtx context.Context) error {
		return m.engine.DeactivateSchemas(txCtx)
	})
}

func (m *Module) SetSchemaActive(ctx context.Context, id int64, active bool) error {
	return m.transactions.RunInTx(ctx, func(txCtx context.Context) error {
		return m.engine.SetSchemaActive(txCtx, id, active)
	})
}

func (m *Module) RenameSchemaLineage(ctx context.Context, oldName, newName string) error {
	return m.transactions.RunInTx(ctx, func(txCtx context.Context) error {
		return m.engine.RenameSchemaLineage(txCtx, oldName, newName)
	})
}

func (m *Module) DeleteSchemaLineage(ctx context.Context, name string) error {
	return m.transactions.RunInTx(ctx, func(txCtx context.Context) error {
		return m.engine.DeleteSchemaLineage(txCtx, name)
	})
}

func (m *Module) LockSchemaLineages(ctx context.Context) error {
	return m.transactions.RunInTx(ctx, func(txCtx context.Context) error {
		return m.engine.LockSchemaLineages(txCtx)
	})
}

func (m *Module) SchemaReferencesLegalDocument(ctx context.Context, storedURL, publicURL string) (bool, error) {
	var result bool
	err := m.transactions.RunInTx(ctx, func(txCtx context.Context) error {
		var operationErr error
		result, operationErr = m.engine.SchemaReferencesLegalDocument(txCtx, storedURL, publicURL)
		return operationErr
	})
	return result, err
}
