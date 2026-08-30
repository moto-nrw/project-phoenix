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
// through this picker. A hidden school stays visible only to an account that
// holds an actual FAMILY link there — a guardian_profile with at least one
// students_guardians row, backed by an ACTIVE auth.account_tenants mapping
// (guard.has_family_link) — so existing families keep seeing their own
// school's re-enrollment phases.
//
// The membership mapping alone (at.account_id / the already_linked display
// flag) is deliberately NOT the visibility key: auth.account_tenants also
// carries staff, admins, and any other role at that tenant, so keying on it
// leaked a hidden school's name, phase names, and service dates to accounts
// with no guardian relationship at all. already_linked keeps its membership
// meaning — it only orders and labels rows that already passed this gate.
//
// Eligibility (#1663): a phase whose audience is linked_parents OR
// existing_students is listed only when the account holds a guardian
// relationship at the school — backed by an ACTIVE auth.account_tenants
// mapping — that grants parent_portal.enrollment.submit. The active-mapping
// conjunction stops a former guardian, whose historical guardian rows linger
// after the mapping was deactivated, from still submitting those phases.
//
// existing_students needs the SAME permission requirement as linked_parents,
// including for an account with no guardian relationship at that school at
// all: such an account can never complete one of those phases. A child it
// does not already have fails the enrolled-student gate (ErrChildNotEnrolled),
// and a child it does have pins a matched student the account holds no
// per-student submit permission on (ErrChildEnrollmentNotPermitted). Listing
// it would only advertise a guaranteed dead end.
//
// existing_students goes one step further and requires the permission-granting
// relationship to point at a student that is still ACTIVE or PENDING (its
// person not soft-deleted) — the same "enrolled" scope
// ExistsEnrolledByNameAndBirthday / FindEnrolledStudentIDByNameAndBirthday use
// on submit. An account whose only submit-permitted relationship is to an
// inactive (or otherwise un-enrolled) child would otherwise see the phase, find
// that child filtered out of the form, and hit ErrChildNotEnrolled on submit:
// another guaranteed dead end. linked_parents keeps the looser probe on
// purpose — it lets a parent enroll a genuinely NEW sibling, so an inactive
// child still legitimately grants access there.
//
// The denial stays deliberately scoped to those two audiences: a pickup-only
// relationship on one child must NOT hide open / new_students phases the same
// account can bootstrap a genuinely new child into and submit via a direct URL
// — the authenticated submit path applies no such account-wide denial
// (per-child authorization happens inside Submit), so the picker must not
// either. Genuinely new-school applicants (no guardian rows at all) still see
// open phases.
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
		SchoolSubdomain   string        `bun:"school_subdomain"`
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

	// The caller applies the enrollment master switch through the settings
	// platform's typed query seam. This repository owns only the care-plan
	// projection and must not read config.setting_values directly.
	//
	// The LATERAL guard resolves the account's submit permission once per
	// phase row; the audience WHERE clause below consumes it (see doc
	// comment).
	const query = `
		SELECT
			sch.id        AS school_id,
			sch.name      AS school_name,
			sch.slug      AS school_slug,
			sch.subdomain AS school_subdomain,
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
					JOIN auth.account_tenants AS act
						ON act.tenant_id  = gp.tenant_id
						AND act.account_id = gp.account_id
						AND act.status     = 'active'
					WHERE gp.tenant_id = ph.tenant_id
						AND gp.account_id = ?
						AND COALESCE((sg.permissions ->> ?)::boolean, false) = TRUE
				) AS has_submit_permission,
				EXISTS (
					SELECT 1
					FROM users.guardian_profiles AS gp
					JOIN users.students_guardians AS sg
						ON sg.guardian_profile_id = gp.id
						AND sg.tenant_id = gp.tenant_id
					JOIN users.students AS st
						ON st.id = sg.student_id
						AND st.tenant_id = sg.tenant_id
						AND st.status IN ('active', 'pending')
					JOIN users.persons AS pe
						ON pe.id = st.person_id
						AND pe.deleted_at IS NULL
					JOIN auth.account_tenants AS act
						ON act.tenant_id  = gp.tenant_id
						AND act.account_id = gp.account_id
						AND act.status     = 'active'
					WHERE gp.tenant_id = ph.tenant_id
						AND gp.account_id = ?
						AND COALESCE((sg.permissions ->> ?)::boolean, false) = TRUE
				) AS has_enrolled_submit_permission,
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
				) AS has_family_link
		) AS guard
		WHERE ph.is_active = TRUE
		  AND sch.active   = TRUE
		  AND sch.deleted_at IS NULL
		  AND (sch.hidden = FALSE OR guard.has_family_link)
		  AND (ph.enrollment_open_at IS NULL OR ph.enrollment_open_at <= NOW())
		  AND (ph.enrollment_close_at IS NULL OR ph.enrollment_close_at >= NOW())
		  AND (ph.audience <> 'linked_parents' OR guard.has_submit_permission)
		  AND (ph.audience <> 'existing_students' OR guard.has_enrolled_submit_permission)
		ORDER BY already_linked DESC, sch.name, ph.service_start_date
	`

	var rows []row
	if err := base.GetDB(ctx, r.db).NewRaw(query,
		accountID,
		accountID,
		authorize.GuardianPermissionEnrollmentSubmit,
		accountID,
		authorize.GuardianPermissionEnrollmentSubmit,
		accountID,
	).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("parent: list enrollable phases: %w", err)
	}

	out := make([]*parentModels.EnrollablePhase, 0, len(rows))
	for _, rr := range rows {
		out = append(out, &parentModels.EnrollablePhase{
			SchoolID:          rr.SchoolID,
			SchoolName:        rr.SchoolName,
			SchoolSlug:        rr.SchoolSlug,
			SchoolSubdomain:   rr.SchoolSubdomain,
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
// enrollment submit and form-load paths gate on (#1663). One round trip,
// four EXISTS probes. HasSubmitPermission additionally requires an ACTIVE
// auth.account_tenants mapping so a deactivated guardian's lingering
// guardian rows cannot report submit authority.
//
// HasEnrolledSubmitPermission is the existing_students counterpart and MUST
// stay identical to guard.has_enrolled_submit_permission in ListEnrollable
// above: it additionally requires the permission-granting relationship to
// point at an ACTIVE or PENDING student whose person is not soft-deleted.
// The picker and the authenticated form gate have to agree — a form that
// loads for a phase the picker hides is a dead end whose submit always
// fails. Cross-tenant — must run inside tenant.WithAdminTx.
func (r *EnrollablePhaseRepository) GuardianSubmitStatus(ctx context.Context, accountID, tenantID int64) (*parentModels.GuardianSubmitStatus, error) {
	if accountID <= 0 || tenantID <= 0 {
		return nil, fmt.Errorf("parent: account_id and tenant_id must be positive")
	}

	type row struct {
		Linked                      bool `bun:"linked"`
		HasGuardianLink             bool `bun:"has_guardian_link"`
		HasSubmitPermission         bool `bun:"has_submit_permission"`
		HasEnrolledSubmitPermission bool `bun:"has_enrolled_submit_permission"`
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
			) AS has_submit_permission,
			EXISTS (
				SELECT 1
				FROM users.guardian_profiles AS gp
				JOIN users.students_guardians AS sg
					ON sg.guardian_profile_id = gp.id
					AND sg.tenant_id = gp.tenant_id
				JOIN users.students AS st
					ON st.id = sg.student_id
					AND st.tenant_id = sg.tenant_id
					AND st.status IN ('active', 'pending')
				JOIN users.persons AS pe
					ON pe.id = st.person_id
					AND pe.deleted_at IS NULL
				JOIN auth.account_tenants AS act
					ON act.tenant_id  = gp.tenant_id
					AND act.account_id = gp.account_id
					AND act.status     = 'active'
				WHERE gp.tenant_id = ? AND gp.account_id = ?
					AND COALESCE((sg.permissions ->> ?)::boolean, false) = TRUE
			) AS has_enrolled_submit_permission
	`

	var out row
	if err := base.GetDB(ctx, r.db).NewRaw(query,
		tenantID, accountID,
		tenantID, accountID,
		tenantID, accountID,
		authorize.GuardianPermissionEnrollmentSubmit,
		tenantID, accountID,
		authorize.GuardianPermissionEnrollmentSubmit,
	).Scan(ctx, &out); err != nil {
		return nil, fmt.Errorf("parent: guardian submit status: %w", err)
	}

	return &parentModels.GuardianSubmitStatus{
		Linked:                      out.Linked,
		HasGuardianLink:             out.HasGuardianLink,
		HasSubmitPermission:         out.HasSubmitPermission,
		HasEnrolledSubmitPermission: out.HasEnrolledSubmitPermission,
	}, nil
}
