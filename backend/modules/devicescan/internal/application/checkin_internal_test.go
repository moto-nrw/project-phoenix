package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/facilities"
)

func TestShouldSkipCheckin(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	s := h.service()
	sameRoom := currentVisitCheckout{CheckedOut: true, Visit: &ports.CurrentVisit{Session: &ports.SessionRef{RoomID: 1, StartTime: fixedNow}}}

	assert.False(t, s.shouldSkipCheckin(nil, sameRoom, fixedNow), "no room requested")
	assert.False(t, s.shouldSkipCheckin(ptr(int64(1)), currentVisitCheckout{Visit: sameRoom.Visit}, fixedNow), "nothing was checked out")
	assert.False(t, s.shouldSkipCheckin(ptr(int64(1)), currentVisitCheckout{CheckedOut: true}, fixedNow), "no visit")
	assert.False(t, s.shouldSkipCheckin(ptr(int64(1)), currentVisitCheckout{CheckedOut: true, Visit: &ports.CurrentVisit{}}, fixedNow), "no session")
	assert.True(t, s.shouldSkipCheckin(ptr(int64(1)), sameRoom, fixedNow), "same room today")
	assert.False(t, s.shouldSkipCheckin(ptr(int64(2)), sameRoom, fixedNow), "different room")
	yesterday := currentVisitCheckout{CheckedOut: true, Visit: &ports.CurrentVisit{Session: &ports.SessionRef{RoomID: 1, StartTime: fixedNow.AddDate(0, 0, -1)}}}
	assert.False(t, s.shouldSkipCheckin(ptr(int64(1)), yesterday, fixedNow), "yesterday's session is a rollover recovery")
}

func TestBuildScanResult(t *testing.T) {
	t.Parallel()
	person := &ports.Person{FirstName: "Max"}
	newVisit, oldVisit := int64(123), int64(100)

	transfer := buildScanResult(person, currentVisitCheckout{CheckedOut: true, VisitID: &oldVisit, PreviousRoomName: "Room A"}, &checkinResult{NewVisitID: &newVisit, RoomName: "Room B"})
	assert.Equal(t, devicescan.ScanActionTransferred, transfer.Action)
	assert.Equal(t, "Gewechselt von Room A zu Room B!", transfer.Message)
	assert.Equal(t, &newVisit, transfer.VisitID)
	assert.Equal(t, "Room B", transfer.RoomName)
	assert.Equal(t, "Room A", transfer.PreviousRoomName)

	same := buildScanResult(person, currentVisitCheckout{CheckedOut: true, VisitID: &oldVisit, PreviousRoomName: "Room A"}, &checkinResult{NewVisitID: &newVisit, RoomName: "Room A"})
	assert.Equal(t, devicescan.ScanActionCheckedIn, same.Action)
	assert.Equal(t, "Hallo Max!", same.Message)

	unknownPrevious := buildScanResult(person, currentVisitCheckout{CheckedOut: true, VisitID: &oldVisit}, &checkinResult{NewVisitID: &newVisit, RoomName: "Room B"})
	assert.Equal(t, devicescan.ScanActionCheckedIn, unknownPrevious.Action)

	checkout := buildScanResult(person, currentVisitCheckout{CheckedOut: true, VisitID: &oldVisit}, &checkinResult{RoomName: "Room A"})
	assert.Equal(t, devicescan.ScanActionCheckedOut, checkout.Action)
	assert.Equal(t, "Tschüss Max!", checkout.Message)
	assert.Equal(t, &oldVisit, checkout.VisitID)

	checkin := buildScanResult(person, currentVisitCheckout{}, &checkinResult{NewVisitID: &newVisit, RoomName: "Room A"})
	assert.Equal(t, devicescan.ScanActionCheckedIn, checkin.Action)
	assert.Equal(t, &newVisit, checkin.VisitID)

	nothing := buildScanResult(person, currentVisitCheckout{}, &checkinResult{})
	assert.Empty(t, nothing.Action)
	assert.Empty(t, nothing.Message)
}

