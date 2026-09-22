package statistics

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

// Course participation (#2891), the third Statistik section.
//
// Definitions (binding, mirrored on the screen and in the export):
//
//   - course            = one Betreuungsplan template plus every segment a
//     split produced from it (keyed by series root)
//   - occurrence        = one materialized date of that course; cancelled
//     dates count in neither numerator nor denominator
//   - participation day = an attendance row marked present
//   - absence day       = an attendance row marked absent
//   - open              = an occurrence nobody decided (never completed);
//     reported separately, never inside the quota
//   - participation rate = participation days / (participation + absence)
//
// The population is exactly the one the attendance section reports: children
// enrolled in the window, alumni excluded, narrowed by the same group filter.
// Anything else would put two contradicting child counts on one screen. Rows
// that only record "the care plan did not place this child in the OGS that
// day" are dropped in the repository — they are no course absence.

// courseSection computes both course views over the already-filtered child
// population, so the section can never report a child the attendance section
// does not.
//
// from is the window start already clamped to the retention cutoff the screen
// names (CourseDataFrom): the cleanup job may run late or the tenant may have
// shortened the window, so the reported cutoff is enforced here rather than
// trusted to be enforced by deletion.
func (s *service) courseSection(ctx context.Context, filters studentpresence.StatisticsFilters, from, to, today timezone.Date, students []studentpresence.StatisticsStudentRow) ([]studentpresence.StatisticsCourseRow, []studentpresence.StatisticsCourseStudentRow, studentpresence.StatisticsCourseRow, error) {
	var totals studentpresence.StatisticsCourseRow
	totals.Name = totalsRowName
	if s.cfg.Courses == nil || from.After(to) {
		// The whole window lies behind the retention cutoff — there is
		// nothing left to read, and the screen says so.
		return nil, nil, totals, nil
	}
	instances, err := s.cfg.Courses.CourseInstances(ctx, from, to, today)
	if err != nil {
		return nil, nil, totals, fmt.Errorf("load course instances: %w", err)
	}
	participation, err := s.cfg.Courses.CourseParticipation(ctx, from, to, today)
	if err != nil {
		return nil, nil, totals, fmt.Errorf("load course participation: %w", err)
	}

	courses := indexCourses(instances)
	childRows := collectCourseParticipation(courses, participation, students)
	rows := finishCourseRows(courses, len(filters.GroupIDs) > 0, &totals)
	totals.StudentCount = distinctStudents(childRows)
	totals.ParticipationRate = rate(totals.PresentDays, totals.PresentDays+totals.AbsentDays)

	sortCourseRows(rows, childRows)
	return rows, childRows, totals, nil
}

func indexCourses(instances []ports.CourseInstance) map[int64]*studentpresence.StatisticsCourseRow {
	courses := make(map[int64]*studentpresence.StatisticsCourseRow, len(instances))
	for _, row := range instances {
		courses[row.CourseID] = &studentpresence.StatisticsCourseRow{
			CourseID:           row.CourseID,
			Name:               row.Name,
			CategoryName:       row.CategoryName,
			MaxParticipants:    row.MaxParticipants,
			HeldInstances:      row.HeldInstances,
			CancelledInstances: row.CancelledInstances,
		}
	}
	return courses
}

