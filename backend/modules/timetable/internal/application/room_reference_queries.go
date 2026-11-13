package application

import (
	"context"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/timetable"
	"github.com/moto-ogs/project-phoenix/modules/timetable/internal/domain"
	internalports "github.com/moto-ogs/project-phoenix/modules/timetable/internal/ports"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// RoomReferenceRecords is the store read behind RoomReferenceQuery.
type RoomReferenceRecords interface {
	CountRoomReferences(context.Context, int64, calendar.Date) (timetable.RoomReferences, domain.OperationStats, error)
}

// NewRoomReferenceQueries answers the Facilities room removal's question
// which Timetable rows still use a room (#6845).
func NewRoomReferenceQueries(store RoomReferenceRecords, observe func(internalports.Observation)) timetable.RoomReferenceQuery {
	return roomReferenceQueries{store: store, observe: observe}
}

type roomReferenceQueries struct {
	store   RoomReferenceRecords
	observe func(internalports.Observation)
}

func (q roomReferenceQueries) CountRoomReferences(ctx context.Context, roomID int64) (timetable.RoomReferences, error) {
	if roomID <= 0 {
		q.observe(internalports.Observation{Operation: "count_room_references", Err: timetable.ErrInvalidGroupQuery})
		return timetable.RoomReferences{}, timetable.ErrInvalidGroupQuery
	}
	started := time.Now()
	// The room removal asks inside its own tenant transaction; the count
	// joins it, so it sees the rows the removal's locks hold.
	result, stats, err := q.store.CountRoomReferences(ctx, roomID, calendar.TodayDate())
	q.observe(internalports.Observation{Operation: "count_room_references", Duration: time.Since(started), Stats: stats, Err: err})
	if err != nil {
		return timetable.RoomReferences{}, err
	}
	return result, nil
}