func TestRoomNameForResponse(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.rooms.rooms[7] = facilities.Room{ID: 7, Name: "Lookup Room"}
	s := h.service()
	ctx := context.Background()

	withRoom := &ports.CurrentVisit{Session: &ports.SessionRef{Room: &ports.Room{Name: "Library"}}}
	assert.Equal(t, "Library", s.roomNameForResponse(ctx, withRoom, nil))
	assert.Equal(t, "", s.roomNameForResponse(ctx, nil, nil))
	assert.Equal(t, "Lookup Room", s.roomNameForResponse(ctx, &ports.CurrentVisit{Session: &ports.SessionRef{}}, ptr(int64(7))))
	assert.Equal(t, "Room 999999", s.roomNameForResponse(ctx, nil, ptr(int64(999999))))
}

func TestProcessStudentCheckin(t *testing.T) {
	t.Parallel()
	student := &ports.Student{ID: 1}
	person := &ports.Person{FirstName: "Test"}

	t.Run("no room and nothing checked out is invalid", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		_, err := h.service().processStudentCheckin(context.Background(), student, person, checkinInput{})
		requireFailure(t, err, devicescan.FailureInvalidRequest, devicescan.MessageRoomIDRequired)
	})
	t.Run("a skipped check-in only names the room", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		visit := &ports.CurrentVisit{Session: &ports.SessionRef{Room: &ports.Room{Name: "SkipCheckinRoom"}}}
		result, err := h.service().processStudentCheckin(context.Background(), student, person, checkinInput{
			RoomID: ptr(int64(1)), SkipCheckin: true, Checkout: currentVisitCheckout{CheckedOut: true, Visit: visit},
		})
		require.NoError(t, err)
		assert.Equal(t, "SkipCheckinRoom", result.RoomName)
		assert.Nil(t, result.NewVisitID)
		assert.Empty(t, h.visits.recorded)
	})
	t.Run("a room lookup failure is the pinned text", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.rooms.findErr = errBoom
		_, err := h.service().processStudentCheckin(context.Background(), student, person, checkinInput{RoomID: ptr(testRoomID)})
		requireFailure(t, err, devicescan.FailureInternal, devicescan.MessageGetRoomFailed)
	})
	t.Run("a session lookup failure is the pinned text", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.sessions.listErr = errBoom
		_, err := h.service().processStudentCheckin(context.Background(), student, person, checkinInput{RoomID: ptr(testRoomID)})
		requireFailure(t, err, devicescan.FailureInternal, devicescan.MessageFindActiveGroupsFailed)
	})
	t.Run("a room occupancy failure is the pinned text", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.rooms.rooms[testRoomID] = facilities.Room{ID: testRoomID, Name: "Klassenraum 1a", Capacity: ptr(5)}
		h.presence.roomErr = errBoom
		_, err := h.service().processStudentCheckin(context.Background(), student, person, checkinInput{RoomID: ptr(testRoomID)})
		requireFailure(t, err, devicescan.FailureInternal, devicescan.MessageCheckRoomCapacityFailed)
	})
}

func TestCheckActivityCapacity(t *testing.T) {
	t.Parallel()
	t.Run("an unlimited activity passes without counting", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.presence.groupErr = errBoom
		require.NoError(t, h.service().checkActivityCapacity(context.Background(), &ports.Session{Activity: &ports.Activity{Name: "Sporthalle"}}))
	})
	t.Run("a template is loaded when the session did not carry it", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.activities.byID[301] = &ports.Activity{ID: 301, Name: "Hausaufgaben", MaxParticipants: 1}
		h.presence.groupCounts[201] = 1
		err := h.service().checkActivityCapacity(context.Background(), &ports.Session{ID: 201, TemplateID: ptr(int64(301))})
		_, ok := errors.AsType[*devicescan.ActivityCapacityExceededError](err)
		assert.True(t, ok, "got %v", err)
	})
	t.Run("a spontaneous session is unsupported", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		err := h.service().checkActivityCapacity(context.Background(), &ports.Session{ID: 201})
		requireFailure(t, err, devicescan.FailureInternal, devicescan.MessageSpontaneousUnsupported)
	})
	t.Run("a template lookup failure is the pinned text", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.activities.findErr = errBoom
		err := h.service().checkActivityCapacity(context.Background(), &ports.Session{ID: 201, TemplateID: ptr(int64(301))})
		requireFailure(t, err, devicescan.FailureInternal, devicescan.MessageGetActivityFailed)
	})
	t.Run("an occupancy failure is the pinned text", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.presence.groupErr = errBoom
		err := h.service().checkActivityCapacity(context.Background(), &ports.Session{ID: 201, Activity: &ports.Activity{MaxParticipants: 3}})
		requireFailure(t, err, devicescan.FailureInternal, devicescan.MessageCheckActivityCapFailed)
	})
}

