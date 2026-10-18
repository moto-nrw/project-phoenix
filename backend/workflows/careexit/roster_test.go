package careexit_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/workflows/careexit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type rosterTimetable struct {
	planned       []timetable.CareExitRosterRow
	restored      []timetable.CareExitRosterRow
	restoreInput  []timetable.CareExitRosterRow
	participants  []timetable.InstanceStudent
	studentFilter timetable.InstanceStudentFilter
}

func (t *rosterTimetable) LockPlannedRosterForCareExit(context.Context, []int64, string) error {
	return nil
}

func (t *rosterTimetable) PreviewPlannedRosterForCareExit(context.Context, []int64, string) ([]timetable.CareExitRosterRow, error) {
	return t.planned, nil
}

func (t *rosterTimetable) RemovePlannedRosterForCareExit(context.Context, []int64, string) ([]timetable.CareExitRosterRow, error) {
	return t.planned, nil
}

func (t *rosterTimetable) RestoreRosterForCareExit(_ context.Context, _ []int64, rows []timetable.CareExitRosterRow) ([]timetable.CareExitRosterRow, error) {
	t.restoreInput = rows
	return t.restored, nil
}

func (t *rosterTimetable) CountPlannedRosterForCareExit(context.Context, []int64, string, []timetable.CareExitRosterRow) (map[int64]int, error) {
	return map[int64]int{}, nil
}

func (t *rosterTimetable) ListInstanceStudents(_ context.Context, filter timetable.InstanceStudentFilter) ([]timetable.InstanceStudent, error) {
	t.studentFilter = filter
	return t.participants, nil
}

type rosterPresence struct {
	attendance []studentpresence.SessionAttendance
	restored   []studentpresence.SessionAttendanceRestore
	locked     []int64
	links      []studentpresence.ParticipantPickupException
}

func (p *rosterPresence) ListSessionAttendance(context.Context, []int64) ([]studentpresence.SessionAttendance, error) {
	return p.attendance, nil
}

func (p *rosterPresence) RestoreSessionAttendance(_ context.Context, rows []studentpresence.SessionAttendanceRestore) error {
	p.restored = rows
	return nil
}

func (p *rosterPresence) LockSessionAttendance(_ context.Context, ids []int64) error {
	p.locked = ids
	return nil
}

func (p *rosterPresence) CloseOpenParticipants(_ context.Context, ids []int64, _ time.Time) (int64, error) {
	return int64(len(ids)), nil
}

func (p *rosterPresence) ReconnectParticipantPickupExceptions(_ context.Context, links []studentpresence.ParticipantPickupException) error {
	p.links = links
	return nil
}

type rosterAttendance struct {
	open   []careexit.OpenParticipant
	latest *calendar.Date
}

func (a rosterAttendance) OpenParticipants(context.Context, []int64) ([]careexit.OpenParticipant, error) {
	return a.open, nil
}

func (a rosterAttendance) LatestAttendedBlockDate(context.Context, int64) (*calendar.Date, error) {
	return a.latest, nil
}

func TestRosterRemovalKeepsTheAttendanceAndDefaultsToExpected(t *testing.T) {
	t.Parallel()
	room := int64(70)
	tt := &rosterTimetable{planned: []timetable.CareExitRosterRow{
		{ParticipantID: 31, TenantID: 5, StudentID: 3, InstanceID: 301, RoomID: &room},
		{ParticipantID: 32, TenantID: 5, StudentID: 3, InstanceID: 302},
	}}
	note := "krank"
	presence := &rosterPresence{attendance: []studentpresence.SessionAttendance{
		{ParticipantID: 31, Status: studentpresence.SessionAttendanceAbsent, Note: &note, IsUnplanned: true},
	}}
	roster := careexit.NewRoster(tt, presence, rosterAttendance{})

	removals, err := roster.RemovePlannedRosterForCareExit(context.Background(), []int64{3}, calendar.Date("2026-10-19"))
	require.NoError(t, err)
	require.Len(t, removals, 2)
	first := removals[0]
	assert.Equal(t, careplan.CareExitRemovalRoster, first.Kind)
	assert.Equal(t, int64(301), *first.InstanceID)
	assert.Equal(t, &room, first.RoomID)
	assert.Equal(t, studentpresence.SessionAttendanceAbsent, *first.Status)
	assert.Equal(t, &note, first.Note)
	assert.True(t, *first.IsUnplanned)
	assert.Equal(t, studentpresence.SessionAttendanceExpected, *removals[1].Status, "a participant without a row was expected")
	assert.False(t, *removals[1].IsUnplanned)
}

