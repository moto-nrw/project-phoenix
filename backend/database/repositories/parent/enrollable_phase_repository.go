package parent

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
)

// EnrollablePhaseRepository implements parentModels.EnrollablePhaseRepository.
type EnrollablePhaseRepository struct {
	db *bun.DB
}

// NewEnrollablePhaseRepository wires a fresh repository.
func NewEnrollablePhaseRepository(db *bun.DB) parentModels.EnrollablePhaseRepository {
	return &EnrollablePhaseRepository{db: db}
}

// ListEnrollable returns one row per (school, active+open phase) pair
// the account is eligible for.
//
// "Open" = phase.is_active AND now BETWEEN enrollment_open_at and
// enrollment_close_at (treating NULL bounds as open-ended). The query
// uses the parent's account_id to LEFT JOIN against account_tenants
// so each row carries an already_linked flag.
//
// Hidden schools (platform.schools.hidden) are excluded from cross-school
// discovery the same way the public tenant listing excludes them: an
// unlinked parent must not learn a hidden school's name or phase details
// through this picker. A hidden school stays visible only to a parent who
// is ALREADY an active member of it (at.account_id IS NOT NULL / the
// already_linked flag), so existing families keep seeing their own
// school's re-enrollment phases.
//
// Eligibility (#1663): a phase with audience=linked_parents is listed
// only when the account holds a guardian relationship at the school —
// backed by an ACTIVE auth.account_tenants mapping — that grants
// parent_portal.enrollment.submit. The active-mapping conjunction stops
// a former guardian, whose historical guardian rows linger after the
// mapping was deactivated, from still submitting linked_parents phases.
// Independently, a school where the account HAS guardian relationships
// but NONE grants that permission (via an active mapping) is dropped for
// every audience — an explicitly revoked or lapsed submit permission
// must not be bypassable through the picker. Genuinely new-school
// applicants (no guardian rows at all) still see open phases.
//
// Cross-tenant query — must run inside tenant.WithAdminTx.
func (r *EnrollablePhaseRepository) ListEnrollable(ctx context.Context, accountID int64) ([]*parentModels.EnrollablePhase, error) {
	if accountID <= 0 {
		return nil, fmt.Errorf("parent: account_id must be positive")
	}

	type row struct {
		SchoolID          int64         `bun:"school_id"`
		SchoolName        string        `bun:"school_name"`
		SchoolSlug        string        `bun:"school_slug"`
		PhaseID           int64         `bun:"phase_id"`
		PhaseName         string        `bun:"phase_name"`
		PhaseKind         string        `bun:"phase_kind"`
		ServiceStartDate  timezone.Date `bun:"service_start_date"`
		ServiceEndDate    timezone.Date `bun:"service_end_date"`
		EnrollmentOpenAt  *time.Time    `bun:"enrollment_open_at"`
		EnrollmentCloseAt *time.Time    `bun:"enrollment_close_at"`
		AlreadyLinked     bool          `bun:"already_linked"`
		Audience          string        `bun:"audience"`
	}

	// We INNER JOIN config.setting_values on enrollment.enabled=true so a
	// tenant whose master toggle is off (or never set) drops out of the
	// list entirely. The registry default for enrollment.enabled is
	// false, so "no override" must be treated as disabled.
	//
	// The LATERAL guard resolves the account's guardian facts once per
	// phase row; both WHERE clauses below consume it (see doc comment).
	const query = `
		SELECT
			sch.id        AS school_id,
			sch.name      AS school_name,
			sch.slug      AS school_slug,
			ph.id         AS phase_id,
			ph.name       AS phase_name,
			ph.kind       AS phase_kind,
			ph.service_start_date AS service_start_date,
			ph.service_end_date   AS service_end_date,
			ph.enrollment_open_at  AS enrollment_open_at,
			ph.enrollment_close_at AS enrollment_close_at,
			ph.audience   AS audience,
			(at.account_id IS NOT NULL) AS already_linked
		FROM enrollment.phases AS ph
		JOIN platform.schools AS sch
			ON sch.id = ph.tenant_id
		JOIN config.setting_values AS sv
			ON sv.tenant_id = ph.tenant_id
			AND sv.setting_key = 'enrollment.enabled'
			AND sv.value::text = 'true'
		LEFT JOIN auth.account_tenants AS at
			ON at.tenant_id  = ph.tenant_id
			AND at.account_id = ?
			AND at.status     = 'active'
		CROSS JOIN LATERAL (
			SELECT
				EXISTS (
					SELECT 1
					FROM users.guardian_profiles AS gp
					JOIN users.students_guardians AS sg
						ON sg.guardian_profile_id = gp.id
						AND sg.tenant_id = gp.tenant_id
					WHERE gp.tenant_id = ph.tenant_id
						AND gp.account_id = ?
				) AS has_guardian_link,
				EXISTS (
					SELECT 1
					FROM users.guardian_profiles AS gp
					JOIN users.students_guardians AS sg
						ON sg.guardian_profile_id = gp.id
						AND sg.tenant_id = gp.tenant_id
					JOIN auth.account_tenants AS act
						ON act.tenant_id  = gp.tenant_id
						AND act.account_id = gp.account_id
						AND act.status     = 'active'
					WHERE gp.tenant_id = ph.tenant_id
						AND gp.account_id = ?
						AND COALESCE((sg.permissions ->> ?)::boolean, false) = TRUE
				) AS has_submit_permission
		) AS guard
		WHERE ph.is_active = TRUE
		  AND sch.active   = TRUE
		  AND sch.deleted_at IS NULL
		  AND (sch.hidden = FALSE OR at.account_id IS NOT NULL)
		  AND (ph.enrollment_open_at IS NULL OR ph.enrollment_open_at <= NOW())
		  AND (ph.enrollment_close_at IS NULL OR ph.enrollment_close_at >= NOW())
		  AND (ph.audience <> 'linked_parents' OR guard.has_submit_permission)
		  AND (NOT guard.has_guardian_link OR guard.has_submit_permission)
		ORDER BY already_linked DESC, sch.name, ph.service_start_date
	`

	var rows []row
	if err := base.GetDB(ctx, r.db).NewRaw(query,
		accountID,
		accountID,
		accountID,
		authorize.GuardianPermissionEnrollmentSubmit,
	).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("parent: list enrollable phases: %w", err)
	}

	out := make([]*parentModels.EnrollablePhase, 0, len(rows))
	for _, rr := range rows {
		out = append(out, &parentModels.EnrollablePhase{
			SchoolID:          rr.SchoolID,
			SchoolName:        rr.SchoolName,
			SchoolSlug:        rr.SchoolSlug,
			PhaseID:           rr.PhaseID,
			PhaseName:         rr.PhaseName,
			PhaseKind:         rr.PhaseKind,
			ServiceStartDate:  rr.ServiceStartDate,
			ServiceEndDate:    rr.ServiceEndDate,
			EnrollmentOpenAt:  rr.EnrollmentOpenAt,
			EnrollmentCloseAt: rr.EnrollmentCloseAt,
			AlreadyLinked:     rr.AlreadyLinked,
			Audience:          rr.Audience,
		})
	}
	return out, nil
}

