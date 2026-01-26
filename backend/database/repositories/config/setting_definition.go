package config

import (
	"context"
	"database/sql"
	"errors"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/uptrace/bun"
)

// Table and query constants for setting definitions
const (
	tableSettingDefinitions      = "config.setting_definitions"
	tableSettingDefinitionsAlias = `config.setting_definitions AS "setting_definition"`

	whereID              = "id = ?"
	orderCategorySortKey = `category ASC, sort_order ASC, "key" ASC`
)

// SettingDefinitionRepository implements config.SettingDefinitionRepository
type SettingDefinitionRepository struct {
	db *bun.DB
}

// NewSettingDefinitionRepository creates a new SettingDefinitionRepository
func NewSettingDefinitionRepository(db *bun.DB) config.SettingDefinitionRepository {
	return &SettingDefinitionRepository{db: db}
}

// Create inserts a new setting definition
func (r *SettingDefinitionRepository) Create(ctx context.Context, def *config.SettingDefinition) error {
	if err := def.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewInsert().
		Model(def).
		ModelTableExpr(tableSettingDefinitions).
		Returning("*").
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "create setting definition",
			Err: err,
		}
	}

	return nil
}

// FindByID retrieves a setting definition by its ID
func (r *SettingDefinitionRepository) FindByID(ctx context.Context, id int64) (*config.SettingDefinition, error) {
	def := new(config.SettingDefinition)
	err := r.db.NewSelect().
		Model(def).
		ModelTableExpr(tableSettingDefinitionsAlias).
		Where(whereID, id).
		Scan(ctx)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{
			Op:  "find definition by id",
			Err: err,
		}
	}

	return def, nil
}

// Update modifies an existing setting definition
func (r *SettingDefinitionRepository) Update(ctx context.Context, def *config.SettingDefinition) error {
	if err := def.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewUpdate().
		Model(def).
		ModelTableExpr(tableSettingDefinitions).
		Where(whereID, def.ID).
		Returning("*").
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update setting definition",
			Err: err,
		}
	}

	return nil
}

// Delete removes a setting definition
func (r *SettingDefinitionRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.NewDelete().
		Model((*config.SettingDefinition)(nil)).
		ModelTableExpr(tableSettingDefinitions).
		Where(whereID, id).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "delete setting definition",
			Err: err,
		}
	}

	return nil
}

// FindByKey retrieves a setting definition by its unique key
func (r *SettingDefinitionRepository) FindByKey(ctx context.Context, key string) (*config.SettingDefinition, error) {
	def := new(config.SettingDefinition)
	err := r.db.NewSelect().
		Model(def).
		ModelTableExpr(tableSettingDefinitionsAlias).
		Where(`"key" = ?`, key).
		Scan(ctx)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{
			Op:  "find definition by key",
			Err: err,
		}
	}

	return def, nil
}

// FindByCategory retrieves setting definitions by category
func (r *SettingDefinitionRepository) FindByCategory(ctx context.Context, category string) ([]*config.SettingDefinition, error) {
	var defs []*config.SettingDefinition
	err := r.db.NewSelect().
		Model(&defs).
		ModelTableExpr(tableSettingDefinitionsAlias).
		Where("category = ?", category).
		OrderExpr("sort_order ASC, \"key\" ASC").
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find definitions by category",
			Err: err,
		}
	}

	return defs, nil
}

// FindByScope retrieves setting definitions that allow a specific scope
func (r *SettingDefinitionRepository) FindByScope(ctx context.Context, scopeType config.ScopeType) ([]*config.SettingDefinition, error) {
	var defs []*config.SettingDefinition
	err := r.db.NewSelect().
		Model(&defs).
		ModelTableExpr(tableSettingDefinitionsAlias).
		Where("? = ANY(allowed_scopes)", string(scopeType)).
		OrderExpr(orderCategorySortKey).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find definitions by scope",
			Err: err,
		}
	}

	return defs, nil
}

// FindByGroup retrieves setting definitions by group name
func (r *SettingDefinitionRepository) FindByGroup(ctx context.Context, groupName string) ([]*config.SettingDefinition, error) {
	var defs []*config.SettingDefinition
	err := r.db.NewSelect().
		Model(&defs).
		ModelTableExpr(tableSettingDefinitionsAlias).
		Where("group_name = ?", groupName).
		OrderExpr("sort_order ASC, \"key\" ASC").
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find definitions by group",
			Err: err,
		}
	}

	return defs, nil
}

// List retrieves setting definitions with optional filters
func (r *SettingDefinitionRepository) List(ctx context.Context, filters map[string]interface{}) ([]*config.SettingDefinition, error) {
	var defs []*config.SettingDefinition
	query := r.db.NewSelect().
		Model(&defs).
		ModelTableExpr(tableSettingDefinitionsAlias)

	// Apply filters
	if category, ok := filters["category"].(string); ok && category != "" {
		query = query.Where("category = ?", category)
	}

	if scopeType, ok := filters["scope"].(string); ok && scopeType != "" {
		query = query.Where("? = ANY(allowed_scopes)", scopeType)
	}

	if groupName, ok := filters["group"].(string); ok && groupName != "" {
		query = query.Where("group_name = ?", groupName)
	}

	if search, ok := filters["search"].(string); ok && search != "" {
		pattern := "%" + search + "%"
		query = query.Where(`("key" ILIKE ? OR description ILIKE ?)`, pattern, pattern)
	}

	// Default ordering
	query = query.OrderExpr(orderCategorySortKey)

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list definitions",
			Err: err,
		}
	}

	return defs, nil
}

// ListAll retrieves all setting definitions
func (r *SettingDefinitionRepository) ListAll(ctx context.Context) ([]*config.SettingDefinition, error) {
	var defs []*config.SettingDefinition
	err := r.db.NewSelect().
		Model(&defs).
		ModelTableExpr(tableSettingDefinitionsAlias).
		OrderExpr(orderCategorySortKey).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list all definitions",
			Err: err,
		}
	}

	return defs, nil
}

// Upsert inserts or updates a setting definition by key
func (r *SettingDefinitionRepository) Upsert(ctx context.Context, def *config.SettingDefinition) error {
	if err := def.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewInsert().
		Model(def).
		ModelTableExpr(tableSettingDefinitions).
		On(`CONFLICT ("key") DO UPDATE`).
		Set("type = EXCLUDED.type").
		Set("default_value = EXCLUDED.default_value").
		Set("category = EXCLUDED.category").
		Set("description = EXCLUDED.description").
		Set("validation = EXCLUDED.validation").
		Set("allowed_scopes = EXCLUDED.allowed_scopes").
		Set("scope_permissions = EXCLUDED.scope_permissions").
		Set("depends_on = EXCLUDED.depends_on").
		Set("group_name = EXCLUDED.group_name").
		Set("sort_order = EXCLUDED.sort_order").
		Set("updated_at = NOW()").
		Returning("*").
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "upsert setting definition",
			Err: err,
		}
	}

	return nil
}
