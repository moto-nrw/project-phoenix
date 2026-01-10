// backend/database/repositories/active/combined_group.go
package active

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
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
	return &CombinedGroupRepository{
		Repository: base.NewRepository[*active.CombinedGroup](db, "active.combined_groups", "CombinedGroup"),
		db:         db,
	}
}

// FindActive finds all currently active combined groups
func (r *CombinedGroupRepository) FindActive(ctx context.Context) ([]*active.CombinedGroup, error) {
	var groups []*active.CombinedGroup
	err := r.db.NewSelect().
		Model(&groups).
		ModelTableExpr(tableExprCombinedGroupsAsCG).
		Where("end_time IS NULL").
		Scan(ctx)

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
	err := r.db.NewSelect().
		Model(&groups).
		ModelTableExpr(tableExprCombinedGroupsAsCG).
		Where("start_time <= ? AND (end_time IS NULL OR end_time >= ?)", end, start).
		Scan(ctx)

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
	_, err := r.db.NewUpdate().
		Table(tableCombinedGroups).
		Set("end_time = ?", time.Now()).
		Where("id = ? AND end_time IS NULL", id).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "end combination",
			Err: err,
		}
	}

	return nil
}

// FindWithGroups finds a combined group with all its associated active groups
func (r *CombinedGroupRepository) FindWithGroups(ctx context.Context, id int64) (*active.CombinedGroup, error) {
	combinedGroup := new(active.CombinedGroup)
	err := r.db.NewSelect().
		Model(combinedGroup).
		ModelTableExpr(tableExprCombinedGroupsAsCG).
		Where("id = ?", id).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find combined group",
			Err: err,
		}
	}

	// Load group mappings (multi-schema requires explicit ModelTableExpr)
	groupMappings := make([]*active.GroupMapping, 0)
	err = r.db.NewSelect().
		Model(&groupMappings).
		ModelTableExpr(`active.group_mappings AS "group_mapping"`).
		Where("active_combined_group_id = ?", id).
		Scan(ctx)

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
			agErr := r.db.NewSelect().
				Model(activeGroup).
				ModelTableExpr(`active.groups AS "group"`).
				Where("id = ?", mapping.ActiveGroupID).
				Scan(ctx)
			if agErr == nil {
				mapping.ActiveGroup = activeGroup
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

// Create overrides base Create to handle validation
func (r *CombinedGroupRepository) Create(ctx context.Context, combinedGroup *active.CombinedGroup) error {
	if combinedGroup == nil {
		return fmt.Errorf("combined group cannot be nil")
	}

	// Validate combined group
	if err := combinedGroup.Validate(); err != nil {
		return err
	}

	// Use the base Create method
	return r.Repository.Create(ctx, combinedGroup)
}

// List overrides the base List method to accept the new QueryOptions type
func (r *CombinedGroupRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*active.CombinedGroup, error) {
	var groups []*active.CombinedGroup
	query := r.db.NewSelect().Model(&groups).ModelTableExpr(tableExprCombinedGroupsAsCG)

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

	return groups, nil
}
