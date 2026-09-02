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

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
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
// Alumni are excluded for the same reason. Graduation is a soft delete:
// the student row survives but the child has left the OGS, and every
// staff-facing read path already filters them out. Leaving them visible
// here would let a guardian keep filing sick notes, care exceptions and
// notes for a child the school no longer cares for (#405 review).
//
// Result ordering: school name, then student first/last so the dashboard
// can render with stable grouping.
//
// Implementation note: the rows are scanned via bun.NewRaw(...).Scan(...)
// rather than a plain database/sql QueryContext loop. pgdriver returns
// Postgres `date` columns (s.enrolled_from / enrolled_until) as Go
// strings, and database/sql can't auto-convert those into *time.Time;
// bun's scanner handles the conversion transparently via the bun tags.
func (r *ChildRepository) ListByAccount(ctx context.Context, accountID int64) ([]*parentModels.ChildSummary, error) {
	if accountID <= 0 {
		return nil, fmt.Errorf("parent: account_id must be positive")
	}

	type row struct {
		StudentID         int64          `bun:"student_id"`
		TenantID          int64          `bun:"tenant_id"`
		PersonID          int64          `bun:"person_id"`
		GuardianProfileID int64          `bun:"guardian_profile_id"`
		SchoolClass       string         `bun:"school_class"`
		Status            string         `bun:"status"`
		EnrolledFrom      *timezone.Date `bun:"enrolled_from"`
		EnrolledUntil     *timezone.Date `bun:"enrolled_until"`
		Permissions       map[string]any `bun:"guardian_permissions"`
	}

	// The person names, the deleted-person filter, and the name order are
	// applied by the composition layer through the People Directory (#2661).
	const query = `
		SELECT
			s.id           AS student_id,
			s.tenant_id    AS tenant_id,
			s.person_id    AS person_id,
			gp.id          AS guardian_profile_id,
			COALESCE(s.school_class, '') AS school_class,
			s.status       AS status,
			s.enrolled_from  AS enrolled_from,
			s.enrolled_until AS enrolled_until,
			COALESCE(sg.permissions, '{}'::jsonb) AS guardian_permissions
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
		WHERE at.account_id = ?
		  AND at.status     = 'active'
		  AND s.status     <> ?
		  AND COALESCE((sg.permissions ->> ?)::boolean, false) = TRUE
		ORDER BY s.tenant_id, s.id
	`

	var rows []row
	if err := base.GetDB(ctx, r.db).NewRaw(query,
		accountID,
		string(usersModels.StudentStatusAlumnus),
		authorize.GuardianPermissionPortalAccess,
	).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("parent: list children: %w", err)
	}

	out := make([]*parentModels.ChildSummary, 0, len(rows))
	for _, rr := range rows {
		out = append(out, &parentModels.ChildSummary{
			StudentID:           rr.StudentID,
			TenantID:            rr.TenantID,
			PersonID:            rr.PersonID,
			GuardianProfileID:   rr.GuardianProfileID,
			SchoolClass:         rr.SchoolClass,
			Status:              rr.Status,
			EnrolledFrom:        rr.EnrolledFrom,
			EnrolledUntil:       rr.EnrolledUntil,
			GuardianPermissions: rr.Permissions,
		})
	}
	return out, nil
}

// FindForAccount resolves a single child the account is a guardian of.
// Same cross-tenant join as ListByAccount, narrowed to one student id.
// Returns nil, nil when the student is not linked to the account so the
// caller can map "not yours" to a 403/404 without leaking existence.
//
// This is THE authorization gate for every per-child parent write
// (services/parent resolvePermittedChild), so the alumnus exclusion that
// hides graduates from the dashboard also stops the writes: a guardian
// cannot submit a future-dated sick note or care exception for a child
// who has left the school (#405 review).
//
// MUST run inside a tenant.WithAdminTx — the join spans tenant_id
// boundaries scoped only by auth.account_tenants membership.
func (r *ChildRepository) FindForAccount(ctx context.Context, accountID, studentID int64) (*parentModels.ChildSummary, error) {
	if accountID <= 0 {
		return nil, fmt.Errorf("parent: account_id must be positive")
	}
	if studentID <= 0 {
		return nil, fmt.Errorf("parent: student_id must be positive")
	}

	type row struct {
		StudentID         int64          `bun:"student_id"`
		TenantID          int64          `bun:"tenant_id"`
		PersonID          int64          `bun:"person_id"`
		GuardianProfileID int64          `bun:"guardian_profile_id"`
		SchoolClass       string         `bun:"school_class"`
		Status            string         `bun:"status"`
		EnrolledFrom      *timezone.Date `bun:"enrolled_from"`
		EnrolledUntil     *timezone.Date `bun:"enrolled_until"`
		Permissions       map[string]any `bun:"guardian_permissions"`
	}

	const query = `
		SELECT
			s.id           AS student_id,
			s.tenant_id    AS tenant_id,
			s.person_id    AS person_id,
			gp.id          AS guardian_profile_id,
			COALESCE(s.school_class, '') AS school_class,
			s.status       AS status,
			s.enrolled_from  AS enrolled_from,
			s.enrolled_until AS enrolled_until,
			COALESCE(sg.permissions, '{}'::jsonb) AS guardian_permissions
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
		WHERE at.account_id = ?
		  AND s.id          = ?
		  AND at.status     = 'active'
		  AND s.status     <> ?
		  AND COALESCE((sg.permissions ->> ?)::boolean, false) = TRUE
		LIMIT 1
	`

	var rows []row
	if err := base.GetDB(ctx, r.db).NewRaw(query,
		accountID,
		studentID,
		string(usersModels.StudentStatusAlumnus),
		authorize.GuardianPermissionPortalAccess,
	).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("parent: find child for account: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}

	rr := rows[0]
	return &parentModels.ChildSummary{
		StudentID:           rr.StudentID,
		TenantID:            rr.TenantID,
		PersonID:            rr.PersonID,
		GuardianProfileID:   rr.GuardianProfileID,
		SchoolClass:         rr.SchoolClass,
		Status:              rr.Status,
		EnrolledFrom:        rr.EnrolledFrom,
		EnrolledUntil:       rr.EnrolledUntil,
		GuardianPermissions: rr.Permissions,
	}, nil
}
