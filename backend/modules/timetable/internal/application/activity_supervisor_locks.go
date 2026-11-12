package application

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/timetable"
	activityContract "github.com/moto-ogs/project-phoenix/modules/timetable/activitycontract"
	"github.com/moto-ogs/project-phoenix/modules/timetable/internal/domain"
)

// lockActivitySupervisorStaff takes the staff rows of the activity's live
// supervisors and of the requested people in one ascending batch before the
// first supervisor row (#6702). An activity supervisor write may demote the
// current lead, promote another supervisor and add a new one; taking those
// people one by one, in roster or request order, let two writers hold one
// person each and wait for the other. The later supervisor writes re-read
// held rows only. Offboarded people are left out: a write that names one
// still fails on its own staff check.
func (s *ActivityService) lockActivitySupervisorStaff(ctx context.Context, groupID int64, staffIDs ...int64) error {
	ids := append([]int64(nil), staffIDs...)
	if groupID > 0 {
		current, err := s.timetable.ListPlannedSupervisors(ctx, timetable.PlannedSupervisorFilter{GroupIDs: []int64{groupID}})
		if err != nil {
			return &activityContract.ActivityError{Op: opFindGroupSupervisors, Err: err}
		}
		for _, supervisor := range current {
			ids = append(ids, supervisor.StaffID)
		}
	}
	ids = domain.UniquePositiveIDs(ids)
	if len(ids) == 0 {
		return nil
	}
	if s.directories.StaffPersons != nil {
		live, err := s.directories.StaffPersons(ctx, ids, false)
		if err != nil {
			return &activityContract.ActivityError{Op: "lock supervisor staff", Err: err}
		}
		ids = ids[:0]
		for id := range live {
			ids = append(ids, id)
		}
	}
	if err := lockStaffRows(ctx, s.timetable, ids); err != nil {
		return &activityContract.ActivityError{Op: "lock supervisor staff", Err: err}
	}
	return nil
}
