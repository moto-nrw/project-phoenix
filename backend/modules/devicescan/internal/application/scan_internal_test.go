package application

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/facilities"
)

func scanCommand(roomID *int64) devicescan.ScanCommand {
	return devicescan.ScanCommand{RFIDTag: testTag, RoomID: roomID}
}

func ptr[T any](value T) *T { return &value }

// classifiedFailure asserts the kind and wire text of a refusal and hands
// the failure back for further assertions.
func classifiedFailure(t *testing.T, err error, kind devicescan.FailureKind, message string) *devicescan.Failure {
	t.Helper()
	failure, ok := devicescan.IsFailure(err)
	require.True(t, ok, "expected a classified failure, got %v", err)
	assert.Equal(t, kind, failure.Kind)
	assert.Equal(t, message, failure.Message)
	return failure
}

func requireFailure(t *testing.T, err error, kind devicescan.FailureKind, message string) {
	t.Helper()
	_ = classifiedFailure(t, err, kind, message)
}

func TestScan_RequiresDevice(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.principals.device = nil

	_, err := h.service().Scan(context.Background(), scanCommand(ptr(testRoomID)))

	require.ErrorIs(t, err, devicescan.ErrDeviceUnauthorized)
	assert.Empty(t, h.visits.recorded)
}

func TestScan_UnknownCardIsRecordedAndNotFound(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	_, err := h.service().Scan(context.Background(), devicescan.ScanCommand{RFIDTag: "UNKNOWN"})

	failure := classifiedFailure(t, err, devicescan.FailureNotFound, devicescan.MessageRFIDTagNotFound)
	assert.Equal(t, devicescan.CodeRFIDTagNotFound, failure.Code)
	require.Len(t, h.fleet.scans, 1)
	assert.Equal(t, "UNKNOWN", h.fleet.scans[0].TagUID)
	require.NotNil(t, h.fleet.scans[0].DeviceID)
	assert.Equal(t, testDeviceID, *h.fleet.scans[0].DeviceID)
}

func TestScan_CardLookupFailureDoesNotLeak(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.people.personErr = errors.New("rfid lookup exploded")

	_, err := h.service().Scan(context.Background(), scanCommand(nil))

	failure := classifiedFailure(t, err, devicescan.FailureInternal, devicescan.MessageInternalServerError)
	assert.ErrorContains(t, failure.Cause, "rfid lookup exploded")
	assert.Empty(t, h.fleet.scans, "a backend failure is not an unregistered card")
}

func TestScan_UnassignedCardIsNotFound(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.people.persons[testTag].HasTag = false

	_, err := h.service().Scan(context.Background(), scanCommand(nil))

	requireFailure(t, err, devicescan.FailureNotFound, devicescan.MessageRFIDTagUnassigned)
}

func TestScan_UnregisteredScanFailureOnlyLogs(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.fleet.scansErr = errBoom

	_, err := h.service().Scan(context.Background(), devicescan.ScanCommand{RFIDTag: "UNKNOWN"})

	requireFailure(t, err, devicescan.FailureNotFound, devicescan.MessageRFIDTagNotFound)
}

func TestScan_PersonNeitherStudentNorStaff(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	delete(h.people.students, testPersonID)

	_, err := h.service().Scan(context.Background(), scanCommand(nil))

	requireFailure(t, err, devicescan.FailureNotFound, devicescan.MessageRFIDTagNotStudentOrStaff)
}

func TestScan_StaffLookupFailureIsNotFound(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	delete(h.people.students, testPersonID)
	h.people.staffErr = errBoom

	_, err := h.service().Scan(context.Background(), scanCommand(nil))

	requireFailure(t, err, devicescan.FailureNotFound, devicescan.MessageRFIDTagNotStudentOrStaff)
}

func TestScan_AlumnusIsTreatedAsUnknownStudent(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.people.students[testPersonID].Alumnus = true

	_, err := h.service().Scan(context.Background(), scanCommand(nil))

	requireFailure(t, err, devicescan.FailureNotFound, devicescan.MessageRFIDTagNotStudentOrStaff)
	assert.Empty(t, h.visits.ended)
}

