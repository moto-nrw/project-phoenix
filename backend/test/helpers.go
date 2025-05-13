package test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/require"
)

// TestData holds test data for authorization tests
type TestData struct {
	// Users
	AdminUser   *userModels.Person
	TeacherUser *userModels.Person
	StudentUser *userModels.Person
	RegularUser *userModels.Person

	// Students
	Student1 *userModels.Student
	Student2 *userModels.Student

	// Staff
	TeacherStaff *userModels.Staff

	// Teachers
	Teacher1 *userModels.Teacher

	// Groups
	Group1 *education.Group
	Group2 *education.Group

	// Active Groups
	ActiveGroup1 *active.Group
	ActiveGroup2 *active.Group

	// Visits
	Visit1 *active.Visit
	Visit2 *active.Visit

	// JWT Tokens
	AdminToken   string
	TeacherToken string
	StudentToken string
	UserToken    string
}

// CreateTestData creates a comprehensive test data set
func CreateTestData(t *testing.T) *TestData {
	data := &TestData{}

	// Create JWT auth service
	tokenAuth, err := jwt.NewTokenAuth()
	require.NoError(t, err)

	// Create admin user
	adminID := int64(1)
	data.AdminUser = &userModels.Person{
		Model: base.Model{
			ID: 1,
		},
		FirstName: "Admin",
		LastName:  "User",
		AccountID: &adminID,
	}

	// Create teacher user
	teacherID := int64(2)
	data.TeacherUser = &userModels.Person{
		Model: base.Model{
			ID: 2,
		},
		FirstName: "Teacher",
		LastName:  "User",
		AccountID: &teacherID,
	}

	// Create student user
	studentID := int64(3)
	data.StudentUser = &userModels.Person{
		Model: base.Model{
			ID: 3,
		},
		FirstName: "Student",
		LastName:  "User",
		AccountID: &studentID,
	}

	// Create regular user
	userID := int64(4)
	data.RegularUser = &userModels.Person{
		Model: base.Model{
			ID: 4,
		},
		FirstName: "Regular",
		LastName:  "User",
		AccountID: &userID,
	}

	// Create groups
	data.Group1 = &education.Group{
		Model: base.Model{
			ID: 1,
		},
		Name: "Class A",
	}

	data.Group2 = &education.Group{
		Model: base.Model{
			ID: 2,
		},
		Name: "Class B",
	}

	// Create teacher staff
	data.TeacherStaff = &userModels.Staff{
		Model: base.Model{
			ID: 1,
		},
		PersonID: data.TeacherUser.ID,
		Person:   data.TeacherUser,
	}

	// Create teacher
	data.Teacher1 = &userModels.Teacher{
		Model: base.Model{
			ID: 1,
		},
		StaffID: data.TeacherStaff.ID,
		Staff:   data.TeacherStaff,
	}

	// Create students
	groupID1 := data.Group1.ID
	data.Student1 = &userModels.Student{
		Model: base.Model{
			ID: 1,
		},
		PersonID: data.StudentUser.ID,
		Person:   data.StudentUser,
		GroupID:  &groupID1,
	}

	groupID2 := data.Group2.ID
	data.Student2 = &userModels.Student{
		Model: base.Model{
			ID: 2,
		},
		PersonID: 5, // Different person
		GroupID:  &groupID2,
	}

	// Create active groups
	data.ActiveGroup1 = &active.Group{
		Model: base.Model{
			ID: 1,
		},
		GroupID:   data.Group1.ID,
		RoomID:    1,
		StartTime: time.Now().Add(-1 * time.Hour),
	}

	data.ActiveGroup2 = &active.Group{
		Model: base.Model{
			ID: 2,
		},
		GroupID:   data.Group2.ID,
		RoomID:    2,
		StartTime: time.Now().Add(-1 * time.Hour),
	}

	// Create visits
	data.Visit1 = &active.Visit{
		Model: base.Model{
			ID: 1,
		},
		StudentID:     data.Student1.ID,
		ActiveGroupID: data.ActiveGroup1.ID,
		EntryTime:     time.Now().Add(-30 * time.Minute),
		Student:       data.Student1,
		ActiveGroup:   data.ActiveGroup1,
	}

	data.Visit2 = &active.Visit{
		Model: base.Model{
			ID: 2,
		},
		StudentID:     data.Student2.ID,
		ActiveGroupID: data.ActiveGroup2.ID,
		EntryTime:     time.Now().Add(-30 * time.Minute),
		Student:       data.Student2,
		ActiveGroup:   data.ActiveGroup2,
	}

	// Create JWT tokens
	adminClaims := jwt.AppClaims{
		ID:          1,
		Username:    "admin",
		Roles:       []string{"admin"},
		Permissions: []string{"admin:*"},
	}
	data.AdminToken, err = tokenAuth.CreateJWT(adminClaims)
	require.NoError(t, err)

	teacherClaims := jwt.AppClaims{
		ID:          2,
		Username:    "teacher",
		Roles:       []string{"teacher"},
		Permissions: []string{"groups:read", "visits:read"},
	}
	data.TeacherToken, err = tokenAuth.CreateJWT(teacherClaims)
	require.NoError(t, err)

	studentClaims := jwt.AppClaims{
		ID:          3,
		Username:    "student",
		Roles:       []string{"student"},
		Permissions: []string{},
	}
	data.StudentToken, err = tokenAuth.CreateJWT(studentClaims)
	require.NoError(t, err)

	userClaims := jwt.AppClaims{
		ID:          4,
		Username:    "user",
		Roles:       []string{"user"},
		Permissions: []string{},
	}
	data.UserToken, err = tokenAuth.CreateJWT(userClaims)
	require.NoError(t, err)

	return data
}

