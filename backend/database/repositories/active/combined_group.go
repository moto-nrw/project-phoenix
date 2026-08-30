// backend/database/repositories/active/combined_group.go
package active

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Table name constants for BUN ORM schema qualification
const (
	tableCombinedGroups         = "active.combined_groups"
	tableExprCombinedGroupsAsCG = `active.combined_groups AS "combined_group"`
)

// CombinedGroupRepository implements active.CombinedGroupRepository interface
type CombinedGroupRepository struct {
	*base.Repository[*active.CombinedGroup]
	db *bun.DB
}

// NewCombinedGroupRepository creates a new CombinedGroupRepository
func NewCombinedGroupRepository(db *bun.DB) active.CombinedGroupRepository {
	repo := base.NewRepository[*active.CombinedGroup](db, "active.combined_groups", "CombinedGroup")
	repo.TenantScoped = true
	return &CombinedGroupRepository{
		Repository: repo,
		db:         db,
	}
}

// FindActive finds all currently active combined groups
func (r *CombinedGroupRepository) FindActive(ctx context.Context) ([]*active.CombinedGroup, error) {
	var groups []*active.CombinedGroup
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&groups).
		ModelTableExpr(tableExprCombinedGroupsAsCG).
		Where("end_time IS NULL")

	query = base.WithTenantFilter(ctx, query, "combined_group")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find active",
			Err: err,
		}
	}

	return groups, nil
}

// FindByTimeRange finds all combined groups active during a specific time range
func (r *CombinedGroupRepository) FindByTimeRange(ctx context.Context, start, end time.Time) ([]*active.CombinedGroup, error) {
	var groups []*active.CombinedGroup
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&groups).
		ModelTableExpr(tableExprCombinedGroupsAsCG).
		Where("start_time <= ? AND (end_time IS NULL OR end_time >= ?)", end, start)

	query = base.WithTenantFilter(ctx, query, "combined_group")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by time range",
			Err: err,
		}
	}

	return groups, nil
}

// EndCombination marks a combined group as ended at the current time
func (r *CombinedGroupRepository) EndCombination(ctx context.Context, id int64) error {
	query := base.GetDB(ctx, r.db).NewUpdate().
		Table(tableCombinedGroups).
		Set("end_time = ?", time.Now()).
		Where("id = ? AND end_time IS NULL", id)

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}

	result, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "end combination",
			Err: err,
		}
	}

	return base.AssertRowsAffected(result, 1, "end combination")
}

// FindWithGroups finds a combined group with all its associated active groups
func (r *CombinedGroupRepository) FindWithGroups(ctx context.Context, id int64) (*active.CombinedGroup, error) {
	combinedGroup := new(active.CombinedGroup)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(combinedGroup).
		ModelTableExpr(tableExprCombinedGroupsAsCG).
		Where("id = ?", id)

	query = base.WithTenantFilter(ctx, query, "combined_group")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find combined group",
			Err: err,
		}
	}

	// Load group mappings (multi-schema requires explicit ModelTableExpr)
	groupMappings := make([]*active.GroupMapping, 0)
	mappingQuery := base.GetDB(ctx, r.db).NewSelect().
		Model(&groupMappings).
		ModelTableExpr(`active.group_mappings AS "group_mapping"`).
		Where("active_combined_group_id = ?", id)

	mappingQuery = base.WithTenantFilter(ctx, mappingQuery, "group_mapping")

	err = mappingQuery.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find group mappings",
			Err: err,
		}
	}

	// Load ActiveGroup for each mapping separately (multi-schema)
	for _, mapping := range groupMappings {
		if mapping.ActiveGroupID > 0 {
			activeGroup := new(active.Group)
			agQuery := base.GetDB(ctx, r.db).NewSelect().
				Model(activeGroup).
				ModelTableExpr(`active.groups AS "group"`).
				Where("id = ?", mapping.ActiveGroupID)

			agQuery = base.WithTenantFilter(ctx, agQuery, "group")

			agErr := agQuery.Scan(ctx)
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
	}

	// Set mappings
	combinedGroup.GroupMappings = groupMappings

	// Extract active groups from mappings
	activeGroups := make([]*active.Group, 0, len(groupMappings))
	for _, mapping := range groupMappings {
		if mapping.ActiveGroup != nil {
			activeGroups = append(activeGroups, mapping.ActiveGroup)
		}
	}
	combinedGroup.ActiveGroups = activeGroups

	return combinedGroup, nil
}

// List overrides the base List method to accept the new QueryOptions type
func (r *CombinedGroupRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*active.CombinedGroup, error) {
	if options != nil && options.Filter != nil {
		rewriteActiveOnlyFilter(options.Filter, "end_time", bun.Safe("NOW()"))
	}

	groups, err := r.ListWithOptions(ctx, options)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "list", Err: base.DatabaseErrorCause(err)}
	}
	if len(groups) == 0 {
		return nil, nil
	}

	return groups, nil
}
