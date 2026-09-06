package postgres

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

// courseKeyExpr folds every segment a template split produced back onto the
// original row: the original keeps series_root_id NULL and is its own root.
const courseKeyExpr = `COALESCE("template".series_root_id, "template".id)`

// heldInstance is true for an occurrence that has taken place. It excludes
// what still lies ahead: the period picker reaches to today, and a date the
// school has not run yet must not be counted as a Termin, der stattgefunden
// hat — which is what the screen and the export promise. So an occurrence
// counts once it has started or been completed, and otherwise only from the
// day after its date: a block that ran last Tuesday took place even when
// nobody ever pressed "abschließen" — that omission shows up as Offen, not as
// a date that never happened. Cancelled occurrences never match.
//
// The bind parameter is the report date the service captured once, so every
// classification inside one report agrees on "today" even when the request
// spans Berlin midnight.
const heldInstance = `(
		"instance".status IN (?, ?)
		OR ("instance".status = ? AND "instance".date < ?)
	)`

// CourseInstances counts the occurrences of every course in [from, to],
// separating the ones that happened from the cancelled ones. Name, category
// and Teilnehmergrenze come from the series root, so a renamed successor
// segment does not split one course into two rows.
//
// What counts as happened is the heldInstance predicate above.
//
// Only Betreuungsplan templates are courses. An ad-hoc or spontaneous instance
// may point at an operational group (a supervision or session group), and
// those materialize rows just as well; without the is_template gate they would
// appear as courses. Archived templates stay in on purpose: the section is
// retrospective, and a course archived in March still took place in February.
func (s *Store) CourseInstances(ctx context.Context, from, to, today string) ([]domain.CourseInstanceRow, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []domain.CourseInstanceRow
	query := db.NewSelect().
		TableExpr(`schedule.activity_instances AS "instance"`).
		Join(`JOIN activities.groups AS "template" ON "template".id = "instance".activity_group_id`).
		Join(`JOIN activities.groups AS "root" ON "root".id = `+courseKeyExpr).
		Join(`LEFT JOIN activities.categories AS "category" ON "category".id = "root".category_id`).
		ColumnExpr(courseKeyExpr+` AS course_id`).
		ColumnExpr(`"root".name AS name`).
		ColumnExpr(`COALESCE("category".name, '') AS category_name`).
		ColumnExpr(`COALESCE("root".max_participants, 0) AS max_participants`).
		ColumnExpr(`COUNT(*) FILTER (WHERE `+heldInstance+`) AS held_instances`,
			"active", "completed",
			"planned", today).
		ColumnExpr(`COUNT(*) FILTER (WHERE "instance".status = ?) AS cancelled_instances`, "cancelled").
		Where(`"instance".date >= ? AND "instance".date <= ?`, from, to).
		Where(`"template".tenant_id = "instance".tenant_id`).
		Where(`"root".tenant_id = "instance".tenant_id`).
		Where(`"template".is_template`).
		Where(`"root".is_template`).
		GroupExpr(courseKeyExpr + `, "root".name, "root".max_participants, "category".name`)
	query = query.Where(`"instance".tenant_id = ?`, tenantID)
	stats, err := scanAllInto(ctx, query, &rows, "statistics course instances")
	return rows, stats, err
}

