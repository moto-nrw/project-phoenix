package policies_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/authorize/policies"
	"github.com/moto-nrw/project-phoenix/auth/authorize/policy"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/uptrace/bun"
)

// SimpleMockEducationService provides the minimum implementation needed for tests
type SimpleMockEducationService struct {
	mock.Mock
}

func (m *SimpleMockEducationService) GetTeacherGroups(ctx context.Context, teacherID int64) ([]*education.Group, error) {
	args := m.Called(ctx, teacherID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*education.Group), args.Error(1)
}

func (m *SimpleMockEducationService) AssignRoomToGroup(ctx context.Context, groupID, roomID int64) error {
	// This method is required by the interface but not used in our tests
	return nil
}

func (m *SimpleMockEducationService) UpdateGroupTeachers(ctx context.Context, groupID int64, teacherIDs []int64) error {
	// This method is required by the interface but not used in our tests
	return nil
}

func (m *SimpleMockEducationService) WithTx(tx bun.Tx) interface{} {
	// Required by base.TransactionalService
	return m
}

// SimpleMockUserService provides the minimum implementation needed for tests
type SimpleMockUserService struct {
	mock.Mock
}

func (m *SimpleMockUserService) Get(ctx context.Context, id interface{}) (*userModels.Person, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*userModels.Person), args.Error(1)
}

func (m *SimpleMockUserService) Create(ctx context.Context, person *userModels.Person) error {
	args := m.Called(ctx, person)
	return args.Error(0)
}

func (m *SimpleMockUserService) Update(ctx context.Context, person *userModels.Person) error {
	args := m.Called(ctx, person)
	return args.Error(0)
}

func (m *SimpleMockUserService) Delete(ctx context.Context, id interface{}) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *SimpleMockUserService) List(ctx context.Context, options *base.QueryOptions) ([]*userModels.Person, error) {
	args := m.Called(ctx, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*userModels.Person), args.Error(1)
}

func (m *SimpleMockUserService) FindByTagID(ctx context.Context, tagID string) (*userModels.Person, error) {
	args := m.Called(ctx, tagID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*userModels.Person), args.Error(1)
}

func (m *SimpleMockUserService) FindByAccountID(ctx context.Context, accountID int64) (*userModels.Person, error) {
	args := m.Called(ctx, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*userModels.Person), args.Error(1)
}

func (m *SimpleMockUserService) FindByName(ctx context.Context, firstName, lastName string) ([]*userModels.Person, error) {
	args := m.Called(ctx, firstName, lastName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*userModels.Person), args.Error(1)
}

func (m *SimpleMockUserService) LinkToAccount(ctx context.Context, personID int64, accountID int64) error {
	args := m.Called(ctx, personID, accountID)
	return args.Error(0)
}

func (m *SimpleMockUserService) UnlinkFromAccount(ctx context.Context, personID int64) error {
	args := m.Called(ctx, personID)
	return args.Error(0)
}

func (m *SimpleMockUserService) LinkToRFIDCard(ctx context.Context, personID int64, tagID string) error {
	args := m.Called(ctx, personID, tagID)
	return args.Error(0)
}

func (m *SimpleMockUserService) UnlinkFromRFIDCard(ctx context.Context, personID int64) error {
	args := m.Called(ctx, personID)
	return args.Error(0)
}

func (m *SimpleMockUserService) GetFullProfile(ctx context.Context, personID int64) (*userModels.Person, error) {
	args := m.Called(ctx, personID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*userModels.Person), args.Error(1)
}

func (m *SimpleMockUserService) FindByGuardianID(ctx context.Context, guardianAccountID int64) ([]*userModels.Person, error) {
	args := m.Called(ctx, guardianAccountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*userModels.Person), args.Error(1)
}

func (m *SimpleMockUserService) StudentRepository() userModels.StudentRepository {
	args := m.Called()
	return args.Get(0).(userModels.StudentRepository)
}

func (m *SimpleMockUserService) StaffRepository() userModels.StaffRepository {
	args := m.Called()
	return args.Get(0).(userModels.StaffRepository)
}

func (m *SimpleMockUserService) TeacherRepository() userModels.TeacherRepository {
	args := m.Called()
	return args.Get(0).(userModels.TeacherRepository)
}

func (m *SimpleMockUserService) ListAvailableRFIDCards(ctx context.Context) ([]*userModels.RFIDCard, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*userModels.RFIDCard), args.Error(1)
}

