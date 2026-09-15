package api

import (
	"context"

	presenceAPI "github.com/moto-nrw/project-phoenix/modules/studentpresence/inbound/presence"
)

type activePeople struct {
	source interface {
		FindByAccountID(context.Context, int64) (int64, bool, error)
		GetStaffByPersonID(context.Context, int64) (int64, int64, bool, error)
		GetStudentByPersonID(context.Context, int64) (int64, *int64, bool, error)
		GetTeacherByStaffID(context.Context, int64) (int64, bool, error)
		GetStudentByID(context.Context, int64) (int64, *int64, bool, error)
	}
}

func (p activePeople) FindByAccountID(ctx context.Context, key int64) (*presenceAPI.PersonIdentity, error) {
	id, found, err := p.source.FindByAccountID(ctx, key)
	if !found {
		return nil, err
	}
	return &presenceAPI.PersonIdentity{ID: id}, err
}

func (p activePeople) GetStaffByPersonID(ctx context.Context, key int64) (*presenceAPI.StaffIdentity, error) {
	id, tenantID, found, err := p.source.GetStaffByPersonID(ctx, key)
	if !found {
		return nil, err
	}
	return &presenceAPI.StaffIdentity{ID: id, TenantID: tenantID}, err
}

func (p activePeople) GetStudentByPersonID(ctx context.Context, key int64) (*presenceAPI.StudentIdentity, error) {
	id, groupID, found, err := p.source.GetStudentByPersonID(ctx, key)
	if !found {
		return nil, err
	}
	return &presenceAPI.StudentIdentity{ID: id, GroupID: groupID}, err
}

func (p activePeople) GetTeacherByStaffID(ctx context.Context, key int64) (*presenceAPI.TeacherIdentity, error) {
	id, found, err := p.source.GetTeacherByStaffID(ctx, key)
	if !found {
		return nil, err
	}
	return &presenceAPI.TeacherIdentity{ID: id}, err
}

func (p activePeople) GetStudentByID(ctx context.Context, key int64) (*presenceAPI.StudentIdentity, error) {
	id, groupID, found, err := p.source.GetStudentByID(ctx, key)
	if !found {
		return nil, err
	}
	return &presenceAPI.StudentIdentity{ID: id, GroupID: groupID}, err
}

type activeStaffAccess struct {
	source interface {
		GetCurrentStaff(context.Context) (int64, int64, bool, error)
		HasCurrentStaff(context.Context) (bool, error)
	}
}

func (p activeStaffAccess) GetCurrentStaff(ctx context.Context) (*presenceAPI.StaffIdentity, error) {
	id, tenantID, found, err := p.source.GetCurrentStaff(ctx)
	if !found {
		return nil, err
	}
	return &presenceAPI.StaffIdentity{ID: id, TenantID: tenantID}, err
}
func (p activeStaffAccess) HasCurrentStaff(ctx context.Context) (bool, error) {
	return p.source.HasCurrentStaff(ctx)
}
