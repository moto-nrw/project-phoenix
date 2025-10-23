package active

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	facilityModels "github.com/moto-nrw/project-phoenix/models/facilities"
	locationModels "github.com/moto-nrw/project-phoenix/models/location"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

func TestGetStudentLocationStatus_Home(t *testing.T) {
	t.Helper()
	svc, deps := setupService(t)
	defer deps.Cleanup()

	// Create student without any check-in
	student := createTestStudent(t, deps)

	// Get location status - should be HOME
	status, err := svc.GetStudentLocationStatus(context.Background(), student.ID)
	require.NoError(t, err)
	require.NotNil(t, status)

	assert.Equal(t, locationModels.StateHome, status.State)
	assert.Nil(t, status.Room, "HOME state should not have room metadata")
}

func TestGetStudentLocationStatus_Transit(t *testing.T) {
	t.Helper()
	svc, deps := setupService(t)
	defer deps.Cleanup()

	// Create student and check them in (but don't assign to a room visit)
	student := createTestStudent(t, deps)
	createTestCheckIn(t, deps, student.ID)

	// Student is checked in but has no active visit → TRANSIT
	status, err := svc.GetStudentLocationStatus(context.Background(), student.ID)
	require.NoError(t, err)
	require.NotNil(t, status)

	assert.Equal(t, locationModels.StateTransit, status.State)
	assert.Nil(t, status.Room, "TRANSIT state should not have room metadata")
}

func TestGetStudentLocationStatus_PresentInGroupRoom(t *testing.T) {
	t.Helper()
	svc, deps := setupService(t)
	defer deps.Cleanup()

	// Create student, educational group with room, and active group
	student := createTestStudent(t, deps)
	eduGroup := createTestEducationGroup(t, deps)
	room := createTestRoom(t, deps, "Raum A")

	// Assign student to educational group and set group's room
	assignStudentToGroup(t, deps, student.ID, eduGroup.ID)
	assignGroupToRoom(t, deps, eduGroup.ID, room.ID)

	// Create active group session
	activeGroup := createTestActiveGroup(t, deps, eduGroup.ID, room.ID)

	// Check student in and create visit to the group
	createTestCheckIn(t, deps, student.ID)
	createTestVisit(t, deps, student.ID, activeGroup.ID)

	// Get location status - should be PRESENT_IN_ROOM with isGroupRoom=true
	status, err := svc.GetStudentLocationStatus(context.Background(), student.ID)
	require.NoError(t, err)
	require.NotNil(t, status)

	assert.Equal(t, locationModels.StatePresentInRoom, status.State)
	if assert.NotNil(t, status.Room, "PRESENT_IN_ROOM should have room metadata") {
		assert.Equal(t, room.ID, status.Room.ID)
		assert.Equal(t, "Raum A", status.Room.Name)
		assert.True(t, status.Room.IsGroupRoom, "Student should be in their educational group room")
		assert.Equal(t, locationModels.RoomOwnerGroup, status.Room.OwnerType)
	}
}

func TestGetStudentLocationStatus_PresentInOtherRoom(t *testing.T) {
	t.Helper()
	svc, deps := setupService(t)
	defer deps.Cleanup()

	// Create student with educational group (room A)
	student := createTestStudent(t, deps)
	eduGroup := createTestEducationGroup(t, deps)
	groupRoom := createTestRoom(t, deps, "Gruppenraum")
	assignStudentToGroup(t, deps, student.ID, eduGroup.ID)
	assignGroupToRoom(t, deps, eduGroup.ID, groupRoom.ID)

	// Create activity with different room (room B)
	activityRoom := createTestRoom(t, deps, "Aktivitätsraum")
	activityGroup := createTestActivityGroup(t, deps, "Sport")
	activeGroup := createTestActiveGroup(t, deps, activityGroup.ID, activityRoom.ID)

	// Check student in and create visit to the activity room
	createTestCheckIn(t, deps, student.ID)
	createTestVisit(t, deps, student.ID, activeGroup.ID)

	// Get location status - should be PRESENT_IN_ROOM with isGroupRoom=false
	status, err := svc.GetStudentLocationStatus(context.Background(), student.ID)
	require.NoError(t, err)
	require.NotNil(t, status)

	assert.Equal(t, locationModels.StatePresentInRoom, status.State)
	if assert.NotNil(t, status.Room) {
		assert.Equal(t, activityRoom.ID, status.Room.ID)
		assert.Equal(t, "Aktivitätsraum", status.Room.Name)
		assert.False(t, status.Room.IsGroupRoom, "Student should be in a different room")
		assert.Equal(t, locationModels.RoomOwnerActivity, status.Room.OwnerType)
	}
}

