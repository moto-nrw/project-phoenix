package enrollment

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// ManualPlanningOccurrence is one future timetable slot that an offering
// change cannot reconcile because its recurring template is not sourced from
// care offerings.
type ManualPlanningOccurrence struct {
	ActivityGroupID   int64         `bun:"activity_group_id"`
	ActivityGroupName string        `bun:"activity_group_name"`
	InstanceID        int64         `bun:"instance_id"`
	Date              timezone.Date `bun:"date"`
}

// CourseOfferingReference names an offering and its optional legacy course
// link. The timetable projection combines that legacy shape with templates
// that declare the offering as a source.
type CourseOfferingReference struct {
	OfferingID      int64
	ActivityGroupID *int64
}

// CourseGroup is the course-specific read model owned by the timetable
// projection. It deliberately exposes only the facts an enrollment workflow
// needs, never timetable domain objects or repositories.
type CourseGroup struct {
	ID                  int64
	ParticipantLimit    *int
	SourceGradeLevels   []int
	SourceSchoolClasses []string
}

// OfferingChangeImpactRepository reads the timetable rows outside the
// offering-driven reconciliation path. It is deliberately read-only: the
// office must decide how to resolve these rows after seeing the warning.
type OfferingChangeImpactRepository interface {
	ListManualPlanningOccurrences(
		ctx context.Context,
		studentID int64,
		from, to timezone.Date,
	) ([]ManualPlanningOccurrence, error)
	CourseGroupsForOfferings(ctx context.Context, offerings []CourseOfferingReference) (map[int64][]CourseGroup, error)
	LockCourseGroups(ctx context.Context, groupIDs []int64) ([]CourseGroup, error)
	CountActiveCourseEnrollments(ctx context.Context, groupIDs []int64, onDate timezone.Date, excludeStudentID int64) (map[int64]int, error)
}