func TestScan_StudentLookupFailureFallsThroughToStaff(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.people.studentErr = errBoom
	h.people.staff[testPersonID] = &ports.StaffMember{ID: testStaffID}
	h.sessions.current = &ports.Session{ID: 201, RoomName: "Kreativraum", ActivityName: "Basteln"}

	result, err := h.service().Scan(context.Background(), scanCommand(nil))

	require.NoError(t, err)
	assert.Equal(t, devicescan.ScanOutcomeSupervisor, result.Outcome)
}

func TestScan_SupervisorJoinsSession(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	delete(h.people.students, testPersonID)
	h.people.staff[testPersonID] = &ports.StaffMember{ID: testStaffID}
	h.sessions.current = &ports.Session{ID: 201, RoomName: "Kreativraum", ActivityName: "Basteln"}
	h.sessions.supervisors = []ports.Supervisor{{StaffID: 7}, {StaffID: 8, Ended: true}}

	result, err := h.service().Scan(context.Background(), scanCommand(nil))

	require.NoError(t, err)
	assert.Equal(t, devicescan.ScanOutcomeSupervisor, result.Outcome)
	assert.Equal(t, testStaffID, result.PersonID)
	assert.Equal(t, "Max Muster", result.PersonName)
	assert.Equal(t, devicescan.ScanActionSupervisorAuthenticated, result.Action)
	assert.Equal(t, "Kreativraum", result.RoomName)
	assert.Equal(t, "Supervisor authenticated for Basteln", result.Message)
	assert.Equal(t, fixedNow, result.ProcessedAt)
	require.Len(t, h.sessions.replaced, 1)
	assert.Equal(t, []int64{7, testStaffID}, h.sessions.replaced[0], "ended supervisors are dropped, the new one appended")
}

func TestScan_SupervisorScanIsIdempotent(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	delete(h.people.students, testPersonID)
	h.people.staff[testPersonID] = &ports.StaffMember{ID: testStaffID}
	h.sessions.current = &ports.Session{ID: 201, RoomName: "Kreativraum"}
	h.sessions.supervisors = []ports.Supervisor{{StaffID: testStaffID}}

	result, err := h.service().Scan(context.Background(), scanCommand(nil))

	require.NoError(t, err)
	assert.Equal(t, "Supervisor authenticated", result.Message, "no activity name, no suffix")
	assert.Empty(t, h.sessions.replaced)
}

func TestScan_SupervisorWithoutSessionIsNotFound(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	delete(h.people.students, testPersonID)
	h.people.staff[testPersonID] = &ports.StaffMember{ID: testStaffID}

	_, err := h.service().Scan(context.Background(), scanCommand(nil))

	requireFailure(t, err, devicescan.FailureNotFound, devicescan.MessageNoActiveSession)
}

func TestScan_SupervisorWriteFailures(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		arrange func(h *harness)
		message string
	}{
		{"loading supervisors fails", func(h *harness) { h.sessions.supervisorsErr = errBoom }, devicescan.MessageLoadSupervisorsFailed},
		{"replacing supervisors fails", func(h *harness) { h.sessions.replaceErr = errBoom }, devicescan.MessageUpdateSupervisorsFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			delete(h.people.students, testPersonID)
			h.people.staff[testPersonID] = &ports.StaffMember{ID: testStaffID}
			h.sessions.current = &ports.Session{ID: 201}
			tc.arrange(h)

			_, err := h.service().Scan(context.Background(), scanCommand(nil))

			failure := classifiedFailure(t, err, devicescan.FailureInternal, tc.message)
			assert.ErrorIs(t, failure.Cause, errBoom, "the cause stays available for the log")
		})
	}
}

func TestScan_PresenceModeFailureStopsBeforeAnyWrite(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.settings.modeErr = errors.New("presence mode unavailable")

	_, err := h.service().Scan(context.Background(), scanCommand(ptr(testRoomID)))

	requireFailure(t, err, devicescan.FailureInternal, devicescan.MessageInternalServerError)
	assert.Empty(t, h.visits.ended)
	assert.Empty(t, h.visits.recorded)
}

