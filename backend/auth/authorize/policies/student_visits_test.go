package policies_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/authorize/policies"
	"github.com/moto-nrw/project-phoenix/auth/authorize/policy"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock services
type MockEducationService struct {
	mock.Mock
}

func (m *MockEducationService) GetTeacherGroups(ctx context.Context, teacherID int64) ([]*education.Group, error) {
	args := m.Called(ctx, teacherID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*education.Group), args.Error(1)
}

// Implement remaining education.Service methods as needed...

type MockUserService struct {
	mock.Mock
}

func (m *MockUserService) FindByAccountID(ctx context.Context, accountID int64) (*userModels.Person, error) {
	args := m.Called(ctx, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*userModels.Person), args.Error(1)
}

func (m *MockUserService) StudentRepository() userModels.StudentRepository {
	args := m.Called()
	return args.Get(0).(userModels.StudentRepository)
}

func (m *MockUserService) StaffRepository() userModels.StaffRepository {
	args := m.Called()
	return args.Get(0).(userModels.StaffRepository)
}

func (m *MockUserService) TeacherRepository() userModels.TeacherRepository {
	args := m.Called()
	return args.Get(0).(userModels.TeacherRepository)
}

// Implement remaining users.PersonService methods as needed...

type MockActiveService struct {
	mock.Mock
}

func (m *MockActiveService) GetVisit(ctx context.Context, visitID int64) (*active.Visit, error) {
	args := m.Called(ctx, visitID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*active.Visit), args.Error(1)
}

// Mock repositories
type MockStudentRepository struct {
	mock.Mock
}

func (m *MockStudentRepository) FindByID(ctx context.Context, id int64) (*userModels.Student, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*userModels.Student), args.Error(1)
}

func (m *MockStudentRepository) FindByPersonID(ctx context.Context, personID int64) (*userModels.Student, error) {
	args := m.Called(ctx, personID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*userModels.Student), args.Error(1)
}

type MockStaffRepository struct {
	mock.Mock
}

func (m *MockStaffRepository) FindByPersonID(ctx context.Context, personID int64) (*userModels.Staff, error) {
	args := m.Called(ctx, personID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*userModels.Staff), args.Error(1)
}

type MockTeacherRepository struct {
	mock.Mock
}

func (m *MockTeacherRepository) FindByStaffID(ctx context.Context, staffID int64) (*userModels.Teacher, error) {
	args := m.Called(ctx, staffID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*userModels.Teacher), args.Error(1)
}