func TestGetStudentLocationStatus_Schoolyard(t *testing.T) {
	t.Helper()
	svc, deps := setupService(t)
	defer deps.Cleanup()

	// Create student and schoolyard room (detected by name)
	student := createTestStudent(t, deps)
	schoolyardRoom := createTestRoom(t, deps, "Schulhof")
	eduGroup := createTestEducationGroup(t, deps)
	activeGroup := createTestActiveGroup(t, deps, eduGroup.ID, schoolyardRoom.ID)

	// Check student in and create visit to schoolyard
	createTestCheckIn(t, deps, student.ID)
	createTestVisit(t, deps, student.ID, activeGroup.ID)

	// Get location status - should be SCHOOLYARD
	status, err := svc.GetStudentLocationStatus(context.Background(), student.ID)
	require.NoError(t, err)
	require.NotNil(t, status)

	assert.Equal(t, locationModels.StateSchoolyard, status.State)
	if assert.NotNil(t, status.Room) {
		assert.Equal(t, schoolyardRoom.ID, status.Room.ID)
		assert.Equal(t, "Schulhof", status.Room.Name)
	}
}

func TestGetStudentLocationStatus_DefaultsToHomeOnError(t *testing.T) {
	t.Helper()
	svc, deps := setupService(t)
	defer deps.Cleanup()

	// Try to get status for non-existent student
	status, err := svc.GetStudentLocationStatus(context.Background(), 999999)

	// Should return HOME status even with error
	require.NoError(t, err, "Should not fail even if student not found")
	require.NotNil(t, status)
	assert.Equal(t, locationModels.StateHome, status.State)
}

// --- Test harness helpers --------------------------------------------------------------------

type testDeps struct {
	groupRepo          *fakeGroupRepo
	visitRepo          *fakeVisitRepo
	roomRepo           *fakeRoomRepo
	studentRepo        *fakeStudentRepo
	educationGroupRepo *fakeEducationGroupRepo
	attendanceRepo     *fakeAttendanceRepo
	nextStudentID      int64
	nextGroupID        int64
	nextRoomID         int64
	nextActiveGroupID  int64
}

func (d *testDeps) Cleanup() {}

func setupService(t *testing.T) (*service, *testDeps) {
	t.Helper()

	deps := &testDeps{
		groupRepo:          newFakeGroupRepo(),
		visitRepo:          newFakeVisitRepo(),
		roomRepo:           newFakeRoomRepo(),
		studentRepo:        newFakeStudentRepo(),
		educationGroupRepo: newFakeEducationGroupRepo(),
		attendanceRepo:     newFakeAttendanceRepo(),
		nextStudentID:      1000,
		nextGroupID:        2000,
		nextRoomID:         3000,
		nextActiveGroupID:  4000,
	}

	svc := &service{
		groupRepo:          deps.groupRepo,
		visitRepo:          deps.visitRepo,
		studentRepo:        deps.studentRepo,
		roomRepo:           deps.roomRepo,
		educationGroupRepo: deps.educationGroupRepo,
		attendanceRepo:     deps.attendanceRepo,
	}

	return svc, deps
}

func createTestStudent(t *testing.T, deps *testDeps) *userModels.Student {
	t.Helper()
	deps.nextStudentID++
	id := deps.nextStudentID
	student := &userModels.Student{
		Model:    base.Model{ID: id},
		PersonID: id,
	}
	require.NoError(t, deps.studentRepo.Create(context.Background(), student))
	return student
}

func createTestEducationGroup(t *testing.T, deps *testDeps) *educationModels.Group {
	t.Helper()
	deps.nextGroupID++
	group := &educationModels.Group{
		Model: base.Model{ID: deps.nextGroupID},
		Name:  "Group-" + time.Now().Format("150405.000000"),
	}
	require.NoError(t, deps.educationGroupRepo.Create(context.Background(), group))
	return group
}

func createTestActivityGroup(t *testing.T, deps *testDeps, name string) *educationModels.Group {
	t.Helper()
	deps.nextGroupID++
	group := &educationModels.Group{
		Model: base.Model{ID: deps.nextGroupID},
		Name:  name,
	}
	return group
}

