package audit

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModel "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

var errBookingConsistencyTenantRequired = errors.New("booking consistency audit requires a tenant context")

type bookingConsistencyRepository struct {
	db *bun.DB
}

func NewBookingConsistencyRepository(db *bun.DB) auditModel.BookingConsistencyRepository {
	return &bookingConsistencyRepository{db: db}
}

// Audit compares the authoritative approved booking windows with the next
// seven arrival/pickup days and every future planned instance. Legacy stored
// pickup rows with source=care_offering are deliberately absent: the read-time
// projection introduced by #2494 ignores them too.
func (r *bookingConsistencyRepository) Audit(
	ctx context.Context,
	auditDate timezone.Date,
) (*auditModel.BookingConsistencyReport, error) {
	if auditDate.IsZero() {
		return nil, errors.New("booking consistency audit date is required")
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return nil, errBookingConsistencyTenantRequired
	}

	report := &auditModel.BookingConsistencyReport{}
	err := base.GetDB(ctx, r.db).NewRaw(bookingConsistencyQuery, tenantID, auditDate).Scan(ctx, report)
	if err != nil {
		return nil, fmt.Errorf("audit booking consistency for tenant %d: %w", tenantID, err)
	}
	return report, nil
}

const bookingConsistencyQuery = `
WITH params AS (
	SELECT ?::bigint AS tenant_id, ?::date AS audit_date
), audit_dates AS (
	SELECT (params.audit_date + day_offset.day)::date AS date
	FROM params
	CROSS JOIN generate_series(0, 6) AS day_offset(day)
), approved_students AS (
	SELECT
		request_child.id AS request_child_id,
		COALESCE(request_child.created_student_id, request_child.matched_student_id) AS student_id,
		phase.id AS phase_id,
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
), planned_rows AS (
	SELECT DISTINCT
		instance_student.id AS row_id,
		instance_student.student_id,
		instance.date
	FROM schedule.instance_students AS instance_student
	INNER JOIN schedule.activity_instances AS instance
		ON instance.tenant_id = instance_student.tenant_id
		AND instance.id = instance_student.instance_id
	INNER JOIN activities.groups AS activity_group
		ON activity_group.tenant_id = instance.tenant_id
		AND activity_group.id = instance.activity_group_id
		AND activity_group.type = 'care'
	INNER JOIN approved_students
		ON approved_students.student_id = instance_student.student_id
	INNER JOIN params ON params.tenant_id = instance_student.tenant_id
	WHERE instance.status = 'planned'
		AND instance.date >= params.audit_date
		AND instance_student.is_unplanned = FALSE
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
		VALUES (CASE EXTRACT(ISODOW FROM audit_dates.date)::int
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
		AND EXTRACT(ISODOW FROM audit_dates.date)::int <= 5
		AND (link.valid_from IS NULL OR link.valid_from <= audit_dates.date)
		AND (link.valid_until IS NULL OR link.valid_until > audit_dates.date)
		AND EXISTS (
			SELECT 1
			FROM jsonb_array_elements_text(CASE
				WHEN COALESCE(jsonb_array_length(link.selected_days), 0) > 0
					OR care_offering.days_of_week_mode <> 'fixed'
					THEN COALESCE(link.selected_days, '[]'::jsonb)
				ELSE COALESCE(care_offering.available_days, '[]'::jsonb)
			END) AS selected_day(value)
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
), arrival_days AS (
	SELECT DISTINCT
		arrival.student_id,
		audit_dates.date
	FROM schedule.student_arrival_schedules AS arrival
	INNER JOIN approved_students ON approved_students.student_id = arrival.student_id
	INNER JOIN params ON params.tenant_id = arrival.tenant_id
	INNER JOIN audit_dates
		ON EXTRACT(ISODOW FROM audit_dates.date)::int = arrival.weekday
	LEFT JOIN schedule.student_arrival_exceptions AS arrival_exception
		ON arrival_exception.tenant_id = arrival.tenant_id
		AND arrival_exception.student_id = arrival.student_id
		AND arrival_exception.exception_date = audit_dates.date
	WHERE arrival_exception.id IS NULL
	UNION
	SELECT DISTINCT
		arrival_exception.student_id,
		arrival_exception.exception_date
	FROM schedule.student_arrival_exceptions AS arrival_exception
	INNER JOIN approved_students ON approved_students.student_id = arrival_exception.student_id
	INNER JOIN params ON params.tenant_id = arrival_exception.tenant_id
	INNER JOIN audit_dates ON audit_dates.date = arrival_exception.exception_date
	WHERE arrival_exception.expected_arrival IS NOT NULL
), arrival_without_booking AS (
	SELECT arrival_days.student_id, arrival_days.date
	FROM arrival_days
	LEFT JOIN care_days
		ON care_days.student_id = arrival_days.student_id
		AND care_days.date = arrival_days.date
	WHERE care_days.student_id IS NULL
), booking_without_arrival AS (
	SELECT care_days.student_id, care_days.date
	FROM care_days
	INNER JOIN audit_dates ON audit_dates.date = care_days.date
	CROSS JOIN params
	LEFT JOIN arrival_days
		ON arrival_days.student_id = care_days.student_id
		AND arrival_days.date = care_days.date
	LEFT JOIN schedule.student_arrival_exceptions AS arrival_exception
		ON arrival_exception.tenant_id = params.tenant_id
		AND arrival_exception.student_id = care_days.student_id
		AND arrival_exception.exception_date = care_days.date
	WHERE arrival_days.student_id IS NULL
		AND arrival_exception.id IS NULL
), planned_without_booking AS (
	SELECT planned_rows.row_id
	FROM planned_rows
	WHERE NOT EXISTS (
		SELECT 1
		FROM approved_students AS booking_child
		INNER JOIN enrollment.request_child_offerings AS link
			ON link.tenant_id = booking_child.tenant_id
			AND link.request_child_id = booking_child.request_child_id
		INNER JOIN enrollment.care_offerings AS care_offering
			ON care_offering.tenant_id = link.tenant_id
			AND care_offering.id = link.care_offering_id
			AND care_offering.phase_id = booking_child.phase_id
		WHERE booking_child.student_id = planned_rows.student_id
			AND care_offering.is_active = TRUE
			AND care_offering.counts_as_care = TRUE
			AND (link.valid_from IS NULL OR link.valid_from <= planned_rows.date)
			AND (link.valid_until IS NULL OR link.valid_until > planned_rows.date)
			AND EXISTS (
				SELECT 1
				FROM jsonb_array_elements_text(CASE
					WHEN COALESCE(jsonb_array_length(link.selected_days), 0) > 0
						OR care_offering.days_of_week_mode <> 'fixed'
						THEN COALESCE(link.selected_days, '[]'::jsonb)
					ELSE COALESCE(care_offering.available_days, '[]'::jsonb)
				END) AS selected_day(value)
				WHERE LOWER(BTRIM(selected_day.value)) = CASE EXTRACT(ISODOW FROM planned_rows.date)::int
					WHEN 1 THEN 'mon'
					WHEN 2 THEN 'tue'
					WHEN 3 THEN 'wed'
					WHEN 4 THEN 'thu'
					WHEN 5 THEN 'fri'
					WHEN 6 THEN 'sat'
					ELSE 'sun'
				END
			)
	)
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
				AND NOT EXISTS (
					SELECT 1
					FROM enrollment.request_child_offerings AS required_link
					WHERE required_link.tenant_id = request_child.tenant_id
						AND required_link.request_child_id = request_child.id
						AND required_link.care_offering_id = required_offering.id
						AND (required_link.valid_from IS NULL OR required_link.valid_from <= phase.service_start_date)
						AND (required_link.valid_until IS NULL OR required_link.valid_until > phase.service_start_date)
				)
		) AS missing_required_offering,
		EXISTS (
			SELECT 1
			FROM enrollment.request_child_offerings AS link
			INNER JOIN enrollment.care_offerings AS care_offering
				ON care_offering.tenant_id = link.tenant_id
				AND care_offering.id = link.care_offering_id
				AND care_offering.phase_id = phase.id
				AND care_offering.is_required = FALSE
			WHERE link.tenant_id = request_child.tenant_id
				AND link.request_child_id = request_child.id
				AND (link.valid_from IS NULL OR link.valid_from <= phase.service_start_date)
				AND (link.valid_until IS NULL OR link.valid_until > phase.service_start_date)
		) AS has_choosable_offering
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
)
SELECT
	params.tenant_id,
	params.audit_date,
	(SELECT COUNT(*)
		FROM care_days
		INNER JOIN audit_dates ON audit_dates.date = care_days.date
		WHERE pickup_time IS NULL OR has_invalid_pickup)::int AS pickup_projection_missing_days,
	(SELECT COUNT(*) FROM arrival_without_booking)::int AS arrival_without_booking_days,
	(SELECT COUNT(*) FROM booking_without_arrival)::int AS booking_without_arrival_days,
	(SELECT COUNT(*) FROM planned_without_booking)::int AS planned_without_booking_rows,
	(SELECT COUNT(*) FROM approved_without_offering
		WHERE missing_required_offering
			OR (care_offering_selection_mode <> 'optional' AND NOT has_choosable_offering))::int AS approved_without_required_offering,
	(SELECT COUNT(*) FROM approved_without_offering
		WHERE care_offering_selection_mode = 'optional'
			AND NOT missing_required_offering
			AND NOT has_choosable_offering)::int AS approved_without_optional_offering
FROM params
`
