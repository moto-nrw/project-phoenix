package schedule

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// CourseInstanceRow counts the occurrences of one course inside the report
// window (#2891). A "course" is a Betreuungsplan template (activities.groups)
// together with every segment a split produced from it, keyed by its series
// root — a template split mid-year is still one course to a school.
//
// Spontaneous instances (no template) carry no course identity and are left
// out entirely; they are single dates, not a course somebody attends.
type CourseInstanceRow struct {
	CourseID     int64  `bun:"course_id"`
	Name         string `bun:"name"`
	CategoryName string `bun:"category_name"`
	// MaxParticipants is the Teilnehmergrenze (#2233); 0 means unlimited.
	MaxParticipants int `bun:"max_participants"`
	// HeldInstances counts every occurrence that was not cancelled.
	HeldInstances int `bun:"held_instances"`
	// CancelledInstances counts the cancelled ones; they are reported next to
	// the quota but never inside it.
	CancelledInstances int `bun:"cancelled_instances"`
}

// CourseParticipationRow is one child's attendance in one course over the
// window, already aggregated over the occurrences.
//
// The three counters partition the child's attendance rows on non-cancelled
// occurrences, minus the rows that mean "the care plan did not place this
// child in the OGS that day" (not_scheduled while still expected — see
// InstanceStudent.NotScheduled). Those are no course absence and drop out.
type CourseParticipationRow struct {
	CourseID  int64 `bun:"course_id"`
	StudentID int64 `bun:"student_id"`
	// PresentDays counts occurrences the child took part in.
	PresentDays int `bun:"present_days"`
	// AbsentDays counts occurrences the child was marked absent for.
	AbsentDays int `bun:"absent_days"`
	// OpenDays counts occurrences nobody has decided yet (the block was never
	// completed). They stay out of the quota so a forgotten Abschluss does not
	// read as a missing child.
	OpenDays int `bun:"open_days"`
}

// CourseStatisticsRepository serves the course participation aggregates
// behind the Statistik section "Kurse" (#2891). Both methods are
// tenant-scoped through the request context.
type CourseStatisticsRepository interface {
	// CourseInstances returns one row per course with occurrences dated in
	// [from, to].
	CourseInstances(ctx context.Context, from, to timezone.Date) ([]CourseInstanceRow, error)
	// CourseParticipation returns one row per (course, child) with attendance
	// on non-cancelled occurrences dated in [from, to].
	CourseParticipation(ctx context.Context, from, to timezone.Date) ([]CourseParticipationRow, error)
}
