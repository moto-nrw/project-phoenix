package timetracking

import (
	"context"
	"time"
)

// mockGroupRepository is the retained active-group double the work-session
// tests drive for supervision lookups.
type mockGroupRepository struct {
	GroupRepository
	createFunc                      func(ctx context.Context, entity *Group) error
	findByIDFunc                    func(ctx context.Context, id interface{}) (*Group, error)
	findByIDForUpdateFunc           func(ctx context.Context, id int64) (*Group, error)
	listFunc                        func(ctx context.Context) ([]*Group, error)
	findActiveByDeviceIDFunc        func(ctx context.Context, deviceID int64) (*Group, error)
	findActiveByGroupIDFunc         func(ctx context.Context, groupID int64) ([]*Group, error)
	updateLastActivityFunc          func(ctx context.Context, id int64, lastActivity time.Time) error
	findActiveSessionsOlderThanFunc func(ctx context.Context, cutoffTime time.Time) ([]*Group, error)
	checkRoomConflictFunc           func(ctx context.Context, roomID int64, excludeGroupID int64) (bool, *Group, error)
}

func (m *mockGroupRepository) Create(ctx context.Context, entity *Group) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, entity)
	}
	return nil
}

func (m *mockGroupRepository) FindByID(ctx context.Context, id int64) (*Group, error) {
	if m.findByIDFunc != nil {
		return m.findByIDFunc(ctx, id)
	}
	return &Group{
		ID: 1,
	}, nil
}

func (m *mockGroupRepository) FindByIDForUpdate(ctx context.Context, id int64) (*Group, error) {
	if m.findByIDForUpdateFunc != nil {
		return m.findByIDForUpdateFunc(ctx, id)
	}
	if m.findByIDFunc != nil {
		return m.FindByID(ctx, id)
	}
	return &Group{
		ID: id,
	}, nil
}

func (m *mockGroupRepository) Update(ctx context.Context, entity *Group) error {
	return nil
}

func (m *mockGroupRepository) Delete(ctx context.Context, id int64) error {
	return nil
}

func (m *mockGroupRepository) FindActiveByRoomID(ctx context.Context, roomID int64) ([]*Group, error) {
	return nil, nil
}

func (m *mockGroupRepository) FindActiveByRoomIDAndDeviceID(ctx context.Context, roomID int64, deviceID int64) (*Group, error) {
	return nil, nil
}

func (m *mockGroupRepository) FindActiveByGroupID(ctx context.Context, groupID int64) ([]*Group, error) {
	if m.findActiveByGroupIDFunc != nil {
		return m.findActiveByGroupIDFunc(ctx, groupID)
	}
	return nil, nil
}

func (m *mockGroupRepository) FindActiveByGroupIDs(ctx context.Context, groupIDs []int64) ([]*Group, error) {
	return nil, nil
}

func (m *mockGroupRepository) FindByTimeRange(ctx context.Context, start, end time.Time) ([]*Group, error) {
	return nil, nil
}

func (m *mockGroupRepository) FindWithVisits(ctx context.Context, id int64) (*Group, error) {
	return nil, nil
}

func (m *mockGroupRepository) FindWithSupervisors(ctx context.Context, id int64) (*Group, error) {
	return nil, nil
}

func (m *mockGroupRepository) FindActiveByGroupIDWithDevice(ctx context.Context, groupID int64) ([]*Group, error) {
	return nil, nil
}

func (m *mockGroupRepository) FindActiveByDeviceID(ctx context.Context, deviceID int64) (*Group, error) {
	if m.findActiveByDeviceIDFunc != nil {
		return m.findActiveByDeviceIDFunc(ctx, deviceID)
	}
	return nil, nil
}

func (m *mockGroupRepository) FindActiveByDeviceIDWithRelations(ctx context.Context, deviceID int64) (*Group, error) {
	return nil, nil
}

func (m *mockGroupRepository) CheckRoomConflict(ctx context.Context, roomID int64, excludeGroupID int64) (bool, *Group, error) {
	if m.checkRoomConflictFunc != nil {
		return m.checkRoomConflictFunc(ctx, roomID, excludeGroupID)
	}
	return false, nil, nil
}

func (m *mockGroupRepository) UpdateLastActivity(ctx context.Context, id int64, lastActivity time.Time) error {
	if m.updateLastActivityFunc != nil {
		return m.updateLastActivityFunc(ctx, id, lastActivity)
	}
	return nil
}

func (m *mockGroupRepository) FindActiveSessionsOlderThan(ctx context.Context, cutoffTime time.Time) ([]*Group, error) {
	if m.findActiveSessionsOlderThanFunc != nil {
		return m.findActiveSessionsOlderThanFunc(ctx, cutoffTime)
	}
	return nil, nil
}

func (m *mockGroupRepository) FindInactiveSessions(ctx context.Context, inactiveDuration time.Duration) ([]*Group, error) {
	return nil, nil
}

func (m *mockGroupRepository) FindUnclaimed(ctx context.Context) ([]*Group, error) {
	return nil, nil
}

func (m *mockGroupRepository) FindActiveGroups(ctx context.Context) ([]*Group, error) {
	if m.listFunc == nil {
		return nil, nil
	}
	groups, err := m.listFunc(ctx)
	if err != nil {
		return nil, err
	}
	var open []*Group
	for _, group := range groups {
		if group.IsOpen() {
			open = append(open, group)
		}
	}
	return open, nil
}

func (m *mockGroupRepository) GetOccupiedRoomIDs(ctx context.Context, roomIDs []int64) (map[int64]bool, error) {
	return nil, nil
}

func (m *mockGroupRepository) GetOccupiedActivityGroupIDs(ctx context.Context, groupIDs []int64) (map[int64]bool, error) {
	return nil, nil
}