func (m *SimpleMockUserService) WithTx(tx bun.Tx) interface{} {
	// Required by base.TransactionalService
	return m
}

// SimpleMockActiveService provides the minimum implementation needed for tests
type SimpleMockActiveService struct {
	mock.Mock
}

func (m *SimpleMockActiveService) GetVisit(ctx context.Context, visitID int64) (*active.Visit, error) {
	args := m.Called(ctx, visitID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*active.Visit), args.Error(1)
}

func (m *SimpleMockActiveService) WithTx(tx bun.Tx) interface{} {
	// Required by base.TransactionalService
	return m
}

// Simplified mock repositories
type SimpleMockStudentRepository struct {
	mock.Mock
}

func (m *SimpleMockStudentRepository) FindByID(ctx context.Context, id interface{}) (*userModels.Student, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*userModels.Student), args.Error(1)
}

func (m *SimpleMockStudentRepository) FindByPersonID(ctx context.Context, personID int64) (*userModels.Student, error) {
	args := m.Called(ctx, personID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*userModels.Student), args.Error(1)
}

// Add the remaining methods required by the StudentRepository interface
func (m *SimpleMockStudentRepository) Create(ctx context.Context, student *userModels.Student) error {
	return nil
}

func (m *SimpleMockStudentRepository) ListWithOptions(ctx context.Context, options *base.QueryOptions) ([]*userModels.Student, error) {
	return nil, nil
}

func (m *SimpleMockStudentRepository) Update(ctx context.Context, student *userModels.Student) error {
	return nil
}

func (m *SimpleMockStudentRepository) Delete(ctx context.Context, id interface{}) error {
	return nil
}

func (m *SimpleMockStudentRepository) List(ctx context.Context, filters map[string]interface{}) ([]*userModels.Student, error) {
	return nil, nil
}

func (m *SimpleMockStudentRepository) FindByGroupID(ctx context.Context, groupID int64) ([]*userModels.Student, error) {
	return nil, nil
}

func (m *SimpleMockStudentRepository) FindBySchoolClass(ctx context.Context, schoolClass string) ([]*userModels.Student, error) {
	return nil, nil
}

func (m *SimpleMockStudentRepository) UpdateLocation(ctx context.Context, id int64, location string) error {
	return nil
}

func (m *SimpleMockStudentRepository) AssignToGroup(ctx context.Context, studentID int64, groupID int64) error {
	return nil
}

func (m *SimpleMockStudentRepository) RemoveFromGroup(ctx context.Context, studentID int64) error {
	return nil
}

func (m *SimpleMockStudentRepository) FindByGroupIDs(ctx context.Context, groupIDs []int64) ([]*userModels.Student, error) {
	args := m.Called(ctx, groupIDs)
	return args.Get(0).([]*userModels.Student), args.Error(1)
}

type SimpleMockStaffRepository struct {
	mock.Mock
}

func (m *SimpleMockStaffRepository) FindByPersonID(ctx context.Context, personID int64) (*userModels.Staff, error) {
	args := m.Called(ctx, personID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*userModels.Staff), args.Error(1)
}

// Add the remaining methods required by the StaffRepository interface
func (m *SimpleMockStaffRepository) Create(ctx context.Context, staff *userModels.Staff) error {
	return nil
}

func (m *SimpleMockStaffRepository) FindByID(ctx context.Context, id interface{}) (*userModels.Staff, error) {
	return nil, nil
}

func (m *SimpleMockStaffRepository) Update(ctx context.Context, staff *userModels.Staff) error {
	return nil
}

func (m *SimpleMockStaffRepository) Delete(ctx context.Context, id interface{}) error {
	return nil
}

func (m *SimpleMockStaffRepository) List(ctx context.Context, filters map[string]interface{}) ([]*userModels.Staff, error) {
	return nil, nil
}

func (m *SimpleMockStaffRepository) UpdateNotes(ctx context.Context, id int64, notes string) error {
	return nil
}

type SimpleMockTeacherRepository struct {
	mock.Mock
}

func (m *SimpleMockTeacherRepository) FindByStaffID(ctx context.Context, staffID int64) (*userModels.Teacher, error) {
	args := m.Called(ctx, staffID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*userModels.Teacher), args.Error(1)
}

