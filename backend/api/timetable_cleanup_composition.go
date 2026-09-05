package api

import (
	"context"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/uptrace/bun"
)

// NewCleanupTimetable composes the unobserved Timetable owner for CLI roots.
// The serving root uses composeModuleServices so it can attach metrics.
func NewCleanupTimetable(db *bun.DB) (timetable.Capability, error) {
	students, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	if err != nil {
		return nil, err
	}
	rooms, err := repositories.NewFacilities(db)
	if err != nil {
		return nil, err
	}
	return timetableCompose.New(timetableCompose.Dependencies{
		DB: db, Students: timetableStudents(students), Rooms: cleanupTimetableRooms(rooms), CareDays: scheduleSvc.TimetableCareDayLocker(db), Observe: func(timetableCompose.Observation) {},
	})
}

func cleanupTimetableRooms(rooms facilities.Query) timetable.RoomDirectory {
	return timetable.RoomDirectoryFunc(func(ctx context.Context, ids []int64) ([]timetable.RoomRef, error) {
		values, err := rooms.LockRoomsByID(ctx, ids)
		result := make([]timetable.RoomRef, 0, len(values))
		for _, value := range values {
			result = append(result, timetable.RoomRef{ID: value.ID, TenantID: value.TenantID})
		}
		return result, err
	})
}
