package mocks

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/mock"
	"github.com/uptrace/bun"
)

// EducationServiceMock provides a testify mock for education.Service
type EducationServiceMock struct {
	mock.Mock
}

// NewEducationServiceMock creates a new mock instance
func NewEducationServiceMock() *EducationServiceMock {
	return &EducationServiceMock{}
}

// WithTx implements base.TransactionalService
func (m *EducationServiceMock) WithTx(tx bun.Tx) interface{} {
	return m
}

// GetTeacherGroups is commonly used in policy tests
func (m *EducationServiceMock) GetTeacherGroups(ctx context.Context, teacherID int64) ([]*education.Group, error) {
	args := m.Called(ctx, teacherID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*education.Group), args.Error(1)
}

// --- Group operations (rarely mocked, default returns) ---

func (m *EducationServiceMock) GetGroup(ctx context.Context, id int64) (*education.Group, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*education.Group), args.Error(1)
}

func (m *EducationServiceMock) GetGroupsByIDs(ctx context.Context, ids []int64) (map[int64]*education.Group, error) {
	args := m.Called(ctx, ids)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[int64]*education.Group), args.Error(1)
}

func (m *EducationServiceMock) CreateGroup(ctx context.Context, group *education.Group) error {
	args := m.Called(ctx, group)
	return args.Error(0)
}

func (m *EducationServiceMock) UpdateGroup(ctx context.Context, group *education.Group) error {
	args := m.Called(ctx, group)
	return args.Error(0)
}

func (m *EducationServiceMock) DeleteGroup(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *EducationServiceMock) ListGroups(ctx context.Context, options *base.QueryOptions) ([]*education.Group, error) {
	args := m.Called(ctx, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*education.Group), args.Error(1)
}

func (m *EducationServiceMock) FindGroupByName(ctx context.Context, name string) (*education.Group, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*education.Group), args.Error(1)
}

func (m *EducationServiceMock) FindGroupsByRoom(ctx context.Context, roomID int64) ([]*education.Group, error) {
	args := m.Called(ctx, roomID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*education.Group), args.Error(1)
}

func (m *EducationServiceMock) FindGroupWithRoom(ctx context.Context, groupID int64) (*education.Group, error) {
	args := m.Called(ctx, groupID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*education.Group), args.Error(1)
}

func (m *EducationServiceMock) AssignRoomToGroup(ctx context.Context, groupID, roomID int64) error {
	args := m.Called(ctx, groupID, roomID)
	return args.Error(0)
}

func (m *EducationServiceMock) RemoveRoomFromGroup(ctx context.Context, groupID int64) error {
	args := m.Called(ctx, groupID)
	return args.Error(0)
}

// --- Group-Teacher operations ---

func (m *EducationServiceMock) AddTeacherToGroup(ctx context.Context, groupID, teacherID int64) error {
	args := m.Called(ctx, groupID, teacherID)
	return args.Error(0)
}

func (m *EducationServiceMock) RemoveTeacherFromGroup(ctx context.Context, groupID, teacherID int64) error {
	args := m.Called(ctx, groupID, teacherID)
	return args.Error(0)
}

func (m *EducationServiceMock) UpdateGroupTeachers(ctx context.Context, groupID int64, teacherIDs []int64) error {
	args := m.Called(ctx, groupID, teacherIDs)
	return args.Error(0)
}

func (m *EducationServiceMock) GetGroupTeachers(ctx context.Context, groupID int64) ([]*users.Teacher, error) {
	args := m.Called(ctx, groupID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*users.Teacher), args.Error(1)
}

// --- Substitution operations ---

func (m *EducationServiceMock) CreateSubstitution(ctx context.Context, substitution *education.GroupSubstitution) error {
	args := m.Called(ctx, substitution)
	return args.Error(0)
}

func (m *EducationServiceMock) UpdateSubstitution(ctx context.Context, substitution *education.GroupSubstitution) error {
	args := m.Called(ctx, substitution)
	return args.Error(0)
}

func (m *EducationServiceMock) DeleteSubstitution(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *EducationServiceMock) GetSubstitution(ctx context.Context, id int64) (*education.GroupSubstitution, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*education.GroupSubstitution), args.Error(1)
}

func (m *EducationServiceMock) ListSubstitutions(ctx context.Context, options *base.QueryOptions) ([]*education.GroupSubstitution, error) {
	args := m.Called(ctx, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*education.GroupSubstitution), args.Error(1)
}

func (m *EducationServiceMock) GetActiveSubstitutions(ctx context.Context, date time.Time) ([]*education.GroupSubstitution, error) {
	args := m.Called(ctx, date)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*education.GroupSubstitution), args.Error(1)
}

func (m *EducationServiceMock) GetActiveGroupSubstitutions(ctx context.Context, groupID int64, date time.Time) ([]*education.GroupSubstitution, error) {
	args := m.Called(ctx, groupID, date)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*education.GroupSubstitution), args.Error(1)
}

func (m *EducationServiceMock) GetStaffSubstitutions(ctx context.Context, staffID int64, asRegular bool) ([]*education.GroupSubstitution, error) {
	args := m.Called(ctx, staffID, asRegular)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*education.GroupSubstitution), args.Error(1)
}

func (m *EducationServiceMock) CheckSubstitutionConflicts(ctx context.Context, staffID int64, startDate, endDate time.Time) ([]*education.GroupSubstitution, error) {
	args := m.Called(ctx, staffID, startDate, endDate)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*education.GroupSubstitution), args.Error(1)
}