// collectCourseParticipation adds each eligible child's days to their course
// and returns the per-child view.
func collectCourseParticipation(courses map[int64]*studentpresence.StatisticsCourseRow, participation []ports.CourseParticipation, students []studentpresence.StatisticsStudentRow) []studentpresence.StatisticsCourseStudentRow {
	eligible := make(map[int64]studentpresence.StatisticsStudentRow, len(students))
	for _, st := range students {
		eligible[st.StudentID] = st
	}

	childRows := make([]studentpresence.StatisticsCourseStudentRow, 0, len(participation))
	for _, row := range participation {
		course, ok := courses[row.CourseID]
		if !ok {
			// A course whose only occurrences in the window were cancelled has
			// no attendance rows, and attendance without an occurrence cannot
			// happen — but never invent a course row from a stray child row.
			continue
		}
		student, ok := eligible[row.StudentID]
		if !ok {
			continue
		}
		course.StudentCount++
		course.PresentDays += row.PresentDays
		course.AbsentDays += row.AbsentDays
		course.OpenDays += row.OpenDays
		childRows = append(childRows, studentpresence.StatisticsCourseStudentRow{
			StudentID:         student.StudentID,
			FirstName:         student.FirstName,
			LastName:          student.LastName,
			SchoolClass:       student.SchoolClass,
			GroupName:         student.GroupName,
			CourseID:          course.CourseID,
			CourseName:        course.Name,
			PresentDays:       row.PresentDays,
			AbsentDays:        row.AbsentDays,
			OpenDays:          row.OpenDays,
			ParticipationRate: rate(row.PresentDays, row.PresentDays+row.AbsentDays),
		})
	}
	return childRows
}

// finishCourseRows prices every reported course and adds it to the totals.
func finishCourseRows(courses map[int64]*studentpresence.StatisticsCourseRow, groupFiltered bool, totals *studentpresence.StatisticsCourseRow) []studentpresence.StatisticsCourseRow {
	rows := make([]studentpresence.StatisticsCourseRow, 0, len(courses))
	for _, course := range courses {
		if groupFiltered && course.StudentCount == 0 {
			// With a group filter on, the screen shows that group's courses.
			// A course none of its children attended is not "a course with
			// zero participation" but a course belonging to somebody else,
			// and its occurrences must not land in the totals either.
			continue
		}
		course.ParticipationRate = rate(course.PresentDays, course.PresentDays+course.AbsentDays)
		if course.MaxParticipants > 0 {
			course.OccupancyPercent = rate(course.StudentCount, course.MaxParticipants)
		}
		totals.HeldInstances += course.HeldInstances
		totals.CancelledInstances += course.CancelledInstances
		totals.PresentDays += course.PresentDays
		totals.AbsentDays += course.AbsentDays
		totals.OpenDays += course.OpenDays
		rows = append(rows, *course)
	}
	return rows
}

func sortCourseRows(rows []studentpresence.StatisticsCourseRow, childRows []studentpresence.StatisticsCourseStudentRow) {
	sort.SliceStable(rows, func(i, j int) bool { return sortKey(rows[i].Name) < sortKey(rows[j].Name) })
	sort.SliceStable(childRows, func(i, j int) bool {
		if c := compareStudentName(childRows[i].LastName, childRows[i].FirstName, childRows[j].LastName, childRows[j].FirstName); c != 0 {
			return c < 0
		}
		if childRows[i].StudentID != childRows[j].StudentID {
			return childRows[i].StudentID < childRows[j].StudentID
		}
		return sortKey(childRows[i].CourseName) < sortKey(childRows[j].CourseName)
	})
}

// distinctStudents counts children, not (child, course) pairs — a child in
// three courses is one child.
func distinctStudents(rows []studentpresence.StatisticsCourseStudentRow) int {
	seen := make(map[int64]bool, len(rows))
	for _, row := range rows {
		seen[row.StudentID] = true
	}
	return len(seen)
}

// courseRetentionDays returns how far back the course section can reach: the
// tenant's Betreuungsplan retention window, after which finished occurrences
// are deleted by the cleanup job. Unlike the room window this is one number
// for the whole school, not a per-child consent.
func (s *service) courseRetentionDays(ctx context.Context) (int, error) {
	if s.cfg.Retention == nil {
		return 0, fmt.Errorf("statistics retention policy is required")
	}
	days, err := s.cfg.Retention.CourseRetentionDays(ctx)
	if err != nil {
		return 0, fmt.Errorf("load course retention: %w", err)
	}
	return days, nil
}

// compareStudentName orders two children the way every child table in the
// report does: by last name, then first name, umlaut-folded.
func compareStudentName(lastA, firstA, lastB, firstB string) int {
	if c := strings.Compare(sortKey(lastA), sortKey(lastB)); c != 0 {
		return c
	}
	return strings.Compare(sortKey(firstA), sortKey(firstB))
}