func TestScan_BinaryModeTogglesAttendance(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.settings.mode = presenceModeBinary
	h.principals.staff = &ports.Staff{ID: testStaffID}
	h.attendance.toggleAction = devicescan.ScanActionCheckedIn

	result, err := h.service().Scan(context.Background(), scanCommand(ptr(testRoomID)))

	require.NoError(t, err)
	assert.Equal(t, devicescan.ScanOutcomeAttendance, result.Outcome)
	assert.Equal(t, devicescan.ScanActionCheckedIn, result.Action)
	assert.Equal(t, "Willkommen, Max!", result.Message)
	assert.Equal(t, testStudentID, result.PersonID)
	require.Len(t, h.attendance.toggles, 1)
	assert.Equal(t, toggleCall{StudentID: testStudentID, StaffID: testStaffID, DeviceID: testDeviceID, SkipAuthCheck: true}, h.attendance.toggles[0])
	assert.Empty(t, h.visits.recorded, "binary mode opens no visit")
}

func TestScan_BinaryModeStaysDeviceAttributedWithoutStaff(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.settings.mode = presenceModeBinary
	h.attendance.toggleAction = devicescan.ScanActionCheckedOut

	result, err := h.service().Scan(context.Background(), scanCommand(nil))

	require.NoError(t, err)
	assert.Equal(t, "Tschüss, Max!", result.Message)
	require.Len(t, h.attendance.toggles, 1)
	assert.Zero(t, h.attendance.toggles[0].StaffID)
}

func TestScan_BinaryModeFailures(t *testing.T) {
	t.Parallel()
	t.Run("a departed child is not a student", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.settings.mode = presenceModeBinary
		h.attendance.toggleErr = ports.ErrStudentNotInCare

		_, err := h.service().Scan(context.Background(), scanCommand(nil))

		requireFailure(t, err, devicescan.FailureNotFound, devicescan.MessagePersonNotStudent)
	})
	t.Run("other refusals stay server faults with their text", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.settings.mode = presenceModeBinary
		h.attendance.toggleErr = errors.New("active: ToggleStudentAttendance: session conflict detected")

		_, err := h.service().Scan(context.Background(), scanCommand(nil))

		requireFailure(t, err, devicescan.FailureInternal, "active: ToggleStudentAttendance: session conflict detected")
	})
}

func TestBinaryModeGreeting(t *testing.T) {
	t.Parallel()
	tests := []struct{ action, first, want string }{
		{"checked_in", "Max", "Willkommen, Max!"},
		{"checked_out", "Lena", "Tschüss, Lena!"},
		{"no_action", "Max", "Anwesenheit aktualisiert"},
		{"", "Max", "Anwesenheit aktualisiert"},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.want, binaryModeGreeting(tc.action, tc.first))
	}
}

func TestScan_CheckinOpensVisitInRoomSession(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.settings.feedback = true
	h.presence.roomCounts[testRoomID] = 3

	result, err := h.service().Scan(context.Background(), scanCommand(ptr(testRoomID)))

	require.NoError(t, err)
	assert.Equal(t, devicescan.ScanOutcomeVisit, result.Outcome)
	assert.Equal(t, devicescan.ScanActionCheckedIn, result.Action)
	assert.Equal(t, "Hallo Max!", result.Message)
	assert.Equal(t, "Klassenraum 1a", result.RoomName)
	require.NotNil(t, result.VisitID)
	assert.Equal(t, int64(1001), *result.VisitID)
	assert.Equal(t, []recordedVisit{{StudentID: testStudentID, SessionID: 201}}, h.visits.recorded)
	assert.False(t, result.DailyCheckoutAvailable, "flags only follow a checkout")
	assert.False(t, result.FeedbackEnabled)
	require.NotNil(t, result.ActiveStudents)
	assert.Equal(t, 3, *result.ActiveStudents, "no device session: the whole room counts")
	assert.Equal(t, fixedNow, result.ProcessedAt)
}

func TestScan_CheckoutClosesCurrentVisit(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.settings.feedback = true
	h.visits.current = &ports.CurrentVisit{ID: 55, EntryTime: fixedNow.Add(-time.Hour), Session: &ports.SessionRef{ID: 201, RoomID: testRoomID, StartTime: fixedNow.Add(-time.Hour), Room: &ports.Room{ID: testRoomID, Name: "Klassenraum 1a"}}}

	result, err := h.service().Scan(context.Background(), scanCommand(nil))

	require.NoError(t, err)
	assert.Equal(t, devicescan.ScanActionCheckedOut, result.Action)
	assert.Equal(t, "Tschüss Max!", result.Message)
	assert.Equal(t, []int64{55}, h.visits.ended)
	require.NotNil(t, result.VisitID)
	assert.Equal(t, int64(55), *result.VisitID)
	assert.Equal(t, "Klassenraum 1a", result.PreviousRoomName)
	assert.True(t, result.FeedbackEnabled, "feedback follows a checkout when the tenant enabled it")
	assert.Nil(t, result.ActiveStudents, "no room requested, no count")
}

