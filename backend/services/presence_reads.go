package services

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/uptrace/bun"
)

// The reads below join the Timetable plan with the Student Presence
// execution and attendance (#2762) through the Timetable compose binding of
// the tenant-safe projection; the retained services consume them through
// their own ports.

// partialAbsenceBlocks previews the blocks of a child's day an earlier pickup
// can still excuse.
type partialAbsenceBlocks struct {
	preview repositories.PartialAbsencePreview
}

func newPartialAbsenceBlocks(db *bun.DB) partialAbsenceBlocks {
	return partialAbsenceBlocks{preview: repositories.NewPartialAbsencePreview(db)}
}

func (p partialAbsenceBlocks) FindPartialAbsenceBlocks(ctx context.Context, studentID int64, date timezone.Date, clock time.Time) ([]carerequests.Block, error) {
	rows, err := p.preview.FindPartialAbsenceBlocks(ctx, studentID, date.String(), clock)
	if err != nil {
		return nil, err
	}
	blocks := make([]carerequests.Block, 0, len(rows))
	for _, row := range rows {
		blocks = append(blocks, carerequests.Block{ID: row.ID, Title: row.Title, StartTime: row.StartTime, EndTime: row.EndTime})
	}
	return blocks, nil
}

// courseStatistics serves the Statistik report's course inputs.
type courseStatistics struct {
	reads timetableCompose.PresenceReads
}

func newCourseStatistics(db *bun.DB) courseStatistics {
	return courseStatistics{reads: repositories.NewPresenceReads(db)}
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