func createTestRoom(t *testing.T, deps *testDeps, name string) *facilityModels.Room {
	t.Helper()
	deps.nextRoomID++
	room := &facilityModels.Room{
		Model:    base.Model{ID: deps.nextRoomID},
		Name:     name,
		Category: name,
	}
	require.NoError(t, deps.roomRepo.Create(context.Background(), room))
	return room
}

func assignStudentToGroup(t *testing.T, deps *testDeps, studentID, groupID int64) {
	t.Helper()
	require.NoError(t, deps.studentRepo.AssignToGroup(context.Background(), studentID, groupID))
}

func assignGroupToRoom(t *testing.T, deps *testDeps, groupID, roomID int64) {
	t.Helper()
	require.NoError(t, deps.educationGroupRepo.SetRoom(context.Background(), groupID, roomID))
}

func createTestActiveGroup(t *testing.T, deps *testDeps, groupID, roomID int64) *active.Group {
	t.Helper()
	deps.nextActiveGroupID++
	room, err := deps.roomRepo.FindByID(context.Background(), roomID)
	require.NoError(t, err)

	activeGroup := &active.Group{
		Model:        base.Model{ID: deps.nextActiveGroupID},
		GroupID:      groupID,
		RoomID:       roomID,
		StartTime:    time.Now(),
		LastActivity: time.Now(),
		Room:         room,
	}

	deps.groupRepo.Store(activeGroup)
	return activeGroup
}

func createTestCheckIn(t *testing.T, deps *testDeps, studentID int64) {
	t.Helper()
	deps.attendanceRepo.Set(studentID, &active.Attendance{
		StudentID:   studentID,
		Date:        time.Now().Truncate(24 * time.Hour),
		CheckInTime: time.Now(),
	})
}

func createTestVisit(t *testing.T, deps *testDeps, studentID, activeGroupID int64) {
	t.Helper()
	deps.visitRepo.AddVisit(&active.Visit{
		Model:         base.Model{ID: time.Now().UnixNano()},
		StudentID:     studentID,
		ActiveGroupID: activeGroupID,
		EntryTime:     time.Now(),
	})
}

// --- Fake repositories ----------------------------------------------------------------------

type fakeGroupRepo struct {
	groups map[int64]*active.Group
}

var (
	errNotImplemented = errors.New("not implemented")
	errInvalidID      = errors.New("invalid id")
	errNotFound       = errors.New("not found")
)

func newFakeGroupRepo() *fakeGroupRepo {
	return &fakeGroupRepo{groups: make(map[int64]*active.Group)}
}

func (r *fakeGroupRepo) Store(g *active.Group) {
	r.groups[g.ID] = g
}

func (r *fakeGroupRepo) Create(context.Context, *active.Group) error { return errNotImplemented }
func (r *fakeGroupRepo) Update(context.Context, *active.Group) error { return errNotImplemented }
func (r *fakeGroupRepo) Delete(context.Context, interface{}) error   { return errNotImplemented }
func (r *fakeGroupRepo) List(context.Context, *base.QueryOptions) ([]*active.Group, error) {
	return nil, errNotImplemented
}

func (r *fakeGroupRepo) FindByID(ctx context.Context, id interface{}) (*active.Group, error) {
	val, ok := toInt64(id)
	if !ok {
		return nil, errInvalidID
	}
	group, exists := r.groups[val]
	if !exists {
		return nil, errNotFound
	}
	return group, nil
}