// Session selection in a room: device-linked sessions win, otherwise the
// newest; the special rooms close yesterday's session before choosing.

func TestUseExistingSession_CannotSelectPreviousDaySession(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	s := h.service()
	room := facilities.Room{ID: 42, Name: "Schulhof"}
	deviceID := testDeviceID
	stale := ports.Session{ID: 100, RoomID: room.ID, DeviceID: &deviceID, StartTime: fixedNow.AddDate(0, 0, -1)}
	current := ports.Session{ID: 101, RoomID: room.ID, StartTime: fixedNow.Add(-time.Hour)}

	selection := s.useExistingSession(context.Background(), s.sessionsStartedToday([]ports.Session{stale, current}, fixedNow), room, deviceID)

	assert.Equal(t, current.ID, selection.Session.ID)
	assert.False(t, selection.DeviceScoped, "yesterday's device match must not win")
}

func TestEndPreviousDaySessions(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	endedAt := fixedNow.Add(-time.Hour)
	sessions := []ports.Session{
		{ID: 100, StartTime: fixedNow.AddDate(0, 0, -1)},
		{ID: 101, StartTime: fixedNow.Add(-time.Hour)},
		{ID: 102, StartTime: fixedNow.AddDate(0, 0, -2), EndTime: &endedAt},
		{ID: 103, StartTime: fixedNow.AddDate(0, 0, 1)},
	}

	require.NoError(t, h.service().endPreviousDaySessions(context.Background(), sessions, fixedNow))
	assert.Equal(t, []int64{100}, h.sessions.ended)
}

func TestFindOrCreateSession_SpecialRoomEndsStaleSessionBeforeCreate(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	room := facilities.Room{ID: 42, Name: facilities.WCRoomName}
	h.sessions.open = []ports.Session{{ID: 100, RoomID: room.ID, StartTime: fixedNow.AddDate(0, 0, -1)}}
	h.rooms.toilet = &room
	h.activities.byName[facilities.WCActivityName] = []ports.Activity{{ID: 300, Name: facilities.WCActivityName}}

	selection, err := h.service().findOrCreateSessionForRoom(context.Background(), room, testStudentID, testDeviceID)

	require.NoError(t, err)
	assert.Equal(t, []int64{100}, h.sessions.ended)
	require.Len(t, h.sessions.started, 1)
	assert.Equal(t, ports.NewSession{ActivityID: 300, RoomID: 42}, h.sessions.started[0])
	assert.Equal(t, int64(201), selection.Session.ID)
	assert.True(t, selection.created)
}

func TestFindOrCreateSession_RegularRoomReusesPreviousDaySession(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	room := facilities.Room{ID: 42, Name: "Klassenraum 1a"}
	h.sessions.open = []ports.Session{{ID: 100, RoomID: room.ID, StartTime: fixedNow.AddDate(0, 0, -1)}}

	selection, err := h.service().findOrCreateSessionForRoom(context.Background(), room, testStudentID, testDeviceID)

	require.NoError(t, err)
	assert.Empty(t, h.sessions.ended, "regular rooms must not end sessions on scan")
	assert.Equal(t, int64(100), selection.Session.ID)
}

