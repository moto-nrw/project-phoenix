package audit

import (
	"context"
	"errors"
	"fmt"

	auditModel "github.com/moto-nrw/project-phoenix/models/audit"
)

var errBookingConsistencyTenantRequired = errors.New("booking consistency audit requires a tenant context")

type bookingConsistencyRepository struct {
	runtime Runtime
}

func NewBookingConsistencyRepository(runtime Runtime) auditModel.BookingConsistencyRepository {
	return &bookingConsistencyRepository{runtime: requireRuntime(runtime)}
}

// Audit checks approved booking windows for missing pickup projections and
// offering coverage. Raw arrival rows and materialized class rosters are not
// consistency signals: booking-led care deliberately ignores the former on
// unbooked days and marks the latter not scheduled at read time.
func (r *bookingConsistencyRepository) Audit(
	ctx context.Context,
	auditDate auditModel.Date,
) (*auditModel.BookingConsistencyReport, error) {
	if auditDate.IsZero() {
		return nil, errors.New("booking consistency audit date is required")
	}
	tenantID := runtimeTenantID(ctx, r.runtime)
	if tenantID <= 0 {
		return nil, errBookingConsistencyTenantRequired
	}

	report := &auditModel.BookingConsistencyReport{}
	err := runtimeDB(ctx, r.runtime).NewRaw(`
WITH params AS (
	SELECT ?::bigint AS tenant_id, ?::date AS audit_date
), audit_dates AS (
	SELECT (params.audit_date + day_offset.day)::date AS date
	FROM params
	CROSS JOIN (VALUES (0), (1), (2), (3), (4), (5), (6)) AS day_offset(day)
), approved_students AS (
	SELECT
		request_child.id AS request_child_id,
		COALESCE(request_child.created_student_id, request_child.matched_student_id) AS student_id,
		phase.id AS phase_id,
		phase.service_start_date,
		phase.service_end_date,
		request_child.tenant_id
	FROM enrollment.request_children AS request_child
	INNER JOIN enrollment.requests AS request
		ON request.tenant_id = request_child.tenant_id
		AND request.id = request_child.request_id
	INNER JOIN enrollment.phases AS phase
		ON phase.tenant_id = request.tenant_id
		AND phase.id = request.phase_id
	INNER JOIN users.students AS student
		ON student.tenant_id = request_child.tenant_id
		AND student.id = COALESCE(request_child.created_student_id, request_child.matched_student_id)
		AND student.status <> 'alumnus'
	INNER JOIN params ON params.tenant_id = request_child.tenant_id
	WHERE request_child.status = 'approved'
), care_inputs AS (
	SELECT
		approved_students.student_id,
		audit_dates.date,
		NULLIF(care_offering.pickup_times ->> day_code.value, '') AS pickup_time
	FROM approved_students
	INNER JOIN enrollment.request_child_offerings AS link
		ON link.request_child_id = approved_students.request_child_id
	INNER JOIN params ON params.tenant_id = link.tenant_id
	INNER JOIN enrollment.care_offerings AS care_offering
		ON care_offering.tenant_id = link.tenant_id
		AND care_offering.id = link.care_offering_id
		AND care_offering.phase_id = approved_students.phase_id
	CROSS JOIN audit_dates
	CROSS JOIN LATERAL (
		VALUES (CASE date_part('isodow', audit_dates.date)::int
			WHEN 1 THEN 'mon'
			WHEN 2 THEN 'tue'
			WHEN 3 THEN 'wed'
			WHEN 4 THEN 'thu'
			WHEN 5 THEN 'fri'
			WHEN 6 THEN 'sat'
			ELSE 'sun'
		END)
	) AS day_code(value)
	WHERE care_offering.is_active = TRUE
		AND care_offering.counts_as_care = TRUE
		AND audit_dates.date >= approved_students.service_start_date
		AND audit_dates.date <= approved_students.service_end_date
		AND date_part('isodow', audit_dates.date)::int <= 5
		AND (link.valid_from IS NULL OR link.valid_from <= audit_dates.date)
		AND (link.valid_until IS NULL OR link.valid_until > audit_dates.date)
		AND EXISTS (
			SELECT 1
			FROM (
				SELECT jsonb_array_elements_text(CASE
					WHEN COALESCE(jsonb_array_length(link.selected_days), 0) > 0
						OR care_offering.days_of_week_mode <> 'fixed'
						THEN COALESCE(link.selected_days, '[]'::jsonb)
					ELSE COALESCE(care_offering.available_days, '[]'::jsonb)
				END) AS value
			) AS selected_day
			WHERE LOWER(BTRIM(selected_day.value)) = day_code.value
		)
), care_days AS (
	SELECT
		student_id,
		date,
		BOOL_OR(
			pickup_time IS NOT NULL
			AND pickup_time !~ '^([01][0-9]|2[0-3]):[0-5][0-9]$'
		) AS has_invalid_pickup,
		MAX(pickup_time) FILTER (
			WHERE pickup_time ~ '^([01][0-9]|2[0-3]):[0-5][0-9]$'
		) AS pickup_time
	FROM care_inputs
	GROUP BY student_id, date
), approved_without_offering AS (
	SELECT
		phase.care_offering_selection_mode,
		EXISTS (
			SELECT 1
			FROM enrollment.care_offerings AS required_offering
			WHERE required_offering.tenant_id = phase.tenant_id
				AND required_offering.phase_id = phase.id
				AND required_offering.is_active = TRUE
				AND required_offering.is_required = TRUE
				AND NOT COALESCE((
					SELECT range_agg(daterange(
						GREATEST(COALESCE(required_link.valid_from, phase.service_start_date), phase.service_start_date),
						LEAST(COALESCE(required_link.valid_until, phase.service_end_date + 1), phase.service_end_date + 1),
						'[)'
					)) @> daterange(phase.service_start_date, phase.service_end_date + 1, '[)')
					FROM enrollment.request_child_offerings AS required_link
					WHERE required_link.tenant_id = request_child.tenant_id
						AND required_link.request_child_id = request_child.id
						AND required_link.care_offering_id = required_offering.id
						AND (required_link.valid_from IS NULL OR required_link.valid_from <= phase.service_end_date)
						AND (required_link.valid_until IS NULL OR required_link.valid_until > phase.service_start_date)
				), FALSE)
		) AS missing_required_offering,
		EXISTS (
			SELECT 1
			FROM enrollment.care_offerings AS care_offering
			WHERE care_offering.tenant_id = phase.tenant_id
				AND care_offering.phase_id = phase.id
				AND care_offering.is_active = TRUE
				AND care_offering.is_required = FALSE
				AND care_offering.counts_as_care = TRUE
				AND COALESCE((
					SELECT range_agg(daterange(
						GREATEST(COALESCE(link.valid_from, phase.service_start_date), phase.service_start_date),
						LEAST(COALESCE(link.valid_until, phase.service_end_date + 1), phase.service_end_date + 1),
						'[)'
					)) @> daterange(phase.service_start_date, phase.service_end_date + 1, '[)')
					FROM enrollment.request_child_offerings AS link
					WHERE link.tenant_id = request_child.tenant_id
						AND link.request_child_id = request_child.id
						AND link.care_offering_id = care_offering.id
						AND (link.valid_from IS NULL OR link.valid_from <= phase.service_end_date)
						AND (link.valid_until IS NULL OR link.valid_until > phase.service_start_date)
				), FALSE)
		) AS has_choosable_offering
	FROM enrollment.request_children AS request_child
	INNER JOIN enrollment.requests AS request
		ON request.tenant_id = request_child.tenant_id
		AND request.id = request_child.request_id
	INNER JOIN enrollment.phases AS phase
		ON phase.tenant_id = request.tenant_id
		AND phase.id = request.phase_id
	LEFT JOIN users.students AS student
		ON student.tenant_id = request_child.tenant_id
		AND student.id = COALESCE(request_child.created_student_id, request_child.matched_student_id)
	INNER JOIN params ON params.tenant_id = request_child.tenant_id
	WHERE request_child.status = 'approved'
		AND (student.id IS NULL OR student.status <> 'alumnus')
		AND phase.service_start_date <= params.audit_date + 6
		AND phase.service_end_date >= params.audit_date
)
SELECT
	params.tenant_id,
	params.audit_date,
	(SELECT COUNT(*)
		FROM care_days
		INNER JOIN audit_dates ON audit_dates.date = care_days.date
		WHERE pickup_time IS NULL OR has_invalid_pickup)::int AS pickup_projection_missing_days,
	(SELECT COUNT(*) FROM approved_without_offering
		WHERE missing_required_offering
			OR (care_offering_selection_mode <> 'optional' AND NOT has_choosable_offering))::int AS approved_without_required_offering,
	(SELECT COUNT(*) FROM approved_without_offering
		WHERE care_offering_selection_mode = 'optional'
			AND NOT missing_required_offering
			AND NOT has_choosable_offering)::int AS approved_without_optional_offering
FROM params
`, tenantID, auditDate).Scan(ctx, report)
	if err != nil {
		return nil, fmt.Errorf("audit booking consistency for tenant %d: %w", tenantID, err)
	}
	return report, nil
}
