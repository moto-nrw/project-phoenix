package authorize_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	educationModel "github.com/moto-nrw/project-phoenix/models/education"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/services/education"
	"github.com/moto-nrw/project-phoenix/services/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// setupVisitAccessServices creates real services for CanViewVisit testing.
func setupVisitAccessServices(t *testing.T, db *bun.DB) (education.Service, users.PersonService, active.Service) {
	t.Helper()

	repos := repositories.NewFactory(db)

	eduService := education.NewService(
		repos.Group,
		repos.GroupTeacher,
		repos.ClassTeacher,
		repos.GroupSubstitution,
		repos.Room,
		repos.Teacher,
		repos.Staff,
		repos.Student,
	)

	usersService := users.NewPersonService(users.PersonServiceDependencies{
		PersonRepo:  repos.Person,
		RFIDRepo:    repos.RFIDCard,
		AccountRepo: repos.Account,
		StudentRepo: repos.Student,
		StaffRepo:   repos.Staff,
		TeacherRepo: repos.Teacher,
		DB:          db,
	})

	activeService := active.NewService(active.ServiceDependencies{
		GroupRepo:          repos.ActiveGroup,
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

func TestCanViewVisit_AdminCanAlwaysAccess(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	eduService, usersService, activeService := setupVisitAccessServices(t, db)

	// Admin role bypasses all checks - no fixtures needed
	result, err := authorize.CanViewVisit(context.Background(), 99999, []string{"admin"}, []string{}, 123, activeService, usersService, eduService)

	require.NoError(t, err)
	assert.True(t, result, "Admin should always have access")
}

func TestCanViewVisit_UserWithPermissionCanAccess(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	eduService, usersService, activeService := setupVisitAccessServices(t, db)

	// User with VisitsRead permission bypasses relationship checks
	result, err := authorize.CanViewVisit(context.Background(), 99999, []string{"staff"}, []string{permissions.VisitsRead}, 456, activeService, usersService, eduService)

	require.NoError(t, err)
	assert.True(t, result, "User with VisitsRead permission should have access")
}

func TestCanViewVisit_StudentCanAccessOwnVisit(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	eduService, usersService, activeService := setupVisitAccessServices(t, db)

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
	result, err := authorize.CanViewVisit(context.Background(), studentAccount.ID, []string{"student"}, []string{}, visit.ID, activeService, usersService, eduService)

	// ASSERT
	require.NoError(t, err)
	assert.True(t, result, "Student should be able to access their own visit")
}

func TestCanViewVisit_StudentCannotAccessOtherStudentsVisit(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	eduService, usersService, activeService := setupVisitAccessServices(t, db)

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
	result, err := authorize.CanViewVisit(context.Background(), student1Account.ID, []string{"student"}, []string{}, visit.ID, activeService, usersService, eduService)

	// ASSERT
	require.NoError(t, err)
	assert.False(t, result, "Student should NOT be able to access another student's visit")
}

func TestCanViewVisit_TeacherCanAccessVisitOfStudentInTheirGroup(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	eduService, usersService, activeService := setupVisitAccessServices(t, db)

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
	result, err := authorize.CanViewVisit(context.Background(), teacherAccount.ID, []string{"teacher"}, []string{}, visit.ID, activeService, usersService, eduService)

	// ASSERT
	require.NoError(t, err)
	assert.True(t, result, "Teacher should be able to access visit of student in their group")
}

func TestCanViewVisit_TeacherCannotAccessVisitOfStudentNotInTheirGroup(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	eduService, usersService, activeService := setupVisitAccessServices(t, db)

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
	result, err := authorize.CanViewVisit(context.Background(), teacherAccount.ID, []string{"teacher"}, []string{}, visit.ID, activeService, usersService, eduService)

	// ASSERT
	require.NoError(t, err)
	assert.False(t, result, "Teacher should NOT be able to access visit of student not in their group")
}

func TestCanViewVisit_RegularUserWithoutPermissionsCannotAccess(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	eduService, usersService, activeService := setupVisitAccessServices(t, db)

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
	result, err := authorize.CanViewVisit(context.Background(), account.ID, []string{"user"}, []string{}, visit.ID, activeService, usersService, eduService)

	// ASSERT
	require.NoError(t, err)
	assert.False(t, result, "Regular user without permissions should NOT be able to access visits")
}

// =============================================================================
// RequireVisitView middleware rendering (ports the retired
// resource_middleware_test.go allow/deny/error branches)
// =============================================================================

// fakeVisitActive / fakeVisitPersons / fakeVisitEducation drive CanViewVisit
// through the middleware without a database.
type fakeVisitActive struct {
	visit *activeModel.Visit
	err   error
}

func (f *fakeVisitActive) GetVisit(_ context.Context, _ int64) (*activeModel.Visit, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.visit, nil
}

func (f *fakeVisitActive) FindSupervisorsByStaffID(_ context.Context, _ int64) ([]*activeModel.GroupSupervisor, error) {
	return nil, nil
}

func (f *fakeVisitActive) GetStudentCurrentVisit(_ context.Context, _ int64) (*activeModel.Visit, error) {
	return nil, nil
}

type fakeVisitPersons struct {
	person  *usersModel.Person
	staff   *usersModel.Staff
	teacher *usersModel.Teacher
}

func (f *fakeVisitPersons) FindByAccountID(_ context.Context, _ int64) (*usersModel.Person, error) {
	return f.person, nil
}

func (f *fakeVisitPersons) GetStudentByPersonID(_ context.Context, _ int64) (*usersModel.Student, error) {
	return nil, errors.New("no student")
}

func (f *fakeVisitPersons) GetStaffByPersonID(_ context.Context, _ int64) (*usersModel.Staff, error) {
	return f.staff, nil
}

func (f *fakeVisitPersons) GetTeacherByStaffID(_ context.Context, _ int64) (*usersModel.Teacher, error) {
	return f.teacher, nil
}

func (f *fakeVisitPersons) GetStudentByID(_ context.Context, _ int64) (*usersModel.Student, error) {
	return nil, errors.New("no student")
}

type fakeVisitEducation struct {
	err error
}

func (f *fakeVisitEducation) GetTeacherGroups(_ context.Context, _ int64) ([]*educationModel.Group, error) {
	return nil, f.err
}

func TestCanViewVisit_PropagatesVisitLookupError(t *testing.T) {
	lookupErr := errors.New("visit lookup failed")

	result, err := authorize.CanViewVisit(
		context.Background(),
		1000,
		[]string{"teacher"},
		[]string{},
		7777,
		&fakeVisitActive{err: lookupErr},
		&fakeVisitPersons{},
		&fakeVisitEducation{},
	)

	require.ErrorIs(t, err, lookupErr)
	assert.False(t, result)
}

func TestRequireVisitView(t *testing.T) {
	// Fixture graph shared by the deny and error cases: a non-admin caller
	// who resolves to a teacher, so the education lookup decides the outcome.
	teacherPersons := func() *fakeVisitPersons {
		return &fakeVisitPersons{
			person:  &usersModel.Person{},
			staff:   &usersModel.Staff{},
			teacher: &usersModel.Teacher{},
		}
	}
	visitActive := func() *fakeVisitActive {
		return &fakeVisitActive{visit: &activeModel.Visit{StudentID: 7777}}
	}

	tests := []struct {
		name           string
		claims         jwt.AppClaims
		active         *fakeVisitActive
		education      *fakeVisitEducation
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "allows access on admin bypass",
			claims:         jwt.AppClaims{ID: 1000, Roles: []string{"admin"}},
			education:      &fakeVisitEducation{},
			expectedStatus: http.StatusOK,
			expectedBody:   "Success",
		},
		{
			name:           "denies access with 403 when no relationship matches",
			claims:         jwt.AppClaims{ID: 1000, Roles: []string{"teacher"}},
			education:      &fakeVisitEducation{},
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "renders 500 Authorization error when the education lookup fails",
			claims:         jwt.AppClaims{ID: 1000, Roles: []string{"teacher"}},
			education:      &fakeVisitEducation{err: errors.New("education lookup failed")},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "policy student_visit_access evaluation failed: education lookup failed",
		},
		{
			name:           "renders 500 Authorization error when the visit lookup fails",
			claims:         jwt.AppClaims{ID: 1000, Roles: []string{"teacher"}},
			active:         &fakeVisitActive{err: errors.New("visit lookup failed")},
			education:      &fakeVisitEducation{},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "policy student_visit_access evaluation failed: visit lookup failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("Success"))
			})

			activeSvc := tt.active
			if activeSvc == nil {
				activeSvc = visitActive()
			}
			middleware := authorize.RequireVisitView(activeSvc, teacherPersons(), tt.education)
			protectedHandler := middleware(handler)

			req := httptest.NewRequest("GET", "/visits/7777", nil)
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", "7777")
			ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
			ctx = context.WithValue(ctx, jwt.CtxClaims, tt.claims)
			ctx = context.WithValue(ctx, jwt.CtxPermissions, []string{})
			req = req.WithContext(ctx)

			rr := httptest.NewRecorder()
			protectedHandler.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
			if tt.expectedBody != "" {
				assert.Contains(t, rr.Body.String(), tt.expectedBody)
			}
		})
	}
}
