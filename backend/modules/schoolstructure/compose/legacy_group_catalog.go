package compose

import (
	"context"

	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// These methods implement retained consumer-owned row ports. The private
// Postgres provider never escapes composition; no repository protocol is
// added to the public School Structure contract.
func (r *GroupRepository) Create(ctx context.Context, group *educationModels.Group) error {
	return r.store.Create(ctx, group)
}

func (r *GroupRepository) Update(ctx context.Context, group *educationModels.Group) error {
	return r.store.Update(ctx, group)
}

func (r *GroupRepository) Delete(ctx context.Context, id any) error {
	return r.store.Delete(ctx, id)
}

func (r *GroupRepository) FindByID(ctx context.Context, id any) (*educationModels.Group, error) {
	return r.store.FindByID(ctx, id)
}

func (r *GroupRepository) FindByIDForUpdate(ctx context.Context, id any) (*educationModels.Group, error) {
	return r.store.FindByIDForUpdate(ctx, id)
}

func (r *GroupRepository) FindByName(ctx context.Context, name string) (*educationModels.Group, error) {
	return r.store.FindByName(ctx, name)
}

func (r *GroupRepository) FindWithRoom(ctx context.Context, id int64) (*educationModels.Group, error) {
	return r.store.FindWithRoom(ctx, id)
}

func (r *GroupRepository) FindByIDs(ctx context.Context, ids []int64) (map[int64]*educationModels.Group, error) {
	return r.store.FindByIDs(ctx, ids)
}

func (r *GroupRepository) FindByIDsWithRooms(ctx context.Context, ids []int64) (map[int64]*educationModels.Group, error) {
	return r.store.FindByIDsWithRooms(ctx, ids)
}

func (r *GroupRepository) FindByTeacher(ctx context.Context, id int64) ([]*educationModels.Group, error) {
	return r.store.FindByTeacher(ctx, id)
}

func (r *GroupRepository) ListWithRooms(ctx context.Context, query *educationModels.GroupListQuery) ([]*educationModels.Group, error) {
	return r.store.ListWithRooms(ctx, query)
}

func (r *GroupRepository) List(ctx context.Context, filters map[string]any) ([]*educationModels.Group, error) {
	return r.store.List(ctx, filters)
}

func (r *GroupRepository) CountGroups(ctx context.Context, query *educationModels.GroupListQuery) (int, error) {
	return r.store.CountGroups(ctx, query)
}

func (r *GroupRepository) Exists(ctx context.Context, id int64) (bool, error) {
	return r.store.Exists(ctx, id)
}

func (r *GroupRepository) ListStaffIDsByEducationGroupIDs(ctx context.Context, ids []int64, on calendar.Date) ([]educationModels.StaffGroupID, error) {
	return r.store.ListStaffIDsByEducationGroupIDs(ctx, ids, on)
}