func TestScan_TransferMovesBetweenRooms(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.visits.current = &ports.CurrentVisit{ID: 55, EntryTime: fixedNow.Add(-time.Hour), Session: &ports.SessionRef{ID: 150, RoomID: 7, StartTime: fixedNow.Add(-time.Hour), Room: &ports.Room{ID: 7, Name: "Musikraum"}}}

	result, err := h.service().Scan(context.Background(), scanCommand(ptr(testRoomID)))

	require.NoError(t, err)
	assert.Equal(t, devicescan.ScanActionTransferred, result.Action)
	assert.Equal(t, "Gewechselt von Musikraum zu Klassenraum 1a!", result.Message)
	assert.Equal(t, "Musikraum", result.PreviousRoomName)
	assert.Equal(t, []int64{55}, h.visits.ended)
	require.Len(t, h.visits.recorded, 1)
}

func TestScan_SameRoomScanOnlyChecksOut(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.visits.current = &ports.CurrentVisit{ID: 55, EntryTime: fixedNow.Add(-time.Hour), Session: &ports.SessionRef{ID: 201, RoomID: testRoomID, StartTime: fixedNow.Add(-time.Hour), Room: &ports.Room{ID: testRoomID, Name: "Klassenraum 1a"}}}

	result, err := h.service().Scan(context.Background(), scanCommand(ptr(testRoomID)))

	require.NoError(t, err)
	assert.Equal(t, devicescan.ScanActionCheckedOut, result.Action)
	assert.Equal(t, "Klassenraum 1a", result.RoomName)
	assert.Empty(t, h.visits.recorded, "the same room is a checkout, not a re-entry")
}

func TestScan_PreviousDayVisitInSameRoomReenters(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	yesterday := fixedNow.AddDate(0, 0, -1)
	h.visits.current = &ports.CurrentVisit{ID: 55, EntryTime: yesterday, Session: &ports.SessionRef{ID: 150, RoomID: testRoomID, StartTime: yesterday, Room: &ports.Room{ID: testRoomID, Name: "Klassenraum 1a"}}}

	result, err := h.service().Scan(context.Background(), scanCommand(ptr(testRoomID)))

	require.NoError(t, err)
	assert.Equal(t, devicescan.ScanActionCheckedIn, result.Action, "a rollover recovery continues into today's session")
	require.Len(t, h.visits.recorded, 1)
}

func TestScan_NoRoomAndNoVisitIsInvalid(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	_, err := h.service().Scan(context.Background(), scanCommand(nil))

	requireFailure(t, err, devicescan.FailureInvalidRequest, devicescan.MessageRoomIDRequired)
}

func TestScan_VisitReadFailureDoesNotCreateVisit(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.visits.currentErr = errors.New("presence lookup unavailable")

	_, err := h.service().Scan(context.Background(), scanCommand(ptr(testRoomID)))

	requireFailure(t, err, devicescan.FailureInternal, devicescan.MessageInternalServerError)
	assert.Empty(t, h.visits.recorded)
	assert.Empty(t, h.visits.ended)
}

func TestScan_EndVisitFailure(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.visits.current = &ports.CurrentVisit{ID: 55, Session: &ports.SessionRef{ID: 201, RoomID: testRoomID}}
	h.visits.endErr = errBoom

	_, err := h.service().Scan(context.Background(), scanCommand(nil))

	requireFailure(t, err, devicescan.FailureInternal, devicescan.MessageEndVisitFailed)
}

func TestScan_NoSessionInRoomIsNotFound(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.sessions.open = nil

	_, err := h.service().Scan(context.Background(), scanCommand(ptr(testRoomID)))

	requireFailure(t, err, devicescan.FailureNotFound, devicescan.MessageNoGroupsInRoom)
}

