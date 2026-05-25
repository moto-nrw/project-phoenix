package parent

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
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

// ListEnrollable returns one row per (school, active+open phase) pair.
//
// "Open" = phase.is_active AND now BETWEEN enrollment_open_at and
// enrollment_close_at (treating NULL bounds as open-ended). The query
// uses the parent's account_id to LEFT JOIN against account_tenants
// so each row carries an already_linked flag.
//
// Cross-tenant query — must run inside tenant.WithAdminTx.
func (r *EnrollablePhaseRepository) ListEnrollable(ctx context.Context, accountID int64) ([]*parentModels.EnrollablePhase, error) {
	if accountID <= 0 {
		return nil, fmt.Errorf("parent: account_id must be positive")
	}

	type row struct {
		SchoolID          int64      `bun:"school_id"`
		SchoolName        string     `bun:"school_name"`
		SchoolSlug        string     `bun:"school_slug"`
		PhaseID           int64      `bun:"phase_id"`
		PhaseName         string     `bun:"phase_name"`
		PhaseKind         string     `bun:"phase_kind"`
		ServiceStartDate  time.Time  `bun:"service_start_date"`
		ServiceEndDate    time.Time  `bun:"service_end_date"`
		EnrollmentOpenAt  *time.Time `bun:"enrollment_open_at"`
		EnrollmentCloseAt *time.Time `bun:"enrollment_close_at"`
		AlreadyLinked     bool       `bun:"already_linked"`
	}

	// We INNER JOIN config.setting_values on enrollment.enabled=true so a
	// tenant whose master toggle is off (or never set) drops out of the
	// list entirely. The registry default for enrollment.enabled is
	// false, so "no override" must be treated as disabled.
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
		WHERE ph.is_active = TRUE
		  AND sch.active   = TRUE
		  AND sch.deleted_at IS NULL
		  AND (ph.enrollment_open_at IS NULL OR ph.enrollment_open_at <= NOW())
		  AND (ph.enrollment_close_at IS NULL OR ph.enrollment_close_at >= NOW())
		ORDER BY already_linked DESC, sch.name, ph.service_start_date
	`

	var rows []row
	if err := base.GetDB(ctx, r.db).NewRaw(query, accountID).Scan(ctx, &rows); err != nil {
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
		})
	}
	return out, nil
}
