package statistics

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	configService "github.com/moto-nrw/project-phoenix/services/config"
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

// CourseRow is one course over the window.
type CourseRow struct {
	CourseID     int64
	Name         string
	CategoryName string
	// MaxParticipants is the Teilnehmergrenze; 0 means unlimited.
	MaxParticipants    int
	HeldInstances      int
	CancelledInstances int
	// StudentCount is how many children had at least one decided or open
	// attendance row in the window.
	StudentCount int
	PresentDays  int
	AbsentDays   int
	OpenDays     int
	// ParticipationRate is present / (present + absent) in percent; nil when
	// nothing was decided yet.
	ParticipationRate *float64
	// OccupancyPercent is StudentCount against the Teilnehmergrenze; nil
	// without a limit.
	OccupancyPercent *float64
}

// CourseStudentRow is one child in one course.
type CourseStudentRow struct {
	StudentID         int64
	FirstName         string
	LastName          string
	SchoolClass       string
	GroupName         string
	CourseID          int64
	CourseName        string
	PresentDays       int
	AbsentDays        int
	OpenDays          int
	ParticipationRate *float64
}

// courseSection computes both course views over the already-filtered child
// population, so the section can never report a child the attendance section
// does not.
//
// from is the window start already clamped to the retention cutoff the screen
// names (CourseDataFrom): the cleanup job may run late or the tenant may have
// shortened the window, so the reported cutoff is enforced here rather than
// trusted to be enforced by deletion.
func (s *service) courseSection(ctx context.Context, filters Filters, from, to, today timezone.Date, students []StudentRow) ([]CourseRow, []CourseStudentRow, CourseRow, error) {
	var totals CourseRow
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

	eligible := make(map[int64]StudentRow, len(students))
	for _, st := range students {
		eligible[st.StudentID] = st
	}

	courses := make(map[int64]*CourseRow, len(instances))
	for _, row := range instances {
		courses[row.CourseID] = &CourseRow{
			CourseID:           row.CourseID,
			Name:               row.Name,
			CategoryName:       row.CategoryName,
			MaxParticipants:    row.MaxParticipants,
			HeldInstances:      row.HeldInstances,
			CancelledInstances: row.CancelledInstances,
		}
	}

	childRows := make([]CourseStudentRow, 0, len(participation))
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
		childRows = append(childRows, CourseStudentRow{
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

	rows := make([]CourseRow, 0, len(courses))
	for _, course := range courses {
		if len(filters.GroupIDs) > 0 && course.StudentCount == 0 {
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
	totals.StudentCount = distinctStudents(childRows)
	totals.ParticipationRate = rate(totals.PresentDays, totals.PresentDays+totals.AbsentDays)

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
	return rows, childRows, totals, nil
}

// distinctStudents counts children, not (child, course) pairs — a child in
// three courses is one child.
func distinctStudents(rows []CourseStudentRow) int {
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
func (s *service) courseRetentionDays(ctx context.Context) int {
	if s.cfg.Settings == nil {
		return defaultTimetableRetentionDays
	}
	return configService.ResolveIntOrDefault(ctx, s.cfg.Settings, configModel.KeyGDPRTimetableRetentionDays, defaultTimetableRetentionDays, s.cfg.Logger)
}

// compareStudentName orders two children the way every child table in the
// report does: by last name, then first name, umlaut-folded.
func compareStudentName(lastA, firstA, lastB, firstB string) int {
	if c := strings.Compare(sortKey(lastA), sortKey(lastB)); c != 0 {
		return c
	}
	return strings.Compare(sortKey(firstA), sortKey(firstB))
}

// defaultTimetableRetentionDays mirrors the registry default of
// gdpr.timetable_retention_days and is only reached without a settings service.
const defaultTimetableRetentionDays = 365
