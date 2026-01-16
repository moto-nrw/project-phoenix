package policies_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/authorize/policies"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/authorize/policy"
	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres"
	"github.com/moto-nrw/project-phoenix/internal/core/service/active"
	"github.com/moto-nrw/project-phoenix/internal/core/service/education"
	"github.com/moto-nrw/project-phoenix/internal/core/service/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// setupPolicyServices creates real services for policy testing.
// This replaces the mock-based approach with actual service implementations.
func setupPolicyServices(t *testing.T, db *bun.DB) (education.Service, users.PersonService, active.Service) {
	t.Helper()

	repos := repositories.NewFactory(db)

	// Create education service
	eduService := education.NewService(
		repos.Group,
		repos.GroupTeacher,
		repos.GroupSubstitution,
		repos.GroupSubstitutionRelations,
		repos.Room,
		repos.Teacher,
		repos.Staff,
		db,
	)

	// Create users service
	usersService := users.NewPersonService(users.PersonServiceDependencies{
		PersonRepo:         repos.Person,
		RFIDRepo:           repos.RFIDCard,
		AccountRepo:        repos.Account,
		PersonGuardianRepo: repos.PersonGuardian,
		StudentRepo:        repos.Student,
		StaffRepo:          repos.Staff,
		TeacherRepo:        repos.Teacher,
		DB:                 db,
	})

	// Create active service (without broadcaster for tests)
	activeService := active.NewService(active.ServiceDependencies{
		GroupReadRepo:      repos.ActiveGroup,
		GroupWriteRepo:     repos.ActiveGroup,
		GroupRelationsRepo: repos.ActiveGroup,
		VisitRepo:          repos.ActiveVisit,
		SupervisorRepo:     repos.GroupSupervisor,
		CombinedGroupRepo:  repos.CombinedGroup,
		GroupMappingRepo:   repos.GroupMapping,
		AttendanceRepo:     repos.Attendance,
		StudentRepo:        repos.Student,
		PersonRepo:         repos.Person,
		TeacherRepo:        repos.Teacher,
		StaffRepo:          repos.Staff,
		RoomRepo:           repos.Room,
		ActivityGroupRepo:  repos.ActivityGroup,
		ActivityCatRepo:    repos.ActivityCategory,
		EducationGroupRepo: repos.Group,
		EducationService:   eduService,
		UsersService:       usersService,
		DB:                 db,
		Broadcaster:        nil, // No SSE for tests
	})

	return eduService, usersService, activeService
}

func TestStudentVisitPolicy_AdminCanAlwaysAccess(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	eduService, usersService, activeService := setupPolicyServices(t, db)

	// Create the policy with real services
	p := policies.NewStudentVisitPolicy(eduService, usersService, activeService)

	// Admin role bypasses all checks - no fixtures needed
	authCtx := &policy.Context{
		Subject: policy.Subject{
			AccountID:   99999, // Doesn't need to exist for admin bypass
			Roles:       []string{"admin"},
			Permissions: []string{},
		},
		Resource: policy.Resource{Type: "visit", ID: int64(123)},
		Action:   policy.ActionView,
	}

	result, err := p.Evaluate(context.Background(), authCtx)

	require.NoError(t, err)
	assert.True(t, result, "Admin should always have access")
}

func TestStudentVisitPolicy_UserWithPermissionCanAccess(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	eduService, usersService, activeService := setupPolicyServices(t, db)
	p := policies.NewStudentVisitPolicy(eduService, usersService, activeService)

	// User with VisitsRead permission bypasses relationship checks
	authCtx := &policy.Context{
		Subject: policy.Subject{
			AccountID:   99999, // Doesn't need to exist for permission bypass
			Roles:       []string{"staff"},
			Permissions: []string{permissions.VisitsRead},
		},
		Resource: policy.Resource{Type: "visit", ID: int64(456)},
		Action:   policy.ActionView,
	}

	result, err := p.Evaluate(context.Background(), authCtx)

	require.NoError(t, err)
	assert.True(t, result, "User with VisitsRead permission should have access")
}

func TestStudentVisitPolicy_StudentCanAccessOwnVisit(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	eduService, usersService, activeService := setupPolicyServices(t, db)
	p := policies.NewStudentVisitPolicy(eduService, usersService, activeService)

	// ARRANGE: Create student with account
	student, studentAccount := testpkg.CreateTestStudentWithAccount(t, db, "Test", "Student", "1a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID, studentAccount.ID)

	// Create activity and room for the visit
	activity := testpkg.CreateTestActivityGroup(t, db, "Test Activity")
	room := testpkg.CreateTestRoom(t, db, "Test Room")
	defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID)

	// Create active group (session)
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	defer testpkg.CleanupActivityFixtures(t, db, activeGroup.ID)

	// Create visit for this student
	visit := testpkg.CreateTestVisit(t, db, student.ID, activeGroup.ID, time.Now(), nil)
	defer testpkg.CleanupActivityFixtures(t, db, visit.ID)

	// ACT: Student tries to access their own visit
	authCtx := &policy.Context{
		Subject: policy.Subject{
			AccountID:   studentAccount.ID, // Real account ID
			Roles:       []string{"student"},
			Permissions: []string{},
		},
		Resource: policy.Resource{Type: "visit", ID: visit.ID}, // Real visit ID
		Action:   policy.ActionView,
	}

	result, err := p.Evaluate(context.Background(), authCtx)

	// ASSERT
	require.NoError(t, err)
	assert.True(t, result, "Student should be able to access their own visit")
}