// Add missing method: ListWithOptions
func (m *SimpleMockTeacherRepository) ListWithOptions(ctx context.Context, options *base.QueryOptions) ([]*userModels.Teacher, error) {
	return nil, nil
}

// Add the remaining methods required by the TeacherRepository interface
func (m *SimpleMockTeacherRepository) Create(ctx context.Context, teacher *userModels.Teacher) error {
	return nil
}

func (m *SimpleMockTeacherRepository) FindByID(ctx context.Context, id interface{}) (*userModels.Teacher, error) {
	return nil, nil
}

func (m *SimpleMockTeacherRepository) FindBySpecialization(ctx context.Context, specialization string) ([]*userModels.Teacher, error) {
	return nil, nil
}

func (m *SimpleMockTeacherRepository) Update(ctx context.Context, teacher *userModels.Teacher) error {
	return nil
}

func (m *SimpleMockTeacherRepository) Delete(ctx context.Context, id interface{}) error {
	return nil
}

func (m *SimpleMockTeacherRepository) List(ctx context.Context, filters map[string]interface{}) ([]*userModels.Teacher, error) {
	return nil, nil
}

func (m *SimpleMockTeacherRepository) FindByGroupID(ctx context.Context, groupID int64) ([]*userModels.Teacher, error) {
	return nil, nil
}

func (m *SimpleMockTeacherRepository) UpdateQualifications(ctx context.Context, id int64, qualifications string) error {
	return nil
}

func (m *SimpleMockTeacherRepository) FindWithStaffAndPerson(ctx context.Context, id int64) (*userModels.Teacher, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*userModels.Teacher), args.Error(1)
}

