package audit

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/uptrace/bun"
)

// DataImportRepository implements audit.DataImportRepository
type DataImportRepository struct {
	db *bun.DB
}

// NewDataImportRepository creates a new data import repository
func NewDataImportRepository(db *bun.DB) *DataImportRepository {
	return &DataImportRepository{db: db}
}

// Create creates a new data import audit record
func (r *DataImportRepository) Create(ctx context.Context, dataImport *audit.DataImport) error {
	_, err := r.db.NewInsert().
		Model(dataImport).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("create data import: %w", err)
	}
	return nil
}

// FindByID finds a data import record by ID
func (r *DataImportRepository) FindByID(ctx context.Context, id int64) (*audit.DataImport, error) {
	dataImport := &audit.DataImport{}
	err := r.db.NewSelect().
		Model(dataImport).
		Where("id = ?", id).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("find data import by id: %w", err)
	}
	return dataImport, nil
}

// FindByImportedBy finds recent imports by a specific account
func (r *DataImportRepository) FindByImportedBy(ctx context.Context, accountID int64, limit int) ([]*audit.DataImport, error) {
	var imports []*audit.DataImport
	err := r.db.NewSelect().
		Model(&imports).
		Where("imported_by = ?", accountID).
		Order(orderByCreatedAtDesc).
		Limit(limit).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("find data imports by account: %w", err)
	}
	return imports, nil
}

// FindByEntityType finds recent imports for a specific entity type
func (r *DataImportRepository) FindByEntityType(ctx context.Context, entityType string, limit int) ([]*audit.DataImport, error) {
	var imports []*audit.DataImport
	err := r.db.NewSelect().
		Model(&imports).
		Where("entity_type = ?", entityType).
		Order(orderByCreatedAtDesc).
		Limit(limit).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("find data imports by entity type: %w", err)
	}
	return imports, nil
}

// FindRecent finds the most recent imports
func (r *DataImportRepository) FindRecent(ctx context.Context, limit int) ([]*audit.DataImport, error) {
	var imports []*audit.DataImport
	err := r.db.NewSelect().
		Model(&imports).
		Order(orderByCreatedAtDesc).
		Limit(limit).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("find recent data imports: %w", err)
	}
	return imports, nil
}

// List lists data imports with optional filters
func (r *DataImportRepository) List(ctx context.Context, filters map[string]interface{}) ([]*audit.DataImport, error) {
	var imports []*audit.DataImport
	query := r.db.NewSelect().Model(&imports)

	// Apply filters
	if entityType, ok := filters["entity_type"].(string); ok {
		query = query.Where("entity_type = ?", entityType)
	}
	if importedBy, ok := filters["imported_by"].(int64); ok {
		query = query.Where("imported_by = ?", importedBy)
	}
	if dryRun, ok := filters["dry_run"].(bool); ok {
		query = query.Where("dry_run = ?", dryRun)
	}

	query = query.Order(orderByCreatedAtDesc)

	err := query.Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list data imports: %w", err)
	}
	return imports, nil
}