func TestStudentVisitPolicy_StudentCannotAccessOtherStudentsVisit(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	eduService, usersService, activeService := setupPolicyServices(t, db)
	p := policies.NewStudentVisitPolicy(eduService, usersService, activeService)

	// ARRANGE: Create two students
	student1, student1Account := testpkg.CreateTestStudentWithAccount(t, db, "Student", "One", "1a")
	student2, student2Account := testpkg.CreateTestStudentWithAccount(t, db, "Student", "Two", "1b")
	defer testpkg.CleanupActivityFixtures(t, db, student1.ID, student1Account.ID, student2.ID, student2Account.ID)

	// Create activity and room
	activity := testpkg.CreateTestActivityGroup(t, db, "Test Activity")
	room := testpkg.CreateTestRoom(t, db, "Test Room")
	defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID)

	// Create active group
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	defer testpkg.CleanupActivityFixtures(t, db, activeGroup.ID)

	// Create visit belonging to student2
	visit := testpkg.CreateTestVisit(t, db, student2.ID, activeGroup.ID, time.Now(), nil)
	defer testpkg.CleanupActivityFixtures(t, db, visit.ID)

	// ACT: Student1 tries to access Student2's visit
	authCtx := &policy.Context{
		Subject: policy.Subject{
			AccountID:   student1Account.ID, // Student1's account
			Roles:       []string{"student"},
			Permissions: []string{},
		},
		Resource: policy.Resource{Type: "visit", ID: visit.ID}, // Student2's visit
		Action:   policy.ActionView,
	}

	result, err := p.Evaluate(context.Background(), authCtx)

	// ASSERT
	require.NoError(t, err)
	assert.False(t, result, "Student should NOT be able to access another student's visit")
}

func TestStudentVisitPolicy_TeacherCanAccessVisitOfStudentInTheirGroup(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	eduService, usersService, activeService := setupPolicyServices(t, db)
	p := policies.NewStudentVisitPolicy(eduService, usersService, activeService)

	// ARRANGE: Create teacher with account
	teacher, teacherAccount := testpkg.CreateTestTeacherWithAccount(t, db, "Test", "Teacher")
	defer testpkg.CleanupActivityFixtures(t, db, teacher.ID, teacher.Staff.ID, teacherAccount.ID)

	// Create education group and assign teacher
	eduGroup := testpkg.CreateTestEducationGroup(t, db, "Class 1a")
	testpkg.CreateTestGroupTeacher(t, db, eduGroup.ID, teacher.ID)
	defer testpkg.CleanupActivityFixtures(t, db, eduGroup.ID)

	// Create student IN THE SAME GROUP as teacher
	student, studentAccount := testpkg.CreateTestStudentWithAccount(t, db, "Test", "Student", "1a")
	testpkg.AssignStudentToGroup(t, db, student.ID, eduGroup.ID)
	defer testpkg.CleanupActivityFixtures(t, db, student.ID, studentAccount.ID)

	// Create activity and room for the visit
	activity := testpkg.CreateTestActivityGroup(t, db, "Test Activity")
	room := testpkg.CreateTestRoom(t, db, "Test Room")
	defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID)

	// Create active group (session)
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	defer testpkg.CleanupActivityFixtures(t, db, activeGroup.ID)

	// Create visit for student
	visit := testpkg.CreateTestVisit(t, db, student.ID, activeGroup.ID, time.Now(), nil)
	defer testpkg.CleanupActivityFixtures(t, db, visit.ID)

	// ACT: Teacher tries to access student's visit
	authCtx := &policy.Context{
		Subject: policy.Subject{
			AccountID:   teacherAccount.ID,
			Roles:       []string{"teacher"},
			Permissions: []string{},
		},
		Resource: policy.Resource{Type: "visit", ID: visit.ID},
		Action:   policy.ActionView,
	}

	result, err := p.Evaluate(context.Background(), authCtx)

	// ASSERT
	require.NoError(t, err)
	assert.True(t, result, "Teacher should be able to access visit of student in their group")
}