func (r *fakeGroupRepo) FindActiveByRoomID(context.Context, int64) ([]*active.Group, error) {
	return nil, errNotImplemented
}
func (r *fakeGroupRepo) FindActiveByGroupID(context.Context, int64) ([]*active.Group, error) {
	return nil, errNotImplemented
}
func (r *fakeGroupRepo) FindByTimeRange(context.Context, time.Time, time.Time) ([]*active.Group, error) {
	return nil, errNotImplemented
}
func (r *fakeGroupRepo) EndSession(context.Context, int64) error { return errNotImplemented }
func (r *fakeGroupRepo) FindBySourceIDs(context.Context, []int64, string) ([]*active.Group, error) {
	return nil, errNotImplemented
}
func (r *fakeGroupRepo) FindWithRelations(context.Context, int64) (*active.Group, error) {
	return nil, errNotImplemented
}
func (r *fakeGroupRepo) FindWithVisits(context.Context, int64) (*active.Group, error) {
	return nil, errNotImplemented
}
func (r *fakeGroupRepo) FindWithSupervisors(context.Context, int64) (*active.Group, error) {
	return nil, errNotImplemented
}
func (r *fakeGroupRepo) FindActiveByGroupIDWithDevice(context.Context, int64) ([]*active.Group, error) {
	return nil, errNotImplemented
}
func (r *fakeGroupRepo) FindActiveByDeviceID(context.Context, int64) (*active.Group, error) {
	return nil, errNotImplemented
}
func (r *fakeGroupRepo) FindActiveByDeviceIDWithRelations(context.Context, int64) (*active.Group, error) {
	return nil, errNotImplemented
}
func (r *fakeGroupRepo) FindActiveByDeviceIDWithNames(context.Context, int64) (*active.Group, error) {
	return nil, errNotImplemented
}
func (r *fakeGroupRepo) CheckRoomConflict(context.Context, int64, int64) (bool, *active.Group, error) {
	return false, nil, errNotImplemented
}
func (r *fakeGroupRepo) UpdateLastActivity(context.Context, int64, time.Time) error {
	return errNotImplemented
}
func (r *fakeGroupRepo) FindActiveSessionsOlderThan(context.Context, time.Time) ([]*active.Group, error) {
	return nil, errNotImplemented
}
func (r *fakeGroupRepo) FindInactiveSessions(context.Context, time.Duration) ([]*active.Group, error) {
	return nil, errNotImplemented
}
func (r *fakeGroupRepo) FindUnclaimed(context.Context) ([]*active.Group, error) {
	return nil, errNotImplemented
}
func (r *fakeGroupRepo) FindActiveGroups(context.Context) ([]*active.Group, error) {
	return nil, errNotImplemented
}

type fakeVisitRepo struct {
	visits map[int64][]*active.Visit
}

func newFakeVisitRepo() *fakeVisitRepo {
	return &fakeVisitRepo{visits: make(map[int64][]*active.Visit)}
}

func (r *fakeVisitRepo) AddVisit(visit *active.Visit) {
	r.visits[visit.StudentID] = append(r.visits[visit.StudentID], visit)
}

func (r *fakeVisitRepo) Create(context.Context, *active.Visit) error { return errNotImplemented }
func (r *fakeVisitRepo) Update(context.Context, *active.Visit) error { return errNotImplemented }
func (r *fakeVisitRepo) Delete(context.Context, interface{}) error   { return errNotImplemented }
func (r *fakeVisitRepo) List(context.Context, *base.QueryOptions) ([]*active.Visit, error) {
	return nil, errNotImplemented
}

func (r *fakeVisitRepo) FindByID(context.Context, interface{}) (*active.Visit, error) {
	return nil, errNotImplemented
}

func (r *fakeVisitRepo) FindActiveByStudentID(ctx context.Context, studentID int64) ([]*active.Visit, error) {
	visits := r.visits[studentID]
	copied := make([]*active.Visit, 0, len(visits))
	for _, v := range visits {
		if v.IsActive() {
			copied = append(copied, v)
		}
	}
	return copied, nil
}

func (r *fakeVisitRepo) FindByActiveGroupID(context.Context, int64) ([]*active.Visit, error) {
	return nil, errNotImplemented
}
func (r *fakeVisitRepo) FindByTimeRange(context.Context, time.Time, time.Time) ([]*active.Visit, error) {
	return nil, errNotImplemented
}
func (r *fakeVisitRepo) EndVisit(context.Context, int64) error { return errNotImplemented }
func (r *fakeVisitRepo) TransferVisitsFromRecentSessions(context.Context, int64, int64) (int, error) {
	return 0, errNotImplemented
}
func (r *fakeVisitRepo) DeleteExpiredVisits(context.Context, int64, int) (int64, error) {
	return 0, errNotImplemented
}
func (r *fakeVisitRepo) DeleteVisitsBeforeDate(context.Context, int64, time.Time) (int64, error) {
	return 0, errNotImplemented
}
func (r *fakeVisitRepo) GetVisitRetentionStats(context.Context) (map[int64]int, error) {
	return nil, errNotImplemented
}
func (r *fakeVisitRepo) CountExpiredVisits(context.Context) (int64, error) {
	return 0, errNotImplemented
}
func (r *fakeVisitRepo) GetCurrentByStudentID(context.Context, int64) (*active.Visit, error) {
	return nil, errNotImplemented
}
func (r *fakeVisitRepo) FindActiveVisits(context.Context) ([]*active.Visit, error) {
	return nil, errNotImplemented
}

