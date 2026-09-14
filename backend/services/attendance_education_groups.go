package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/users"

	schoolStructure "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	"github.com/moto-nrw/project-phoenix/services/active"
)

type attendanceEducationGroups struct {
	groups   *schoolStructure.GroupRoomDirectory
	students attendanceGroupStudents
}

type attendanceGroupStudents interface {
	FindByID(context.Context, any) (*users.Student, error)
	FindByIDs(context.Context, []int64) (map[int64]*users.Student, error)
}

func (r attendanceEducationGroups) StudentGroupID(ctx context.Context, id int64) (*int64, error) {
	student, err := r.students.FindByID(ctx, id)
	if err != nil || student == nil {
		return nil, err
	}
	return student.GroupID, nil
}

// NewAttendanceTeacherGroups supplies assignment IDs for the active route composer.
func NewAttendanceTeacherGroups(records schoolStructure.TeacherGroupRecords) *schoolStructure.TeacherGroupIDs {
	return schoolStructure.NewTeacherGroupIDs(records)
}

// NewAttendanceEducationGroups projects the tenant's group-to-room directory.
func NewAttendanceEducationGroups(groups schoolStructure.GroupRoomRecords, students attendanceGroupStudents) active.AttendanceEducationGroups {
	return attendanceEducationGroups{groups: schoolStructure.NewGroupRoomDirectory(groups), students: students}
}

func (r attendanceEducationGroups) StudentGroupIDs(ctx context.Context, ids []int64) (map[int64]int64, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := r.students.FindByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	groups := make(map[int64]int64, len(rows))
	for _, row := range rows {
		if row.GroupID != nil {
			groups[row.ID] = *row.GroupID
		}
	}
	return groups, nil
}

func (r attendanceEducationGroups) ListGroupRooms(ctx context.Context) ([]*active.EducationGroupRoom, error) {
	rows, err := r.groups.ListGroupRooms(ctx)
	if rows == nil {
		return nil, err
	}
	result := make([]*active.EducationGroupRoom, len(rows))
	for i, row := range rows {
		if row != nil {
			result[i] = &active.EducationGroupRoom{ID: row.ID, RoomID: row.RoomID}
		}
	}
	return result, err
}
