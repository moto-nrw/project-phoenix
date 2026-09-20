package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// CountActiveAccountsBySchoolGroups aggregates membership facts without
// exporting accounts or interpreting the caller's school groups.
func (s *Store) CountActiveAccountsBySchoolGroups(ctx context.Context, groups map[int64][]int64) (map[int64]int, domain.OperationStats, error) {
	result := make(map[int64]int, len(groups))
	type selection struct {
		GroupID  int64 `json:"group_id"`
		SchoolID int64 `json:"school_id"`
	}
	var selections []selection
	for groupID, schoolIDs := range groups {
		result[groupID] = 0
		for _, schoolID := range schoolIDs {
			selections = append(selections, selection{GroupID: groupID, SchoolID: schoolID})
		}
	}
	if len(selections) == 0 {
		return result, domain.OperationStats{}, nil
	}
	payload, err := json.Marshal(selections)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []struct {
		GroupID      int64
		AccountCount int
	}
	started := time.Now()
	err = db.NewRaw(`SELECT requested.group_id, COUNT(DISTINCT membership.account_id) AS account_count
		FROM jsonb_to_recordset(?::jsonb) AS requested(group_id bigint, school_id bigint)
		JOIN auth.account_tenants AS membership ON membership.tenant_id = requested.school_id
		WHERE membership.status = 'active'
		GROUP BY requested.group_id`, string(payload)).Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, Rows: int64(len(rows)), StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, err
	}
	for _, row := range rows {
		result[row.GroupID] = row.AccountCount
	}
	return result, stats, nil
}