func TestFindOrCreateSession_SpecialRoomEndsStaleSessionBeforeSelectingCurrent(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	// Released, like every provisioned yard (#3064).
	room := facilities.Room{ID: 42, Name: facilities.SchulhofRoomName, IsOpenRoom: true}
	h.sessions.open = []ports.Session{
		{ID: 100, RoomID: room.ID, StartTime: fixedNow.AddDate(0, 0, -1)},
		{ID: 101, RoomID: room.ID, StartTime: fixedNow.Add(-time.Hour)},
	}

	selection, err := h.service().findOrCreateSessionForRoom(context.Background(), room, testStudentID, testDeviceID)

	require.NoError(t, err)
	assert.Equal(t, []int64{100}, h.sessions.ended)
	assert.Equal(t, int64(101), selection.Session.ID)
}

func TestFindOrCreateSession_StaleSessionEndFailure(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	room := facilities.Room{ID: 42, Name: facilities.SchulhofRoomName, IsOpenRoom: true}
	h.sessions.open = []ports.Session{{ID: 100, RoomID: room.ID, StartTime: fixedNow.AddDate(0, 0, -1)}}
	h.sessions.endErr = errBoom

	_, err := h.service().findOrCreateSessionForRoom(context.Background(), room, testStudentID, testDeviceID)

	requireFailure(t, err, devicescan.FailureInternal, devicescan.MessageCreateSchulhofFailed)
}

func TestProcessCheckin_RejectedCapacityRemovesNewSpecialRoomSession(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	room := facilities.Room{ID: 42, Name: facilities.WCRoomName}
	h.rooms.rooms[room.ID] = room
	h.rooms.toilet = &room
	h.sessions.open = nil
	h.activities.byName[facilities.WCActivityName] = []ports.Activity{{ID: 300, Name: "WC", MaxParticipants: 999}}
	h.visits.recordErr = &ports.RoomCapacityExceeded{RoomID: 42, RoomName: facilities.WCRoomName, CurrentOccupancy: 1, MaxCapacity: 1}

	_, _, err := h.service().processCheckin(context.Background(), &ports.Student{ID: 500}, &ports.Person{FirstName: "Capacity"}, room.ID, testDeviceID)

	_, ok := errors.AsType[*devicescan.RoomCapacityExceededError](err)
	require.True(t, ok, "got %v", err)
	require.Len(t, h.sessions.started, 1)
	assert.Equal(t, []int64{201}, h.sessions.deleted, "the empty session this scan provisioned is removed")
}

func TestProcessCheckin_FullRoomDoesNotCreateSpecialRoomSession(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	room := facilities.Room{ID: 42, Name: facilities.WCRoomName, Capacity: ptr(1)}
	h.rooms.rooms[room.ID] = room
	h.presence.roomCounts[room.ID] = 1
	h.sessions.open = nil

	_, _, err := h.service().processCheckin(context.Background(), &ports.Student{ID: 500}, &ports.Person{FirstName: "Capacity"}, room.ID, testDeviceID)

	_, ok := errors.AsType[*devicescan.RoomCapacityExceededError](err)
	require.True(t, ok)
	assert.Empty(t, h.sessions.started)
	assert.Empty(t, h.sessions.deleted)
}

// A deactivated yard follows the ordinary room rules again (#3064): the
// release is enforced server-side, not by a hidden kiosk button.
func TestDeactivatedSchulhofFollowsRegularRoomRules(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	room := facilities.Room{ID: 42, Name: facilities.SchulhofRoomName}
	h.sessions.open = []ports.Session{{ID: 100, RoomID: room.ID, StartTime: fixedNow.AddDate(0, 0, -1)}}

	selection, err := h.service().findOrCreateSessionForRoom(context.Background(), room, testStudentID, testDeviceID)

	require.NoError(t, err)
	assert.Empty(t, h.sessions.ended, "a deactivated Schulhof must not end sessions on scan any more")
	assert.Equal(t, int64(100), selection.Session.ID, "an existing session stays reusable under the ordinary room rules")
}

func TestDeactivatedSchulhofDoesNotAutoCreateASession(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	room := facilities.Room{ID: 42, Name: facilities.SchulhofRoomName}
	h.sessions.open = nil
	h.activities.byName[facilities.SchulhofActivityName] = []ports.Activity{{ID: 300}}

	selection, err := h.service().findOrCreateSessionForRoom(context.Background(), room, testStudentID, testDeviceID)

	assert.Nil(t, selection)
	requireFailure(t, err, devicescan.FailureNotFound, devicescan.MessageNoGroupsInRoom)
	assert.Empty(t, h.sessions.ended)
}

