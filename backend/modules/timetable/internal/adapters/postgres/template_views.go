package postgres

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

const templateListSelect = `
	SELECT
		g.id AS template_id,
		g.name,
		g.type,
		g.category_id,
		COALESCE(c.name, '') AS category_name,
		g.planning_track_id,
		COALESCE(pt.name, '') AS planning_track_name,
		COALESCE(pt.color, '') AS planning_track_color,
		pt.sort_order AS planning_track_sort_order,
		g.planned_room_id AS room_id,
		'' AS room_name,
		g.education_group_id,
		g.is_open,
		COALESCE(g.max_participants, 0) AS max_participants,
		g.required_staff,
		g.calendar_period_id AS template_calendar_period_id,
		g.target_group_type,
		g.target_grade_level,
		g.target_school_class,
		COALESCE(g.source_care_offering_ids::text, '') AS source_care_offering_ids_json,
		COALESCE(g.source_grade_levels::text, '') AS source_grade_levels_json,
		COALESCE(g.source_school_classes::text, '') AS source_school_classes_json,
		g.list_kind,
		g.notes,
		c.shift_type_id,
		'' AS shift_type_name,
		'' AS shift_type_color,
		COALESCE(enrollments.count, 0) AS enrollment_count,
		COALESCE(supervisors.count, 0) AS supervisor_count,
		COALESCE(enrollments.student_ids, ARRAY[]::BIGINT[]) AS student_ids,
		COALESCE(supervisors.staff_ids, ARRAY[]::BIGINT[]) AS staff_ids,
		supervisors.primary_staff_id,
		s.id AS schedule_id,
		s.weekday,
		COALESCE(TO_CHAR(tf.start_time, 'HH24:MI'), '') AS start_time,
		COALESCE(TO_CHAR(tf.end_time, 'HH24:MI'), '') AS end_time,
		s.week_pattern,
		s.calendar_period_id,
		TO_CHAR(s.valid_from, 'YYYY-MM-DD') AS schedule_valid_from,
		TO_CHAR(s.valid_until, 'YYYY-MM-DD') AS schedule_valid_until
	FROM activities.groups AS g
	INNER JOIN activities.schedules AS s
		ON s.activity_group_id = g.id AND s.tenant_id = g.tenant_id
	LEFT JOIN schedule.timeframes AS tf
		ON tf.id = s.timeframe_id AND tf.tenant_id = g.tenant_id
	LEFT JOIN activities.categories AS c
		ON c.id = g.category_id AND c.tenant_id = g.tenant_id
	LEFT JOIN schedule.planning_tracks AS pt
		ON pt.id = g.planning_track_id AND pt.tenant_id = g.tenant_id
`

const enrollmentDisplayValidityFilter = `
	AND (enrollment.valid_until IS NULL OR (enrollment.valid_until > ? AND EXISTS (
		SELECT 1
		FROM activities.groups AS sourced_template
		WHERE sourced_template.id = enrollment.activity_group_id
			AND sourced_template.tenant_id = enrollment.tenant_id
			AND sourced_template.source_care_offering_ids IS NOT NULL
	)))`

func (s *Store) ListTemplateRows(ctx context.Context, templateID *int64, today string) ([]domain.TemplateListRow, domain.OperationStats, error) {
	const query = templateListSelect + `
	LEFT JOIN (
		SELECT activity_group_id, COUNT(*) AS count,
			ARRAY_AGG(student_id ORDER BY student_id) AS student_ids
		FROM (
			SELECT DISTINCT enrollment.activity_group_id, enrollment.student_id
			FROM activities.student_enrollments AS enrollment
			WHERE enrollment.tenant_id = ?` + enrollmentDisplayValidityFilter + `
		) AS active_enrollments
		GROUP BY activity_group_id
	) AS enrollments ON enrollments.activity_group_id = g.id
	LEFT JOIN (
		SELECT group_id, COUNT(*) AS count,
			ARRAY_AGG(staff_id ORDER BY is_primary DESC, staff_id) AS staff_ids,
			MAX(staff_id) FILTER (WHERE is_primary) AS primary_staff_id
		FROM (
			SELECT group_id, staff_id, BOOL_OR(is_primary) AS is_primary
			FROM activities.supervisors
			WHERE tenant_id = ? AND valid_until IS NULL
			GROUP BY group_id, staff_id
		) AS active_supervisors
		GROUP BY group_id
	) AS supervisors ON supervisors.group_id = g.id
	WHERE g.tenant_id = ?
		AND g.is_template = TRUE
		AND g.archived_at IS NULL
		AND s.valid_until IS NULL
		AND (?::BIGINT IS NULL OR g.id = ?)
	ORDER BY g.name ASC, s.weekday ASC, tf.start_time ASC`
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	return scanTemplateRows(ctx, db.NewRaw(query, tenantID, today, tenantID, tenantID, templateID, templateID))
}