func TestRosterRestoreReplaysOnlyAttendanceThatDiffers(t *testing.T) {
	t.Parallel()
	absent, expected := studentpresence.SessionAttendanceAbsent, studentpresence.SessionAttendanceExpected
	instanceA, instanceB, instanceC := int64(401), int64(402), int64(403)
	removals := []careplan.CareExitRemoval{
		{TenantID: 5, StudentID: 4, Kind: careplan.CareExitRemovalRoster, InstanceID: &instanceA, Status: &absent},
		{TenantID: 5, StudentID: 4, Kind: careplan.CareExitRemovalRoster, InstanceID: &instanceB, Status: &expected},
		{TenantID: 5, StudentID: 4, Kind: "enrollment", InstanceID: &instanceC, Status: &absent},
	}
	tt := &rosterTimetable{restored: []timetable.CareExitRosterRow{
		{ParticipantID: 51, StudentID: 4, InstanceID: instanceA},
		{ParticipantID: 52, StudentID: 4, InstanceID: instanceB},
	}}
	presence := &rosterPresence{}
	roster := careexit.NewRoster(tt, presence, rosterAttendance{})

	restored, err := roster.RestoreRosterForCareExit(context.Background(), []int64{4}, removals)
	require.NoError(t, err)
	assert.Equal(t, 2, restored)
	assert.Len(t, tt.restoreInput, 2, "only roster entries of the ledger go back to the timetable")
	require.Len(t, presence.restored, 1, "expected attendance is not replayed")
	assert.Equal(t, studentpresence.SessionAttendanceRestore{ParticipantID: 51, Status: absent}, presence.restored[0])
}

func TestRosterReconnectsOnlyStillValidPickupExceptions(t *testing.T) {
	t.Parallel()
	instanceA, instanceB := int64(601), int64(602)
	kept, dropped := int64(71), int64(72)
	removals := []careplan.CareExitRemoval{
		{StudentID: 6, Kind: careplan.CareExitRemovalRoster, InstanceID: &instanceA, PickupExceptionID: &kept},
		{StudentID: 6, Kind: careplan.CareExitRemovalRoster, InstanceID: &instanceB, PickupExceptionID: &dropped},
	}
	tt := &rosterTimetable{participants: []timetable.InstanceStudent{
		{ID: 81, StudentID: 6, InstanceID: instanceA}, {ID: 82, StudentID: 6, InstanceID: instanceB},
	}}
	presence := &rosterPresence{}
	roster := careexit.NewRoster(tt, presence, rosterAttendance{})

	require.NoError(t, roster.ReconnectCareExitAssignmentPickupExceptions(context.Background(), []int64{6}, []int64{kept}, removals))
	assert.Equal(t, []int64{instanceA}, tt.studentFilter.InstanceIDs)
	assert.Equal(t, []studentpresence.ParticipantPickupException{{ParticipantID: 81, PickupExceptionID: kept}}, presence.links)

	presence.links = nil
	require.NoError(t, roster.ReconnectCareExitAssignmentPickupExceptions(context.Background(), []int64{6}, nil, removals))
	assert.Nil(t, presence.links, "without a valid excusal nothing is reconnected")
}

func TestRosterOpenAssignmentsAndLatestAttendance(t *testing.T) {
	t.Parallel()
	day := calendar.Date("2026-10-16")
	presence := &rosterPresence{}
	roster := careexit.NewRoster(&rosterTimetable{}, presence, rosterAttendance{
		open:   []careexit.OpenParticipant{{ParticipantID: 91, StudentID: 9}, {ParticipantID: 92, StudentID: 9}, {ParticipantID: 93, StudentID: 8}},
		latest: &day,
	})

	students, err := roster.ListOpenStudentAssignments(context.Background(), []int64{8, 9})
	require.NoError(t, err)
	assert.Equal(t, []int64{9, 8}, students, "every child once, in participant order")
	require.NoError(t, roster.LockOpenStudentAssignments(context.Background(), []int64{8, 9}))
	assert.Equal(t, []int64{91, 92, 93}, presence.locked)
	closed, err := roster.CloseOpenStudentAssignments(context.Background(), []int64{8, 9}, time.Now())
	require.NoError(t, err)
	assert.EqualValues(t, 3, closed)
	latest, err := roster.LatestStudentAssignmentAttendanceDate(context.Background(), 9)
	require.NoError(t, err)
	assert.Equal(t, day, *latest)

	invalid := calendar.Date("16.10.2026")
	_, err = careexit.NewRoster(&rosterTimetable{}, presence, rosterAttendance{latest: &invalid}).LatestStudentAssignmentAttendanceDate(context.Background(), 9)
	require.EqualError(t, err, `care exit: invalid calendar day "16.10.2026"`)
}

func TestRosterRequiresEveryOwner(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { careexit.NewRoster(nil, &rosterPresence{}, rosterAttendance{}) })
	assert.Panics(t, func() { careexit.NewRoster(&rosterTimetable{}, nil, rosterAttendance{}) })
	assert.Panics(t, func() { careexit.NewRoster(&rosterTimetable{}, &rosterPresence{}, nil) })
}