func TestRoomProvisioningPredicatesFollowTheRelease(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name           string
		room           facilities.Room
		releasedYard   bool
		selfProvisions bool
	}{
		{"released Schulhof", facilities.Room{ID: 1, Name: facilities.SchulhofRoomName, IsOpenRoom: true}, true, true},
		{"deactivated Schulhof", facilities.Room{ID: 2, Name: facilities.SchulhofRoomName}, false, false},
		{"WC stays name-based", facilities.Room{ID: 3, Name: facilities.WCRoomName}, false, true},
		{"Toilette alias stays name-based", facilities.Room{ID: 4, Name: facilities.WCRoomAliasName}, false, true},
		{"a released ordinary room provisions nothing yet", facilities.Room{ID: 5, Name: "Turnhalle", IsOpenRoom: true}, false, false},
		{"ordinary room", facilities.Room{ID: 6, Name: "Klassenraum 1a"}, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.releasedYard, isReleasedSchulhofRoom(tc.room))
			assert.Equal(t, tc.selfProvisions, isSelfProvisioningRoom(tc.room))
		})
	}
}

func TestSpecialRoomCreationError(t *testing.T) {
	t.Parallel()
	requireFailure(t, specialRoomCreationError(facilities.SchulhofRoomName), devicescan.FailureInternal, devicescan.MessageCreateSchulhofFailed)
	requireFailure(t, specialRoomCreationError(facilities.WCRoomAliasName), devicescan.FailureInternal, devicescan.MessageCreateWCFailed)
	requireFailure(t, specialRoomCreationError("Turnhalle"), devicescan.FailureInternal, devicescan.MessageCreateSessionFailed)
}

func TestCreateSpecialRoomSession_WCNeedsStaffText(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	room := facilities.Room{ID: 42, Name: facilities.WCRoomName}
	h.rooms.toilet = &room
	h.activities.listErr = errors.New("WC activity auto-create requires staff context")

	_, err := h.service().createSpecialRoomSession(context.Background(), room)

	requireFailure(t, err, devicescan.FailureInternal, devicescan.MessageWCNeedsStaff)
}

func TestCreateSpecialRoomSession_SchulhofNotConfigured(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	room := facilities.Room{ID: 42, Name: facilities.SchulhofRoomName, IsOpenRoom: true}
	h.rooms.byName[facilities.SchulhofRoomName] = facilities.Room{ID: 42, Name: "schulhof", IsSystem: true}

	_, err := h.service().createSpecialRoomSession(context.Background(), room)

	requireFailure(t, err, devicescan.FailureInternal, devicescan.MessageSchulhofNotConfigured)
}

func TestCreateSpecialRoomSession_StartFailure(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	room := facilities.Room{ID: 42, Name: facilities.WCRoomName}
	h.rooms.toilet = &room
	h.activities.byName[facilities.WCActivityName] = []ports.Activity{{ID: 300, Name: facilities.WCActivityName}}
	h.sessions.startErr = errBoom

	_, err := h.service().createSpecialRoomSession(context.Background(), room)

	requireFailure(t, err, devicescan.FailureInternal, devicescan.MessageCreateWCFailed)
}

// Yard scans while blocks run (ADR 0019, #3282): a child joins the running
// block whose day roster lists it, the newest on several matches; any other
// child gets an independent stay in the Freispiel session.

const (
	yardFreeplayActivityID = int64(310)
	yardBlockAActivityID   = int64(320)
	yardBlockBActivityID   = int64(321)
	yardFreeplaySessionID  = int64(110)
	yardBlockASessionID    = int64(111)
	yardBlockBSessionID    = int64(112)
)