func (s *Store) ListTemplateRowsForTemplatePeriod(ctx context.Context, templateID, periodID int64, today string) ([]domain.TemplateListRow, domain.OperationStats, error) {
	const query = templateListSelect + `
	LEFT JOIN (
		SELECT activity_group_id, COUNT(*) AS count,
			ARRAY_AGG(student_id ORDER BY student_id) AS student_ids
		FROM (
			SELECT DISTINCT enrollment.activity_group_id, enrollment.student_id
			FROM activities.student_enrollments AS enrollment
			WHERE enrollment.tenant_id = ?` + enrollmentDisplayValidityFilter + `
				AND (enrollment.calendar_period_id IS NULL OR enrollment.calendar_period_id = ?)
		) AS active_enrollments
		GROUP BY activity_group_id
	) AS enrollments ON enrollments.activity_group_id = g.id
	LEFT JOIN (
		SELECT group_id, COUNT(*) AS count,
			ARRAY_AGG(staff_id ORDER BY is_primary DESC, primary_rank DESC, staff_id) AS staff_ids,
			(ARRAY_AGG(staff_id ORDER BY primary_rank DESC, staff_id)
				FILTER (WHERE is_primary))[1] AS primary_staff_id
		FROM (
			SELECT group_id, staff_id, BOOL_OR(is_primary) AS is_primary,
				MAX(CASE WHEN is_primary THEN
					CASE WHEN calendar_period_id = ? THEN 2 ELSE 0 END
					+ CASE WHEN weekday IS NOT NULL THEN 1 ELSE 0 END
					ELSE -1 END) AS primary_rank
			FROM activities.supervisors
			WHERE tenant_id = ? AND valid_until IS NULL
				AND (calendar_period_id IS NULL OR calendar_period_id = ?)
			GROUP BY group_id, staff_id
		) AS active_supervisors
		GROUP BY group_id
	) AS supervisors ON supervisors.group_id = g.id
	WHERE g.tenant_id = ?
		AND g.is_template = TRUE
		AND g.archived_at IS NULL
		AND s.valid_until IS NULL
		AND (s.calendar_period_id = ? OR (s.calendar_period_id IS NULL
			AND (g.calendar_period_id = ? OR g.calendar_period_id IS NULL)))
		AND g.id = ?
	ORDER BY g.name ASC, s.weekday ASC, tf.start_time ASC`
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	return scanTemplateRows(ctx, db.NewRaw(query,
		tenantID, today, periodID, periodID, tenantID, periodID,
		tenantID, periodID, periodID, templateID,
	))
}

func (s *Store) ListTemplateRowsForPeriod(ctx context.Context, periodID *int64, today string) ([]domain.TemplateListRow, domain.OperationStats, error) {
	const query = templateListSelect + `
	LEFT JOIN (
		SELECT activity_group_id, COUNT(*) AS count,
			ARRAY_AGG(student_id ORDER BY student_id) AS student_ids
		FROM (
			SELECT DISTINCT enrollment.activity_group_id, enrollment.student_id
			FROM activities.student_enrollments AS enrollment
			WHERE enrollment.tenant_id = ?` + enrollmentDisplayValidityFilter + `
		) AS active_enrollments
		GROUP BY activity_group_id
	) AS enrollments ON enrollments.activity_group_id = g.id
	LEFT JOIN (
		SELECT group_id, COUNT(*) AS count,
			ARRAY_AGG(staff_id ORDER BY is_primary DESC, staff_id) AS staff_ids,
			MAX(staff_id) FILTER (WHERE is_primary) AS primary_staff_id
		FROM (
			SELECT group_id, staff_id, BOOL_OR(is_primary) AS is_primary
			FROM activities.supervisors
			WHERE tenant_id = ? AND valid_until IS NULL
			GROUP BY group_id, staff_id
		) AS active_supervisors
		GROUP BY group_id
	) AS supervisors ON supervisors.group_id = g.id
	WHERE g.tenant_id = ?
		AND g.is_template = TRUE
		AND g.archived_at IS NULL
		AND s.valid_until IS NULL
		AND (?::BIGINT IS NULL OR (s.calendar_period_id = ? OR
			(s.calendar_period_id IS NULL AND (g.calendar_period_id = ? OR g.calendar_period_id IS NULL))))
	ORDER BY g.name ASC, s.weekday ASC, tf.start_time ASC`
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	return scanTemplateRows(ctx, db.NewRaw(query,
		tenantID, today, tenantID, tenantID, periodID, periodID, periodID,
	))
}

