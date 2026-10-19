package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/schoolsetup"
)

type progressFlags interface {
	ProgressFlags() (bool, bool, bool, bool, bool)
}
type progressSource[Flags progressFlags] interface {
	Progress(context.Context, int64) (Flags, error)
}
type progress[Flags progressFlags] struct{ source progressSource[Flags] }

func NewProgress[Flags progressFlags](source progressSource[Flags]) schoolsetup.Progress {
	return progress[Flags]{source: source}
}
func (p progress[Flags]) Facts(ctx context.Context, tenantID int64) (schoolsetup.Facts, error) {
	row, err := p.source.Progress(ctx, tenantID)
	if err != nil {
		return schoolsetup.Facts{}, err
	}
	staff, room, group, student, guardian := row.ProgressFlags()
	return schoolsetup.Facts{StaffInvited: staff, RoomCreated: room, GroupCreated: group, StudentEnrolled: student, GuardianInvited: guardian}, nil
}