type fakeRoomRepo struct {
	rooms map[int64]*facilityModels.Room
}

func newFakeRoomRepo() *fakeRoomRepo {
	return &fakeRoomRepo{rooms: make(map[int64]*facilityModels.Room)}
}

func (r *fakeRoomRepo) Create(_ context.Context, room *facilityModels.Room) error {
	r.rooms[room.ID] = room
	return nil
}

func (r *fakeRoomRepo) FindByID(_ context.Context, id interface{}) (*facilityModels.Room, error) {
	val, ok := toInt64(id)
	if !ok {
		return nil, errInvalidID
	}
	room, exists := r.rooms[val]
	if !exists {
		return nil, errNotFound
	}
	return room, nil
}

func (r *fakeRoomRepo) FindByName(context.Context, string) (*facilityModels.Room, error) {
	return nil, errNotImplemented
}
func (r *fakeRoomRepo) FindByBuilding(context.Context, string) ([]*facilityModels.Room, error) {
	return nil, errNotImplemented
}
func (r *fakeRoomRepo) FindByCategory(context.Context, string) ([]*facilityModels.Room, error) {
	return nil, errNotImplemented
}
func (r *fakeRoomRepo) FindByFloor(context.Context, string, int) ([]*facilityModels.Room, error) {
	return nil, errNotImplemented
}
func (r *fakeRoomRepo) Update(context.Context, *facilityModels.Room) error { return errNotImplemented }
func (r *fakeRoomRepo) Delete(context.Context, interface{}) error          { return errNotImplemented }
func (r *fakeRoomRepo) List(context.Context, map[string]interface{}) ([]*facilityModels.Room, error) {
	return nil, errNotImplemented
}

type fakeStudentRepo struct {
	students map[int64]*userModels.Student
}

func newFakeStudentRepo() *fakeStudentRepo {
	return &fakeStudentRepo{students: make(map[int64]*userModels.Student)}
}

func (r *fakeStudentRepo) Create(_ context.Context, student *userModels.Student) error {
	r.students[student.ID] = student
	return nil
}

func (r *fakeStudentRepo) FindByID(_ context.Context, id interface{}) (*userModels.Student, error) {
	val, ok := toInt64(id)
	if !ok {
		return nil, errInvalidID
	}
	student, exists := r.students[val]
	if !exists {
		return nil, errNotFound
	}
	return student, nil
}

func (r *fakeStudentRepo) AssignToGroup(_ context.Context, studentID, groupID int64) error {
	student, exists := r.students[studentID]
	if !exists {
		return errNotFound
	}
	student.GroupID = &groupID
	return nil
}

// Unused interface methods
func (r *fakeStudentRepo) FindByPersonID(context.Context, int64) (*userModels.Student, error) {
	return nil, errNotImplemented
}
func (r *fakeStudentRepo) FindByGroupID(context.Context, int64) ([]*userModels.Student, error) {
	return nil, errNotImplemented
}
func (r *fakeStudentRepo) FindByGroupIDs(context.Context, []int64) ([]*userModels.Student, error) {
	return nil, errNotImplemented
}
func (r *fakeStudentRepo) FindBySchoolClass(context.Context, string) ([]*userModels.Student, error) {
	return nil, errNotImplemented
}
func (r *fakeStudentRepo) Update(context.Context, *userModels.Student) error {
	return errNotImplemented
}
func (r *fakeStudentRepo) Delete(context.Context, interface{}) error { return errNotImplemented }
func (r *fakeStudentRepo) List(context.Context, map[string]interface{}) ([]*userModels.Student, error) {
	return nil, errNotImplemented
}
func (r *fakeStudentRepo) ListWithOptions(context.Context, *base.QueryOptions) ([]*userModels.Student, error) {
	return nil, errNotImplemented
}
func (r *fakeStudentRepo) CountWithOptions(context.Context, *base.QueryOptions) (int, error) {
	return 0, errNotImplemented
}
func (r *fakeStudentRepo) UpdateLocation(context.Context, int64, string) error {
	return errNotImplemented
}
func (r *fakeStudentRepo) RemoveFromGroup(context.Context, int64) error { return errNotImplemented }
func (r *fakeStudentRepo) FindByTeacherID(context.Context, int64) ([]*userModels.Student, error) {
	return nil, errNotImplemented
}
func (r *fakeStudentRepo) FindByTeacherIDWithGroups(context.Context, int64) ([]*userModels.StudentWithGroupInfo, error) {
	return nil, errNotImplemented
}