func TestStudentVisitPolicy_Evaluate(t *testing.T) {
	tests := []struct {
		name           string
		authContext    *policy.Context
		setupMocks     func(*MockEducationService, *MockUserService, *MockActiveService, *MockStudentRepository, *MockStaffRepository, *MockTeacherRepository)
		expectedResult bool
		expectError    bool
	}{
		{
			name: "admin can always access",
			authContext: &policy.Context{
				Subject: policy.Subject{
					AccountID:   1,
					Roles:       []string{"admin"},
					Permissions: []string{},
				},
				Resource: policy.Resource{Type: "visit", ID: int64(123)},
				Action:   policy.ActionView,
			},
			setupMocks: func(e *MockEducationService, u *MockUserService, a *MockActiveService, s *MockStudentRepository, st *MockStaffRepository, t *MockTeacherRepository) {
			},
			expectedResult: true,
			expectError:    false,
		},
		{
			name: "user with visit read permission can access",
			authContext: &policy.Context{
				Subject: policy.Subject{
					AccountID:   2,
					Roles:       []string{"staff"},
					Permissions: []string{permissions.VisitsRead},
				},
				Resource: policy.Resource{Type: "visit", ID: int64(456)},
				Action:   policy.ActionView,
			},
			setupMocks: func(e *MockEducationService, u *MockUserService, a *MockActiveService, s *MockStudentRepository, st *MockStaffRepository, t *MockTeacherRepository) {
			},
			expectedResult: true,
			expectError:    false,
		},
		{
			name: "student can access own visit via visit ID",
			authContext: &policy.Context{
				Subject: policy.Subject{
					AccountID:   3,
					Roles:       []string{"student"},
					Permissions: []string{},
				},
				Resource: policy.Resource{Type: "visit", ID: int64(789)},
				Action:   policy.ActionView,
			},
			setupMocks: func(eduService *MockEducationService, userService *MockUserService, activeService *MockActiveService, studentRepo *MockStudentRepository, staffRepo *MockStaffRepository, teacherRepo *MockTeacherRepository) {
				// Mock getting visit to find student ID
				visit := &active.Visit{
					Model: base.Model{
						ID: 789,
					},
					StudentID: 100,
				}
				activeService.On("GetVisit", mock.Anything, int64(789)).Return(visit, nil)

				// Mock finding person by account ID
				person := &userModels.Person{
					Model:     base.Model{ID: 10},
					AccountID: &[]int64{3}[0],
				}
				userService.On("FindByAccountID", mock.Anything, int64(3)).Return(person, nil)

				// Mock student repository
				userService.On("StudentRepository").Return(studentRepo)
				student := &userModels.Student{
					Model:    base.Model{ID: 100},
					PersonID: 10,
				}
				studentRepo.On("FindByPersonID", mock.Anything, int64(10)).Return(student, nil)

				// Mock staff repository (returns nil - person is not staff)
				userService.On("StaffRepository").Return(staffRepo)
				staffRepo.On("FindByPersonID", mock.Anything, int64(10)).Return(nil, errors.New("not found"))
			},
			expectedResult: true,
			expectError:    false,
		},
		{
			name: "teacher can access visit of student in their group",
			authContext: &policy.Context{
				Subject: policy.Subject{
					AccountID:   4,
					Roles:       []string{"teacher"},
					Permissions: []string{},
				},
				Resource: policy.Resource{Type: "visit", ID: int64(999)},
				Action:   policy.ActionView,
			},
			setupMocks: func(eduService *MockEducationService, userService *MockUserService, activeService *MockActiveService, studentRepo *MockStudentRepository, staffRepo *MockStaffRepository, teacherRepo *MockTeacherRepository) {
				// Mock getting visit to find student ID
				visit := &active.Visit{
					Model: base.Model{
						ID: 999,
					},
					StudentID: 200,
				}
				activeService.On("GetVisit", mock.Anything, int64(999)).Return(visit, nil)

				// Mock finding person by account ID
				person := &userModels.Person{
					Model:     base.Model{ID: 20},
					AccountID: &[]int64{4}[0],
				}
				userService.On("FindByAccountID", mock.Anything, int64(4)).Return(person, nil)

				// Mock staff repository
				userService.On("StaffRepository").Return(staffRepo)
				staff := &userModels.Staff{
					Model:    base.Model{ID: 30},
					PersonID: 20,
				}
				staffRepo.On("FindByPersonID", mock.Anything, int64(20)).Return(staff, nil)

				// Mock teacher repository
				userService.On("TeacherRepository").Return(teacherRepo)
				teacher := &userModels.Teacher{
					Model:   base.Model{ID: 40},
					StaffID: 30,
				}
				teacherRepo.On("FindByStaffID", mock.Anything, int64(30)).Return(teacher, nil)

				// Mock getting teacher's groups
				groups := []*education.Group{
					{
						Model: base.Model{ID: 1},
						Name:  "Class A",
					},
					{
						Model: base.Model{ID: 2},
						Name:  "Class B",
					},
				}
				eduService.On("GetTeacherGroups", mock.Anything, int64(40)).Return(groups, nil)

				// Mock student repository to get student's group
				userService.On("StudentRepository").Return(studentRepo)
				groupID := int64(2)
				student := &userModels.Student{
					Model:    base.Model{ID: 200},
					PersonID: 50,
					GroupID:  &groupID,
				}
				studentRepo.On("FindByID", mock.Anything, int64(200)).Return(student, nil)
			},
			expectedResult: true,
			expectError:    false,
		},
		{
			name: "teacher cannot access visit of student not in their group",
			authContext: &policy.Context{
				Subject: policy.Subject{
					AccountID:   5,
					Roles:       []string{"teacher"},
					Permissions: []string{},
				},
				Resource: policy.Resource{Type: "visit", ID: int64(777)},
				Action:   policy.ActionView,
			},
			setupMocks: func(eduService *MockEducationService, userService *MockUserService, activeService *MockActiveService, studentRepo *MockStudentRepository, staffRepo *MockStaffRepository, teacherRepo *MockTeacherRepository) {
				// Mock getting visit to find student ID
				visit := &active.Visit{
					Model: base.Model{
						ID: 777,
					},
					StudentID: 300,
				}
				activeService.On("GetVisit", mock.Anything, int64(777)).Return(visit, nil)

				// Mock finding person by account ID
				person := &userModels.Person{
					Model:     base.Model{ID: 60},
					AccountID: &[]int64{5}[0],
				}
				userService.On("FindByAccountID", mock.Anything, int64(5)).Return(person, nil)

				// Mock staff repository
				userService.On("StaffRepository").Return(staffRepo)
				staff := &userModels.Staff{
					Model:    base.Model{ID: 70},
					PersonID: 60,
				}
				staffRepo.On("FindByPersonID", mock.Anything, int64(60)).Return(staff, nil)

				// Mock teacher repository
				userService.On("TeacherRepository").Return(teacherRepo)
				teacher := &userModels.Teacher{
					Model:   base.Model{ID: 80},
					StaffID: 70,
				}
				teacherRepo.On("FindByStaffID", mock.Anything, int64(70)).Return(teacher, nil)

				// Mock getting teacher's groups
				groups := []*education.Group{
					{ID: 1, Name: "Class A"},
					{ID: 2, Name: "Class B"},
				}
				eduService.On("GetTeacherGroups", mock.Anything, int64(80)).Return(groups, nil)

				// Mock student repository to get student's group
				userService.On("StudentRepository").Return(studentRepo)
				groupID := int64(3) // Different group
				student := &userModels.Student{
					Model:    base.Model{ID: 300},
					PersonID: 90,
					GroupID:  &groupID,
				}
				studentRepo.On("FindByID", mock.Anything, int64(300)).Return(student, nil)
			},
			expectedResult: false,
			expectError:    false,
		},
		{
			name: "regular user without permissions cannot access",
			authContext: &policy.Context{
				Subject: policy.Subject{
					AccountID:   6,
					Roles:       []string{"user"},
					Permissions: []string{},
				},
				Resource: policy.Resource{Type: "visit", ID: int64(555)},
				Action:   policy.ActionView,
			},
			setupMocks: func(eduService *MockEducationService, userService *MockUserService, activeService *MockActiveService, studentRepo *MockStudentRepository, staffRepo *MockStaffRepository, teacherRepo *MockTeacherRepository) {
				// Mock getting visit to find student ID
				visit := &active.Visit{
					Model: base.Model{
						ID: 555,
					},
					StudentID: 400,
				}
				activeService.On("GetVisit", mock.Anything, int64(555)).Return(visit, nil)

				// Mock finding person by account ID
				person := &userModels.Person{
					Model:     base.Model{ID: 100},
					AccountID: &[]int64{6}[0],
				}
				userService.On("FindByAccountID", mock.Anything, int64(6)).Return(person, nil)

				// Mock student repository - user is not a student
				userService.On("StudentRepository").Return(studentRepo)
				studentRepo.On("FindByPersonID", mock.Anything, int64(100)).Return(nil, errors.New("not found"))

				// Mock staff repository - user is not staff
				userService.On("StaffRepository").Return(staffRepo)
				staffRepo.On("FindByPersonID", mock.Anything, int64(100)).Return(nil, errors.New("not found"))
			},
			expectedResult: false,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			eduService := new(MockEducationService)
			userService := new(MockUserService)
			activeService := new(MockActiveService)
			studentRepo := new(MockStudentRepository)
			staffRepo := new(MockStaffRepository)
			teacherRepo := new(MockTeacherRepository)

			// Setup mocks
			tt.setupMocks(eduService, userService, activeService, studentRepo, staffRepo, teacherRepo)

			// Create policy
			policy := policies.NewStudentVisitPolicy(eduService, userService, activeService)

			// Evaluate
			result, err := policy.Evaluate(context.Background(), tt.authContext)

			// Assert
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectedResult, result)

			// Assert mock expectations
			eduService.AssertExpectations(t)
			userService.AssertExpectations(t)
			activeService.AssertExpectations(t)
			studentRepo.AssertExpectations(t)
			staffRepo.AssertExpectations(t)
			teacherRepo.AssertExpectations(t)
		})
	}
}

func TestStudentVisitPolicy_Metadata(t *testing.T) {
	policy := policies.NewStudentVisitPolicy(nil, nil, nil)

	assert.Equal(t, "student_visit_access", policy.Name())
	assert.Equal(t, "visit", policy.ResourceType())
}