// yardHarness is a released canonical yard with a Freispiel session and
// two running blocks, A started before B, and a roster lookup that lists
// the scanned child nowhere.
func yardHarness(t *testing.T) (*harness, facilities.Room) {
	t.Helper()
	h := newHarness(t)
	room := facilities.Room{ID: testRoomID, Name: facilities.SchulhofRoomName, IsSystem: true, IsOpenRoom: true}
	h.rooms.rooms[room.ID] = room
	h.rooms.byName[room.Name] = room
	freeplay := ports.Activity{ID: yardFreeplayActivityID, Name: facilities.SchulhofActivityName, MaxParticipants: 300, PlannedRoomID: ptr(room.ID), IsSystem: true, IsOpen: true}
	h.activities.byID[freeplay.ID] = &freeplay
	h.activities.byName[freeplay.Name] = []ports.Activity{freeplay}
	h.activities.byID[yardBlockAActivityID] = &ports.Activity{ID: yardBlockAActivityID, Name: "Fußball"}
	h.activities.byID[yardBlockBActivityID] = &ports.Activity{ID: yardBlockBActivityID, Name: "Seilspringen"}
	h.sessions.open = []ports.Session{
		{ID: yardFreeplaySessionID, RoomID: room.ID, StartTime: fixedNow.Add(-3 * time.Hour), TemplateID: ptr(yardFreeplayActivityID)},
		{ID: yardBlockASessionID, RoomID: room.ID, StartTime: fixedNow.Add(-2 * time.Hour), TemplateID: ptr(yardBlockAActivityID)},
		{ID: yardBlockBSessionID, RoomID: room.ID, StartTime: fixedNow.Add(-time.Hour), TemplateID: ptr(yardBlockBActivityID)},
	}
	h.rosters = &fakeRosters{rostered: map[int64]bool{}}
	return h, room
}

func TestYardScan_RosteredChildJoinsOwnBlockRegardlessOfStartOrder(t *testing.T) {
	t.Parallel()
	h, room := yardHarness(t)
	h.rosters.rostered[yardBlockASessionID] = true

	selection, err := h.service().findOrCreateSessionForRoom(context.Background(), room, testStudentID, testDeviceID)

	require.NoError(t, err)
	assert.Equal(t, yardBlockASessionID, selection.Session.ID, "block A, although block B started later")
	assert.False(t, selection.created)
	assert.False(t, selection.DeviceScoped)
	require.NotNil(t, selection.Session.Activity, "the loaded activity feeds the capacity check")
	assert.Equal(t, yardBlockAActivityID, selection.Session.Activity.ID)
	assert.Equal(t, [][]int64{{yardBlockASessionID, yardBlockBSessionID}}, h.rosters.calls, "only blocks are looked up")
	assert.Empty(t, h.sessions.started)
}

func TestYardScan_ChildInTwoBlocksJoinsTheNewest(t *testing.T) {
	t.Parallel()
	h, room := yardHarness(t)
	h.rosters.rostered[yardBlockASessionID] = true
	h.rosters.rostered[yardBlockBSessionID] = true

	selection, err := h.service().findOrCreateSessionForRoom(context.Background(), room, testStudentID, testDeviceID)

	require.NoError(t, err)
	assert.Equal(t, yardBlockBSessionID, selection.Session.ID)
}

func TestYardScan_RosteredChildJoinsDeviceLinkedBlockAsDeviceScoped(t *testing.T) {
	t.Parallel()
	h, room := yardHarness(t)
	h.sessions.open[1].DeviceID = ptr(testDeviceID)
	h.rosters.rostered[yardBlockASessionID] = true

	selection, err := h.service().findOrCreateSessionForRoom(context.Background(), room, testStudentID, testDeviceID)

	require.NoError(t, err)
	assert.Equal(t, yardBlockASessionID, selection.Session.ID)
	assert.True(t, selection.DeviceScoped)
}

func TestYardScan_UnrosteredChildStaysOutOfRunningBlocks(t *testing.T) {
	t.Parallel()
	h, room := yardHarness(t)
	// A block another kiosk runs does not absorb a child it does not list.
	h.sessions.open[2].DeviceID = ptr(testDeviceID + 1)

	selection, err := h.service().findOrCreateSessionForRoom(context.Background(), room, testStudentID, testDeviceID)

	require.NoError(t, err)
	assert.Equal(t, yardFreeplaySessionID, selection.Session.ID)
	assert.False(t, selection.DeviceScoped)
	assert.Empty(t, h.sessions.started)
	assert.Empty(t, h.sessions.ensured)
	assert.Empty(t, h.sessions.ended, "running blocks stay untouched")
}