// CreateTestAuthorizationService creates a test authorization service with registered policies
func CreateTestAuthorizationService(t *testing.T) authorize.AuthorizationService {
	authService := authorize.NewAuthorizationService()

	// Register test policies here as needed
	// For example:
	// testPolicy := NewTestPolicy()
	// err := authService.RegisterPolicy(testPolicy)
	// require.NoError(t, err)

	return authService
}

// MockJWTContext adds JWT claims and permissions to a context
func MockJWTContext(ctx context.Context, claims jwt.AppClaims, permissions []string) context.Context {
	ctx = context.WithValue(ctx, ctxKeyClaims, claims)
	ctx = context.WithValue(ctx, ctxKeyPermissions, permissions)
	return ctx
}

// Context keys for testing
type ctxKey int

const (
	ctxKeyClaims ctxKey = iota
	ctxKeyPermissions
)

// AssertPermissionDenied asserts that a permission error occurred
func AssertPermissionDenied(t *testing.T, err error) {
	require.Error(t, err)
	require.Contains(t, err.Error(), "forbidden")
}

// AssertPermissionGranted asserts that no permission error occurred
func AssertPermissionGranted(t *testing.T, err error) {
	require.NoError(t, err)
}

// CreateTestVisitForStudent creates a test visit for a student
func CreateTestVisitForStudent(studentID int64, activeGroupID int64) *active.Visit {
	return &active.Visit{
		Model: base.Model{
			ID: 100 + studentID,
		},
		StudentID:     studentID,
		ActiveGroupID: activeGroupID,
		EntryTime:     time.Now().Add(-30 * time.Minute),
	}
}

// CreateTestActiveGroup creates a test active group
func CreateTestActiveGroup(groupID int64, roomID int64) *active.Group {
	return &active.Group{
		Model: base.Model{
			ID: 100 + groupID,
		},
		GroupID:   groupID,
		RoomID:    roomID,
		StartTime: time.Now().Add(-1 * time.Hour),
	}
}

// CreateTestStudent creates a test student
func CreateTestStudent(personID int64, groupID *int64) *userModels.Student {
	return &userModels.Student{
		Model: base.Model{
			ID: 100 + personID,
		},
		PersonID: personID,
		GroupID:  groupID,
	}
}

// CreateTestPerson creates a test person
func CreateTestPerson(accountID *int64, firstName, lastName string) *userModels.Person {
	id := int64(100)
	if accountID != nil {
		id = *accountID
	}
	return &userModels.Person{
		Model: base.Model{
			ID: id,
		},
		FirstName: firstName,
		LastName:  lastName,
		AccountID: accountID,
	}
}

// CreateTestClaims creates test JWT claims
func CreateTestClaims(id int64, username string, roles []string, permissions []string) jwt.AppClaims {
	return jwt.AppClaims{
		ID:          int(id),
		Username:    username,
		Roles:       roles,
		Permissions: permissions,
		CommonClaims: jwt.CommonClaims{
			IssuedAt:  time.Now().Unix(),
			ExpiresAt: time.Now().Add(1 * time.Hour).Unix(),
		},
	}
}

// TestPermissionScenario tests a permission scenario
type TestPermissionScenario struct {
	Name            string
	Permission      string
	UserPermissions []string
	ExpectedResult  bool
}

// RunPermissionScenarios runs a series of permission test scenarios
func RunPermissionScenarios(t *testing.T, scenarios []TestPermissionScenario) {
	for _, scenario := range scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			// This is a simplified example - real implementation would
			// test against actual permission checking logic
			hasPermission := false
			for _, perm := range scenario.UserPermissions {
				if perm == scenario.Permission || perm == "admin:*" || perm == "*:*" {
					hasPermission = true
					break
				}
			}

			require.Equal(t, scenario.ExpectedResult, hasPermission)
		})
	}
}