func TestScan_RoomCapacityConflictRollsBackSourceCheckout(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.rooms.rooms[testRoomID] = facilities.Room{ID: testRoomID, Name: "Klassenraum 1a", Capacity: ptr(1)}
	h.presence.roomCounts[testRoomID] = 1
	h.visits.current = &ports.CurrentVisit{ID: 55, Session: &ports.SessionRef{ID: 150, RoomID: 7, Room: &ports.Room{ID: 7, Name: "Musikraum"}}}

	_, err := h.service().Scan(context.Background(), scanCommand(ptr(testRoomID)))

	conflict, ok := errors.AsType[*devicescan.RoomCapacityExceededError](err)
	require.True(t, ok, "got %v", err)
	assert.Equal(t, int64(testRoomID), conflict.RoomID)
	assert.Equal(t, 1, conflict.CurrentOccupancy)
	assert.Equal(t, 1, conflict.MaxCapacity)
	assert.True(t, conflict.Details, "the room disclosure defaults to shown")
	assert.Equal(t, []int64{55}, h.visits.ended, "the source checkout ran first")
	assert.Equal(t, 1, h.unit.rollbacks, "and is undone with the request transaction")
}

func TestScan_CapacityConflictWithoutCheckoutKeepsTransaction(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.rooms.rooms[testRoomID] = facilities.Room{ID: testRoomID, Name: "Klassenraum 1a", Capacity: ptr(1)}
	h.presence.roomCounts[testRoomID] = 1

	_, err := h.service().Scan(context.Background(), scanCommand(ptr(testRoomID)))

	_, ok := errors.AsType[*devicescan.RoomCapacityExceededError](err)
	require.True(t, ok)
	assert.Zero(t, h.unit.rollbacks)
}

func TestScan_ActivityCapacityConflict(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.sessions.open[0].Activity.MaxParticipants = 1
	h.presence.groupCounts[201] = 1
	h.settings.activityDetails = true

	_, err := h.service().Scan(context.Background(), scanCommand(ptr(testRoomID)))

	conflict, ok := errors.AsType[*devicescan.ActivityCapacityExceededError](err)
	require.True(t, ok, "got %v", err)
	assert.Equal(t, "Hausaufgaben", conflict.ActivityName)
	assert.True(t, conflict.Details)
	assert.Empty(t, h.visits.recorded)
}

func TestScan_CapacityDetailDisclosureFailsSafe(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.settings.detailsErr = errBoom
	h.sessions.open[0].Activity.MaxParticipants = 1
	h.presence.groupCounts[201] = 1

	_, err := h.service().Scan(context.Background(), scanCommand(ptr(testRoomID)))

	conflict, ok := errors.AsType[*devicescan.ActivityCapacityExceededError](err)
	require.True(t, ok)
	assert.False(t, conflict.Details, "activity details default to hidden")
}

func TestScan_DuplicateVisitCarriesExistingStay(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.visits.recordErr = ports.ErrStudentAlreadyActive
	entry := fixedNow.Add(-5 * time.Minute)
	h.visits.current = &ports.CurrentVisit{ID: 66, EntryTime: entry, Session: &ports.SessionRef{ID: 150, RoomID: 7, Room: &ports.Room{ID: 7, Name: "Conflict Room A"}}}
	h.visits.endErr = nil

	_, err := h.service().Scan(context.Background(), scanCommand(ptr(testRoomID)))

	conflict, ok := errors.AsType[*devicescan.StudentAlreadyActiveError](err)
	require.True(t, ok, "got %v", err)
	assert.Equal(t, testStudentID, conflict.StudentID)
	assert.Equal(t, int64(66), conflict.ExistingVisitID)
	require.NotNil(t, conflict.EntryTime)
	assert.Equal(t, entry, *conflict.EntryTime)
	require.NotNil(t, conflict.RoomID)
	assert.Equal(t, int64(7), *conflict.RoomID)
	assert.Equal(t, "Conflict Room A", conflict.RoomName)
}

func TestScan_DuplicateVisitDegradesWithoutExistingStay(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.visits.recordErr = ports.ErrStudentAlreadyActive

	_, err := h.service().Scan(context.Background(), scanCommand(ptr(testRoomID)))

	conflict, ok := errors.AsType[*devicescan.StudentAlreadyActiveError](err)
	require.True(t, ok)
	assert.Equal(t, devicescan.StudentAlreadyActiveError{StudentID: testStudentID}, *conflict)
}