type rawScanner interface {
	Scan(context.Context, ...interface{}) error
}

func scanTemplateRows(ctx context.Context, query rawScanner) ([]domain.TemplateListRow, domain.OperationStats, error) {
	rows := make([]domain.TemplateListRow, 0)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err := query.Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	stats.Rows = int64(len(rows))
	return rows, stats, err
}

const templateWeekdayRosterQuery = `
	WITH parameters AS (
		SELECT ?::BIGINT AS tenant_id, ?::BIGINT AS period_id, ?::BIGINT AS template_id
	), per_weekday_templates AS (
		SELECT supervisor.group_id AS template_id
		FROM activities.supervisors AS supervisor, parameters
		WHERE supervisor.tenant_id = parameters.tenant_id
			AND supervisor.valid_until IS NULL AND supervisor.weekday IS NOT NULL
			AND ((parameters.period_id IS NULL AND supervisor.calendar_period_id IS NULL)
				OR (parameters.period_id IS NOT NULL AND
					(supervisor.calendar_period_id IS NULL OR supervisor.calendar_period_id = parameters.period_id)))
		GROUP BY supervisor.group_id
		UNION
		SELECT enrollment.activity_group_id AS template_id
		FROM activities.student_enrollments AS enrollment, parameters
		WHERE enrollment.tenant_id = parameters.tenant_id` + enrollmentDisplayValidityFilter + `
			AND enrollment.weekday IS NOT NULL
			AND ((parameters.period_id IS NULL AND enrollment.calendar_period_id IS NULL)
				OR (parameters.period_id IS NOT NULL AND
					(enrollment.calendar_period_id IS NULL OR enrollment.calendar_period_id = parameters.period_id)))
		GROUP BY enrollment.activity_group_id
	), scheduled_template_weekdays AS (
		SELECT activity_group.id AS template_id, schedule.weekday
		FROM activities.groups AS activity_group
		JOIN activities.schedules AS schedule
			ON schedule.activity_group_id = activity_group.id AND schedule.tenant_id = activity_group.tenant_id
		CROSS JOIN parameters
		WHERE activity_group.tenant_id = parameters.tenant_id
			AND activity_group.is_template = TRUE AND activity_group.archived_at IS NULL
			AND schedule.valid_until IS NULL
			AND (parameters.period_id IS NULL OR schedule.calendar_period_id = parameters.period_id
				OR (schedule.calendar_period_id IS NULL AND
					(activity_group.calendar_period_id = parameters.period_id OR activity_group.calendar_period_id IS NULL)))
			AND (parameters.template_id IS NULL OR activity_group.id = parameters.template_id)
		GROUP BY activity_group.id, schedule.weekday
	), template_weekdays AS (
		SELECT scheduled.template_id, scheduled.weekday
		FROM scheduled_template_weekdays AS scheduled
		JOIN per_weekday_templates AS scoped ON scoped.template_id = scheduled.template_id
	), effective_primary_staff AS (
		SELECT template_day.template_id, template_day.weekday, primary_staff.staff_id
		FROM template_weekdays AS template_day
		CROSS JOIN parameters
		LEFT JOIN LATERAL (
			SELECT candidate.staff_id
			FROM activities.supervisors AS candidate
			WHERE candidate.tenant_id = parameters.tenant_id
				AND candidate.group_id = template_day.template_id AND candidate.valid_until IS NULL
				AND ((parameters.period_id IS NULL AND candidate.calendar_period_id IS NULL)
					OR (parameters.period_id IS NOT NULL AND
						(candidate.calendar_period_id IS NULL OR candidate.calendar_period_id = parameters.period_id)))
				AND (candidate.weekday IS NULL OR candidate.weekday = template_day.weekday)
				AND candidate.is_primary
			ORDER BY (candidate.calendar_period_id IS NOT NULL) DESC,
				(candidate.weekday IS NOT NULL) DESC, candidate.id DESC
			LIMIT 1
		) AS primary_staff ON TRUE
	)
	SELECT template_day.template_id, template_day.weekday, 'empty' AS kind,
		0 AS person_id, FALSE AS is_primary
	FROM template_weekdays AS template_day
	UNION ALL
	SELECT supervisor.group_id, template_day.weekday, 'staff', supervisor.staff_id,
		COALESCE(BOOL_OR(supervisor.staff_id = primary_staff.staff_id), FALSE)
	FROM activities.supervisors AS supervisor
	JOIN template_weekdays AS template_day
		ON template_day.template_id = supervisor.group_id
		AND (supervisor.weekday IS NULL OR template_day.weekday = supervisor.weekday)
	JOIN effective_primary_staff AS primary_staff
		ON primary_staff.template_id = template_day.template_id AND primary_staff.weekday = template_day.weekday
	CROSS JOIN parameters
	WHERE supervisor.tenant_id = parameters.tenant_id AND supervisor.valid_until IS NULL
		AND ((parameters.period_id IS NULL AND supervisor.calendar_period_id IS NULL)
			OR (parameters.period_id IS NOT NULL AND
				(supervisor.calendar_period_id IS NULL OR supervisor.calendar_period_id = parameters.period_id)))
	GROUP BY supervisor.group_id, template_day.weekday, supervisor.staff_id
	UNION ALL
	SELECT enrollment.activity_group_id, template_day.weekday, 'student', enrollment.student_id, FALSE
	FROM activities.student_enrollments AS enrollment
	JOIN template_weekdays AS template_day
		ON template_day.template_id = enrollment.activity_group_id
		AND (enrollment.weekday IS NULL OR template_day.weekday = enrollment.weekday)
	CROSS JOIN parameters
	WHERE enrollment.tenant_id = parameters.tenant_id` + enrollmentDisplayValidityFilter + `
		AND ((parameters.period_id IS NULL AND enrollment.calendar_period_id IS NULL)
			OR (parameters.period_id IS NOT NULL AND
				(enrollment.calendar_period_id IS NULL OR enrollment.calendar_period_id = parameters.period_id)))
		AND (enrollment.selected_weekdays IS NULL OR jsonb_array_length(enrollment.selected_weekdays) = 0
			OR enrollment.selected_weekdays @> jsonb_build_array(template_day.weekday))
	GROUP BY enrollment.activity_group_id, template_day.weekday, enrollment.student_id
	UNION ALL
	SELECT enrollment.activity_group_id, template_day.weekday, 'protected_student', enrollment.student_id, FALSE
	FROM activities.student_enrollments AS enrollment
	JOIN scheduled_template_weekdays AS template_day ON template_day.template_id = enrollment.activity_group_id
	CROSS JOIN parameters
	WHERE enrollment.tenant_id = parameters.tenant_id` + enrollmentDisplayValidityFilter + `
		AND ((parameters.period_id IS NULL AND enrollment.calendar_period_id IS NULL)
			OR (parameters.period_id IS NOT NULL AND
				(enrollment.calendar_period_id IS NULL OR enrollment.calendar_period_id = parameters.period_id)))
		AND (enrollment.enrollment_request_child_id IS NOT NULL
			OR COALESCE(jsonb_array_length(enrollment.selected_weekdays), 0) > 0)
		AND (enrollment.weekday IS NULL OR enrollment.weekday = template_day.weekday)
		AND (enrollment.selected_weekdays IS NULL OR jsonb_array_length(enrollment.selected_weekdays) = 0
			OR enrollment.selected_weekdays @> jsonb_build_array(template_day.weekday))
	GROUP BY enrollment.activity_group_id, template_day.weekday, enrollment.student_id
	ORDER BY template_id ASC, weekday ASC, kind ASC, is_primary DESC, person_id ASC`

