package schedule

import (
	"context"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/uptrace/bun"
)

// CourseStatisticsRepository implements schedule.CourseStatisticsRepository
// (#2891). Like the attendance aggregates it does not fit the generic
// Repository[T] filter surface: both reads group across instances and
// children and are shaped for the Statistik page.
type CourseStatisticsRepository struct {
	db *bun.DB
}

// NewCourseStatisticsRepository creates a course statistics repository.
func NewCourseStatisticsRepository(db *bun.DB) scheduleModels.CourseStatisticsRepository {
	return &CourseStatisticsRepository{db: db}
}

// courseKeyExpr folds every segment a template split produced back onto the
// original row: the original keeps series_root_id NULL and is its own root.
const courseKeyExpr = `COALESCE("template".series_root_id, "template".id)`

// CourseInstances counts the occurrences of every course in [from, to],
// separating the ones that happened from the cancelled ones. Name, category
// and Teilnehmergrenze come from the series root, so a renamed successor
// segment does not split one course into two rows.
func (r *CourseStatisticsRepository) CourseInstances(ctx context.Context, from, to timezone.Date) ([]scheduleModels.CourseInstanceRow, error) {
	var rows []scheduleModels.CourseInstanceRow
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`schedule.activity_instances AS "instance"`).
		Join(`JOIN activities.groups AS "template" ON "template".id = "instance".activity_group_id`).
		Join(`JOIN activities.groups AS "root" ON "root".id = `+courseKeyExpr).
		Join(`LEFT JOIN activities.categories AS "category" ON "category".id = "root".category_id`).
		ColumnExpr(courseKeyExpr+` AS course_id`).
		ColumnExpr(`"root".name AS name`).
		ColumnExpr(`COALESCE("category".name, '') AS category_name`).
		ColumnExpr(`COALESCE("root".max_participants, 0) AS max_participants`).
		ColumnExpr(`COUNT(*) FILTER (WHERE "instance".status <> ?) AS held_instances`, scheduleModels.InstanceStatusCancelled).
		ColumnExpr(`COUNT(*) FILTER (WHERE "instance".status = ?) AS cancelled_instances`, scheduleModels.InstanceStatusCancelled).
		Where(`"instance".date >= ? AND "instance".date <= ?`, from, to).
		Where(`"template".tenant_id = "instance".tenant_id`).
		Where(`"root".tenant_id = "instance".tenant_id`).
		GroupExpr(courseKeyExpr + `, "root".name, "root".max_participants, "category".name`)
	query = base.WithTenantFilter(ctx, query, "instance")
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "statistics course instances", Err: err}
	}
	return rows, nil
}

// enrolledOnInstanceDate keeps only the attendance rows the child's course
// enrollment actually covers on that date. A shortened or later-started
// enrollment leaves its already-materialized rows behind, and counting those
// would credit a child with days outside the enrollment. The interval is the
// one every other reader uses: valid_from inclusive, valid_until exclusive.
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
		)
		OR NOT EXISTS (
			SELECT 1 FROM activities.student_enrollments AS "enrollment"
			WHERE "enrollment".tenant_id = "attendance".tenant_id
				AND "enrollment".student_id = "attendance".student_id
				AND "enrollment".activity_group_id = "instance".activity_group_id
		)
	)`

// CourseParticipation aggregates the attendance rows of every child per
// course. Cancelled occurrences drop out completely — they are neither a
// participation nor an absence — and so do the rows that only record that the
// care plan never placed the child in the OGS that day, and the rows outside
// the child's enrollment interval (see enrolledOnInstanceDate).
//
// The three counters are returned separately instead of a ready-made quota so
// the service can state the denominator on the screen: present + absent are
// decided, open occurrences are not.
func (r *CourseStatisticsRepository) CourseParticipation(ctx context.Context, from, to timezone.Date) ([]scheduleModels.CourseParticipationRow, error) {
	var rows []scheduleModels.CourseParticipationRow
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`schedule.instance_students AS "attendance"`).
		Join(`JOIN schedule.activity_instances AS "instance" ON "instance".id = "attendance".instance_id`).
		Join(`JOIN activities.groups AS "template" ON "template".id = "instance".activity_group_id`).
		ColumnExpr(courseKeyExpr+` AS course_id`).
		ColumnExpr(`"attendance".student_id AS student_id`).
		ColumnExpr(`COUNT(*) FILTER (WHERE "attendance".status = ?) AS present_days`, scheduleModels.AttendanceStatusPresent).
		ColumnExpr(`COUNT(*) FILTER (WHERE "attendance".status = ?) AS absent_days`, scheduleModels.AttendanceStatusAbsent).
		ColumnExpr(`COUNT(*) FILTER (WHERE "attendance".status = ?) AS open_days`, scheduleModels.AttendanceStatusExpected).
		Where(`"instance".date >= ? AND "instance".date <= ?`, from, to).
		Where(`"instance".status <> ?`, scheduleModels.InstanceStatusCancelled).
		Where(`NOT ("attendance".not_scheduled AND "attendance".status = ?)`, scheduleModels.AttendanceStatusExpected).
		Where(enrolledOnInstanceDate).
		Where(`"instance".tenant_id = "attendance".tenant_id`).
		Where(`"template".tenant_id = "attendance".tenant_id`).
		GroupExpr(courseKeyExpr + `, "attendance".student_id`)
	query = base.WithTenantFilter(ctx, query, "attendance")
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "statistics course participation", Err: err}
	}
	return rows, nil
}
