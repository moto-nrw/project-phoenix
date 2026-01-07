package mocks

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/stretchr/testify/mock"
	"github.com/uptrace/bun"
)

// UserServiceMock provides a testify mock for users.PersonService
type UserServiceMock struct {
	mock.Mock
	studentRepo *StudentRepositoryMock
	staffRepo   *StaffRepositoryMock
	teacherRepo *TeacherRepositoryMock
}

// NewUserServiceMock creates a new mock with embedded repository mocks
func NewUserServiceMock() *UserServiceMock {
	return &UserServiceMock{
		studentRepo: NewStudentRepositoryMock(),
		staffRepo:   NewStaffRepositoryMock(),
		teacherRepo: NewTeacherRepositoryMock(),
	}
}

// NewUserServiceMockWithRepos creates a mock with custom repository mocks
func NewUserServiceMockWithRepos(studentRepo *StudentRepositoryMock, staffRepo *StaffRepositoryMock, teacherRepo *TeacherRepositoryMock) *UserServiceMock {
	return &UserServiceMock{
		studentRepo: studentRepo,
		staffRepo:   staffRepo,
		teacherRepo: teacherRepo,
	}
}

// Repository accessors - commonly used in policy tests
func (m *UserServiceMock) StudentRepository() users.StudentRepository {
	m.Called()
	return m.studentRepo
}

func (m *UserServiceMock) StaffRepository() users.StaffRepository {
	m.Called()
	return m.staffRepo
}

func (m *UserServiceMock) TeacherRepository() users.TeacherRepository {
	m.Called()
	return m.teacherRepo
}

// GetStudentMock returns the embedded student repository mock for test setup
func (m *UserServiceMock) GetStudentMock() *StudentRepositoryMock {
	return m.studentRepo
}

// GetStaffMock returns the embedded staff repository mock for test setup
func (m *UserServiceMock) GetStaffMock() *StaffRepositoryMock {
	return m.staffRepo
}

// GetTeacherMock returns the embedded teacher repository mock for test setup
func (m *UserServiceMock) GetTeacherMock() *TeacherRepositoryMock {
	return m.teacherRepo
}

// TransactionalService
func (m *UserServiceMock) WithTx(tx bun.Tx) interface{} {
	return m
}

// Most commonly used methods in policy tests
func (m *UserServiceMock) FindByAccountID(ctx context.Context, accountID int64) (*users.Person, error) {
	args := m.Called(ctx, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*users.Person), args.Error(1)
}

func (m *UserServiceMock) GetStudentsByTeacher(ctx context.Context, teacherID int64) ([]*users.Student, error) {
	args := m.Called(ctx, teacherID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*users.Student), args.Error(1)
}

func (m *UserServiceMock) GetStudentsWithGroupsByTeacher(ctx context.Context, teacherID int64) ([]usersSvc.StudentWithGroup, error) {
	args := m.Called(ctx, teacherID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]usersSvc.StudentWithGroup), args.Error(1)
}

// Remaining interface methods
func (m *UserServiceMock) Get(ctx context.Context, id interface{}) (*users.Person, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*users.Person), args.Error(1)
}

func (m *UserServiceMock) GetByIDs(ctx context.Context, ids []int64) (map[int64]*users.Person, error) {
	args := m.Called(ctx, ids)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[int64]*users.Person), args.Error(1)
}

func (m *UserServiceMock) Create(ctx context.Context, person *users.Person) error {
	args := m.Called(ctx, person)
	return args.Error(0)
}

func (m *UserServiceMock) Update(ctx context.Context, person *users.Person) error {
	args := m.Called(ctx, person)
	return args.Error(0)
}

func (m *UserServiceMock) Delete(ctx context.Context, id interface{}) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *UserServiceMock) List(ctx context.Context, options *base.QueryOptions) ([]*users.Person, error) {
	args := m.Called(ctx, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*users.Person), args.Error(1)
}

func (m *UserServiceMock) FindByTagID(ctx context.Context, tagID string) (*users.Person, error) {
	args := m.Called(ctx, tagID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*users.Person), args.Error(1)
}

func (m *UserServiceMock) FindByName(ctx context.Context, firstName, lastName string) ([]*users.Person, error) {
	args := m.Called(ctx, firstName, lastName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*users.Person), args.Error(1)
}

func (m *UserServiceMock) LinkToAccount(ctx context.Context, personID int64, accountID int64) error {
	args := m.Called(ctx, personID, accountID)
	return args.Error(0)
}

func (m *UserServiceMock) UnlinkFromAccount(ctx context.Context, personID int64) error {
	args := m.Called(ctx, personID)
	return args.Error(0)
}

func (m *UserServiceMock) LinkToRFIDCard(ctx context.Context, personID int64, tagID string) error {
	args := m.Called(ctx, personID, tagID)
	return args.Error(0)
}

func (m *UserServiceMock) UnlinkFromRFIDCard(ctx context.Context, personID int64) error {
	args := m.Called(ctx, personID)
	return args.Error(0)
}

func (m *UserServiceMock) GetFullProfile(ctx context.Context, personID int64) (*users.Person, error) {
	args := m.Called(ctx, personID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*users.Person), args.Error(1)
}

func (m *UserServiceMock) FindByGuardianID(ctx context.Context, guardianAccountID int64) ([]*users.Person, error) {
	args := m.Called(ctx, guardianAccountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*users.Person), args.Error(1)
}

func (m *UserServiceMock) ListAvailableRFIDCards(ctx context.Context) ([]*users.RFIDCard, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*users.RFIDCard), args.Error(1)
}

func (m *UserServiceMock) ValidateStaffPIN(ctx context.Context, pin string) (*users.Staff, error) {
	args := m.Called(ctx, pin)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*users.Staff), args.Error(1)
}

func (m *UserServiceMock) ValidateStaffPINForSpecificStaff(ctx context.Context, staffID int64, pin string) (*users.Staff, error) {
	args := m.Called(ctx, staffID, pin)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*users.Staff), args.Error(1)
}