// The kiosk books into the room it stands in and shows the count of its
// own session, so the activity it runs keeps the children it scans.
func TestYardScan_UnrosteredChildJoinsTheSessionOfTheScanningKiosk(t *testing.T) {
	t.Parallel()
	h, room := yardHarness(t)
	h.sessions.open[1].DeviceID = ptr(testDeviceID)

	selection, err := h.service().findOrCreateSessionForRoom(context.Background(), room, testStudentID, testDeviceID)

	require.NoError(t, err)
	assert.Equal(t, yardBlockASessionID, selection.Session.ID)
	assert.True(t, selection.DeviceScoped)
}

func TestYardScan_KioskSessionWithoutFreeplayOpensNothing(t *testing.T) {
	t.Parallel()
	h, room := yardHarness(t)
	h.sessions.open = h.sessions.open[1:]
	h.sessions.open[0].DeviceID = ptr(testDeviceID)

	selection, err := h.service().findOrCreateSessionForRoom(context.Background(), room, testStudentID, testDeviceID)

	require.NoError(t, err)
	assert.Equal(t, yardBlockASessionID, selection.Session.ID)
	assert.Empty(t, h.sessions.ensured, "no Freispiel session next to the kiosk's own")
}

func TestYardScan_UnrosteredChildPrefersTheFreeplaySessionTheKioskTookOver(t *testing.T) {
	t.Parallel()
	h, room := yardHarness(t)
	h.sessions.open[0].DeviceID = ptr(testDeviceID)
	h.sessions.open = append(h.sessions.open, ports.Session{
		ID: 113, RoomID: room.ID, StartTime: fixedNow.Add(-30 * time.Minute), TemplateID: ptr(yardFreeplayActivityID),
	})

	selection, err := h.service().findOrCreateSessionForRoom(context.Background(), room, testStudentID, testDeviceID)

	require.NoError(t, err)
	assert.Equal(t, yardFreeplaySessionID, selection.Session.ID)
	assert.True(t, selection.DeviceScoped)
}

func TestYardScan_UnrosteredChildGetsAFreshDevicelessFreeplaySession(t *testing.T) {
	t.Parallel()
	h, room := yardHarness(t)
	yesterday := ports.Session{ID: 109, RoomID: room.ID, StartTime: fixedNow.AddDate(0, 0, -1), TemplateID: ptr(yardFreeplayActivityID)}
	h.sessions.open = append([]ports.Session{yesterday}, h.sessions.open[1:]...)

	selection, err := h.service().findOrCreateSessionForRoom(context.Background(), room, testStudentID, testDeviceID)

	require.NoError(t, err)
	assert.Equal(t, []int64{yesterday.ID}, h.sessions.ended, "the stale session is closed as before")
	// The one-session-per-room start would refuse the occupied yard; the
	// room session path of ADR 0018 opens the Freispiel session next to the
	// blocks.
	assert.Empty(t, h.sessions.started)
	require.Len(t, h.sessions.ensured, 1)
	assert.Equal(t, ports.NewSession{ActivityID: yardFreeplayActivityID, RoomID: room.ID}, h.sessions.ensured[0])
	assert.Equal(t, int64(201), selection.Session.ID)
	assert.False(t, selection.created, "a shared room session is never deleted on a refused visit")
	assert.False(t, selection.DeviceScoped)
}

func TestYardScan_FreeplayCreationFailureNamesTheSchulhof(t *testing.T) {
	t.Parallel()
	h, room := yardHarness(t)
	h.sessions.open = h.sessions.open[1:]
	h.sessions.ensureErr = errBoom

	_, err := h.service().findOrCreateSessionForRoom(context.Background(), room, testStudentID, testDeviceID)

	requireFailure(t, err, devicescan.FailureInternal, devicescan.MessageCreateSchulhofFailed)
}