func (s *Store) ListTemplateWeekdayRoster(ctx context.Context, templateID, periodID *int64, today string) ([]domain.TemplateWeekdayRosterRow, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := make([]domain.TemplateWeekdayRosterRow, 0)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewRaw(templateWeekdayRosterQuery,
		tenantID, periodID, templateID, today, today, today,
	).Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	stats.Rows = int64(len(rows))
	return rows, stats, err
}

const templateCapacityOccurrencesQuery = `
	WITH active_periods AS (
		SELECT period.calendar_period_id, period.start_date, period.end_date,
			period.week_cycle_length, NULLIF(period.week_cycle_anchor, '')::DATE AS week_cycle_anchor
		FROM unnest(?::BIGINT[], ?::DATE[], ?::DATE[], ?::INT[], ?::TEXT[])
			AS period(calendar_period_id, start_date, end_date, week_cycle_length, week_cycle_anchor)
	), selected_period AS (
		SELECT calendar_period_id, start_date, end_date, week_cycle_length, week_cycle_anchor
		FROM active_periods
		WHERE (?::BIGINT IS NULL OR calendar_period_id = ?)
	), candidate_occurrences AS (
		SELECT DISTINCT g.id AS template_id, period.calendar_period_id, days.day::DATE AS occurrence_date
		FROM activities.groups AS g
		INNER JOIN activities.schedules AS s
			ON s.activity_group_id = g.id AND s.tenant_id = g.tenant_id
		INNER JOIN schedule.timeframes AS timeframe
			ON timeframe.id = s.timeframe_id AND timeframe.tenant_id = g.tenant_id
			AND timeframe.start_time IS NOT NULL AND timeframe.end_time IS NOT NULL
		CROSS JOIN selected_period AS period
		CROSS JOIN LATERAL generate_series(period.start_date, period.end_date, INTERVAL '1 day') AS days(day)
		LEFT JOIN schedule.activity_exceptions AS exception
			ON exception.tenant_id = g.tenant_id
			AND exception.activity_group_id = g.id
			AND exception.exception_date = days.day::DATE
		WHERE g.tenant_id = ?
			AND g.id IN (?)
			AND g.is_template = TRUE
			AND g.archived_at IS NULL
			AND (exception.exception_type IS NULL OR exception.exception_type <> 'cancelled')
			AND COALESCE(exception.room_id, g.planned_room_id, 0) > 0
			AND DATE_PART('isodow', days.day)::INT = s.weekday
			AND (s.valid_from IS NULL OR s.valid_from <= days.day::DATE)
			AND (s.valid_until IS NULL OR s.valid_until > days.day::DATE)
			AND (s.calendar_period_id = period.calendar_period_id
				OR (s.calendar_period_id IS NULL AND g.calendar_period_id = period.calendar_period_id)
				OR (s.calendar_period_id IS NULL AND g.calendar_period_id IS NULL
					AND period.calendar_period_id = (
						SELECT MIN(active_period.calendar_period_id)
						FROM active_periods AS active_period
						WHERE active_period.start_date <= days.day::DATE
							AND active_period.end_date >= days.day::DATE)))
			AND (s.week_pattern = 0 OR period.week_cycle_length <= 1 OR period.week_cycle_anchor IS NULL
				OR s.week_pattern = (MOD(MOD(
					FLOOR((days.day::DATE - period.week_cycle_anchor) / 7.0)::INT,
					period.week_cycle_length) + period.week_cycle_length,
					period.week_cycle_length) + 1))
	), dynamic_target_students AS (
		SELECT dynamic.template_id, dynamic.student_id
		FROM unnest(?::BIGINT[], ?::BIGINT[]) AS dynamic(template_id, student_id)
	), capacity_parts AS (
		SELECT occurrence.template_id, occurrence.calendar_period_id, occurrence.occurrence_date,
			COUNT(DISTINCT roster.student_id)::INT AS enrollment_count, 0::INT AS supervisor_count
		FROM candidate_occurrences AS occurrence
		CROSS JOIN LATERAL (
			SELECT enrollment.student_id
			FROM activities.student_enrollments AS enrollment
			WHERE enrollment.tenant_id = ?
				AND enrollment.activity_group_id = occurrence.template_id
				AND enrollment.valid_from <= occurrence.occurrence_date
				AND (enrollment.valid_until IS NULL OR enrollment.valid_until > occurrence.occurrence_date)
				AND (enrollment.calendar_period_id IS NULL OR enrollment.calendar_period_id = occurrence.calendar_period_id)
				AND (enrollment.selected_weekdays IS NULL OR jsonb_array_length(enrollment.selected_weekdays) = 0
					OR enrollment.selected_weekdays @> jsonb_build_array(DATE_PART('isodow', occurrence.occurrence_date)::INT))
				AND (enrollment.weekday IS NULL
					OR enrollment.weekday = DATE_PART('isodow', occurrence.occurrence_date)::INT)
			UNION
			SELECT dynamic.student_id
			FROM dynamic_target_students AS dynamic
			WHERE dynamic.template_id = occurrence.template_id
		) AS roster
		GROUP BY occurrence.template_id, occurrence.calendar_period_id, occurrence.occurrence_date
		UNION ALL
		SELECT occurrence.template_id, occurrence.calendar_period_id, occurrence.occurrence_date,
			0::INT AS enrollment_count, COUNT(DISTINCT supervisor.staff_id)::INT AS supervisor_count
		FROM candidate_occurrences AS occurrence
		INNER JOIN activities.supervisors AS supervisor
			ON supervisor.tenant_id = ?
			AND supervisor.group_id = occurrence.template_id
			AND supervisor.valid_from <= occurrence.occurrence_date
			AND (supervisor.valid_until IS NULL OR supervisor.valid_until > occurrence.occurrence_date)
			AND (supervisor.calendar_period_id IS NULL OR supervisor.calendar_period_id = occurrence.calendar_period_id)
			AND (supervisor.weekday IS NULL
				OR supervisor.weekday = DATE_PART('isodow', occurrence.occurrence_date)::INT)
		GROUP BY occurrence.template_id, occurrence.calendar_period_id, occurrence.occurrence_date
		UNION ALL
		SELECT occurrence.template_id, occurrence.calendar_period_id, occurrence.occurrence_date,
			0::INT AS enrollment_count, 0::INT AS supervisor_count
		FROM candidate_occurrences AS occurrence
	)
	SELECT template_id, calendar_period_id, occurrence_date,
		MAX(enrollment_count) AS enrollment_count, MAX(supervisor_count) AS supervisor_count
	FROM capacity_parts
	GROUP BY template_id, calendar_period_id, occurrence_date
	ORDER BY template_id ASC, occurrence_date ASC, calendar_period_id ASC`

