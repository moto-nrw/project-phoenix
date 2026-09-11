package audit

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/models/audit"
)

// DataImportRepository implements audit.DataImportRepository
type DataImportRepository struct {
	runtime Runtime
}

// NewDataImportRepository creates a new data import repository
func NewDataImportRepository(runtime Runtime) *DataImportRepository {
	return &DataImportRepository{runtime: requireRuntime(runtime)}
}

// Create creates a new data import audit record
func (r *DataImportRepository) Create(ctx context.Context, dataImport *audit.DataImport) error {
	return NewAppender(r.runtime).Append(ctx, dataImport)
}

// FindByID finds a data import record by ID
func (r *DataImportRepository) FindByID(ctx context.Context, id int64) (*audit.DataImport, error) {
	dataImport := &audit.DataImport{}
	err := runtimeDB(ctx, r.runtime).NewSelect().
		Model(dataImport).
		ModelTableExpr(`audit.data_imports AS "data_import"`).
		Where("id = ?", id).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("find data import by id: %w", err)
	}
	return dataImport, nil
}

// List lists data imports with optional filters
func (r *DataImportRepository) List(ctx context.Context, filters map[string]interface{}) ([]*audit.DataImport, error) {
	var imports []*audit.DataImport
	query := runtimeDB(ctx, r.runtime).NewSelect().Model(&imports).ModelTableExpr(`audit.data_imports AS "data_import"`)

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
