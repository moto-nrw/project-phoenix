package api

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
)

func timetableStudents(students peopledirectory.StudentQuery) timetableCompose.StudentDirectory {
	return timetableCompose.StudentDirectoryFunc(func(ctx context.Context) ([]timetableCompose.TargetStudent, error) {
		values, err := students.ListEnrolledStudents(ctx)
		if err != nil {
			return nil, err
		}
		result := make([]timetableCompose.TargetStudent, 0, len(values))
		for _, value := range values {
			result = append(result, timetableCompose.TargetStudent{
				ID: value.ID, SchoolClass: value.SchoolClass, EducationGroupID: value.GroupID,
				EnrolledUntil: value.EnrolledUntil,
			})
		}
		return result, nil
	})
}

func timetableRooms(rooms facilities.Query) timetable.RoomDirectory {
	return timetable.RoomDirectoryFunc(func(ctx context.Context, ids []int64) ([]timetable.RoomRef, error) {
		values, err := rooms.LockRoomsByID(ctx, ids)
		result := make([]timetable.RoomRef, 0, len(values))
		for _, value := range values {
			result = append(result, timetable.RoomRef{ID: value.ID, TenantID: value.TenantID})
		}
		return result, err
	})
}
