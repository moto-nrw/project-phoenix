package repositories

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan/legacy/carelifecycle"
)

// CareMembershipCommands is bound by the root to School Membership. Care Plan
// supplies the frozen day and keeps its ledger and membership write in one UOW.
type CareMembershipCommands interface {
	EndCare(context.Context, []int64, string) (int64, error)
	ResumeCare(context.Context, int64, string, string, string) (bool, error)
}

type CareStudents struct {
	users.StudentRepository
	membership CareMembershipCommands
}

func NewCareStudents(students users.StudentRepository, membership CareMembershipCommands) CareStudents {
	return CareStudents{StudentRepository: students, membership: membership}
}

func (s CareStudents) EndCare(ctx context.Context, ids []int64, until string) error {
	changed, err := s.membership.EndCare(ctx, ids, until)
	if err == nil && changed != int64(len(ids)) {
		return carelifecycle.ErrCareExitPreviewChanged
	}
	return err
}

func (s CareStudents) ResumeCare(ctx context.Context, id int64, from, status, on string) error {
	changed, err := s.membership.ResumeCare(ctx, id, from, status, on)
	if err == nil && !changed {
		return carelifecycle.ErrCareResumeNotEnded
	}
	return err
}