type fakeEducationGroupRepo struct {
	groups map[int64]*educationModels.Group
}

func newFakeEducationGroupRepo() *fakeEducationGroupRepo {
	return &fakeEducationGroupRepo{groups: make(map[int64]*educationModels.Group)}
}

func (r *fakeEducationGroupRepo) Create(_ context.Context, group *educationModels.Group) error {
	r.groups[group.ID] = group
	return nil
}

func (r *fakeEducationGroupRepo) FindByID(_ context.Context, id interface{}) (*educationModels.Group, error) {
	val, ok := toInt64(id)
	if !ok {
		return nil, errInvalidID
	}
	group, exists := r.groups[val]
	if !exists {
		return nil, errNotFound
	}
	return group, nil
}

func (r *fakeEducationGroupRepo) SetRoom(_ context.Context, groupID, roomID int64) error {
	group, exists := r.groups[groupID]
	if !exists {
		return errNotFound
	}
	group.RoomID = &roomID
	return nil
}

// Remaining interface methods unused
func (r *fakeEducationGroupRepo) Update(context.Context, *educationModels.Group) error {
	return errNotImplemented
}
func (r *fakeEducationGroupRepo) Delete(context.Context, interface{}) error { return errNotImplemented }
func (r *fakeEducationGroupRepo) List(context.Context, map[string]interface{}) ([]*educationModels.Group, error) {
	return nil, errNotImplemented
}
func (r *fakeEducationGroupRepo) ListWithOptions(context.Context, *base.QueryOptions) ([]*educationModels.Group, error) {
	return nil, errNotImplemented
}
func (r *fakeEducationGroupRepo) FindByName(context.Context, string) (*educationModels.Group, error) {
	return nil, errNotImplemented
}
func (r *fakeEducationGroupRepo) FindByRoom(context.Context, int64) ([]*educationModels.Group, error) {
	return nil, errNotImplemented
}
func (r *fakeEducationGroupRepo) FindByTeacher(context.Context, int64) ([]*educationModels.Group, error) {
	return nil, errNotImplemented
}
func (r *fakeEducationGroupRepo) FindWithRoom(context.Context, int64) (*educationModels.Group, error) {
	return nil, errNotImplemented
}

type fakeAttendanceRepo struct {
	statuses map[int64]*active.Attendance
}

func newFakeAttendanceRepo() *fakeAttendanceRepo {
	return &fakeAttendanceRepo{statuses: make(map[int64]*active.Attendance)}
}

func (r *fakeAttendanceRepo) Set(studentID int64, attendance *active.Attendance) {
	r.statuses[studentID] = attendance
}

func (r *fakeAttendanceRepo) GetStudentCurrentStatus(_ context.Context, studentID int64) (*active.Attendance, error) {
	attendance, exists := r.statuses[studentID]
	if !exists {
		return nil, errNotFound
	}
	return attendance, nil
}

// Other interface methods unused
func (r *fakeAttendanceRepo) Create(context.Context, *active.Attendance) error {
	return errNotImplemented
}
func (r *fakeAttendanceRepo) Update(context.Context, *active.Attendance) error {
	return errNotImplemented
}
func (r *fakeAttendanceRepo) FindByID(context.Context, int64) (*active.Attendance, error) {
	return nil, errNotImplemented
}
func (r *fakeAttendanceRepo) FindByStudentAndDate(context.Context, int64, time.Time) ([]*active.Attendance, error) {
	return nil, errNotImplemented
}
func (r *fakeAttendanceRepo) FindLatestByStudent(context.Context, int64) (*active.Attendance, error) {
	return nil, errNotImplemented
}
func (r *fakeAttendanceRepo) GetTodayByStudentID(context.Context, int64) (*active.Attendance, error) {
	return nil, errNotImplemented
}
func (r *fakeAttendanceRepo) FindForDate(context.Context, time.Time) ([]*active.Attendance, error) {
	return nil, errNotImplemented
}
func (r *fakeAttendanceRepo) Delete(context.Context, int64) error { return errNotImplemented }

// --- Helpers -------------------------------------------------------------------------------

func toInt64(value interface{}) (int64, bool) {
	switch v := value.(type) {
	case int64:
		return v, true
	case int:
		return int64(v), true
	case *int64:
		if v == nil {
			return 0, false
		}
		return *v, true
	case *int:
		if v == nil {
			return 0, false
		}
		return int64(*v), true
	default:
		return 0, false
	}
}