func TestScan_GraduatedRaceIsNotAStudent(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.visits.recordErr = ports.ErrStudentNotInCare

	_, err := h.service().Scan(context.Background(), scanCommand(ptr(testRoomID)))

	requireFailure(t, err, devicescan.FailureNotFound, devicescan.MessagePersonNotStudent)
}

func TestScan_RecordFailureIsPinnedText(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.visits.recordErr = errBoom

	_, err := h.service().Scan(context.Background(), scanCommand(ptr(testRoomID)))

	requireFailure(t, err, devicescan.FailureInternal, devicescan.MessageCreateVisitFailed)
}

func TestScan_ActiveStudentsScopedToDeviceSession(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	deviceID := testDeviceID
	h.sessions.open = append(h.sessions.open, ports.Session{ID: 202, RoomID: testRoomID, StartTime: fixedNow, DeviceID: &deviceID, Activity: &ports.Activity{ID: 302, Name: "Device Session"}})
	h.presence.groupCounts[202] = 1
	h.presence.roomCounts[testRoomID] = 3

	result, err := h.service().Scan(context.Background(), scanCommand(ptr(testRoomID)))

	require.NoError(t, err)
	assert.Equal(t, []recordedVisit{{StudentID: testStudentID, SessionID: 202}}, h.visits.recorded, "the device's own session wins over the newest")
	require.NotNil(t, result.ActiveStudents)
	assert.Equal(t, 1, *result.ActiveStudents)
	assert.Equal(t, []int64{202}, h.sessions.touched)
}

func TestScan_ActiveStudentsFallsBackToDeviceSessionLookup(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.sessions.deviceSession = &ports.Session{ID: 250}
	h.presence.groupCounts[250] = 4

	result, err := h.service().Scan(context.Background(), scanCommand(ptr(testRoomID)))

	require.NoError(t, err)
	require.NotNil(t, result.ActiveStudents)
	assert.Equal(t, 4, *result.ActiveStudents)
	assert.Equal(t, []int64{250}, h.sessions.touched)
}

// The session heartbeat after a check-in is device-location bookkeeping:
// its failure only logs and never turns a recorded visit into an error.
func TestScan_SessionHeartbeatFailureIsBestEffort(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.sessions.deviceSession = &ports.Session{ID: 250}
	h.sessions.touchErr = errBoom

	result, err := h.service().Scan(context.Background(), scanCommand(ptr(testRoomID)))

	require.NoError(t, err)
	assert.Equal(t, devicescan.ScanActionCheckedIn, result.Action)
	assert.Equal(t, []recordedVisit{{StudentID: testStudentID, SessionID: 201}}, h.visits.recorded)
	assert.Equal(t, []int64{250}, h.sessions.touched, "the heartbeat was attempted")
}

func TestScan_ActiveStudentsCountFailuresAreOmitted(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.presence.roomErr = errBoom
	h.sessions.deviceErr = errBoom

	result, err := h.service().Scan(context.Background(), scanCommand(ptr(testRoomID)))

	require.NoError(t, err)
	assert.Nil(t, result.ActiveStudents)
}

func TestScan_PickupTimeIsBestEffort(t *testing.T) {
	t.Parallel()
	t.Run("formatted when known", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.pickups.pickup = &ports.Pickup{Time: ptr(time.Date(2000, 1, 1, 15, 30, 0, 0, time.UTC))}

		result, err := h.service().Scan(context.Background(), scanCommand(ptr(testRoomID)))

		require.NoError(t, err)
		require.NotNil(t, result.PickupTime)
		assert.Equal(t, "15:30", *result.PickupTime)
	})
	t.Run("a lookup failure is skipped", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.pickups.err = errBoom

		result, err := h.service().Scan(context.Background(), scanCommand(ptr(testRoomID)))

		require.NoError(t, err)
		assert.Nil(t, result.PickupTime)
	})
}

func TestScan_CheckoutOffersHomeAndUpgradesFromOwnRoom(t *testing.T) {
	t.Parallel()
	groupRoom := int64(42)
	h := newHarness(t)
	h.people.students[testPersonID].GroupID = ptr(int64(1))
	h.groups.group = &ports.Group{ID: 1, Name: "Hausaufgaben", RoomID: &groupRoom}
	h.visits.current = &ports.CurrentVisit{ID: 55, Session: &ports.SessionRef{ID: 201, RoomID: groupRoom, StartTime: fixedNow.Add(-time.Hour), Room: &ports.Room{ID: groupRoom, Name: "Klassenraum 1a"}}}

	result, err := h.service().Scan(context.Background(), scanCommand(nil))

	require.NoError(t, err)
	assert.Equal(t, devicescan.ScanActionCheckedOutDaily, result.Action, "leaving the own group room sends the child home")
	assert.True(t, result.DailyCheckoutAvailable)
}