// enrolledOnInstanceDate keeps only the attendance rows the child's course
// enrollment actually covers on that date. A shortened, later-started or
// weekday-narrowed enrollment leaves its already-materialized rows behind, and
// counting those would credit a child with days the enrollment never covered.
//
// The coverage predicate is the one the daily reads use (see
// instance_student_repo.go): the interval — valid_from inclusive, valid_until
// exclusive — plus the calendar period, the single-weekday scope (#2129) and
// the selected weekdays owned by the enrollment decision path. Statistics may
// not answer a wider question than the Betreuungsplan itself.
//
// Two shapes survive without a covering interval, and both are real
// participation rather than a leftover: a walk-in the kiosk recorded
// (is_unplanned), and a row for a course the child has no enrollment for at
// all, which no interval can contradict.
const enrolledOnInstanceDate = `(
		"attendance".is_unplanned
		OR EXISTS (
			SELECT 1 FROM activities.student_enrollments AS "enrollment"
			WHERE "enrollment".tenant_id = "attendance".tenant_id
				AND "enrollment".student_id = "attendance".student_id
				AND "enrollment".activity_group_id = "instance".activity_group_id
				AND "enrollment".valid_from <= "instance".date
				AND ("enrollment".valid_until IS NULL OR "enrollment".valid_until > "instance".date)
				AND ("enrollment".calendar_period_id IS NULL
					OR "enrollment".calendar_period_id = "instance".calendar_period_id)
				AND ("enrollment".weekday IS NULL
					OR "enrollment".weekday = date_part('isodow', "instance".date))
				AND (COALESCE(jsonb_array_length("enrollment".selected_weekdays), 0) = 0
					OR "enrollment".selected_weekdays @> to_jsonb(ARRAY[date_part('isodow', "instance".date)::integer]))
		)
		OR NOT EXISTS (
			SELECT 1 FROM activities.student_enrollments AS "enrollment"
			WHERE "enrollment".tenant_id = "attendance".tenant_id
				AND "enrollment".student_id = "attendance".student_id
				AND "enrollment".activity_group_id = "instance".activity_group_id
		)
	)`

// CourseParticipation aggregates the attendance rows of every child per
// course. It reads exactly the occurrences CourseInstances counts as held
// (see heldInstance): a cancelled date is neither a participation nor an
// absence, and a date that has not happened yet is no open slot either — its
// pre-materialized expected rows would otherwise report an appointment as
// unfinished that nobody could have finished yet. Dropped as well are the rows
// that only record that the
// care plan never placed the child in the OGS that day, and the rows the
// child's enrollment does not cover (see enrolledOnInstanceDate). Operational
// groups are no courses here either, for the reason given at CourseInstances.
//
// The three counters are returned separately instead of a ready-made quota so
// the service can state the denominator on the screen: present + absent are
// decided, open occurrences are not.
func (s *Store) CourseParticipation(ctx context.Context, from, to, today string) ([]domain.CourseParticipationRow, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []domain.CourseParticipationRow
	query := db.NewSelect().
		TableExpr(`schedule.instance_students AS "attendance"`).
		Join(`JOIN schedule.activity_instances AS "instance" ON "instance".id = "attendance".instance_id`).
		Join(`JOIN activities.groups AS "template" ON "template".id = "instance".activity_group_id`).
		ColumnExpr(courseKeyExpr+` AS course_id`).
		ColumnExpr(`"attendance".student_id AS student_id`).
		ColumnExpr(`COUNT(*) FILTER (WHERE "attendance".status = ?) AS present_days`, "present").
		ColumnExpr(`COUNT(*) FILTER (WHERE "attendance".status = ?) AS absent_days`, domain.AttendanceAbsent).
		ColumnExpr(`COUNT(*) FILTER (WHERE "attendance".status = ?) AS open_days`, domain.AttendanceExpected).
		Where(`"instance".date >= ? AND "instance".date <= ?`, from, to).
		Where(heldInstance,
			"active", "completed",
			"planned", today).
		Where(`NOT ("attendance".not_scheduled AND "attendance".status = ?)`, domain.AttendanceExpected).
		Where(enrolledOnInstanceDate).
		Where(`"instance".tenant_id = "attendance".tenant_id`).
		Where(`"template".tenant_id = "attendance".tenant_id`).
		Where(`"template".is_template`).
		GroupExpr(courseKeyExpr + `, "attendance".student_id`)
	query = query.Where(`"attendance".tenant_id = ?`, tenantID)
	stats, err := scanAllInto(ctx, query, &rows, "statistics course participation")
	return rows, stats, err
}
