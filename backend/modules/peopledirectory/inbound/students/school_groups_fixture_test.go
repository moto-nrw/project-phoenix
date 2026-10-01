package students_test

import (
	"context"

	studentsAPI "github.com/moto-nrw/project-phoenix/modules/peopledirectory/inbound/students"
)

// fixtureSchoolGroups binds the SchoolGroups port the way the production
// root does (api/students_owner_ports.go): School Structure's group service,
// reduced to the students' view. The fixture fills it from method values, so
// it names neither the service nor its group type.
type fixtureSchoolGroups struct {
	get      func(context.Context, int64) (*studentsAPI.SchoolGroup, error)
	byIDs    func(context.Context, []int64) (map[int64]*studentsAPI.SchoolGroup, error)
	list     func(context.Context) ([]*studentsAPI.SchoolGroup, error)
	teachers func(context.Context, int64) ([]studentsAPI.GroupTeacher, error)
}

func (f fixtureSchoolGroups) GetGroup(ctx context.Context, id int64) (*studentsAPI.SchoolGroup, error) {
	return f.get(ctx, id)
}

func (f fixtureSchoolGroups) GetGroupsByIDs(ctx context.Context, ids []int64) (map[int64]*studentsAPI.SchoolGroup, error) {
	return f.byIDs(ctx, ids)
}

func (f fixtureSchoolGroups) ListGroups(ctx context.Context) ([]*studentsAPI.SchoolGroup, error) {
	return f.list(ctx)
}

func (f fixtureSchoolGroups) GetGroupTeachers(ctx context.Context, groupID int64) ([]studentsAPI.GroupTeacher, error) {
	return f.teachers(ctx, groupID)
}

// schoolGroupView is the students' view of one group.
func schoolGroupView(id int64, name string, roomID *int64, roomName string) *studentsAPI.SchoolGroup {
	return &studentsAPI.SchoolGroup{ID: id, Name: name, RoomID: roomID, RoomName: roomName}
}
