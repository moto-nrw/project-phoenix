package postgres

import (
	"context"
	"fmt"

	"github.com/uptrace/bun/dialect/pgdialect"
)

// membershipGroupIDsSQL takes the school set.
const membershipGroupIDsSQL = `SELECT tenant_id, id, group_id
FROM users.student_school_memberships
WHERE tenant_id = ANY(?) AND deleted_at IS NULL AND group_id IS NOT NULL`

// MembershipGroupIDsByTenant maps each live membership with a group to it.
func (r schoolRosters) MembershipGroupIDsByTenant(ctx context.Context, schools []int64) (map[int64]map[int64]int64, error) {
	result := make(map[int64]map[int64]int64, len(schools))
	if len(schools) == 0 {
		return result, nil
	}
	db, err := r.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		TenantID int64 `bun:"tenant_id"`
		ID       int64 `bun:"id"`
		GroupID  int64 `bun:"group_id"`
	}
	if err := db.NewRaw(membershipGroupIDsSQL, pgdialect.Array(schools)).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("school membership postgres: membership groups: %w", err)
	}
	for _, row := range rows {
		if result[row.TenantID] == nil {
			result[row.TenantID] = map[int64]int64{}
		}
		result[row.TenantID][row.ID] = row.GroupID
	}
	return result, nil
}
