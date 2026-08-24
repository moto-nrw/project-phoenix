package enrollment

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

var errPhaseExpiryTenantRequired = errors.New("phase expiry report requires a tenant context")

type PhaseExpiryRepository struct {
	db *bun.DB
}

func NewPhaseExpiryRepository(db *bun.DB) enrollmentModels.PhaseExpiryRepository {
	return &PhaseExpiryRepository{db: db}
}

func (r *PhaseExpiryRepository) ListSnapshots(
	ctx context.Context,
	asOf, warningThrough timezone.Date,
) ([]*enrollmentModels.PhaseExpirySnapshot, error) {
	if asOf.IsZero() || warningThrough.IsZero() {
		return nil, errors.New("phase expiry report dates are required")
	}
	if warningThrough.Before(asOf) {
		return nil, errors.New("phase expiry warning horizon must not be before the report date")
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return nil, errPhaseExpiryTenantRequired
	}

	var rows []*enrollmentModels.PhaseExpirySnapshot
	err := base.GetDB(ctx, r.db).NewRaw(phaseExpiryQuery, tenantID, asOf, warningThrough).Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("list phase expiry snapshots for tenant %d: %w", tenantID, err)
	}
	return rows, nil
}

const phaseExpiryQuery = `
WITH report_parameters AS (
    SELECT
        ?::bigint AS tenant_id,
        ?::date AS as_of,
        ?::date AS warning_through
),
effective_source_bookings AS (
    SELECT
        source_phase.id AS source_phase_id,
        source_phase.name AS source_phase_name,
        source_phase.service_start_date AS source_start_date,
        source_phase.service_end_date AS source_end_date,
        request_child.id AS source_child_id,
        COALESCE(request_child.created_student_id, request_child.matched_student_id) AS student_id,
        expiry.first_affected_date
    FROM enrollment.phases AS source_phase
    CROSS JOIN report_parameters AS parameters
    JOIN enrollment.requests AS request
      ON request.tenant_id = source_phase.tenant_id
     AND request.phase_id = source_phase.id
    JOIN enrollment.request_children AS request_child
      ON request_child.tenant_id = request.tenant_id
     AND request_child.request_id = request.id
     AND request_child.status = 'approved'
    JOIN users.students AS student
      ON student.tenant_id = request_child.tenant_id
     AND student.id = COALESCE(request_child.created_student_id, request_child.matched_student_id)
    JOIN enrollment.request_child_offerings AS child_offering
      ON child_offering.tenant_id = request_child.tenant_id
     AND child_offering.request_child_id = request_child.id
    JOIN enrollment.care_offerings AS care_offering
      ON care_offering.tenant_id = child_offering.tenant_id
     AND care_offering.id = child_offering.care_offering_id
     AND care_offering.phase_id = source_phase.id
     AND care_offering.is_active
    CROSS JOIN LATERAL (
        SELECT MIN(source_phase.service_end_date + candidate.day_offset) AS first_affected_date
        FROM generate_series(1, 7) AS candidate(day_offset)
        WHERE EXTRACT(ISODOW FROM source_phase.service_end_date + candidate.day_offset)::integer IN (
            SELECT CASE LOWER(BTRIM(booked_day.value))
                WHEN 'mon' THEN 1
                WHEN 'tue' THEN 2
                WHEN 'wed' THEN 3
                WHEN 'thu' THEN 4
                WHEN 'fri' THEN 5
                WHEN 'sat' THEN 6
                WHEN 'sun' THEN 7
            END
            FROM jsonb_array_elements_text(
                CASE
                    WHEN jsonb_array_length(COALESCE(child_offering.selected_days, '[]'::jsonb)) > 0
                      OR care_offering.days_of_week_mode <> 'fixed'
                    THEN COALESCE(child_offering.selected_days, '[]'::jsonb)
                    ELSE care_offering.available_days
                END
            ) AS booked_day(value)
        )
    ) AS expiry
    WHERE source_phase.tenant_id = parameters.tenant_id
      AND source_phase.kind = 'school_year'
      AND source_phase.service_end_date < parameters.warning_through
      AND (child_offering.valid_from IS NULL OR child_offering.valid_from <= source_phase.service_end_date)
      AND (child_offering.valid_until IS NULL OR child_offering.valid_until > source_phase.service_end_date)
      AND expiry.first_affected_date IS NOT NULL
      AND (
          (
              student.status = 'active'
              AND student.enrolled_from IS NOT NULL
              AND student.enrolled_from <= source_phase.service_end_date
              AND (
                  student.enrolled_until IS NULL
                  OR student.enrolled_until >= source_phase.service_end_date
              )
          )
          OR (
              student.status = 'pending'
              AND student.enrolled_from IS NOT NULL
              AND (
                  student.enrolled_until IS NULL
                  OR student.enrolled_until >= source_phase.service_end_date
              )
          )
          OR (
              student.status = 'inactive'
              AND parameters.as_of > source_phase.service_end_date
              AND student.enrolled_until = source_phase.service_end_date
          )
      )
),
source_students AS (
    SELECT
        source_phase_id,
        source_phase_name,
        source_start_date,
        source_end_date,
        student_id,
        MIN(first_affected_date) AS first_affected_date,
        ARRAY_AGG(DISTINCT source_child_id) AS source_child_ids
    FROM effective_source_bookings
    GROUP BY source_phase_id, source_phase_name, source_start_date, source_end_date, student_id
),
phase_summaries AS (
    SELECT
        source_phase_id,
        source_phase_name,
        source_start_date,
        source_end_date,
        MIN(first_affected_date) AS first_affected_date,
        COUNT(*)::integer AS affected_children
    FROM source_students
    GROUP BY source_phase_id, source_phase_name, source_start_date, source_end_date
    HAVING MIN(first_affected_date) <= (SELECT warning_through FROM report_parameters)
),
phases_with_successor AS (
    SELECT
        phase_summary.*,
        successor.id AS successor_phase_id,
        successor.name AS successor_phase_name,
        successor.service_start_date AS successor_start_date,
        successor.service_end_date AS successor_end_date
    FROM phase_summaries AS phase_summary
    LEFT JOIN LATERAL (
        SELECT candidate.*
        FROM enrollment.phases AS candidate
        WHERE candidate.tenant_id = (SELECT tenant_id FROM report_parameters)
          AND candidate.kind = 'school_year'
          AND candidate.id <> phase_summary.source_phase_id
          AND (
              candidate.rollover_source_phase_id = phase_summary.source_phase_id
              OR (
                  candidate.service_start_date > phase_summary.source_end_date
                  AND candidate.service_start_date <= phase_summary.first_affected_date
                  AND candidate.service_end_date >= phase_summary.first_affected_date
              )
          )
        ORDER BY
            (candidate.rollover_source_phase_id = phase_summary.source_phase_id) DESC,
            candidate.service_start_date,
            candidate.id
        LIMIT 1
    ) AS successor ON TRUE
)
SELECT
    phase.source_phase_id,
    phase.source_phase_name,
    phase.successor_phase_id,
    phase.successor_phase_name,
    phase.first_affected_date,
    phase.affected_children,
    CASE
        WHEN phase.successor_phase_id IS NULL THEN phase.affected_children
        ELSE (
            SELECT COUNT(*)::integer
            FROM source_students AS source_student
            WHERE source_student.source_phase_id = phase.source_phase_id
              AND NOT EXISTS (
                  SELECT 1
                  FROM enrollment.requests AS target_request
                  JOIN enrollment.request_children AS target_child
                    ON target_child.tenant_id = target_request.tenant_id
                   AND target_child.request_id = target_request.id
                  WHERE target_request.tenant_id = (SELECT tenant_id FROM report_parameters)
                    AND target_request.phase_id = phase.successor_phase_id
                    AND (
                        target_child.rollover_source_child_id = ANY(source_student.source_child_ids)
                        OR (
                            COALESCE(target_child.created_student_id, target_child.matched_student_id) = source_student.student_id
                            AND NOT EXISTS (
                                SELECT 1
                                FROM enrollment.requests AS lineage_request
                                JOIN enrollment.request_children AS lineage_child
                                  ON lineage_child.tenant_id = lineage_request.tenant_id
                                 AND lineage_child.request_id = lineage_request.id
                                WHERE lineage_request.tenant_id = (SELECT tenant_id FROM report_parameters)
                                  AND lineage_request.phase_id = phase.successor_phase_id
                                  AND lineage_child.rollover_source_child_id = ANY(source_student.source_child_ids)
                            )
                        )
                    )
                    AND (
                        target_child.status IN ('rejected', 'withdrawn')
                        AND NOT EXISTS (
                            SELECT 1
                            FROM enrollment.requests AS competing_request
                            JOIN enrollment.request_children AS competing_child
                              ON competing_child.tenant_id = competing_request.tenant_id
                             AND competing_child.request_id = competing_request.id
                            WHERE competing_request.tenant_id = (SELECT tenant_id FROM report_parameters)
                              AND competing_request.phase_id = phase.successor_phase_id
                              AND (
                                  competing_child.rollover_source_child_id = ANY(source_student.source_child_ids)
                                  OR (
                                      COALESCE(competing_child.created_student_id, competing_child.matched_student_id) = source_student.student_id
                                      AND NOT EXISTS (
                                          SELECT 1
                                          FROM enrollment.requests AS lineage_request
                                          JOIN enrollment.request_children AS lineage_child
                                            ON lineage_child.tenant_id = lineage_request.tenant_id
                                           AND lineage_child.request_id = lineage_request.id
                                          WHERE lineage_request.tenant_id = (SELECT tenant_id FROM report_parameters)
                                            AND lineage_request.phase_id = phase.successor_phase_id
                                            AND lineage_child.rollover_source_child_id = ANY(source_student.source_child_ids)
                                      )
                                  )
                              )
                              AND competing_child.status NOT IN ('rejected', 'withdrawn')
                        )
                        OR (
                            target_child.status = 'approved'
                            AND COALESCE(target_child.created_student_id, target_child.matched_student_id) = source_student.student_id
                            AND phase.successor_start_date <= source_student.first_affected_date
                            AND phase.successor_end_date >= source_student.first_affected_date
                            AND EXISTS (
                                SELECT 1
                                FROM enrollment.request_child_offerings AS target_child_offering
                                JOIN enrollment.care_offerings AS target_offering
                                  ON target_offering.tenant_id = target_child_offering.tenant_id
                                 AND target_offering.id = target_child_offering.care_offering_id
                                 AND target_offering.phase_id = phase.successor_phase_id
                                 AND target_offering.is_active
                                WHERE target_child_offering.tenant_id = target_child.tenant_id
                                  AND target_child_offering.request_child_id = target_child.id
                                  AND (
                                      target_child_offering.valid_from IS NULL
                                      OR target_child_offering.valid_from <= source_student.first_affected_date
                                  )
                                  AND (
                                      target_child_offering.valid_until IS NULL
                                      OR target_child_offering.valid_until > source_student.first_affected_date
                                  )
                                  AND EXISTS (
                                      SELECT 1
                                      FROM jsonb_array_elements_text(
                                          CASE
                                              WHEN jsonb_array_length(COALESCE(target_child_offering.selected_days, '[]'::jsonb)) > 0
                                                OR target_offering.days_of_week_mode <> 'fixed'
                                              THEN COALESCE(target_child_offering.selected_days, '[]'::jsonb)
                                              ELSE target_offering.available_days
                                          END
                                      ) AS booked_day(value)
                                      WHERE CASE LOWER(BTRIM(booked_day.value))
                                          WHEN 'mon' THEN 1
                                          WHEN 'tue' THEN 2
                                          WHEN 'wed' THEN 3
                                          WHEN 'thu' THEN 4
                                          WHEN 'fri' THEN 5
                                          WHEN 'sat' THEN 6
                                          WHEN 'sun' THEN 7
                                      END = EXTRACT(ISODOW FROM source_student.first_affected_date)::integer
                                  )
                            )
                        )
                    )
              )
        )
    END AS unresolved_children
FROM phases_with_successor AS phase
ORDER BY phase.first_affected_date, phase.source_phase_id
`
