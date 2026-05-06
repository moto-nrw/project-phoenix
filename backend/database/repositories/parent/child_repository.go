// Package parent contains repositories for the cross-tenant
// guardian-portal endpoints. Every method MUST be invoked from inside
// a tenant.WithAdminTx — the queries intentionally span tenant_id
// boundaries scoped only by auth.account_tenants membership, which
// requires BYPASSRLS to traverse.
package parent

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
)

// ChildRepository implements parentModels.ChildRepository.
type ChildRepository struct {
	db *bun.DB
}

// NewChildRepository wires a fresh repository.
func NewChildRepository(db *bun.DB) parentModels.ChildRepository {
	return &ChildRepository{db: db}
}

// ListByAccount executes the cross-tenant join from auth.account_tenants
// down to users.students for a single account. Single SQL query, single
// round-trip — N+1 queries (one per tenant) would be cleaner per Rule 11
// but the only safe filter for "students this parent can see" is the
// account_tenants membership, and that's a JOIN not a tenant context.
//
// Soft-deleted person rows are filtered with p.deleted_at IS NULL.
// Inactive account_tenants are excluded so a parent who lost access to
// a school doesn't continue seeing its children.
//
// Result ordering: school name, then student first/last so the dashboard
// can render with stable grouping.
func (r *ChildRepository) ListByAccount(ctx context.Context, accountID int64) ([]*parentModels.ChildSummary, error) {
	if accountID <= 0 {
		return nil, fmt.Errorf("parent: account_id must be positive")
	}

	const query = `
		SELECT
			s.id           AS student_id,
			s.tenant_id    AS tenant_id,
			p.first_name   AS first_name,
			p.last_name    AS last_name,
			COALESCE(s.school_class, '') AS school_class,
			s.status       AS status,
			s.enrolled_from  AS enrolled_from,
			s.enrolled_until AS enrolled_until,
			COALESCE(sch.name, '')  AS school_name,
			COALESCE(sch.slug, '')  AS school_slug
		FROM auth.account_tenants AS at
		JOIN users.guardian_profiles AS gp
			ON gp.account_id = at.account_id
			AND gp.tenant_id  = at.tenant_id
		JOIN users.students_guardians AS sg
			ON sg.guardian_profile_id = gp.id
			AND sg.tenant_id          = at.tenant_id
		JOIN users.students AS s
			ON s.id        = sg.student_id
			AND s.tenant_id = at.tenant_id
		JOIN users.persons AS p
			ON p.id        = s.person_id
			AND p.tenant_id = at.tenant_id
		LEFT JOIN platform.schools AS sch
			ON sch.id = at.tenant_id
		WHERE at.account_id = ?
		  AND at.status     = 'active'
		  AND p.deleted_at IS NULL
		ORDER BY school_name, first_name, last_name
	`

	rows, err := base.GetDB(ctx, r.db).QueryContext(ctx, query, accountID)
	if err != nil {
		return nil, fmt.Errorf("parent: list children: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []*parentModels.ChildSummary
	for rows.Next() {
		c := &parentModels.ChildSummary{}
		if err := rows.Scan(
			&c.StudentID,
			&c.TenantID,
			&c.FirstName,
			&c.LastName,
			&c.SchoolClass,
			&c.Status,
			&c.EnrolledFrom,
			&c.EnrolledUntil,
			&c.SchoolName,
			&c.SchoolSlug,
		); err != nil {
			return nil, fmt.Errorf("parent: scan child row: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("parent: iterate child rows: %w", err)
	}
	return out, nil
}