func TestScan_LogsOmitStudentNameAndGreeting(t *testing.T) {
	t.Parallel()
	const firstName, lastName = "Loggingprobe", "Namenspruefung"
	var logs bytes.Buffer
	h := newHarness(t)
	h.logger = slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelInfo}))
	h.people.persons[testTag] = &ports.Person{ID: testPersonID, FirstName: firstName, LastName: lastName, HasTag: true}

	result, err := h.service().Scan(context.Background(), scanCommand(ptr(testRoomID)))

	require.NoError(t, err)
	assert.Equal(t, "Hallo "+firstName+"!", result.Message, "the kiosk keeps the personalized greeting")
	out := logs.String()
	require.Contains(t, out, `"msg":"checkin complete"`)
	assert.Contains(t, out, `"action":"checked_in"`)
	for _, forbidden := range []string{firstName, lastName, "Hallo ", "Tschüss ", `"message"`, `"class"`} {
		assert.NotContainsf(t, out, forbidden, "Info logs must not carry %q:\n%s", forbidden, out)
	}
}

func TestScan_BinaryLogsOmitStudentNameAndGreeting(t *testing.T) {
	t.Parallel()
	const firstName, lastName = "Loggingprobe", "Namenspruefung"
	var logs bytes.Buffer
	h := newHarness(t)
	h.logger = slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelInfo}))
	h.settings.mode = presenceModeBinary
	h.attendance.toggleAction = devicescan.ScanActionCheckedIn
	h.people.persons[testTag] = &ports.Person{ID: testPersonID, FirstName: firstName, LastName: lastName, HasTag: true}

	result, err := h.service().Scan(context.Background(), scanCommand(nil))

	require.NoError(t, err)
	assert.Equal(t, "Willkommen, "+firstName+"!", result.Message)
	out := logs.String()
	require.Contains(t, out, `"msg":"binary mode checkin complete"`)
	for _, forbidden := range []string{firstName, lastName, "Willkommen", "Tschüss", `"message"`} {
		assert.NotContainsf(t, out, forbidden, "Info logs must not carry %q:\n%s", forbidden, out)
	}
}

func TestPingAndStatus(t *testing.T) {
	t.Parallel()
	t.Run("ping records the heartbeat and reports the session", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.sessions.current = &ports.Session{ID: 201}

		ping, err := h.service().Ping(context.Background())

		require.NoError(t, err)
		assert.Equal(t, "dev-001", ping.Device.DeviceID)
		assert.True(t, ping.IsOnline)
		assert.True(t, ping.SessionActive)
		assert.Equal(t, fixedNow, ping.PingTime)
		assert.Equal(t, []int64{testDeviceID}, h.fleet.seen)
		assert.Equal(t, []int64{201}, h.sessions.touched)
	})
	t.Run("a failed session refresh still reports the session", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.sessions.current = &ports.Session{ID: 201}
		h.sessions.touchErr = errBoom

		ping, err := h.service().Ping(context.Background())

		require.NoError(t, err)
		assert.True(t, ping.SessionActive)
	})
	t.Run("ping without a device is unauthorized", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.principals.device = nil

		_, err := h.service().Ping(context.Background())

		require.ErrorIs(t, err, devicescan.ErrDeviceUnauthorized)
	})
	t.Run("ping of a vanished device is not found", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		delete(h.fleet.devices, "dev-001")

		_, err := h.service().Ping(context.Background())

		requireFailure(t, err, devicescan.FailureInternal, "IoT service error in PingDevice: device not found")
	})
	t.Run("status reports the device", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)

		status, err := h.service().Status(context.Background())

		require.NoError(t, err)
		assert.Equal(t, testDeviceID, status.Device.ID)
		assert.True(t, status.Device.Active)
		assert.True(t, status.IsOnline)
		assert.Equal(t, fixedNow, status.AuthenticatedAt)
	})
}
