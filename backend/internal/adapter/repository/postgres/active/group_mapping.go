// backend/database/repositories/active/group_mapping.go
package active

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres/base"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/active"
	modelBase "github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	activePort "github.com/moto-nrw/project-phoenix/internal/core/port/active"
	"github.com/uptrace/bun"
)

// Table name constants for BUN ORM schema qualification
const (
	tableActiveGroupMappings   = "active.group_mappings"
	tableExprGroupMappingsAsGM = `active.group_mappings AS "group_mapping"`
	whereIDEquals              = "id = ?"
)

// GroupMappingRepository implements active.GroupMappingRepository interface
type GroupMappingRepository struct {
	*base.Repository[*active.GroupMapping]
	db *bun.DB
}

// NewGroupMappingRepository creates a new GroupMappingRepository
func NewGroupMappingRepository(db *bun.DB) activePort.GroupMappingRepository {
	return &GroupMappingRepository{
		Repository: base.NewRepository[*active.GroupMapping](db, "active.group_mappings", "GroupMapping"),
		db:         db,
	}
}

// FindByActiveCombinedGroupID finds all mappings for a specific combined group
func (r *GroupMappingRepository) FindByActiveCombinedGroupID(ctx context.Context, combinedGroupID int64) ([]*active.GroupMapping, error) {
	mappings := make([]*active.GroupMapping, 0)
	err := r.db.NewSelect().
		Model(&mappings).
		ModelTableExpr(tableExprGroupMappingsAsGM).
		Where("active_combined_group_id = ?", combinedGroupID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by active combined group ID",
			Err: err,
		}
	}

	return mappings, nil
}

// FindByActiveGroupID finds all mappings for a specific active group
func (r *GroupMappingRepository) FindByActiveGroupID(ctx context.Context, activeGroupID int64) ([]*active.GroupMapping, error) {
	mappings := make([]*active.GroupMapping, 0)
	err := r.db.NewSelect().
		Model(&mappings).
		ModelTableExpr(tableExprGroupMappingsAsGM).
		Where("active_group_id = ?", activeGroupID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by active group ID",
			Err: err,
		}
	}

	return mappings, nil
}

// AddGroupToCombination adds an active group to a combined group
func (r *GroupMappingRepository) AddGroupToCombination(ctx context.Context, combinedGroupID, activeGroupID int64) error {
	// Check if the mapping already exists
	exists, err := r.db.NewSelect().
		Model((*active.GroupMapping)(nil)).
		ModelTableExpr(tableExprGroupMappingsAsGM).
		Where("active_combined_group_id = ? AND active_group_id = ?", combinedGroupID, activeGroupID).
		Exists(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "check mapping existence",
			Err: err,
		}
	}

	if exists {
		// Mapping already exists, nothing to do
		return nil
	}

	// Create the mapping
	mapping := &active.GroupMapping{
		ActiveCombinedGroupID: combinedGroupID,
		ActiveGroupID:         activeGroupID,
	}

	if err := mapping.Validate(); err != nil {
		return err
	}

	_, err = r.db.NewInsert().
		Model(mapping).
		ModelTableExpr(tableActiveGroupMappings).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "add group to combination",
			Err: err,
		}
	}

	return nil
}

// RemoveGroupFromCombination removes an active group from a combined group
func (r *GroupMappingRepository) RemoveGroupFromCombination(ctx context.Context, combinedGroupID, activeGroupID int64) error {
	_, err := r.db.NewDelete().
		Model((*active.GroupMapping)(nil)).
		ModelTableExpr(tableActiveGroupMappings).
		Where("active_combined_group_id = ? AND active_group_id = ?", combinedGroupID, activeGroupID).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "remove group from combination",
			Err: err,
		}
	}

	return nil
}

// Create overrides base Create to handle validation
func (r *GroupMappingRepository) Create(ctx context.Context, mapping *active.GroupMapping) error {
	if mapping == nil {
		return fmt.Errorf("group mapping cannot be nil")
	}

	// Validate mapping
	if err := mapping.Validate(); err != nil {
		return err
	}

	// Use the base Create method
	return r.Repository.Create(ctx, mapping)
}

// List overrides the base List method to accept the new QueryOptions type
func (r *GroupMappingRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*active.GroupMapping, error) {
	mappings := make([]*active.GroupMapping, 0)
	query := r.db.NewSelect().Model(&mappings).ModelTableExpr(tableExprGroupMappingsAsGM)

	// Apply query options
	if options != nil {
		query = options.ApplyToQuery(query)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list",
			Err: err,
		}
	}

	return mappings, nil
}

// FindWithRelations retrieves a mapping with its associated relations
func (r *GroupMappingRepository) FindWithRelations(ctx context.Context, id int64) (*active.GroupMapping, error) {
	mapping := new(active.GroupMapping)
	err := r.db.NewSelect().
		Model(mapping).
		ModelTableExpr(tableExprGroupMappingsAsGM).
		Where(whereIDEquals, id).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with relations",
			Err: err,
		}
	}

	// Load CombinedGroup relation separately (multi-schema)
	if mapping.ActiveCombinedGroupID > 0 {
		combinedGroup := new(active.CombinedGroup)
		cgErr := r.db.NewSelect().
			Model(combinedGroup).
			ModelTableExpr(`active.combined_groups AS "combined_group"`).
			Where(whereIDEquals, mapping.ActiveCombinedGroupID).
			Scan(ctx)
		if cgErr == nil {
			mapping.CombinedGroup = combinedGroup
		} else if !errors.Is(cgErr, sql.ErrNoRows) {
			// Return actual database errors, but allow "not found" to continue
			return nil, &modelBase.DatabaseError{
				Op:  "find combined group relation",
				Err: cgErr,
			}
		}
	}

	// Load ActiveGroup relation separately (multi-schema)
	if mapping.ActiveGroupID > 0 {
		activeGroup := new(active.Group)
		agErr := r.db.NewSelect().
			Model(activeGroup).
			ModelTableExpr(`active.groups AS "group"`).
			Where(whereIDEquals, mapping.ActiveGroupID).
			Scan(ctx)
		if agErr == nil {
			mapping.ActiveGroup = activeGroup
		} else if !errors.Is(agErr, sql.ErrNoRows) {
			// Return actual database errors, but allow "not found" to continue
			return nil, &modelBase.DatabaseError{
				Op:  "find active group relation",
				Err: agErr,
			}
		}
	}

	return mapping, nil
}
