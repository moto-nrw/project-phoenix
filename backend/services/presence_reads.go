package services

import (
	"context"

	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The reads below join the Timetable plan with the Student Presence
// execution and attendance (#2762) through the Timetable compose binding of
// the tenant-safe projection; the retained services consume them through
// their own ports.

// courseStatistics serves the Statistik report's course inputs.
type courseStatistics struct {
	reads timetableCompose.PresenceReads
}

func NewCourseStatistics(reads timetableCompose.PresenceReads) courseStatistics {
	return courseStatistics{reads: reads}
}

func (c courseStatistics) CourseInstances(ctx context.Context, from, to, today timezone.Date) ([]presenceCompose.CourseInstance, error) {
	values, err := c.reads.CourseInstances(ctx, from.String(), to.String(), today.String())
	if err != nil {
		return nil, err
	}
	result := make([]presenceCompose.CourseInstance, 0, len(values))
	for _, value := range values {
		result = append(result, presenceCompose.CourseInstance(value))
	}
	return result, nil
}

func (c courseStatistics) CourseParticipation(ctx context.Context, from, to, today timezone.Date) ([]presenceCompose.CourseParticipation, error) {
	values, err := c.reads.CourseParticipation(ctx, from.String(), to.String(), today.String())
	if err != nil {
		return nil, err
	}
	result := make([]presenceCompose.CourseParticipation, 0, len(values))
	for _, value := range values {
		result = append(result, presenceCompose.CourseParticipation(value))
	}
	return result, nil
}
