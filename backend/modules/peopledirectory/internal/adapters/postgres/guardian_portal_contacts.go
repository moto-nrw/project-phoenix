package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/guardianlinkview"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
	"github.com/uptrace/bun"
)

// ListPortalContacts batches the two directory-owned tables in one statement.
// The application filters these candidates through the identity projection.
func (s *GuardianStore) ListPortalContacts(ctx context.Context, guardianIDs, studentIDs []int64) ([]domain.GuardianPortalContact, domain.OperationStats, error) {
	result := []domain.GuardianPortalContact{}
	if len(guardianIDs) == 0 && len(studentIDs) == 0 {
		return result, domain.OperationStats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []struct {
		GuardianProfileID   int64
		TenantID            int64
		AccountID           int64
		FirstName, LastName string
		Email, PortalLocale *string
		StudentID           *int64
		PortalPermission    json.RawMessage `bun:"portal_permission,type:jsonb"`
	}
	query := db.NewSelect().TableExpr("users.guardian_profiles AS gp").
		ColumnExpr("gp.id AS guardian_profile_id, gp.tenant_id, gp.account_id, gp.first_name, gp.last_name, gp.email, gp.portal_locale, sg.student_id, sg.permissions -> 'parent_portal.access' AS portal_permission").
		Join("LEFT JOIN (?) AS sg ON sg.guardian_profile_id = gp.id AND sg.tenant_id = gp.tenant_id", guardianlinkview.Query(db, tenantID)).
		Where("gp.account_id IS NOT NULL").
		WhereGroup(" AND ", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("gp.id IN (?)", bun.List(guardianIDs)).WhereOr("sg.student_id IN (?)", bun.List(studentIDs))
		})
	if tenantID > 0 {
		query = query.Where("gp.tenant_id = ?", tenantID)
	}
	started := time.Now()
	err = query.OrderExpr("gp.id, sg.student_id").Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, Rows: int64(len(rows)), StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("people directory postgres: guardian portal contacts: %w", err)
	}
	for _, row := range rows {
		result = append(result, domain.GuardianPortalContact{
			GuardianProfileID: row.GuardianProfileID, TenantID: row.TenantID, AccountID: row.AccountID,
			FirstName: row.FirstName, LastName: row.LastName, Email: row.Email, PortalLocale: row.PortalLocale,
			StudentID:        row.StudentID,
			PortalPermission: row.PortalPermission,
		})
	}
	stats.Rows = int64(len(result))
	return result, stats, nil
}

func (s *GuardianStore) FindPortalMemberships(ctx context.Context, accountIDs []int64) (map[int64][]int64, error) {
	if s.memberships == nil {
		return nil, fmt.Errorf("people directory postgres: portal membership query is required")
	}
	return s.memberships(ctx, accountIDs)
}
