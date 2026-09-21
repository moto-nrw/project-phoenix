package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// Effective admin authority matches authorize.HasEffectiveAdminScope: an
// admin role, or admin:* / *:* via a role or a granted direct permission.
// Every grant is paired with the active mapping's school, not just its account.
const effectiveAdminAccountsSQL = `SELECT DISTINCT a.id
FROM auth.accounts a
JOIN auth.account_tenants membership ON membership.account_id = a.id
WHERE a.active = TRUE AND membership.status = 'active'
AND (? <= 0 OR membership.tenant_id = ?)
AND (EXISTS (
 SELECT 1 FROM auth.account_roles ar
 JOIN auth.roles r ON r.id = ar.role_id
 LEFT JOIN auth.role_permissions rp ON rp.role_id = ar.role_id
 LEFT JOIN auth.permissions p ON p.id = rp.permission_id
 WHERE ar.account_id = a.id AND ar.tenant_id = membership.tenant_id
 AND (LOWER(r.name) = 'admin' OR (p.resource IN ('admin', '*') AND p.action = '*'))
) OR EXISTS (
 SELECT 1 FROM auth.account_permissions ap
 JOIN auth.permissions p ON p.id = ap.permission_id
 WHERE ap.account_id = a.id AND ap.tenant_id = membership.tenant_id
 AND ap.granted = TRUE AND p.resource IN ('admin', '*') AND p.action = '*'
))`

func (s *Store) ListEffectiveAdminAccountIDs(ctx context.Context, tenantID int64) ([]int64, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var ids []int64
	started := time.Now()
	err = db.NewRaw(effectiveAdminAccountsSQL, tenantID, tenantID).Scan(ctx, &ids)
	stats := domain.OperationStats{Queries: 1, Rows: int64(len(ids)), StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("list effective admin account IDs: %w", err)
	}
	return ids, stats, nil
}
