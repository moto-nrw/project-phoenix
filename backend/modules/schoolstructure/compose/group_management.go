package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/application/groups"
)

// NewGroupManagement builds the native facade over the existing group application.
func NewGroupManagement(groupRepo GroupRecords, groupTeachers GroupTeacherStore, classTeachers ClassTeacherStore, rooms RoomDirectory, teachers TeacherDirectory, staff StaffDirectory, students StudentCounter, handovers HandoverReader, runtime Runtime, options ...GroupServiceOptions) schoolstructure.GroupManagement {
	var option GroupServiceOptions
	if len(options) > 0 {
		option = options[0]
	}
	return groups.NewManagement(groupRepo, groups.NewService(groupRepo, groupTeachers, classTeachers, rooms, teachers, staff, students, handovers, runtime, option.Broadcaster, option.Audit))
}

type GroupServiceOptions struct {
	Broadcaster groups.GroupPublisher
	Audit       ClassAssignmentAudit
}
