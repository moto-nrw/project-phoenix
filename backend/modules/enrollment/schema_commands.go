package enrollment

import "context"

// Schemas loads pinned versions in one tenant-scoped query.
func (m *Module) Schemas(ctx context.Context, ids []int64) ([]*FormSchema, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.engine.Schemas(ctx, ids)
}

func (m *Module) InsertSchemaVersion(ctx context.Context, schema *FormSchema) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.engine.InsertSchemaVersion(ctx, schema)
}

func (m *Module) NextSchemaVersion(ctx context.Context) (int, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return 0, err
	}
	return m.engine.NextSchemaVersion(ctx)
}

func (m *Module) NextSchemaVersionForName(ctx context.Context, name string) (int, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return 0, err
	}
	return m.engine.NextSchemaVersionForName(ctx, name)
}

func (m *Module) SchemaNameExists(ctx context.Context, name string) (bool, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return false, err
	}
	return m.engine.SchemaNameExists(ctx, name)
}

func (m *Module) DeactivateSchemas(ctx context.Context) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.engine.DeactivateSchemas(ctx)
}

func (m *Module) SetSchemaActive(ctx context.Context, id int64, active bool) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.engine.SetSchemaActive(ctx, id, active)
}

func (m *Module) LockSchemaLineages(ctx context.Context) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.engine.LockSchemaLineages(ctx)
}

func (m *Module) SchemaReferencesLegalDocument(ctx context.Context, storedURL, publicURL string) (bool, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return false, err
	}
	return m.engine.SchemaReferencesLegalDocument(ctx, storedURL, publicURL)
}