func TestStudentVisitPolicy_TeacherCannotAccessVisitOfStudentNotInTheirGroup(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	eduService, usersService, activeService := setupPolicyServices(t, db)
	p := policies.NewStudentVisitPolicy(eduService, usersService, activeService)

	// ARRANGE: Create teacher with account
	teacher, teacherAccount := testpkg.CreateTestTeacherWithAccount(t, db, "Test", "Teacher")
	defer testpkg.CleanupActivityFixtures(t, db, teacher.ID, teacher.Staff.ID, teacherAccount.ID)

	// Create education group A and assign teacher to it
	groupA := testpkg.CreateTestEducationGroup(t, db, "Class A")
	testpkg.CreateTestGroupTeacher(t, db, groupA.ID, teacher.ID)
	defer testpkg.CleanupActivityFixtures(t, db, groupA.ID)

	// Create education group B (teacher NOT assigned)
	groupB := testpkg.CreateTestEducationGroup(t, db, "Class B")
	defer testpkg.CleanupActivityFixtures(t, db, groupB.ID)

	// Create student in GROUP B (not teacher's group)
	student, studentAccount := testpkg.CreateTestStudentWithAccount(t, db, "Other", "Student", "2b")
	testpkg.AssignStudentToGroup(t, db, student.ID, groupB.ID)
	defer testpkg.CleanupActivityFixtures(t, db, student.ID, studentAccount.ID)

	// Create activity and room for the visit
	activity := testpkg.CreateTestActivityGroup(t, db, "Test Activity")
	room := testpkg.CreateTestRoom(t, db, "Test Room")
	defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID)

	// Create active group (session)
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	defer testpkg.CleanupActivityFixtures(t, db, activeGroup.ID)

	// Create visit for student
	visit := testpkg.CreateTestVisit(t, db, student.ID, activeGroup.ID, time.Now(), nil)
	defer testpkg.CleanupActivityFixtures(t, db, visit.ID)

	// ACT: Teacher tries to access student's visit (student not in their group)
	authCtx := &policy.Context{
		Subject: policy.Subject{
			AccountID:   teacherAccount.ID,
			Roles:       []string{"teacher"},
			Permissions: []string{},
		},
		Resource: policy.Resource{Type: "visit", ID: visit.ID},
		Action:   policy.ActionView,
	}

	result, err := p.Evaluate(context.Background(), authCtx)

	// ASSERT
	require.NoError(t, err)
	assert.False(t, result, "Teacher should NOT be able to access visit of student not in their group")
}

func TestStudentVisitPolicy_RegularUserWithoutPermissionsCannotAccess(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	eduService, usersService, activeService := setupPolicyServices(t, db)
	p := policies.NewStudentVisitPolicy(eduService, usersService, activeService)

	// ARRANGE: Create a regular user (person with account but no student/staff/teacher)
	person, account := testpkg.CreateTestPersonWithAccount(t, db, "Regular", "User")
	defer testpkg.CleanupActivityFixtures(t, db, person.ID, account.ID)

	// Create student and visit
	student, studentAccount := testpkg.CreateTestStudentWithAccount(t, db, "Test", "Student", "1a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID, studentAccount.ID)

	activity := testpkg.CreateTestActivityGroup(t, db, "Test Activity")
	room := testpkg.CreateTestRoom(t, db, "Test Room")
	defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID)

	activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	defer testpkg.CleanupActivityFixtures(t, db, activeGroup.ID)

	visit := testpkg.CreateTestVisit(t, db, student.ID, activeGroup.ID, time.Now(), nil)
	defer testpkg.CleanupActivityFixtures(t, db, visit.ID)

	// ACT: Regular user (not student, not staff) tries to access visit
	authCtx := &policy.Context{
		Subject: policy.Subject{
			AccountID:   account.ID,
			Roles:       []string{"user"},
			Permissions: []string{},
		},
		Resource: policy.Resource{Type: "visit", ID: visit.ID},
		Action:   policy.ActionView,
	}

	result, err := p.Evaluate(context.Background(), authCtx)

	// ASSERT
	require.NoError(t, err)
	assert.False(t, result, "Regular user without permissions should NOT be able to access visits")
}

func TestStudentVisitPolicy_Metadata(t *testing.T) {
	// This test doesn't need database - it's testing static metadata
	p := policies.NewStudentVisitPolicy(nil, nil, nil)

	assert.Equal(t, "student_visit_access", p.Name())
	assert.Equal(t, "visit", p.ResourceType())
}

// =============================================================================
// PolicyRegistry Tests
// =============================================================================

func TestNewPolicyRegistry(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	eduService, usersService, activeService := setupPolicyServices(t, db)

	registry := policies.NewPolicyRegistry(eduService, usersService, activeService)
	assert.NotNil(t, registry)
}

func TestPolicyRegistry_RegisterAll(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	eduService, usersService, activeService := setupPolicyServices(t, db)

	registry := policies.NewPolicyRegistry(eduService, usersService, activeService)

	// Create a mock authorization service to register policies with
	authService := &mockAuthService{}

	err := registry.RegisterAll(authService)
	require.NoError(t, err)
	assert.True(t, authService.registerCalled)
}

// mockAuthService implements authorize.AuthorizationService for testing
type mockAuthService struct {
	registerCalled bool
	policies       []policy.Policy
}

func (m *mockAuthService) RegisterPolicy(p policy.Policy) error {
	m.registerCalled = true
	m.policies = append(m.policies, p)
	return nil
}

func (m *mockAuthService) AuthorizeResource(_ context.Context, _ policy.Subject, _ policy.Resource, _ policy.Action, _ map[string]interface{}) (bool, error) {
	return false, nil
}

func (m *mockAuthService) GetPolicyEngine() policy.PolicyEngine {
	return nil
}
