package services

import (
	"context"
	"fmt"

	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/uptrace/bun"
)

func mustPresenceReads(db *bun.DB) timetableCompose.PresenceReads {
	reads, err := timetableCompose.NewPresenceReads(db)
	if err != nil {
		panic(err)
	}
	return reads
}

// NewPartialAbsencePreview composes the Timetable partial-absence preview over
// the People Directory, the Care Plan pickup exceptions and the joined plan
// reads of the database, for the retained services and their tests.
func NewPartialAbsencePreview(db *bun.DB) timetableCompose.PartialAbsencePreview {
	exceptions, err := carePlanCompose.NewExceptionQueries(db, func(carePlanCompose.Observation) {})
	if err != nil {
		panic(fmt.Sprintf("partial absence preview: compose care plan exception queries: %v", err))
	}
	return newPartialAbsencePreview(mustPresenceReads(db), peopleDirectoryReads(db), exceptions)
}

// NewPresenceReads exposes the joined block, roster and attendance reads of
// the Timetable compose (#2762) to the retained services and their tests.
func NewPresenceReads(db *bun.DB) timetableCompose.PresenceReads { return mustPresenceReads(db) }

// PickupReviewBlocks previews the blocks a pickup change under review would
// excuse, including the blocks a course enrollment covers but no roster
// lists yet. The read joins the Timetable plan with the Student Presence
// execution and attendance (#2762).
type PickupReviewBlocks struct {
	reads timetableCompose.PresenceReads
}

func NewPickupReviewBlocks(db *bun.DB) carePlanCompose.PickupReviewBlocks {
	return PickupReviewBlocks{reads: mustPresenceReads(db)}
}

func (p PickupReviewBlocks) PreviewPickupBlocks(ctx context.Context, input carePlanCompose.PickupReviewImpact) ([]carePlanCompose.PickupReviewBlock, error) {
	rows, err := p.reads.ListPartialAbsenceBlocks(ctx, input.StudentID, input.Date.String(), input.From, input.Enrolled, input.AutoExceptionIDs)
	if err != nil {
		return nil, err
	}
	result := make([]carePlanCompose.PickupReviewBlock, 0, len(rows))
	for _, row := range rows {
		result = append(result, carePlanCompose.PickupReviewBlock{ID: row.ID, Title: row.Title, StartTime: row.StartTime, EndTime: row.EndTime})
	}
	return result, nil
}