func (s *Store) ListTemplateCapacityOccurrences(ctx context.Context, periodID *int64, templateIDs []int64, periods []domain.TemplateCapacityPeriod, dynamicStudents map[int64][]int64) ([]domain.TemplateCapacityOccurrence, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	periodIDs, starts, ends, cycles, anchors := templateCapacityPeriodColumns(periods, tenantID)
	dynamicTemplateIDs, dynamicStudentIDs := templateDynamicStudentColumns(templateIDs, dynamicStudents)
	rows := make([]domain.TemplateCapacityOccurrence, 0)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewRaw(templateCapacityOccurrencesQuery,
		pgdialect.Array(periodIDs), pgdialect.Array(starts), pgdialect.Array(ends),
		pgdialect.Array(cycles), pgdialect.Array(anchors), periodID, periodID,
		tenantID, bun.List(templateIDs), pgdialect.Array(dynamicTemplateIDs),
		pgdialect.Array(dynamicStudentIDs), tenantID, tenantID,
	).Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	stats.Rows = int64(len(rows))
	return rows, stats, err
}

func templateCapacityPeriodColumns(periods []domain.TemplateCapacityPeriod, tenantID int64) (ids []int64, starts, ends []string, cycles []int64, anchors []string) {
	for _, period := range periods {
		if period.TenantID != tenantID {
			continue
		}
		ids = append(ids, period.ID)
		starts = append(starts, period.StartDate)
		ends = append(ends, period.EndDate)
		cycles = append(cycles, int64(period.WeekCycleLength))
		anchors = append(anchors, period.WeekCycleAnchor)
	}
	return ids, starts, ends, cycles, anchors
}

func templateDynamicStudentColumns(templateIDs []int64, students map[int64][]int64) ([]int64, []int64) {
	groups, people := []int64{}, []int64{}
	for _, templateID := range templateIDs {
		for _, studentID := range students[templateID] {
			groups = append(groups, templateID)
			people = append(people, studentID)
		}
	}
	return groups, people
}