// GuardianSubmitStatus resolves the (account, school) facts the parent
// enrollment submit path gates on (#1663). One round trip, three
// EXISTS probes. HasSubmitPermission additionally requires an ACTIVE
// auth.account_tenants mapping so a deactivated guardian's lingering
// guardian rows cannot report submit authority. Cross-tenant — must run
// inside tenant.WithAdminTx.
func (r *EnrollablePhaseRepository) GuardianSubmitStatus(ctx context.Context, accountID, tenantID int64) (*parentModels.GuardianSubmitStatus, error) {
	if accountID <= 0 || tenantID <= 0 {
		return nil, fmt.Errorf("parent: account_id and tenant_id must be positive")
	}

	type row struct {
		Linked              bool `bun:"linked"`
		HasGuardianLink     bool `bun:"has_guardian_link"`
		HasSubmitPermission bool `bun:"has_submit_permission"`
	}

	const query = `
		SELECT
			EXISTS (
				SELECT 1 FROM auth.account_tenants AS at
				WHERE at.tenant_id = ? AND at.account_id = ? AND at.status = 'active'
			) AS linked,
			EXISTS (
				SELECT 1
				FROM users.guardian_profiles AS gp
				JOIN users.students_guardians AS sg
					ON sg.guardian_profile_id = gp.id
					AND sg.tenant_id = gp.tenant_id
				WHERE gp.tenant_id = ? AND gp.account_id = ?
			) AS has_guardian_link,
			EXISTS (
				SELECT 1
				FROM users.guardian_profiles AS gp
				JOIN users.students_guardians AS sg
					ON sg.guardian_profile_id = gp.id
					AND sg.tenant_id = gp.tenant_id
				JOIN auth.account_tenants AS act
					ON act.tenant_id  = gp.tenant_id
					AND act.account_id = gp.account_id
					AND act.status     = 'active'
				WHERE gp.tenant_id = ? AND gp.account_id = ?
					AND COALESCE((sg.permissions ->> ?)::boolean, false) = TRUE
			) AS has_submit_permission
	`

	var out row
	if err := base.GetDB(ctx, r.db).NewRaw(query,
		tenantID, accountID,
		tenantID, accountID,
		tenantID, accountID,
		authorize.GuardianPermissionEnrollmentSubmit,
	).Scan(ctx, &out); err != nil {
		return nil, fmt.Errorf("parent: guardian submit status: %w", err)
	}

	return &parentModels.GuardianSubmitStatus{
		Linked:              out.Linked,
		HasGuardianLink:     out.HasGuardianLink,
		HasSubmitPermission: out.HasSubmitPermission,
	}, nil
}
