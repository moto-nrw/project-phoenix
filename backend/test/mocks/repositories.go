package mocks

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/mock"
)

// StudentRepositoryMock provides a testify mock for users.StudentRepository
type StudentRepositoryMock struct {
	mock.Mock
}

func NewStudentRepositoryMock() *StudentRepositoryMock {
	return &StudentRepositoryMock{}
}

// Most commonly used methods in policy tests
func (m *StudentRepositoryMock) FindByID(ctx context.Context, id interface{}) (*users.Student, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*users.Student), args.Error(1)
}

func (m *StudentRepositoryMock) FindByPersonID(ctx context.Context, personID int64) (*users.Student, error) {
	args := m.Called(ctx, personID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*users.Student), args.Error(1)
}

func (m *StudentRepositoryMock) FindByGroupIDs(ctx context.Context, groupIDs []int64) ([]*users.Student, error) {
	args := m.Called(ctx, groupIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*users.Student), args.Error(1)
}

func (m *StudentRepositoryMock) FindByTeacherID(ctx context.Context, teacherID int64) ([]*users.Student, error) {
	args := m.Called(ctx, teacherID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*users.Student), args.Error(1)
}

func (m *StudentRepositoryMock) FindByTeacherIDWithGroups(ctx context.Context, teacherID int64) ([]*users.StudentWithGroupInfo, error) {
	args := m.Called(ctx, teacherID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*users.StudentWithGroupInfo), args.Error(1)
}

// Remaining interface methods
func (m *StudentRepositoryMock) Create(ctx context.Context, student *users.Student) error {
	args := m.Called(ctx, student)
	return args.Error(0)
}

func (m *StudentRepositoryMock) FindByGroupID(ctx context.Context, groupID int64) ([]*users.Student, error) {
	args := m.Called(ctx, groupID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*users.Student), args.Error(1)
}

func (m *StudentRepositoryMock) FindBySchoolClass(ctx context.Context, schoolClass string) ([]*users.Student, error) {
	args := m.Called(ctx, schoolClass)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*users.Student), args.Error(1)
}

func (m *StudentRepositoryMock) Update(ctx context.Context, student *users.Student) error {
	args := m.Called(ctx, student)
	return args.Error(0)
}

func (m *StudentRepositoryMock) Delete(ctx context.Context, id interface{}) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *StudentRepositoryMock) List(ctx context.Context, filters map[string]interface{}) ([]*users.Student, error) {
	args := m.Called(ctx, filters)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*users.Student), args.Error(1)
}

func (m *StudentRepositoryMock) ListWithOptions(ctx context.Context, options *base.QueryOptions) ([]*users.Student, error) {
	args := m.Called(ctx, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*users.Student), args.Error(1)
}

func (m *StudentRepositoryMock) CountWithOptions(ctx context.Context, options *base.QueryOptions) (int, error) {
	args := m.Called(ctx, options)
	return args.Int(0), args.Error(1)
}

func (m *StudentRepositoryMock) AssignToGroup(ctx context.Context, studentID int64, groupID int64) error {
	args := m.Called(ctx, studentID, groupID)
	return args.Error(0)
}

func (m *StudentRepositoryMock) RemoveFromGroup(ctx context.Context, studentID int64) error {
	args := m.Called(ctx, studentID)
	return args.Error(0)
}

func (m *StudentRepositoryMock) FindByNameAndClass(ctx context.Context, firstName, lastName, schoolClass string) ([]*users.Student, error) {
	args := m.Called(ctx, firstName, lastName, schoolClass)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*users.Student), args.Error(1)
}

// StaffRepositoryMock provides a testify mock for users.StaffRepository
type StaffRepositoryMock struct {
	mock.Mock
}

func NewStaffRepositoryMock() *StaffRepositoryMock {
	return &StaffRepositoryMock{}
}

func (m *StaffRepositoryMock) FindByPersonID(ctx context.Context, personID int64) (*users.Staff, error) {
	args := m.Called(ctx, personID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*users.Staff), args.Error(1)
}

func (m *StaffRepositoryMock) FindWithPerson(ctx context.Context, id int64) (*users.Staff, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*users.Staff), args.Error(1)
}

func (m *StaffRepositoryMock) Create(ctx context.Context, staff *users.Staff) error {
	args := m.Called(ctx, staff)
	return args.Error(0)
}

func (m *StaffRepositoryMock) FindByID(ctx context.Context, id interface{}) (*users.Staff, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*users.Staff), args.Error(1)
}

func (m *StaffRepositoryMock) Update(ctx context.Context, staff *users.Staff) error {
	args := m.Called(ctx, staff)
	return args.Error(0)
}

func (m *StaffRepositoryMock) Delete(ctx context.Context, id interface{}) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *StaffRepositoryMock) List(ctx context.Context, filters map[string]interface{}) ([]*users.Staff, error) {
	args := m.Called(ctx, filters)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*users.Staff), args.Error(1)
}

func (m *StaffRepositoryMock) UpdateNotes(ctx context.Context, id int64, notes string) error {
	args := m.Called(ctx, id, notes)
	return args.Error(0)
}

// TeacherRepositoryMock provides a testify mock for users.TeacherRepository
type TeacherRepositoryMock struct {
	mock.Mock
}

func NewTeacherRepositoryMock() *TeacherRepositoryMock {
	return &TeacherRepositoryMock{}
}

func (m *TeacherRepositoryMock) FindByStaffID(ctx context.Context, staffID int64) (*users.Teacher, error) {
	args := m.Called(ctx, staffID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*users.Teacher), args.Error(1)
}

func (m *TeacherRepositoryMock) FindWithStaffAndPerson(ctx context.Context, id int64) (*users.Teacher, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*users.Teacher), args.Error(1)
}

func (m *TeacherRepositoryMock) Create(ctx context.Context, teacher *users.Teacher) error {
	args := m.Called(ctx, teacher)
	return args.Error(0)
}

func (m *TeacherRepositoryMock) FindByID(ctx context.Context, id interface{}) (*users.Teacher, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*users.Teacher), args.Error(1)
}

func (m *TeacherRepositoryMock) FindBySpecialization(ctx context.Context, specialization string) ([]*users.Teacher, error) {
	args := m.Called(ctx, specialization)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*users.Teacher), args.Error(1)
}

func (m *TeacherRepositoryMock) Update(ctx context.Context, teacher *users.Teacher) error {
	args := m.Called(ctx, teacher)
	return args.Error(0)
}

func (m *TeacherRepositoryMock) Delete(ctx context.Context, id interface{}) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *TeacherRepositoryMock) List(ctx context.Context, filters map[string]interface{}) ([]*users.Teacher, error) {
	args := m.Called(ctx, filters)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*users.Teacher), args.Error(1)
}

func (m *TeacherRepositoryMock) ListWithOptions(ctx context.Context, options *base.QueryOptions) ([]*users.Teacher, error) {
	args := m.Called(ctx, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*users.Teacher), args.Error(1)
}

func (m *TeacherRepositoryMock) FindByGroupID(ctx context.Context, groupID int64) ([]*users.Teacher, error) {
	args := m.Called(ctx, groupID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*users.Teacher), args.Error(1)
}

func (m *TeacherRepositoryMock) UpdateQualifications(ctx context.Context, id int64, qualifications string) error {
	args := m.Called(ctx, id, qualifications)
	return args.Error(0)
}
