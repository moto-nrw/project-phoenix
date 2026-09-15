package test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/models/schedule"
)

// Fixtures of the kiosk scan flows (#2698): rooms with exact names, the
// reserved system rooms, device-linked sessions and the row edits the
// daily-checkout gates read. Adapter tests hold no ORM import, so the row
// reads they assert on live here too.

func fixtureCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}

// CreateTestRoomNamed inserts a room with exactly this name and no capacity.
// The special-room flows match on the name, so no unique suffix is added.
func CreateTestRoomNamed(tb testing.TB, db *bun.DB, name string) *facilities.Room {
	tb.Helper()
	ctx, cancel := fixtureCtx()
	defer cancel()

	room := &facilities.Room{Name: name, Building: "Test Building"}
	room.SetTenantID(fixtureTenantID(tb))
	err := db.NewInsert().Model(room).ModelTableExpr(`facilities.rooms`).Scan(ctx)
	require.NoError(tb, err, "Failed to create test room %q", name)
	return room
}

// CreateTestSystemRoom inserts the reserved system room with exactly this
// name; released marks it as a permanently open room (#3064).
func CreateTestSystemRoom(tb testing.TB, db *bun.DB, name string, released bool) *facilities.Room {
	tb.Helper()
	ctx, cancel := fixtureCtx()
	defer cancel()

	room := &facilities.Room{Name: name, Building: "Test Building", IsSystem: true, IsOpenRoom: released}
	room.SetTenantID(fixtureTenantID(tb))
	err := db.NewInsert().Model(room).ModelTableExpr(`facilities.rooms`).Scan(ctx)
	require.NoError(tb, err, "Failed to create test system room %q", name)
	return room
}

// CreateTestRoomWithCapacity inserts a uniquely named room with a capacity.
func CreateTestRoomWithCapacity(tb testing.TB, db *bun.DB, name string, capacity int) *facilities.Room {
	tb.Helper()
	room := CreateTestRoom(tb, db, name)
	SetRoomCapacity(tb, db, room.ID, capacity)
	room.Capacity = &capacity
	return room
}

// SetRoomCapacity updates the capacity of a room.
func SetRoomCapacity(tb testing.TB, db *bun.DB, roomID int64, capacity int) {
	tb.Helper()
	setColumn(tb, db, "facilities.rooms", "capacity", capacity, roomID)
}

// LinkDeviceToActiveGroup binds a session to the device that started it.
func LinkDeviceToActiveGroup(tb testing.TB, db *bun.DB, activeGroupID, deviceID int64) {
	tb.Helper()
	setColumn(tb, db, "active.groups", "device_id", deviceID, activeGroupID)
}

// SetStudentGroup assigns (or clears, with nil) the education group of a student.
func SetStudentGroup(tb testing.TB, db *bun.DB, studentID int64, groupID *int64) {
	tb.Helper()
	setColumn(tb, db, "users.students", "group_id", groupID, studentID)
}

// SetStudentStatus updates the lifecycle status of a student.
func SetStudentStatus(tb testing.TB, db *bun.DB, studentID int64, status string) {
	tb.Helper()
	setColumn(tb, db, "users.students", "status", status, studentID)
}

// SetEducationGroupRoom assigns (or clears, with nil) the room of an education group.
func SetEducationGroupRoom(tb testing.TB, db *bun.DB, groupID int64, roomID *int64) {
	tb.Helper()
	setColumn(tb, db, "education.groups", "room_id", roomID, groupID)
}

func setColumn(tb testing.TB, db *bun.DB, table, column string, value any, id int64) {
	tb.Helper()
	ctx, cancel := fixtureCtx()
	defer cancel()
	_, err := db.NewUpdate().TableExpr(table).Set(column+" = ?", value).Where("id = ?", id).Exec(ctx)
	require.NoError(tb, err, "failed to set %s.%s", table, column)
}

// CreateTestActivityGroupWithLimit inserts an open activity with a
// participant limit.
func CreateTestActivityGroupWithLimit(tb testing.TB, db *bun.DB, name string, maxParticipants int) *activities.Group {
	tb.Helper()
	group := CreateTestActivityGroup(tb, db, name)
	setColumn(tb, db, "activities.groups", "max_participants", maxParticipants, group.ID)
	group.MaxParticipants = maxParticipants
	return group
}

// CreateTestPickupNote inserts a date-specific pickup note.
func CreateTestPickupNote(tb testing.TB, db *bun.DB, studentID int64, date CalendarDate, staffID int64, content string) *schedule.StudentPickupNote {
	tb.Helper()
	ctx, cancel := fixtureCtx()
	defer cancel()

	row := &schedule.StudentPickupNote{StudentID: studentID, NoteDate: schedule.Date(date.String()), Content: content, CreatedBy: staffID}
	row.SetTenantID(fixtureTenantID(tb))
	_, err := db.NewInsert().Model(row).ModelTableExpr(`schedule.student_pickup_notes`).Exec(ctx)
	require.NoError(tb, err, "Failed to create test pickup note")
	return row
}

// CreateTestPickupScheduleWithNote inserts one weekday pickup time with a
// recurring note.
func CreateTestPickupScheduleWithNote(tb testing.TB, db *bun.DB, studentID int64, weekday int, staffID int64, pickupHHMM, note string) *schedule.StudentPickupSchedule {
	tb.Helper()
	row := CreateTestPickupSchedule(tb, db, studentID, weekday, staffID, pickupHHMM)
	setColumn(tb, db, "schedule.student_pickup_schedules", "notes", note, row.ID)
	row.Notes = &note
	return row
}

// NewCalendarDate builds a fixed calendar day for fixture setup.
func NewCalendarDate(year int, month time.Month, day int) CalendarDate {
	return timezone.NewDate(year, month, day)
}

// LatestActiveGroupInRoom returns the newest open session of a room.
func LatestActiveGroupInRoom(tb testing.TB, db *bun.DB, roomID int64) *active.Group {
	tb.Helper()
	ctx, cancel := fixtureCtx()
	defer cancel()

	group := new(active.Group)
	err := db.NewSelect().Model(group).ModelTableExpr(`active.groups AS "group"`).
		Where(`"group".room_id = ?`, roomID).Where(`"group".end_time IS NULL`).
		OrderExpr(`"group".id DESC`).Limit(1).Scan(ctx)
	require.NoError(tb, err, "no open active group in room %d", roomID)
	return group
}

// ActiveGroupLastActivity reads the heartbeat of a session.
func ActiveGroupLastActivity(tb testing.TB, db *bun.DB, activeGroupID int64) time.Time {
	tb.Helper()
	ctx, cancel := fixtureCtx()
	defer cancel()

	var lastActivity time.Time
	err := db.NewSelect().TableExpr("active.groups").Column("last_activity").Where("id = ?", activeGroupID).Scan(ctx, &lastActivity)
	require.NoError(tb, err)
	return lastActivity
}

// CountRoomsNamed counts the fixture tenant's rooms carrying one of the names.
func CountRoomsNamed(tb testing.TB, db *bun.DB, names ...string) int {
	tb.Helper()
	ctx, cancel := fixtureCtx()
	defer cancel()

	var count int
	err := db.NewSelect().TableExpr(`facilities.rooms AS "room"`).ColumnExpr(`COUNT(*)`).
		Where(`"room".tenant_id = ?`, fixtureTenantID(tb)).Where(`"room".name IN (?)`, bun.List(names)).Scan(ctx, &count)
	require.NoError(tb, err)
	return count
}
