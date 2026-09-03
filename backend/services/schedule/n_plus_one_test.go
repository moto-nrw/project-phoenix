package schedule

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
)

func (r *autoStartStaffRepo) FindByStaffIDsAndDate(context.Context, []int64, timezone.Date) ([]*scheduleModel.InstanceStaff, error) {
	return nil, nil
}

func (r *autoStartInstanceRepo) FindByActiveGroupIDs(context.Context, []int64) ([]*scheduleModel.ActivityInstance, error) {
	return nil, nil
}

func (m *shiftMockRepo) FindByOriginShiftIDs(ctx context.Context, originShiftIDs []int64) ([]*scheduleModel.StaffShift, error) {
	rows := make([]*scheduleModel.StaffShift, 0)
	for _, originShiftID := range originShiftIDs {
		found, err := m.FindByOriginShiftID(ctx, originShiftID)
		if err != nil {
			return nil, err
		}
		rows = append(rows, found...)
	}
	return rows, nil
}

func (r *seriesMockScheduleRepo) List(context.Context, *modelBase.QueryOptions) ([]*activitiesModel.Schedule, error) {
	if r.err != nil {
		return nil, r.err
	}
	rows := make([]*activitiesModel.Schedule, 0)
	for _, found := range r.byGroup {
		rows = append(rows, found...)
	}
	return rows, nil
}

func (r *templateEndUnitScheduleRepo) List(context.Context, *modelBase.QueryOptions) ([]*activitiesModel.Schedule, error) {
	return r.schedules, r.findErr
}

func (r *bridgeInstanceRepoStub) FindByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64) ([]*scheduleModel.ActivityInstance, error) {
	rows := make([]*scheduleModel.ActivityInstance, 0)
	for _, activeGroupID := range activeGroupIDs {
		instance, err := r.FindByActiveGroupID(ctx, activeGroupID)
		if err != nil {
			return nil, err
		}
		if instance != nil {
			rows = append(rows, instance)
		}
	}
	return rows, nil
}

func (s *reopenStudentLockStub) FindByIDsForUpdate(ctx context.Context, ids []int64) (map[int64]*usersModel.Student, error) {
	students := make(map[int64]*usersModel.Student, len(ids))
	for _, id := range ids {
		student, err := s.FindByIDForUpdate(ctx, id)
		if err != nil {
			return nil, err
		}
		students[id] = student
	}
	return students, nil
}

func (r *reopenVisitStub) GetCurrentByStudentIDs(_ context.Context, ids []int64) (map[int64]*activeModel.Visit, error) {
	visits := make(map[int64]*activeModel.Visit, len(ids))
	for _, id := range ids {
		if visit := r.current[id]; visit != nil {
			visits[id] = visit
		}
	}
	return visits, nil
}

func (r *absorbInstanceStudentRepo) FindByInstanceIDs(_ context.Context, instanceIDs []int64) ([]*scheduleModel.InstanceStudent, error) {
	if len(instanceIDs) > 0 && r.unplannedStudentID > 0 {
		r.lookups = append(r.lookups, absorbedAttendanceUpdate{
			instanceID: instanceIDs[0],
			studentID:  r.unplannedStudentID,
		})
	}
	return nil, nil
}

func (r *fakeOpsStaffRepo) FindByInstanceIDs(_ context.Context, instanceIDs []int64) ([]*scheduleModel.InstanceStaff, error) {
	if r.err != nil {
		return nil, r.err
	}
	rows := make([]*scheduleModel.InstanceStaff, 0)
	for _, instanceID := range instanceIDs {
		for _, row := range r.byInstance[instanceID] {
			copy := *row
			copy.InstanceID = instanceID
			rows = append(rows, &copy)
		}
	}
	return rows, nil
}

func (r *fakeOpsInstanceStudentRepo) FindByInstanceIDs(_ context.Context, instanceIDs []int64) ([]*scheduleModel.InstanceStudent, error) {
	if r.err != nil {
		return nil, r.err
	}
	rows := make([]*scheduleModel.InstanceStudent, 0)
	for _, instanceID := range instanceIDs {
		for _, row := range r.byInstance[instanceID] {
			copy := *row
			copy.InstanceID = instanceID
			rows = append(rows, &copy)
		}
	}
	return rows, nil
}