func TestYardScan_WithoutAnySessionStartsTheFreeplaySessionAsBefore(t *testing.T) {
	t.Parallel()
	h, room := yardHarness(t)
	h.sessions.open = nil

	selection, err := h.service().findOrCreateSessionForRoom(context.Background(), room, testStudentID, testDeviceID)

	require.NoError(t, err)
	assert.Empty(t, h.sessions.ensured)
	require.Len(t, h.sessions.started, 1)
	assert.True(t, selection.created)
}

func TestYardScan_WithoutBlocksKeepsTheFormerChoice(t *testing.T) {
	t.Parallel()
	h, room := yardHarness(t)
	h.sessions.open = []ports.Session{
		{ID: yardFreeplaySessionID, RoomID: room.ID, StartTime: fixedNow.Add(-3 * time.Hour), TemplateID: ptr(yardFreeplayActivityID), DeviceID: ptr(testDeviceID)},
		{ID: 113, RoomID: room.ID, StartTime: fixedNow.Add(-time.Hour), TemplateID: ptr(yardFreeplayActivityID)},
	}

	selection, err := h.service().findOrCreateSessionForRoom(context.Background(), room, testStudentID, testDeviceID)

	require.NoError(t, err)
	assert.Equal(t, yardFreeplaySessionID, selection.Session.ID, "the kiosk keeps the session it took over")
	assert.True(t, selection.DeviceScoped)
	assert.Empty(t, h.rosters.calls, "no block, no roster lookup")
}

func TestYardScan_SpontaneousBlockIsNeverAScanTarget(t *testing.T) {
	t.Parallel()
	h, room := yardHarness(t)
	h.sessions.open = []ports.Session{
		h.sessions.open[0],
		{ID: 114, RoomID: room.ID, StartTime: fixedNow.Add(-time.Minute)},
	}

	selection, err := h.service().findOrCreateSessionForRoom(context.Background(), room, testStudentID, testDeviceID)

	require.NoError(t, err)
	assert.Equal(t, yardFreeplaySessionID, selection.Session.ID)
	assert.Empty(t, h.rosters.calls, "a template-less session cannot take a kiosk visit")
}

func TestYardScan_UnreleasedYardUsesTheRosterThenTheRoomRules(t *testing.T) {
	t.Parallel()
	t.Run("rostered child joins its block", func(t *testing.T) {
		t.Parallel()
		h, room := yardHarness(t)
		room.IsOpenRoom = false
		h.sessions.open = h.sessions.open[1:]
		h.rosters.rostered[yardBlockASessionID] = true

		selection, err := h.service().findOrCreateSessionForRoom(context.Background(), room, testStudentID, testDeviceID)

		require.NoError(t, err)
		assert.Equal(t, yardBlockASessionID, selection.Session.ID)
	})
	t.Run("other child follows the ordinary room rules", func(t *testing.T) {
		t.Parallel()
		h, room := yardHarness(t)
		room.IsOpenRoom = false
		h.sessions.open = h.sessions.open[1:]

		selection, err := h.service().findOrCreateSessionForRoom(context.Background(), room, testStudentID, testDeviceID)

		require.NoError(t, err)
		assert.Equal(t, yardBlockBSessionID, selection.Session.ID, "newest session, as in any room")
		assert.Empty(t, h.sessions.started, "no Freispiel session without release")
	})
}

func TestYardScan_LookupFailuresStopTheScan(t *testing.T) {
	t.Parallel()
	t.Run("roster", func(t *testing.T) {
		t.Parallel()
		h, room := yardHarness(t)
		h.rosters.err = errBoom

		_, err := h.service().findOrCreateSessionForRoom(context.Background(), room, testStudentID, testDeviceID)

		requireFailure(t, err, devicescan.FailureInternal, devicescan.MessageFindActiveGroupsFailed)
	})
	t.Run("activity", func(t *testing.T) {
		t.Parallel()
		h, room := yardHarness(t)
		h.activities.findErr = errBoom

		_, err := h.service().findOrCreateSessionForRoom(context.Background(), room, testStudentID, testDeviceID)

		requireFailure(t, err, devicescan.FailureInternal, devicescan.MessageGetActivityFailed)
	})
}
