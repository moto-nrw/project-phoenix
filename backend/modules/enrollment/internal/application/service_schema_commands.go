package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// Schemas loads pinned versions in one tenant-scoped query.
func (m *Service) Schemas(ctx context.Context, ids []int64) ([]*enrollment.FormSchema, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.Schemas(ctx, ids)
}

func (m *Service) InsertSchemaVersion(ctx context.Context, schema *enrollment.FormSchema) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.store.InsertSchemaVersion(ctx, schema)
}

func (m *Service) NextSchemaVersion(ctx context.Context) (int, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return 0, err
	}
	return m.store.NextSchemaVersion(ctx)
}

func (m *Service) NextSchemaVersionForName(ctx context.Context, name string) (int, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return 0, err
	}
	return m.store.NextSchemaVersionForName(ctx, name)
}

func (m *Service) SchemaNameExists(ctx context.Context, name string) (bool, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return false, err
	}
	return m.store.SchemaNameExists(ctx, name)
}

func (m *Service) DeactivateSchemas(ctx context.Context) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.store.DeactivateSchemas(ctx)
}

func (m *Service) SetSchemaActive(ctx context.Context, id int64, active bool) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.store.SetSchemaActive(ctx, id, active)
}

func (m *Service) LockSchemaLineages(ctx context.Context) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.store.LockSchemaLineages(ctx)
}

func (m *Service) SchemaReferencesLegalDocument(ctx context.Context, storedURL, publicURL string) (bool, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return false, err
	}
	return m.store.SchemaReferencesLegalDocument(ctx, storedURL, publicURL)
}
