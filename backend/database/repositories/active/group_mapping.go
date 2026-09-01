// backend/database/repositories/active/group_mapping.go
package active

import (
	"context"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Table name constants for BUN ORM schema qualification
const (
	tableActiveGroupMappings   = "active.group_mappings"
	tableExprGroupMappingsAsGM = `active.group_mappings AS "group_mapping"`
)

// GroupMappingRepository implements active.GroupMappingRepository interface
type GroupMappingRepository struct {
	*base.Repository[*active.GroupMapping]
	db *bun.DB
}

// NewGroupMappingRepository creates a new GroupMappingRepository
func NewGroupMappingRepository(db *bun.DB) active.GroupMappingRepository {
	repo := base.NewRepository[*active.GroupMapping](db, "active.group_mappings", "GroupMapping")
	repo.TenantScoped = true
	return &GroupMappingRepository{
		Repository: repo,
		db:         db,
	}
}

// FindByActiveCombinedGroupID finds all mappings for a specific combined group
func (r *GroupMappingRepository) FindByActiveCombinedGroupID(ctx context.Context, combinedGroupID int64) ([]*active.GroupMapping, error) {
	mappings := make([]*active.GroupMapping, 0)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&mappings).
		ModelTableExpr(tableExprGroupMappingsAsGM).
		Where("active_combined_group_id = ?", combinedGroupID)

	query = base.WithTenantFilter(ctx, query, "group_mapping")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by active combined group ID",
			Err: base.TranslateNotFound(err),
		}
	}

	return mappings, nil
}

// FindByActiveGroupID finds all mappings for a specific active group
func (r *GroupMappingRepository) FindByActiveGroupID(ctx context.Context, activeGroupID int64) ([]*active.GroupMapping, error) {
	mappings := make([]*active.GroupMapping, 0)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&mappings).
		ModelTableExpr(tableExprGroupMappingsAsGM).
		Where("active_group_id = ?", activeGroupID)

	query = base.WithTenantFilter(ctx, query, "group_mapping")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by active group ID",
			Err: base.TranslateNotFound(err),
		}
	}

	return mappings, nil
}

// AddGroupToCombination adds an active group to a combined group
func (r *GroupMappingRepository) AddGroupToCombination(ctx context.Context, combinedGroupID, activeGroupID int64) error {
	// Check if the mapping already exists
	existsQuery := base.GetDB(ctx, r.db).NewSelect().
		Model((*active.GroupMapping)(nil)).
		ModelTableExpr(tableExprGroupMappingsAsGM).
		Where("active_combined_group_id = ? AND active_group_id = ?", combinedGroupID, activeGroupID)

	existsQuery = base.WithTenantFilter(ctx, existsQuery, "group_mapping")

	exists, err := existsQuery.Exists(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "check mapping existence",
			Err: base.TranslateNotFound(err),
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

	base.EnsureTenantID(ctx, mapping)

	_, err = base.GetDB(ctx, r.db).NewInsert().
		Model(mapping).
		ModelTableExpr(tableActiveGroupMappings).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "add group to combination",
			Err: base.TranslateNotFound(err),
		}
	}

	return nil
}

// RemoveGroupFromCombination removes an active group from a combined group
func (r *GroupMappingRepository) RemoveGroupFromCombination(ctx context.Context, combinedGroupID, activeGroupID int64) error {
	query := base.GetDB(ctx, r.db).NewDelete().
		Model((*active.GroupMapping)(nil)).
		ModelTableExpr(tableActiveGroupMappings).
		Where("active_combined_group_id = ? AND active_group_id = ?", combinedGroupID, activeGroupID)

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}

	_, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "remove group from combination",
			Err: base.TranslateNotFound(err),
		}
	}

	return nil
}

// List overrides the base List method to accept the new QueryOptions type
func (r *GroupMappingRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*active.GroupMapping, error) {
	mappings, err := r.ListWithOptions(ctx, options)
	if err != nil {
		return nil, err
	}
	if mappings == nil {
		mappings = make([]*active.GroupMapping, 0)
	}
	return mappings, nil
}