func TestStudentVisitPolicy_Evaluate(t *testing.T) {
	tests := []struct {
		name           string
		authContext    *policy.Context
		setupMocks     func(*SimpleMockEducationService, *SimpleMockUserService, *SimpleMockActiveService, *SimpleMockStudentRepository, *SimpleMockStaffRepository, *SimpleMockTeacherRepository)
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
			setupMocks: func(e *SimpleMockEducationService, u *SimpleMockUserService, a *SimpleMockActiveService, s *SimpleMockStudentRepository, st *SimpleMockStaffRepository, t *SimpleMockTeacherRepository) {
				// No need to set up any expectations here - admin should pass without any service calls
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
			setupMocks: func(e *SimpleMockEducationService, u *SimpleMockUserService, a *SimpleMockActiveService, s *SimpleMockStudentRepository, st *SimpleMockStaffRepository, t *SimpleMockTeacherRepository) {
				// No need to set up any expectations here - user with permission should pass without any service calls
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
			setupMocks: func(eduService *SimpleMockEducationService, userService *SimpleMockUserService, activeService *SimpleMockActiveService, studentRepo *SimpleMockStudentRepository, staffRepo *SimpleMockStaffRepository, teacherRepo *SimpleMockTeacherRepository) {
				// Simple visit mock with student ID
				visit := &active.Visit{
					Model:     base.Model{ID: 789},
					StudentID: 100,
				}
				activeService.On("GetVisit", mock.Anything, int64(789)).Return(visit, nil)

				// Mock person lookup by account ID
				person := &userModels.Person{
					Model:     base.Model{ID: 10},
					AccountID: &[]int64{3}[0],
				}
				userService.On("FindByAccountID", mock.Anything, int64(3)).Return(person, nil)

				// Student repository lookup
				userService.On("StudentRepository").Return(studentRepo)
				student := &userModels.Student{
					Model:    base.Model{ID: 100},
					PersonID: 10,
				}
				studentRepo.On("FindByPersonID", mock.Anything, int64(10)).Return(student, nil)

				// Set up staff repo just in case it's accessed
				userService.On("StaffRepository").Return(staffRepo).Maybe()
				staffRepo.On("FindByPersonID", mock.Anything, mock.Anything).Return(nil, errors.New("not found")).Maybe()
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
			setupMocks: func(eduService *SimpleMockEducationService, userService *SimpleMockUserService, activeService *SimpleMockActiveService, studentRepo *SimpleMockStudentRepository, staffRepo *SimpleMockStaffRepository, teacherRepo *SimpleMockTeacherRepository) {
				// Visit mock with student ID
				visit := &active.Visit{
					Model:     base.Model{ID: 999},
					StudentID: 200,
				}
				activeService.On("GetVisit", mock.Anything, int64(999)).Return(visit, nil)

				// Person lookup
				person := &userModels.Person{
					Model:     base.Model{ID: 20},
					AccountID: &[]int64{4}[0],
				}
				userService.On("FindByAccountID", mock.Anything, int64(4)).Return(person, nil)

				// Student lookup should fail - this is a teacher not a student
				userService.On("StudentRepository").Return(studentRepo)
				studentRepo.On("FindByPersonID", mock.Anything, int64(20)).Return(nil, errors.New("not found"))

				// Staff lookup
				userService.On("StaffRepository").Return(staffRepo)
				staff := &userModels.Staff{
					Model:    base.Model{ID: 30},
					PersonID: 20,
				}
				staffRepo.On("FindByPersonID", mock.Anything, int64(20)).Return(staff, nil)

				// Teacher lookup
				userService.On("TeacherRepository").Return(teacherRepo)
				teacher := &userModels.Teacher{
					Model:   base.Model{ID: 40},
					StaffID: 30,
				}
				teacherRepo.On("FindByStaffID", mock.Anything, int64(30)).Return(teacher, nil)

				// Teacher's groups
				groups := []*education.Group{
					{
						Model: base.Model{ID: 2},
						Name:  "Class B",
					},
				}
				eduService.On("GetTeacherGroups", mock.Anything, int64(40)).Return(groups, nil)

				// Get student's group
				groupID := int64(2) // Same as teacher's group
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
			setupMocks: func(eduService *SimpleMockEducationService, userService *SimpleMockUserService, activeService *SimpleMockActiveService, studentRepo *SimpleMockStudentRepository, staffRepo *SimpleMockStaffRepository, teacherRepo *SimpleMockTeacherRepository) {
				// Visit mock with student ID
				visit := &active.Visit{
					Model:     base.Model{ID: 777},
					StudentID: 300,
				}
				activeService.On("GetVisit", mock.Anything, int64(777)).Return(visit, nil)

				// Person lookup
				person := &userModels.Person{
					Model:     base.Model{ID: 60},
					AccountID: &[]int64{5}[0],
				}
				userService.On("FindByAccountID", mock.Anything, int64(5)).Return(person, nil)

				// Student lookup should fail - this is a teacher not a student
				userService.On("StudentRepository").Return(studentRepo)
				studentRepo.On("FindByPersonID", mock.Anything, int64(60)).Return(nil, errors.New("not found"))

				// Staff lookup
				userService.On("StaffRepository").Return(staffRepo)
				staff := &userModels.Staff{
					Model:    base.Model{ID: 70},
					PersonID: 60,
				}
				staffRepo.On("FindByPersonID", mock.Anything, int64(60)).Return(staff, nil)

				// Teacher lookup
				userService.On("TeacherRepository").Return(teacherRepo)
				teacher := &userModels.Teacher{
					Model:   base.Model{ID: 80},
					StaffID: 70,
				}
				teacherRepo.On("FindByStaffID", mock.Anything, int64(70)).Return(teacher, nil)

				// Teacher's groups
				groups := []*education.Group{
					{
						Model: base.Model{ID: 1},
						Name:  "Class A",
					},
				}
				eduService.On("GetTeacherGroups", mock.Anything, int64(80)).Return(groups, nil)

				// Student is in a different group
				differentGroupID := int64(3) // Different from teacher's group
				student := &userModels.Student{
					Model:    base.Model{ID: 300},
					PersonID: 90,
					GroupID:  &differentGroupID,
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
			setupMocks: func(eduService *SimpleMockEducationService, userService *SimpleMockUserService, activeService *SimpleMockActiveService, studentRepo *SimpleMockStudentRepository, staffRepo *SimpleMockStaffRepository, teacherRepo *SimpleMockTeacherRepository) {
				// Visit mock with student ID
				visit := &active.Visit{
					Model:     base.Model{ID: 555},
					StudentID: 400,
				}
				activeService.On("GetVisit", mock.Anything, int64(555)).Return(visit, nil)

				// Person lookup
				person := &userModels.Person{
					Model:     base.Model{ID: 100},
					AccountID: &[]int64{6}[0],
				}
				userService.On("FindByAccountID", mock.Anything, int64(6)).Return(person, nil)

				// User is not a student
				userService.On("StudentRepository").Return(studentRepo)
				studentRepo.On("FindByPersonID", mock.Anything, int64(100)).Return(nil, errors.New("not found"))

				// User is not staff
				userService.On("StaffRepository").Return(staffRepo)
				staffRepo.On("FindByPersonID", mock.Anything, int64(100)).Return(nil, errors.New("not found"))

				// Teacher repo should not be called, but add it to be safe
				userService.On("TeacherRepository").Return(teacherRepo).Maybe()
			},
			expectedResult: false,
			expectError:    false,
		},
		{
			name: "student cannot access another student's visit",
			authContext: &policy.Context{
				Subject: policy.Subject{
					AccountID:   7,
					Roles:       []string{"student"},
					Permissions: []string{},
				},
				Resource: policy.Resource{Type: "visit", ID: int64(888)},
				Action:   policy.ActionView,
			},
			setupMocks: func(eduService *SimpleMockEducationService, userService *SimpleMockUserService, activeService *SimpleMockActiveService, studentRepo *SimpleMockStudentRepository, staffRepo *SimpleMockStaffRepository, teacherRepo *SimpleMockTeacherRepository) {
				// Visit of a different student
				visit := &active.Visit{
					Model:     base.Model{ID: 888},
					StudentID: 500, // Not the requesting student's ID
				}
				activeService.On("GetVisit", mock.Anything, int64(888)).Return(visit, nil)

				// Person lookup
				person := &userModels.Person{
					Model:     base.Model{ID: 110},
					AccountID: &[]int64{7}[0],
				}
				userService.On("FindByAccountID", mock.Anything, int64(7)).Return(person, nil)

				// The person is a student, but not the one who owns the visit
				userService.On("StudentRepository").Return(studentRepo)
				student := &userModels.Student{
					Model:    base.Model{ID: 600}, // Different from visit's student ID
					PersonID: 110,
				}
				studentRepo.On("FindByPersonID", mock.Anything, int64(110)).Return(student, nil)

				// Staff lookup should fail
				userService.On("StaffRepository").Return(staffRepo)
				staffRepo.On("FindByPersonID", mock.Anything, int64(110)).Return(nil, errors.New("not found"))
			},
			expectedResult: false,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			eduService := new(SimpleMockEducationService)
			userService := new(SimpleMockUserService)
			activeService := new(SimpleMockActiveService)
			studentRepo := new(SimpleMockStudentRepository)
			staffRepo := new(SimpleMockStaffRepository)
			teacherRepo := new(SimpleMockTeacherRepository)

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

// Add missing method implementations for the service interfaces to satisfy the compiler
// These are not used in our tests but are required by the interface

// For SimpleMockEducationService
func (m *SimpleMockEducationService) AddTeacherToGroup(ctx context.Context, groupID, teacherID int64) error {
	return nil
}

func (m *SimpleMockEducationService) RemoveTeacherFromGroup(ctx context.Context, groupID, teacherID int64) error {
	return nil
}

func (m *SimpleMockEducationService) CreateGroup(ctx context.Context, group *education.Group) error {
	return nil
}

func (m *SimpleMockEducationService) UpdateGroup(ctx context.Context, group *education.Group) error {
	return nil
}

func (m *SimpleMockEducationService) DeleteGroup(ctx context.Context, id int64) error {
	return nil
}

func (m *SimpleMockEducationService) GetGroup(ctx context.Context, id int64) (*education.Group, error) {
	return nil, nil
}

func (m *SimpleMockEducationService) ListGroups(ctx context.Context, options *base.QueryOptions) ([]*education.Group, error) {
	return nil, nil
}

func (m *SimpleMockEducationService) AssignSubstituteTeacher(ctx context.Context, groupID, substituteTeacherID int64, startDate, endDate time.Time) error {
	return nil
}

func (m *SimpleMockEducationService) GetActiveSubstitutions(ctx context.Context, date time.Time) ([]*education.GroupSubstitution, error) {
	return nil, nil
}

func (m *SimpleMockEducationService) RemoveRoomFromGroup(ctx context.Context, groupID int64) error {
	return nil
}

func (m *SimpleMockEducationService) FindGroupByName(ctx context.Context, name string) (*education.Group, error) {
	return nil, nil
}

func (m *SimpleMockEducationService) FindGroupsByRoom(ctx context.Context, roomID int64) ([]*education.Group, error) {
	return nil, nil
}

func (m *SimpleMockEducationService) FindGroupWithRoom(ctx context.Context, groupID int64) (*education.Group, error) {
	return nil, nil
}

func (m *SimpleMockEducationService) GetGroupTeachers(ctx context.Context, groupID int64) ([]*userModels.Teacher, error) {
	return nil, nil
}

func (m *SimpleMockEducationService) CreateSubstitution(ctx context.Context, substitution *education.GroupSubstitution) error {
	return nil
}

func (m *SimpleMockEducationService) UpdateSubstitution(ctx context.Context, substitution *education.GroupSubstitution) error {
	return nil
}

func (m *SimpleMockEducationService) DeleteSubstitution(ctx context.Context, id int64) error {
	return nil
}

func (m *SimpleMockEducationService) GetSubstitution(ctx context.Context, id int64) (*education.GroupSubstitution, error) {
	return nil, nil
}

func (m *SimpleMockEducationService) ListSubstitutions(ctx context.Context, options *base.QueryOptions) ([]*education.GroupSubstitution, error) {
	return nil, nil
}

func (m *SimpleMockEducationService) GetActiveGroupSubstitutions(ctx context.Context, groupID int64, date time.Time) ([]*education.GroupSubstitution, error) {
	return nil, nil
}

func (m *SimpleMockEducationService) GetStaffSubstitutions(ctx context.Context, staffID int64, asRegular bool) ([]*education.GroupSubstitution, error) {
	return nil, nil
}

func (m *SimpleMockEducationService) CheckSubstitutionConflicts(ctx context.Context, staffID int64, startDate, endDate time.Time) ([]*education.GroupSubstitution, error) {
	return nil, nil
}

// For SimpleMockActiveService
func (m *SimpleMockActiveService) CreateActiveGroup(ctx context.Context, group *active.Group) error {
	return nil
}

func (m *SimpleMockActiveService) UpdateActiveGroup(ctx context.Context, group *active.Group) error {
	return nil
}

func (m *SimpleMockActiveService) DeleteActiveGroup(ctx context.Context, id int64) error {
	return nil
}

func (m *SimpleMockActiveService) GetActiveGroup(ctx context.Context, id int64) (*active.Group, error) {
	return nil, nil
}

func (m *SimpleMockActiveService) ListActiveGroups(ctx context.Context, options *base.QueryOptions) ([]*active.Group, error) {
	return nil, nil
}

func (m *SimpleMockActiveService) CreateVisit(ctx context.Context, visit *active.Visit) error {
	return nil
}

func (m *SimpleMockActiveService) UpdateVisit(ctx context.Context, visit *active.Visit) error {
	return nil
}

func (m *SimpleMockActiveService) DeleteVisit(ctx context.Context, id int64) error {
	return nil
}

func (m *SimpleMockActiveService) ListVisits(ctx context.Context, options *base.QueryOptions) ([]*active.Visit, error) {
	return nil, nil
}

func (m *SimpleMockActiveService) FindActiveGroupsByRoomID(ctx context.Context, roomID int64) ([]*active.Group, error) {
	return nil, nil
}

func (m *SimpleMockActiveService) FindActiveGroupsByGroupID(ctx context.Context, groupID int64) ([]*active.Group, error) {
	return nil, nil
}

func (m *SimpleMockActiveService) FindActiveGroupsByTimeRange(ctx context.Context, start, end time.Time) ([]*active.Group, error) {
	return nil, nil
}

func (m *SimpleMockActiveService) EndActiveGroupSession(ctx context.Context, id int64) error {
	return nil
}

func (m *SimpleMockActiveService) GetActiveGroupWithVisits(ctx context.Context, id int64) (*active.Group, error) {
	return nil, nil
}

func (m *SimpleMockActiveService) GetActiveGroupWithSupervisors(ctx context.Context, id int64) (*active.Group, error) {
	return nil, nil
}

func (m *SimpleMockActiveService) FindVisitsByStudentID(ctx context.Context, studentID int64) ([]*active.Visit, error) {
	return nil, nil
}

func (m *SimpleMockActiveService) FindVisitsByActiveGroupID(ctx context.Context, activeGroupID int64) ([]*active.Visit, error) {
	return nil, nil
}

func (m *SimpleMockActiveService) FindVisitsByTimeRange(ctx context.Context, start, end time.Time) ([]*active.Visit, error) {
	return nil, nil
}

func (m *SimpleMockActiveService) EndVisit(ctx context.Context, id int64) error {
	return nil
}

func (m *SimpleMockActiveService) GetStudentCurrentVisit(ctx context.Context, studentID int64) (*active.Visit, error) {
	return nil, nil
}

func (m *SimpleMockActiveService) GetGroupSupervisor(ctx context.Context, id int64) (*active.GroupSupervisor, error) {
	return nil, nil
}

func (m *SimpleMockActiveService) CreateGroupSupervisor(ctx context.Context, supervisor *active.GroupSupervisor) error {
	return nil
}

func (m *SimpleMockActiveService) UpdateGroupSupervisor(ctx context.Context, supervisor *active.GroupSupervisor) error {
	return nil
}

func (m *SimpleMockActiveService) DeleteGroupSupervisor(ctx context.Context, id int64) error {
	return nil
}

func (m *SimpleMockActiveService) ListGroupSupervisors(ctx context.Context, options *base.QueryOptions) ([]*active.GroupSupervisor, error) {
	return nil, nil
}

func (m *SimpleMockActiveService) FindSupervisorsByStaffID(ctx context.Context, staffID int64) ([]*active.GroupSupervisor, error) {
	return nil, nil
}

func (m *SimpleMockActiveService) FindSupervisorsByActiveGroupID(ctx context.Context, activeGroupID int64) ([]*active.GroupSupervisor, error) {
	return nil, nil
}

func (m *SimpleMockActiveService) EndSupervision(ctx context.Context, id int64) error {
	return nil
}

func (m *SimpleMockActiveService) GetStaffActiveSupervisions(ctx context.Context, staffID int64) ([]*active.GroupSupervisor, error) {
	return nil, nil
}

func (m *SimpleMockActiveService) GetCombinedGroup(ctx context.Context, id int64) (*active.CombinedGroup, error) {
	return nil, nil
}

func (m *SimpleMockActiveService) CreateCombinedGroup(ctx context.Context, group *active.CombinedGroup) error {
	return nil
}

func (m *SimpleMockActiveService) UpdateCombinedGroup(ctx context.Context, group *active.CombinedGroup) error {
	return nil
}

func (m *SimpleMockActiveService) DeleteCombinedGroup(ctx context.Context, id int64) error {
	return nil
}

func (m *SimpleMockActiveService) ListCombinedGroups(ctx context.Context, options *base.QueryOptions) ([]*active.CombinedGroup, error) {
	return nil, nil
}

func (m *SimpleMockActiveService) FindActiveCombinedGroups(ctx context.Context) ([]*active.CombinedGroup, error) {
	return nil, nil
}

func (m *SimpleMockActiveService) FindCombinedGroupsByTimeRange(ctx context.Context, start, end time.Time) ([]*active.CombinedGroup, error) {
	return nil, nil
}

func (m *SimpleMockActiveService) EndCombinedGroup(ctx context.Context, id int64) error {
	return nil
}

func (m *SimpleMockActiveService) GetCombinedGroupWithGroups(ctx context.Context, id int64) (*active.CombinedGroup, error) {
	return nil, nil
}

func (m *SimpleMockActiveService) AddGroupToCombination(ctx context.Context, combinedGroupID, activeGroupID int64) error {
	return nil
}

func (m *SimpleMockActiveService) RemoveGroupFromCombination(ctx context.Context, combinedGroupID, activeGroupID int64) error {
	return nil
}

func (m *SimpleMockActiveService) GetGroupMappingsByActiveGroupID(ctx context.Context, activeGroupID int64) ([]*active.GroupMapping, error) {
	return nil, nil
}

func (m *SimpleMockActiveService) GetGroupMappingsByCombinedGroupID(ctx context.Context, combinedGroupID int64) ([]*active.GroupMapping, error) {
	return nil, nil
}

func (m *SimpleMockActiveService) GetActiveGroupsCount(ctx context.Context) (int, error) {
	return 0, nil
}

func (m *SimpleMockActiveService) GetTotalVisitsCount(ctx context.Context) (int, error) {
	return 0, nil
}

func (m *SimpleMockActiveService) GetActiveVisitsCount(ctx context.Context) (int, error) {
	return 0, nil
}

func (m *SimpleMockActiveService) GetRoomUtilization(ctx context.Context, roomID int64) (float64, error) {
	return 0, nil
}

func (m *SimpleMockActiveService) GetStudentAttendanceRate(ctx context.Context, studentID int64) (float64, error) {
	return 0, nil
}
