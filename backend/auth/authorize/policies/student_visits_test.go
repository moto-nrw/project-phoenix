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
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/test/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestStudentVisitPolicy_Evaluate(t *testing.T) {
	tests := []struct {
		name           string
		authContext    *policy.Context
		setupMocks     func(*mocks.EducationServiceMock, *mocks.UserServiceMock, *mocks.ActiveServiceMock)
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
			setupMocks: func(e *mocks.EducationServiceMock, u *mocks.UserServiceMock, a *mocks.ActiveServiceMock) {
				// Admin bypasses all checks - no mock setup needed
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
			setupMocks: func(e *mocks.EducationServiceMock, u *mocks.UserServiceMock, a *mocks.ActiveServiceMock) {
				// Permission check bypasses relationship checks
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
			setupMocks: func(e *mocks.EducationServiceMock, u *mocks.UserServiceMock, a *mocks.ActiveServiceMock) {
				// Visit belongs to student ID 100
				a.On("GetVisit", mock.Anything, int64(789)).Return(&active.Visit{
					Model:     base.Model{ID: 789},
					StudentID: 100,
				}, nil)

				// Person lookup by account ID
				accountID := int64(3)
				u.On("FindByAccountID", mock.Anything, int64(3)).Return(&users.Person{
					Model:     base.Model{ID: 10},
					AccountID: &accountID,
				}, nil)

				// Student lookup - this person IS the student who owns the visit
				u.On("StudentRepository").Return(u.GetStudentMock())
				u.GetStudentMock().On("FindByPersonID", mock.Anything, int64(10)).Return(&users.Student{
					Model:    base.Model{ID: 100}, // Same as visit's student ID
					PersonID: 10,
				}, nil)

				// Staff lookup should fail (this is a student, not staff)
				u.On("StaffRepository").Return(u.GetStaffMock()).Maybe()
				u.GetStaffMock().On("FindByPersonID", mock.Anything, mock.Anything).Return(nil, errors.New("not found")).Maybe()
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
			setupMocks: func(e *mocks.EducationServiceMock, u *mocks.UserServiceMock, a *mocks.ActiveServiceMock) {
				// Visit belongs to student 200
				a.On("GetVisit", mock.Anything, int64(999)).Return(&active.Visit{
					Model:     base.Model{ID: 999},
					StudentID: 200,
				}, nil)

				// Person lookup
				accountID := int64(4)
				u.On("FindByAccountID", mock.Anything, int64(4)).Return(&users.Person{
					Model:     base.Model{ID: 20},
					AccountID: &accountID,
				}, nil)

				// Not a student
				u.On("StudentRepository").Return(u.GetStudentMock())
				u.GetStudentMock().On("FindByPersonID", mock.Anything, int64(20)).Return(nil, errors.New("not found"))

				// Is staff
				u.On("StaffRepository").Return(u.GetStaffMock())
				u.GetStaffMock().On("FindByPersonID", mock.Anything, int64(20)).Return(&users.Staff{
					Model:    base.Model{ID: 30},
					PersonID: 20,
				}, nil)

				// Is teacher
				u.On("TeacherRepository").Return(u.GetTeacherMock())
				u.GetTeacherMock().On("FindByStaffID", mock.Anything, int64(30)).Return(&users.Teacher{
					Model:   base.Model{ID: 40},
					StaffID: 30,
				}, nil)

				// Teacher supervises group 2
				e.On("GetTeacherGroups", mock.Anything, int64(40)).Return([]*education.Group{
					{Model: base.Model{ID: 2}, Name: "Class B"},
				}, nil)

				// Student is in group 2 (same as teacher's group)
				groupID := int64(2)
				u.GetStudentMock().On("FindByID", mock.Anything, int64(200)).Return(&users.Student{
					Model:    base.Model{ID: 200},
					PersonID: 50,
					GroupID:  &groupID,
				}, nil)
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
			setupMocks: func(e *mocks.EducationServiceMock, u *mocks.UserServiceMock, a *mocks.ActiveServiceMock) {
				// Visit belongs to student 300
				a.On("GetVisit", mock.Anything, int64(777)).Return(&active.Visit{
					Model:     base.Model{ID: 777},
					StudentID: 300,
				}, nil)

				// Person lookup
				accountID := int64(5)
				u.On("FindByAccountID", mock.Anything, int64(5)).Return(&users.Person{
					Model:     base.Model{ID: 60},
					AccountID: &accountID,
				}, nil)

				// Not a student
				u.On("StudentRepository").Return(u.GetStudentMock())
				u.GetStudentMock().On("FindByPersonID", mock.Anything, int64(60)).Return(nil, errors.New("not found"))

				// Is staff
				u.On("StaffRepository").Return(u.GetStaffMock())
				u.GetStaffMock().On("FindByPersonID", mock.Anything, int64(60)).Return(&users.Staff{
					Model:    base.Model{ID: 70},
					PersonID: 60,
				}, nil)

				// Is teacher
				u.On("TeacherRepository").Return(u.GetTeacherMock())
				u.GetTeacherMock().On("FindByStaffID", mock.Anything, int64(70)).Return(&users.Teacher{
					Model:   base.Model{ID: 80},
					StaffID: 70,
				}, nil)

				// Teacher supervises group 1
				e.On("GetTeacherGroups", mock.Anything, int64(80)).Return([]*education.Group{
					{Model: base.Model{ID: 1}, Name: "Class A"},
				}, nil)

				// Student is in group 3 (DIFFERENT from teacher's group)
				differentGroupID := int64(3)
				u.GetStudentMock().On("FindByID", mock.Anything, int64(300)).Return(&users.Student{
					Model:    base.Model{ID: 300},
					PersonID: 90,
					GroupID:  &differentGroupID,
				}, nil)

				// After group check fails, policy checks if staff supervises active group
				// Staff has no active supervisions
				a.On("FindSupervisorsByStaffID", mock.Anything, int64(70)).Return(nil, nil)
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
			setupMocks: func(e *mocks.EducationServiceMock, u *mocks.UserServiceMock, a *mocks.ActiveServiceMock) {
				// Visit lookup
				a.On("GetVisit", mock.Anything, int64(555)).Return(&active.Visit{
					Model:     base.Model{ID: 555},
					StudentID: 400,
				}, nil)

				// Person lookup
				accountID := int64(6)
				u.On("FindByAccountID", mock.Anything, int64(6)).Return(&users.Person{
					Model:     base.Model{ID: 100},
					AccountID: &accountID,
				}, nil)

				// Not a student
				u.On("StudentRepository").Return(u.GetStudentMock())
				u.GetStudentMock().On("FindByPersonID", mock.Anything, int64(100)).Return(nil, errors.New("not found"))

				// Not staff either
				u.On("StaffRepository").Return(u.GetStaffMock())
				u.GetStaffMock().On("FindByPersonID", mock.Anything, int64(100)).Return(nil, errors.New("not found"))

				u.On("TeacherRepository").Return(u.GetTeacherMock()).Maybe()
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
			setupMocks: func(e *mocks.EducationServiceMock, u *mocks.UserServiceMock, a *mocks.ActiveServiceMock) {
				// Visit belongs to student 500
				a.On("GetVisit", mock.Anything, int64(888)).Return(&active.Visit{
					Model:     base.Model{ID: 888},
					StudentID: 500,
				}, nil)

				// Person lookup
				accountID := int64(7)
				u.On("FindByAccountID", mock.Anything, int64(7)).Return(&users.Person{
					Model:     base.Model{ID: 110},
					AccountID: &accountID,
				}, nil)

				// This person is student 600 (NOT the owner of the visit)
				u.On("StudentRepository").Return(u.GetStudentMock())
				u.GetStudentMock().On("FindByPersonID", mock.Anything, int64(110)).Return(&users.Student{
					Model:    base.Model{ID: 600}, // Different from visit's student ID (500)
					PersonID: 110,
				}, nil)

				// Staff lookup fails
				u.On("StaffRepository").Return(u.GetStaffMock())
				u.GetStaffMock().On("FindByPersonID", mock.Anything, int64(110)).Return(nil, errors.New("not found"))
			},
			expectedResult: false,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks using shared mock package
			eduMock := mocks.NewEducationServiceMock()
			userMock := mocks.NewUserServiceMock()
			activeMock := mocks.NewActiveServiceMock()

			// Setup mock expectations
			tt.setupMocks(eduMock, userMock, activeMock)

			// Create policy with mocks
			policy := policies.NewStudentVisitPolicy(eduMock, userMock, activeMock)

			// Execute
			result, err := policy.Evaluate(context.Background(), tt.authContext)

			// Assert
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectedResult, result)

			// Verify mock expectations
			eduMock.AssertExpectations(t)
			userMock.AssertExpectations(t)
			activeMock.AssertExpectations(t)
			userMock.GetStudentMock().AssertExpectations(t)
			userMock.GetStaffMock().AssertExpectations(t)
			userMock.GetTeacherMock().AssertExpectations(t)
		})
	}
}

func TestStudentVisitPolicy_Metadata(t *testing.T) {
	policy := policies.NewStudentVisitPolicy(nil, nil, nil)

	assert.Equal(t, "student_visit_access", policy.Name())
	assert.Equal(t, "visit", policy.ResourceType())
}
