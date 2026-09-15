package timetableprojection

import (
	"context"
	"fmt"

	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/uptrace/bun"
)

// The group-name and activity-group reads lost their last production caller
// with the legacy active-group reads (#2737). They stay as the tenant-guard
// fixtures of the projection integration test, which proves that an elevated
// transaction still sees only the tenant a projection names.

func GroupNames(ctx context.Context, db bun.IDB, tenantID int64, ids []int64) (map[int64]string, error) {
	if tenantID <= 0 {
		return nil, ErrInvalidTenantID
	}
	result := make(map[int64]string, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	var rows []struct {
		ID   int64  `bun:"id"`
		Name string `bun:"name"`
	}
	query := db.NewSelect().
		TableExpr(`activities.groups AS "group"`).
		ColumnExpr(`"group".id, "group".name`).
		Where(`"group".id IN (?)`, bun.List(ids)).
		Where(`"group".tenant_id = ?`, tenantID)
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("timetable projection: list group names: %w", err)
	}
	for _, row := range rows {
		result[row.ID] = row.Name
	}
	return result, nil
}

func ActivityGroupsByID(ctx context.Context, db bun.IDB, tenantID int64, ids []int64) ([]*activitiesModels.Group, error) {
	if tenantID <= 0 {
		return nil, ErrInvalidTenantID
	}
	if len(ids) == 0 {
		return []*activitiesModels.Group{}, nil
	}
	var groups []*activitiesModels.Group
	err := db.NewSelect().Model(&groups).ModelTableExpr(`activities.groups AS "group"`).
		Where(`"group".tenant_id = ?`, tenantID).Where(`"group".id IN (?)`, bun.List(ids)).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("timetable projection: list activity groups: %w", err)
	}
	return groups, nil
}
