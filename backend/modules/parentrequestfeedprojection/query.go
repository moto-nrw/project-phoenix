// Package parentrequestfeedprojection is the tenant-safe read model for the
// five parent-request queues surfaced by the personal RSS feed.
package parentrequestfeedprojection

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

type Entry struct {
	Kind      string    `bun:"kind"`
	ID        int64     `bun:"id"`
	CreatedAt time.Time `bun:"created_at"`
}

type Access struct {
	GeneralRequests    bool   `bun:"general_requests"`
	EnrollmentRequests bool   `bun:"enrollment_requests"`
	SchoolName         string `bun:"school_name"`
	Subdomain          string `bun:"subdomain"`
}

// ResolveAccess returns the current school-wide request permissions for one
// active account-to-school mapping. Group-scoped request review deliberately
// does not qualify for this feed.
func ResolveAccess(ctx context.Context, db bun.IDB, tenantID, accountID int64) (Access, bool, error) {
	row := Access{}
	err := db.NewRaw(`
		WITH effective_permissions AS (
			SELECT permission.name
			FROM auth.account_permissions AS account_permission
			JOIN auth.permissions AS permission ON permission.id = account_permission.permission_id
			WHERE account_permission.account_id = ?
			  AND account_permission.tenant_id = ?
			  AND account_permission.granted = TRUE
			UNION
			SELECT permission.name
			FROM auth.account_roles AS account_role
			JOIN auth.role_permissions AS role_permission ON role_permission.role_id = account_role.role_id
			JOIN auth.permissions AS permission ON permission.id = role_permission.permission_id
			WHERE account_role.account_id = ?
			  AND account_role.tenant_id = ?
		), school_wide_roles AS (
			SELECT role.name
			FROM auth.account_roles AS account_role
			JOIN auth.roles AS role ON role.id = account_role.role_id
			WHERE account_role.account_id = ?
			  AND account_role.tenant_id = ?
		)
		SELECT
			(EXISTS (SELECT 1 FROM school_wide_roles WHERE LOWER(name) = 'admin')
			 OR EXISTS (SELECT 1 FROM effective_permissions WHERE name IN ('admin:*', '*:*'))) AS general_requests,
			(EXISTS (SELECT 1 FROM school_wide_roles WHERE LOWER(name) = 'admin')
			 OR EXISTS (SELECT 1 FROM effective_permissions WHERE name IN ('config:manage', 'admin:*', '*:*'))) AS enrollment_requests,
			school.name AS school_name,
			school.subdomain
		FROM auth.accounts AS account
		JOIN platform.schools AS school ON school.id = ?
		WHERE account.id = ?
		  AND account.active = TRUE
		  AND school.active = TRUE
		  AND school.deleted_at IS NULL
		  AND EXISTS (
			SELECT 1
			FROM auth.account_tenants AS account_tenant
			WHERE account_tenant.account_id = account.id
			  AND account_tenant.tenant_id = school.id
			  AND account_tenant.status = 'active'
		  )
	`, accountID, tenantID, accountID, tenantID, accountID, tenantID, tenantID, accountID).Scan(ctx, &row)
	if errors.Is(err, sql.ErrNoRows) {
		return Access{}, false, nil
	}
	if err != nil {
		return Access{}, false, fmt.Errorf("parent request feed projection: resolve access: %w", err)
	}
	return row, true, nil
}

func ListNewParentRequests(ctx context.Context, db bun.IDB, tenantID int64, since time.Time, generalRequests, enrollmentRequests bool) ([]Entry, error) {
	rows := []Entry{}
	err := db.NewRaw(`
		SELECT kind, id, created_at
		FROM (
			SELECT 'master_data'::text AS kind, id, created_at
			FROM users.student_data_change_requests
			WHERE tenant_id = ? AND created_at >= ? AND status = 'pending' AND ?
			UNION ALL
			SELECT 'care_schedule'::text, id, created_at
			FROM schedule.care_schedule_change_requests
			WHERE tenant_id = ? AND created_at >= ? AND status = 'pending' AND ?
			UNION ALL
			SELECT 'offering'::text, id, created_at
			FROM enrollment.offering_change_requests
			WHERE tenant_id = ? AND created_at >= ? AND status = 'pending' AND ?
			UNION ALL
			SELECT 'excused_absence'::text, id, created_at
			FROM active.excused_absence_requests
			WHERE tenant_id = ? AND created_at >= ? AND status = 'pending' AND ?
			UNION ALL
			SELECT 'enrollment'::text, id, created_at
			FROM enrollment.change_requests
			WHERE tenant_id = ? AND created_at >= ? AND origin = 'parent' AND status = 'pending_review' AND ?
		) AS requests
		ORDER BY created_at DESC, kind ASC, id DESC
	`,
		tenantID, since, generalRequests,
		tenantID, since, generalRequests,
		tenantID, since, generalRequests,
		tenantID, since, generalRequests,
		tenantID, since, enrollmentRequests,
	).Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("parent request feed projection: list requests: %w", err)
	}
	return rows, nil
}
