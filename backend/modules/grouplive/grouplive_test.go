package grouplive

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/careplan/excusedrequests"
)

const today Date = "2026-08-21"

// fakes implements every consumer-owned port with func fields; a nil field
// answers with an empty value so each test only wires what it asserts.
type fakes struct {
	caller             func(context.Context) (Caller, error)
	supervisedGroups   func(context.Context) ([]GroupRecord, error)
	tenantGroups       func(context.Context) ([]GroupRecord, error)
	substitutedGroups  func(context.Context) (map[int64]bool, error)
	roomNames          func(context.Context, []int64) (map[int64]string, error)
	members            func(context.Context, int64) ([]RosterStudent, error)
	participants       func(context.Context, []int64, Date) (map[int64]bool, error)
	snapshot           func(context.Context, []int64, Date) (PresenceSnapshot, error)
	statuses           func(context.Context, []int64, Date) (map[int64]EffectiveStatus, error)
	tracking           func(context.Context, []int64, []string) (map[int64][]bool, error)
	arrivals           func(context.Context, []int64, Date) (map[int64]Arrival, error)
	pickups            func(context.Context, []int64, Date) (map[int64]Pickup, error)
	timetable          func(context.Context, []int64, Date) (map[int64]bool, error)
	decide             func(DayInputs) DayDecision
	handovers          func(context.Context, int64, Date) ([]Transfer, error)
	prepare            func(context.Context) (context.Context, error)
	photosEnabled      func(context.Context) (bool, error)
	trackingLabels     func(context.Context) ([]string, error)
	pendingByStudent   func(context.Context, excusedrequests.Date) (map[int64]*excusedrequests.Request, error)
	restricted         bool
	accessCalls        int
	tenantGroupsCalls  int
	handoverCalls      int
	pendingCalls       int
	trackingCalls      int
	participantsCalled [][]int64
	snapshotIDs        []int64
}

func (f *fakes) Caller(ctx context.Context) (Caller, error) {
	if f.caller == nil {
		return Caller{}, nil
	}
	return f.caller(ctx)
}

func (f *fakes) FullStudentAccess(context.Context) (bool, error) {
	f.accessCalls++
	return !f.restricted, nil
}

func (f *fakes) SupervisedGroups(ctx context.Context) ([]GroupRecord, error) {
	if f.supervisedGroups == nil {
		return nil, nil
	}
	return f.supervisedGroups(ctx)
}

func (f *fakes) TenantGroups(ctx context.Context) ([]GroupRecord, error) {
	f.tenantGroupsCalls++
	if f.tenantGroups == nil {
		return nil, nil
	}
	return f.tenantGroups(ctx)
}

func (f *fakes) SubstitutedGroupIDs(ctx context.Context) (map[int64]bool, error) {
	if f.substitutedGroups == nil {
		return map[int64]bool{}, nil
	}
	return f.substitutedGroups(ctx)
}

func (f *fakes) GroupRoomNames(ctx context.Context, ids []int64) (map[int64]string, error) {
	if f.roomNames == nil {
		return map[int64]string{}, nil
	}
	return f.roomNames(ctx, ids)
}

func (f *fakes) GroupMembers(ctx context.Context, groupID int64) ([]RosterStudent, error) {
	if f.members == nil {
		return []RosterStudent{}, nil
	}
	return f.members(ctx, groupID)
}

func (f *fakes) CareParticipants(ctx context.Context, ids []int64, date Date) (map[int64]bool, error) {
	f.participantsCalled = append(f.participantsCalled, ids)
	if f.participants == nil {
		all := make(map[int64]bool, len(ids))
		for _, id := range ids {
			all[id] = true
		}
		return all, nil
	}
	return f.participants(ctx, ids, date)
}

func (f *fakes) Snapshot(ctx context.Context, ids []int64, date Date) (PresenceSnapshot, error) {
	f.snapshotIDs = ids
	if f.snapshot == nil {
		return presenceFake{}, nil
	}
	return f.snapshot(ctx, ids, date)
}

func (f *fakes) EffectiveStatuses(ctx context.Context, ids []int64, date Date) (map[int64]EffectiveStatus, error) {
	if f.statuses == nil {
		return map[int64]EffectiveStatus{}, nil
	}
	return f.statuses(ctx, ids, date)
}

func (f *fakes) TrackingIndicators(ctx context.Context, ids []int64, labels []string) (map[int64][]bool, error) {
	f.trackingCalls++
	if f.tracking == nil {
		return map[int64][]bool{}, nil
	}
	return f.tracking(ctx, ids, labels)
}

func (f *fakes) Arrivals(ctx context.Context, ids []int64, date Date) (map[int64]Arrival, error) {
	if f.arrivals == nil {
		return map[int64]Arrival{}, nil
	}
	return f.arrivals(ctx, ids, date)
}

func (f *fakes) Pickups(ctx context.Context, ids []int64, date Date) (map[int64]Pickup, error) {
	if f.pickups == nil {
		return map[int64]Pickup{}, nil
	}
	return f.pickups(ctx, ids, date)
}

func (f *fakes) TimetablePlannedStudentIDs(ctx context.Context, ids []int64, date Date) (map[int64]bool, error) {
	if f.timetable == nil {
		return map[int64]bool{}, nil
	}
	return f.timetable(ctx, ids, date)
}

func (f *fakes) DecideDay(inputs DayInputs) DayDecision {
	if f.decide == nil {
		return DayDecision{Reason: DayReasonNoPlan}
	}
	return f.decide(inputs)
}

func (f *fakes) GroupHandovers(ctx context.Context, groupID int64, date Date) ([]Transfer, error) {
	f.handoverCalls++
	if f.handovers == nil {
		return nil, nil
	}
	return f.handovers(ctx, groupID, date)
}

func (f *fakes) Prepare(ctx context.Context) (context.Context, error) {
	if f.prepare == nil {
		return ctx, nil
	}
	return f.prepare(ctx)
}

func (f *fakes) StudentPhotosEnabled(ctx context.Context) (bool, error) {
	if f.photosEnabled == nil {
		return false, nil
	}
	return f.photosEnabled(ctx)
}

func (f *fakes) TrackingIndicatorLabels(ctx context.Context) ([]string, error) {
	if f.trackingLabels == nil {
		return nil, nil
	}
	return f.trackingLabels(ctx)
}

func (f *fakes) PendingByStudentForDate(ctx context.Context, date excusedrequests.Date) (map[int64]*excusedrequests.Request, error) {
	f.pendingCalls++
	if f.pendingByStudent == nil {
		return map[int64]*excusedrequests.Request{}, nil
	}
	return f.pendingByStudent(ctx, date)
}

func (f *fakes) Today() Date { return today }

func (f *fakes) Clock(at time.Time) string { return at.UTC().Format("15:04") }

// presenceFake answers per student from fixed maps.
type presenceFake struct {
	locations   map[int64]Location
	attendances map[int64]Attendance
	rooms       map[int64]int64
	redacted    map[int64]Location
}

func (p presenceFake) Location(studentID int64, fullAccess bool) Location {
	if !fullAccess {
		if location, ok := p.redacted[studentID]; ok {
			return location
		}
		return Location{Name: "Anwesend"}
	}
	return p.locations[studentID]
}

func (p presenceFake) Attendance(studentID int64) (Attendance, bool) {
	attendance, ok := p.attendances[studentID]
	return attendance, ok
}

func (p presenceFake) CurrentRoomID(studentID int64) *int64 {
	roomID, ok := p.rooms[studentID]
	if !ok {
		return nil
	}
	return &roomID
}

func newService(f *fakes) Query {
	return New(Dependencies{
		Access: f, Groups: f, Roster: f, Presence: f, Planning: f, Transfers: f,
		Settings: f, Calendar: f, ExcusedRequests: f,
	})
}

func fullCaller(context.Context) (Caller, error) {
	return Caller{CanReadGroups: true, CanReviewExcusedRequests: true}, nil
}

const (
	appleGroupID int64 = 1
	zebraGroupID int64 = 2
)

func twoGroups() []GroupRecord {
	return []GroupRecord{{ID: appleGroupID, Name: "Äpfel", RoomID: new(int64(31))}, {ID: zebraGroupID, Name: "Zebra"}}
}

func TestGetReturnsEmptyProjectionWithoutVisibleGroups(t *testing.T) {
	t.Parallel()

	f := &fakes{caller: fullCaller}
	got, err := newService(f).LiveGroup(context.Background(), 0)
	require.NoError(t, err)
	assert.Equal(t, EmptyProjection(), got)

	_, err = newService(f).LiveGroup(context.Background(), 7)
	assert.ErrorIs(t, err, ErrForbiddenGroup, "a requested group cannot be resolved from an empty visible set")
}

func TestGetRejectsGroupsOutsideTheVisibleSet(t *testing.T) {
	t.Parallel()

	f := &fakes{
		caller:           fullCaller,
		supervisedGroups: func(context.Context) ([]GroupRecord, error) { return twoGroups(), nil },
	}
	_, err := newService(f).LiveGroup(context.Background(), 99)
	assert.ErrorIs(t, err, ErrForbiddenGroup)
	assert.Empty(t, f.participantsCalled, "no roster is loaded for a forbidden group")
}

func TestGetSelectsPersonalGroupFirstInDirectoryOrder(t *testing.T) {
	t.Parallel()

	f := &fakes{
		caller: func(context.Context) (Caller, error) {
			return Caller{CanReadGroups: true, OperationalOverview: true}, nil
		},
		supervisedGroups: func(context.Context) ([]GroupRecord, error) {
			return []GroupRecord{{ID: 2, Name: "Zebra"}}, nil
		},
		tenantGroups: func(context.Context) ([]GroupRecord, error) { return twoGroups(), nil },
		substitutedGroups: func(context.Context) (map[int64]bool, error) {
			return map[int64]bool{2: true}, nil
		},
		roomNames: func(_ context.Context, ids []int64) (map[int64]string, error) {
			assert.Equal(t, []int64{1}, ids, "only groups with a room are resolved")
			return map[int64]string{1: "Raum A"}, nil
		},
	}

	got, err := newService(f).LiveGroup(context.Background(), 0)
	require.NoError(t, err)
	require.NotNil(t, got.GroupID)
	assert.Equal(t, zebraGroupID, *got.GroupID, "the default selection prefers a personal group over the first sorted one")
	assert.Equal(t, []Group{
		{ID: 1, Name: "Äpfel", RoomID: new(int64(31)), RoomName: "Raum A"},
		{ID: 2, Name: "Zebra", ViaSubstitution: true, IsPersonal: true},
	}, got.Groups, "groups keep the directory order and carry rooms, substitution, and personal flags")
	assert.Equal(t, 1, f.tenantGroupsCalls)
}

func TestGetKeepsSupervisedGroupsWithoutGroupsRead(t *testing.T) {
	t.Parallel()

	f := &fakes{
		caller: func(context.Context) (Caller, error) {
			return Caller{OperationalOverview: true}, nil
		},
		supervisedGroups: func(context.Context) ([]GroupRecord, error) { return twoGroups()[1:], nil },
	}

	got, err := newService(f).LiveGroup(context.Background(), 2)
	require.NoError(t, err)
	assert.Equal(t, 0, f.tenantGroupsCalls, "operational overview without groups:read never widens the visible set")
	require.NotNil(t, got.GroupID)
	assert.Equal(t, zebraGroupID, *got.GroupID)
	assert.Equal(t, map[string]RoomStatus{}, got.RoomStatus)
	assert.Equal(t, emptyTracking(), got.TrackingIndicators)
	assert.Equal(t, []Transfer{}, got.Transfers)
	assert.Equal(t, 0, f.handoverCalls, "transfers are permission-redacted, not loaded and dropped")
	assert.Equal(t, 0, f.trackingCalls)
}

func TestGetProjectsRosterUnderFullAccess(t *testing.T) {
	t.Parallel()

	since := time.Date(2026, time.August, 21, 6, 30, 0, 0, time.UTC)
	checkIn := time.Date(2026, time.August, 21, 8, 10, 0, 0, time.UTC)
	checkOut := time.Date(2026, time.August, 21, 14, 5, 0, 0, time.UTC)
	arrivalAt := time.Date(2026, time.August, 21, 7, 45, 0, 0, time.UTC)
	pickupAt := time.Date(2026, time.August, 21, 15, 30, 0, 0, time.UTC)
	f := &fakes{
		caller:           fullCaller,
		supervisedGroups: func(context.Context) ([]GroupRecord, error) { return twoGroups(), nil },
		members: func(_ context.Context, groupID int64) ([]RosterStudent, error) {
			assert.Equal(t, appleGroupID, groupID)
			return []RosterStudent{
				{ID: 11, FirstName: "Ada", LastName: "Lovelace", SchoolClass: "2a", Sick: true, SickSince: &since, PhotoPath: new("/uploads/student-photos/ada.jpg")},
				{ID: 12, FirstName: "Bob", LastName: "Builder", SchoolClass: "2b", PhotoPath: new("https://example.test/bob.jpg")},
				{ID: 13, FirstName: "Cy", LastName: "Absent", SchoolClass: "2b"},
			}, nil
		},
		participants: func(_ context.Context, ids []int64, date Date) (map[int64]bool, error) {
			assert.Equal(t, today, date)
			assert.Equal(t, []int64{11, 12, 13}, ids)
			return map[int64]bool{11: true, 12: true}, nil
		},
		snapshot: func(context.Context, []int64, Date) (PresenceSnapshot, error) {
			return presenceFake{
				locations: map[int64]Location{
					11: {Name: "Raum A", Since: &checkIn, RoomColor: new("#A3D977")},
					12: {Name: "Abwesend", Since: &checkOut},
				},
				attendances: map[int64]Attendance{
					11: {Recorded: true, Present: true, CheckInTime: &checkIn},
					12: {Recorded: true, CheckInTime: &checkIn, CheckOutTime: &checkOut},
				},
				rooms: map[int64]int64{11: 31, 12: 32},
			}, nil
		},
		photosEnabled: func(context.Context) (bool, error) { return true, nil },
		statuses: func(_ context.Context, ids []int64, _ Date) (map[int64]EffectiveStatus, error) {
			assert.Equal(t, []int64{11, 12}, ids, "status days are resolved for care participants only")
			return map[int64]EffectiveStatus{12: {Excused: true, ExcusedSince: &since}}, nil
		},
		arrivals: func(_ context.Context, ids []int64, _ Date) (map[int64]Arrival, error) {
			assert.Equal(t, []int64{11, 12}, ids)
			return map[int64]Arrival{12: {ArrivalTime: &arrivalAt, IsException: true, Notes: "später", DayNotes: []string{"Bus", ""}}}, nil
		},
		pickups: func(context.Context, []int64, Date) (map[int64]Pickup, error) {
			return map[int64]Pickup{
				11: {Date: today, WeekdayName: "Freitag", PickupTime: &pickupAt, IsException: true, Notes: "Oma", DayNotes: []DayNote{{ID: 7, Content: "Klingeln"}}},
			}, nil
		},
		timetable: func(context.Context, []int64, Date) (map[int64]bool, error) { return map[int64]bool{11: true}, nil },
		decide: func(inputs DayInputs) DayDecision {
			if inputs.Sick {
				return DayDecision{Reason: DayReasonSick}
			}
			if inputs.Arrival != nil && inputs.Arrival.IsException {
				return DayDecision{ComesToday: true, Reason: DayReasonArrivalException}
			}
			return DayDecision{Reason: DayReasonNoPlan}
		},
		pendingByStudent: func(_ context.Context, date excusedrequests.Date) (map[int64]*excusedrequests.Request, error) {
			assert.Equal(t, excusedrequests.Date(today), date)
			return map[int64]*excusedrequests.Request{12: {Note: "Zahnarzt"}}, nil
		},
		trackingLabels: func(context.Context) ([]string, error) { return []string{" Hausaufgaben ", "", "Mittag"}, nil },
		tracking: func(_ context.Context, ids []int64, labels []string) (map[int64][]bool, error) {
			assert.Equal(t, []int64{11, 12}, ids)
			assert.Equal(t, []string{"Hausaufgaben", "Mittag"}, labels)
			return map[int64][]bool{11: {true, false}}, nil
		},
		handovers: func(_ context.Context, groupID int64, date Date) ([]Transfer, error) {
			assert.Equal(t, appleGroupID, groupID)
			assert.Equal(t, today, date)
			return []Transfer{{ID: 5, GroupID: 1, SubstituteStaffID: 9, SubstituteName: "Vera Vertretung", EndDate: "2026-08-28"}}, nil
		},
	}

	got, err := newService(f).LiveGroup(context.Background(), 1)
	require.NoError(t, err)

	assert.Equal(t, []int64{11, 12}, f.snapshotIDs, "presence is loaded for care participants only")
	require.Len(t, got.Students, 2, "the non-participating child is filtered out")

	ada := got.Students[0]
	assert.Equal(t, "Ada", ada.FirstName)
	assert.Equal(t, "Lovelace", ada.LastName)
	assert.Equal(t, "2a", ada.SchoolClass)
	assert.Equal(t, "Raum A", ada.Location)
	assert.Equal(t, &checkIn, ada.LocationSince)
	assert.Equal(t, new("#A3D977"), ada.RoomColor)
	assert.True(t, ada.Sick)
	assert.Equal(t, &since, ada.SickSince)
	assert.Equal(t, "/api/students/11/photo/ada.jpg", ada.PhotoURL, "stored uploads are rewritten to the authenticated proxy")
	assert.Equal(t, "not_coming_today", ada.DayPlanningStatus)
	assert.Equal(t, DayReasonSick, ada.DayPlanningReason)
	assert.Equal(t, "krank gemeldet", ada.DayPlanningLabel)
	assert.Equal(t, new("08:10"), ada.ActualArrivalTime)
	assert.Nil(t, ada.ActualPickupTime)
	assert.Nil(t, ada.PendingExcusedNote)

	bob := got.Students[1]
	assert.Equal(t, "https://example.test/bob.jpg", bob.PhotoURL, "non-upload URLs pass through untouched")
	assert.True(t, bob.Excused, "the excused status day applies to a child who is not sick")
	assert.Equal(t, &since, bob.ExcusedSince)
	assert.Equal(t, "comes_today", bob.DayPlanningStatus)
	assert.Equal(t, "geplante Ankunft heute", bob.DayPlanningLabel)
	assert.Equal(t, new("07:45"), bob.ArrivalTime)
	assert.True(t, bob.ArrivalIsException)
	assert.Equal(t, "später, Bus", bob.ArrivalNotes)
	assert.Equal(t, new("08:10"), bob.ActualArrivalTime)
	assert.Equal(t, new("14:05"), bob.ActualPickupTime)
	assert.Equal(t, new("Zahnarzt"), bob.PendingExcusedNote)

	assert.Equal(t, []PickupTime{{
		StudentID: 11, Date: string(today), WeekdayName: "Freitag", PickupTime: new("15:30"),
		IsException: true, Notes: "Oma", DayNotes: []DayNote{{ID: 7, Content: "Klingeln"}},
	}}, got.PickupTimes)
	assert.Equal(t, map[string]RoomStatus{
		"11": {InGroupRoom: true, CurrentRoomID: new(int64(31))},
		"12": {InGroupRoom: false, CurrentRoomID: new(int64(32))},
	}, got.RoomStatus)
	assert.Equal(t, TrackingIndicators{Labels: []string{"Hausaufgaben", "Mittag"}, Results: map[int64][]bool{11: {true, false}}}, got.TrackingIndicators)
	assert.Equal(t, []Transfer{{ID: 5, GroupID: 1, SubstituteStaffID: 9, SubstituteName: "Vera Vertretung", EndDate: "2026-08-28"}}, got.Transfers)
}

func TestGetPassesPresenceAndPlansIntoTheDayDecision(t *testing.T) {
	t.Parallel()

	const (
		absentID     int64 = 101
		presentID    int64 = 102
		checkedOutID int64 = 103
		plannedID    int64 = 104
		sickID       int64 = 105
	)
	pickupAt := time.Date(2026, time.August, 21, 15, 0, 0, 0, time.UTC)
	exception := Arrival{IsException: true, Notes: "Zahnarzt"}
	seen := map[int64]DayInputs{}
	f := &fakes{
		caller:           fullCaller,
		supervisedGroups: func(context.Context) ([]GroupRecord, error) { return twoGroups(), nil },
		members: func(context.Context, int64) ([]RosterStudent, error) {
			return []RosterStudent{{ID: absentID}, {ID: presentID}, {ID: checkedOutID}, {ID: plannedID}, {ID: sickID, Sick: true}}, nil
		},
		snapshot: func(context.Context, []int64, Date) (PresenceSnapshot, error) {
			return presenceFake{attendances: map[int64]Attendance{
				presentID:    {Recorded: true, Present: true},
				checkedOutID: {Recorded: true},
				plannedID:    {Recorded: true, Present: true},
				sickID:       {Recorded: true, Present: true},
			}}, nil
		},
		arrivals: func(context.Context, []int64, Date) (map[int64]Arrival, error) {
			return map[int64]Arrival{absentID: exception, presentID: exception, checkedOutID: exception}, nil
		},
		pickups: func(context.Context, []int64, Date) (map[int64]Pickup, error) {
			return map[int64]Pickup{plannedID: {PickupTime: &pickupAt}}, nil
		},
		timetable: func(context.Context, []int64, Date) (map[int64]bool, error) {
			return map[int64]bool{sickID: true}, nil
		},
	}
	f.decide = func(inputs DayInputs) DayDecision {
		switch {
		case inputs.Sick:
			seen[sickID] = inputs
		case inputs.Pickup != nil:
			seen[plannedID] = inputs
		case inputs.Present:
			seen[presentID] = inputs
		case inputs.Arrival != nil && seen[absentID].Arrival == nil:
			seen[absentID] = inputs
		default:
			seen[checkedOutID] = inputs
		}
		if inputs.Arrival != nil && inputs.Arrival.ArrivalTime == nil && inputs.Arrival.IsException {
			return DayDecision{Reason: DayReasonArrivalException, ExceptionNotes: inputs.Arrival.Notes}
		}
		return DayDecision{ComesToday: true, Reason: DayReasonPickupSchedule}
	}

	got, err := newService(f).LiveGroup(context.Background(), 1)
	require.NoError(t, err)

	assert.Equal(t, DayInputs{Arrival: &exception}, seen[absentID])
	assert.Equal(t, DayInputs{Present: true, Arrival: &exception}, seen[presentID])
	assert.Equal(t, DayInputs{Arrival: &exception}, seen[checkedOutID], "a completed attendance is no longer present")
	assert.Equal(t, DayInputs{Present: true, Pickup: &Pickup{PickupTime: &pickupAt}}, seen[plannedID])
	assert.Equal(t, DayInputs{Present: true, Sick: true, HasTimetable: true}, seen[sickID])

	byID := map[int64]Student{}
	for _, student := range got.Students {
		byID[student.ID] = student
	}
	assert.Equal(t, "Zahnarzt", byID[absentID].DayPlanningLabel, "a timeless exception renders its note")
	assert.Equal(t, "not_coming_today", byID[absentID].DayPlanningStatus)
	assert.Equal(t, "Abholplan heute", byID[plannedID].DayPlanningLabel)
	assert.Equal(t, "comes_today", byID[plannedID].DayPlanningStatus)
}

func TestGetRedactsRestrictedCallersButKeepsTheReviewersNote(t *testing.T) {
	t.Parallel()

	checkIn := time.Date(2026, time.August, 21, 8, 10, 0, 0, time.UTC)
	f := &fakes{
		restricted: true,
		caller: func(context.Context) (Caller, error) {
			return Caller{CanReadGroups: true, CanReviewExcusedRequests: true}, nil
		},
		supervisedGroups: func(context.Context) ([]GroupRecord, error) { return twoGroups(), nil },
		members: func(context.Context, int64) ([]RosterStudent, error) {
			return []RosterStudent{{ID: 11, FirstName: "Ada", PhotoPath: new("/uploads/student-photos/ada.jpg")}}, nil
		},
		snapshot: func(context.Context, []int64, Date) (PresenceSnapshot, error) {
			return presenceFake{
				locations:   map[int64]Location{11: {Name: "Raum A", Since: &checkIn}},
				redacted:    map[int64]Location{11: {Name: "Anwesend"}},
				attendances: map[int64]Attendance{11: {Recorded: true, Present: true, CheckInTime: &checkIn}},
				rooms:       map[int64]int64{11: 31},
			}, nil
		},
		photosEnabled: func(context.Context) (bool, error) { return true, nil },
		arrivals: func(_ context.Context, ids []int64, _ Date) (map[int64]Arrival, error) {
			assert.Empty(t, ids, "plans are only loaded for full-access students")
			return map[int64]Arrival{11: {ArrivalTime: &checkIn}}, nil
		},
		pickups: func(context.Context, []int64, Date) (map[int64]Pickup, error) {
			return map[int64]Pickup{11: {PickupTime: &checkIn}}, nil
		},
		pendingByStudent: func(context.Context, excusedrequests.Date) (map[int64]*excusedrequests.Request, error) {
			return map[int64]*excusedrequests.Request{11: {Note: "Arzttermin"}}, nil
		},
	}

	got, err := newService(f).LiveGroup(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, got.Students, 1)
	student := got.Students[0]
	assert.Equal(t, "Anwesend", student.Location)
	assert.Nil(t, student.LocationSince)
	assert.Empty(t, student.PhotoURL)
	assert.Empty(t, student.DayPlanningStatus, "day plans stay gated on full access")
	assert.Nil(t, student.ArrivalTime)
	assert.Nil(t, student.ActualArrivalTime)
	assert.Equal(t, new("Arzttermin"), student.PendingExcusedNote, "the reviewer's note reaches a redacted child")
	assert.Equal(t, []PickupTime{}, got.PickupTimes)
	assert.Equal(t, map[string]RoomStatus{"11": {InGroupRoom: true, CurrentRoomID: new(int64(31))}}, got.RoomStatus,
		"room status follows groups:read, not student access")
}

func TestGetGatesPendingNotesOnTheReviewPermission(t *testing.T) {
	t.Parallel()

	base := func() *fakes {
		return &fakes{
			supervisedGroups: func(context.Context) ([]GroupRecord, error) { return twoGroups(), nil },
			members: func(context.Context, int64) ([]RosterStudent, error) {
				return []RosterStudent{{ID: 11}}, nil
			},
			pendingByStudent: func(context.Context, excusedrequests.Date) (map[int64]*excusedrequests.Request, error) {
				return map[int64]*excusedrequests.Request{11: {Note: "geheim"}}, nil
			},
		}
	}

	f := base()
	f.caller = func(context.Context) (Caller, error) { return Caller{}, nil }
	got, err := newService(f).LiveGroup(context.Background(), 1)
	require.NoError(t, err)
	assert.Nil(t, got.Students[0].PendingExcusedNote)
	assert.Equal(t, 0, f.pendingCalls, "a caller who may not review never queries the queue")

	f = base()
	f.caller = fullCaller
	got, err = New(Dependencies{Access: f, Groups: f, Roster: f, Presence: f, Planning: f, Transfers: f, Settings: f, Calendar: f}).
		LiveGroup(context.Background(), 1)
	require.NoError(t, err)
	assert.Nil(t, got.Students[0].PendingExcusedNote, "without a Care Plan reader no note is attached")
}

func TestGetSurfacesEveryOwnerFailure(t *testing.T) {
	t.Parallel()

	failure := errors.New("owner unavailable")
	fail := func(t *testing.T, mutate func(*fakes), wantPrefix string) {
		t.Helper()
		f := &fakes{
			caller:           fullCaller,
			supervisedGroups: func(context.Context) ([]GroupRecord, error) { return twoGroups(), nil },
			members: func(context.Context, int64) ([]RosterStudent, error) {
				return []RosterStudent{{ID: 11}}, nil
			},
			trackingLabels: func(context.Context) ([]string, error) { return []string{"Mittag"}, nil },
		}
		mutate(f)
		got, err := newService(f).LiveGroup(context.Background(), 1)
		require.ErrorIs(t, err, failure)
		assert.True(t, strings.HasPrefix(err.Error(), wantPrefix), "error %q should start with %q", err, wantPrefix)
		assert.Nil(t, got, "a failing owner query never yields a partial projection")
	}

	cases := []struct {
		name   string
		mutate func(*fakes)
		prefix string
	}{
		{"settings", func(f *fakes) {
			f.prepare = func(ctx context.Context) (context.Context, error) { return ctx, failure }
		}, "resolve projection settings"},
		{"caller", func(f *fakes) {
			f.caller = func(context.Context) (Caller, error) { return Caller{}, failure }
		}, "resolve caller access"},
		{"supervised groups", func(f *fakes) {
			f.supervisedGroups = func(context.Context) ([]GroupRecord, error) { return nil, failure }
		}, "load supervised groups"},
		{"tenant groups", func(f *fakes) {
			f.caller = func(context.Context) (Caller, error) {
				return Caller{CanReadGroups: true, OperationalOverview: true}, nil
			}
			f.tenantGroups = func(context.Context) ([]GroupRecord, error) { return nil, failure }
		}, "load tenant groups"},
		{"substitutions", func(f *fakes) {
			f.substitutedGroups = func(context.Context) (map[int64]bool, error) { return nil, failure }
		}, "load substitution metadata"},
		{"room names", func(f *fakes) {
			f.roomNames = func(context.Context, []int64) (map[int64]string, error) { return nil, failure }
		}, "load group rooms"},
		{"members", func(f *fakes) {
			f.members = func(context.Context, int64) ([]RosterStudent, error) { return nil, failure }
		}, "load group students"},
		{"care participation", func(f *fakes) {
			f.participants = func(context.Context, []int64, Date) (map[int64]bool, error) { return nil, failure }
		}, "apply care participation"},
		{"presence", func(f *fakes) {
			f.snapshot = func(context.Context, []int64, Date) (PresenceSnapshot, error) { return nil, failure }
		}, "load student locations"},
		{"photos setting", func(f *fakes) {
			f.photosEnabled = func(context.Context) (bool, error) { return false, failure }
		}, "resolve student photos setting"},
		{"status days", func(f *fakes) {
			f.statuses = func(context.Context, []int64, Date) (map[int64]EffectiveStatus, error) { return nil, failure }
		}, "apply student status days"},
		{"arrivals", func(f *fakes) {
			f.arrivals = func(context.Context, []int64, Date) (map[int64]Arrival, error) { return nil, failure }
		}, "load arrival times"},
		{"pickups", func(f *fakes) {
			f.pickups = func(context.Context, []int64, Date) (map[int64]Pickup, error) { return nil, failure }
		}, "load pickup times"},
		{"timetable", func(f *fakes) {
			f.timetable = func(context.Context, []int64, Date) (map[int64]bool, error) { return nil, failure }
		}, "load timetable planning"},
		{"pending requests", func(f *fakes) {
			f.pendingByStudent = func(context.Context, excusedrequests.Date) (map[int64]*excusedrequests.Request, error) {
				return nil, failure
			}
		}, "load pending excused requests"},
		{"tracking labels", func(f *fakes) {
			f.trackingLabels = func(context.Context) ([]string, error) { return nil, failure }
		}, "load tracking indicators"},
		{"tracking results", func(f *fakes) {
			f.tracking = func(context.Context, []int64, []string) (map[int64][]bool, error) { return nil, failure }
		}, "load tracking indicators"},
		{"transfers", func(f *fakes) {
			f.handovers = func(context.Context, int64, Date) ([]Transfer, error) { return nil, failure }
		}, "load group transfers"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fail(t, tc.mutate, tc.prefix)
		})
	}
}

func TestGetSkipsStatusDaysAndTrackingForAnEmptyRoster(t *testing.T) {
	t.Parallel()

	f := &fakes{
		caller:           fullCaller,
		supervisedGroups: func(context.Context) ([]GroupRecord, error) { return twoGroups(), nil },
		statuses: func(context.Context, []int64, Date) (map[int64]EffectiveStatus, error) {
			t.Fatal("status days must not be queried for an empty roster")
			return nil, nil
		},
		trackingLabels: func(context.Context) ([]string, error) { return []string{"Mittag"}, nil },
	}

	got, err := newService(f).LiveGroup(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, []Student{}, got.Students)
	assert.Equal(t, 0, f.trackingCalls)
	assert.Equal(t, emptyTracking(), got.TrackingIndicators)
	assert.Equal(t, map[string]RoomStatus{}, got.RoomStatus)
}

func TestListGroupsToleratesMissingRoomNames(t *testing.T) {
	t.Parallel()

	f := &fakes{
		caller:           fullCaller,
		supervisedGroups: func(context.Context) ([]GroupRecord, error) { return twoGroups(), nil },
		roomNames: func(context.Context, []int64) (map[int64]string, error) {
			return nil, errors.New("rooms unavailable")
		},
	}

	got, err := newService(f).ListGroups(context.Background())
	require.NoError(t, err, "the group list degrades to nameless rooms instead of failing")
	assert.Equal(t, 0, f.accessCalls, "the group list never resolves student access")
	assert.Equal(t, []Group{{ID: 1, Name: "Äpfel", RoomID: new(int64(31)), IsPersonal: true}, {ID: 2, Name: "Zebra", IsPersonal: true}}, got)

	empty, err := newService(&fakes{caller: fullCaller}).ListGroups(context.Background())
	require.NoError(t, err)
	assert.Nil(t, empty)
}

func TestNewRejectsIncompleteWiring(t *testing.T) {
	t.Parallel()

	f := &fakes{}
	_, err := New(Dependencies{}).LiveGroup(context.Background(), 0)
	assert.EqualError(t, err, "OGS group live service is not fully configured")
	_, err = New(Dependencies{}).ListGroups(context.Background())
	assert.EqualError(t, err, "OGS group live service is not fully configured")
	_, err = New(Dependencies{Access: f, Groups: f}).LiveGroup(context.Background(), 0)
	assert.EqualError(t, err, "OGS group live service is not fully configured", "group wiring alone cannot build the projection")

	got, ok := New(Dependencies{}).(*service)
	require.True(t, ok)
	assert.NotNil(t, got.deps.Logger)
}

func TestProjectionSerializesAllIDsAsStrings(t *testing.T) {
	t.Parallel()

	id := int64(9223372036854775807)
	projection := Projection{
		Groups:      []Group{{ID: id, RoomID: &id}},
		GroupID:     &id,
		Students:    []Student{{ID: id}},
		RoomStatus:  map[string]RoomStatus{"student": {CurrentRoomID: &id}},
		PickupTimes: []PickupTime{{StudentID: id, DayNotes: []DayNote{{ID: id}}}},
		TrackingIndicators: TrackingIndicators{
			Results: map[int64][]bool{id: {true}},
		},
		Transfers: []Transfer{{ID: id, GroupID: id, SubstituteStaffID: id}},
	}

	body, err := json.Marshal(projection)
	require.NoError(t, err)

	var wire struct {
		Groups []struct {
			ID     string  `json:"id"`
			RoomID *string `json:"room_id"`
		} `json:"groups"`
		GroupID  *string `json:"group_id"`
		Students []struct {
			ID string `json:"id"`
		} `json:"students"`
		RoomStatus map[string]struct {
			CurrentRoomID *string `json:"current_room_id"`
		} `json:"room_status"`
		PickupTimes []struct {
			StudentID string `json:"student_id"`
			DayNotes  []struct {
				ID string `json:"id"`
			} `json:"day_notes"`
		} `json:"pickup_times"`
		TrackingIndicators struct {
			Results map[string][]bool `json:"results"`
		} `json:"tracking_indicators"`
		Transfers []struct {
			ID                string `json:"id"`
			GroupID           string `json:"group_id"`
			SubstituteStaffID string `json:"substitute_staff_id"`
		} `json:"transfers"`
	}
	require.NoError(t, json.Unmarshal(body, &wire), "wire IDs must be JSON strings: %s", body)

	want := "9223372036854775807"
	assert.Equal(t, want, wire.Groups[0].ID)
	assert.Equal(t, want, *wire.Groups[0].RoomID)
	assert.Equal(t, want, *wire.GroupID)
	assert.Equal(t, want, wire.Students[0].ID)
	assert.Equal(t, want, *wire.RoomStatus["student"].CurrentRoomID)
	assert.Equal(t, want, wire.PickupTimes[0].StudentID)
	assert.Equal(t, want, wire.PickupTimes[0].DayNotes[0].ID)
	assert.Equal(t, []bool{true}, wire.TrackingIndicators.Results[want])
	assert.Equal(t, want, wire.Transfers[0].ID)
	assert.Equal(t, want, wire.Transfers[0].GroupID)
	assert.Equal(t, want, wire.Transfers[0].SubstituteStaffID)
}

// TestProjectionWireGolden pins the JSON document the frontend group page
// reads, field names and omissions included, so the cutover from the former
// service keeps byte-for-byte output parity.
func TestProjectionWireGolden(t *testing.T) {
	t.Parallel()

	since := time.Date(2026, time.August, 21, 6, 30, 0, 0, time.UTC)
	projection := Projection{
		Groups:  []Group{{ID: 1, Name: "Äpfel", RoomID: new(int64(31)), RoomName: "Raum A", IsPersonal: true}},
		GroupID: new(appleGroupID),
		Students: []Student{{
			ID: 11, FirstName: "Ada", LastName: "Lovelace", SchoolClass: "2a", Location: "Raum A",
			LocationSince: &since, RoomColor: new("#A3D977"), Excused: true, ExcusedSince: &since,
			DayPlanningStatus: "comes_today", DayPlanningReason: "arrival_exception", DayPlanningLabel: "geplante Ankunft heute",
			PendingExcusedNote: new("Zahnarzt"), ArrivalTime: new("07:45"), ArrivalIsException: true, ArrivalNotes: "später",
			ActualArrivalTime: new("08:10"), PhotoURL: "/api/students/11/photo/ada.jpg",
		}, {ID: 12, FirstName: "Bob", LastName: "Builder", SchoolClass: "2b", Location: "Abwesend"}},
		RoomStatus:         map[string]RoomStatus{"11": {InGroupRoom: true, CurrentRoomID: new(int64(31))}, "12": {}},
		PickupTimes:        []PickupTime{{StudentID: 11, Date: string(today), WeekdayName: "Freitag", PickupTime: new("15:30"), IsException: true, Notes: "Oma", DayNotes: []DayNote{{ID: 7, Content: "Klingeln"}}}},
		TrackingIndicators: TrackingIndicators{Labels: []string{"Mittag"}, Results: map[int64][]bool{11: {true}}},
		Transfers:          []Transfer{{ID: 5, GroupID: 1, SubstituteStaffID: 9, SubstituteName: "Vera Vertretung", EndDate: "2026-08-28"}},
	}

	body, err := json.Marshal(projection)
	require.NoError(t, err)

	const want = `{"groups":[{"id":"1","name":"Äpfel","room_id":"31","room_name":"Raum A","via_substitution":false,"is_personal":true}],` +
		`"group_id":"1",` +
		`"students":[{"id":"11","first_name":"Ada","last_name":"Lovelace","school_class":"2a","current_location":"Raum A",` +
		`"location_since":"2026-08-21T06:30:00Z","current_room_color":"#A3D977","sick":false,"excused":true,` +
		`"excused_since":"2026-08-21T06:30:00Z","class_trip":false,"day_planning_status":"comes_today",` +
		`"day_planning_reason":"arrival_exception","day_planning_label":"geplante Ankunft heute","pending_excused_note":"Zahnarzt",` +
		`"arrival_time":"07:45","arrival_is_exception":true,"arrival_notes":"später","actual_arrival_time":"08:10",` +
		`"photo_url":"/api/students/11/photo/ada.jpg"},` +
		`{"id":"12","first_name":"Bob","last_name":"Builder","school_class":"2b","current_location":"Abwesend","sick":false,"excused":false,"class_trip":false}],` +
		`"room_status":{"11":{"in_group_room":true,"current_room_id":"31"},"12":{"in_group_room":false}},` +
		`"pickup_times":[{"student_id":"11","date":"2026-08-21","weekday_name":"Freitag","pickup_time":"15:30","is_exception":true,"notes":"Oma","day_notes":[{"id":"7","content":"Klingeln"}]}],` +
		`"tracking_indicators":{"labels":["Mittag"],"results":{"11":[true]}},` +
		`"transfers":[{"id":"5","group_id":"1","substitute_staff_id":"9","substitute_name":"Vera Vertretung","end_date":"2026-08-28"}]}`
	assert.JSONEq(t, want, string(body))
	assert.Equal(t, want, string(body), "field order is part of the pinned document")

	empty, err := json.Marshal(EmptyProjection())
	require.NoError(t, err)
	assert.Equal(t, `{"groups":[],"students":[],"room_status":{},"pickup_times":[],"tracking_indicators":{"labels":[],"results":{}},"transfers":[]}`, string(empty))
}

func TestApplyEffectiveStatusHonorsPrecedence(t *testing.T) {
	t.Parallel()

	reportedAt := time.Date(2026, time.August, 1, 8, 30, 0, 0, time.UTC)

	tests := []struct {
		name   string
		start  Student
		status EffectiveStatus
		want   Student
	}{
		{
			name:   "sick clears lower-priority states",
			start:  Student{ClassTrip: true, Excused: true},
			status: EffectiveStatus{Sick: true, SickSince: &reportedAt},
			want:   Student{Sick: true, SickSince: &reportedAt},
		},
		{
			name:   "class trip clears lower-priority states",
			start:  Student{Sick: true, Excused: true},
			status: EffectiveStatus{ClassTrip: true, ClassTripSince: &reportedAt},
			want:   Student{ClassTrip: true, ClassTripSince: &reportedAt},
		},
		{
			name:   "excused applies when not sick",
			status: EffectiveStatus{Excused: true, ExcusedSince: &reportedAt},
			want:   Student{Excused: true, ExcusedSince: &reportedAt},
		},
		{
			name:   "excused does not override sickness",
			start:  Student{Sick: true},
			status: EffectiveStatus{Excused: true, ExcusedSince: &reportedAt},
			want:   Student{Sick: true},
		},
		{
			name: "empty status leaves the student untouched",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.start
			applyEffectiveStatus(&got, tt.status)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPlanningLabelDescribesEveryPlanningReason(t *testing.T) {
	t.Parallel()

	tests := []struct {
		decision DayDecision
		want     string
	}{
		{DayDecision{Reason: DayReasonUnplanned}, "ungeplant anwesend"},
		{DayDecision{Reason: DayReasonSick}, "krank gemeldet"},
		{DayDecision{Reason: DayReasonClassTrip}, "Klassenfahrt"},
		{DayDecision{Reason: DayReasonExcused}, "entschuldigt"},
		{DayDecision{Reason: DayReasonArrivalException, ComesToday: true}, "geplante Ankunft heute"},
		{DayDecision{Reason: DayReasonArrivalException, ExceptionNotes: "Arzttermin"}, "Arzttermin"},
		{DayDecision{Reason: DayReasonArrivalException}, "Tagesausnahme"},
		{DayDecision{Reason: DayReasonArrivalSchedule}, "Ankunftsplan heute"},
		{DayDecision{Reason: DayReasonPickupException, ComesToday: true}, "geplante Abholung heute"},
		{DayDecision{Reason: DayReasonPickupException, ExceptionNotes: "Familientag"}, "Familientag"},
		{DayDecision{Reason: DayReasonPickupException}, "Tagesausnahme"},
		{DayDecision{Reason: DayReasonPickupSchedule}, "Abholplan heute"},
		{DayDecision{Reason: DayReasonTimetable}, "Betreuungsplan heute"},
		{DayDecision{Reason: DayReasonNoPlan}, "kein Plan für heute"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.want, planningLabel(tt.decision))
	}
}

func TestBuildPhotoURLRewritesOnlyStoredUploads(t *testing.T) {
	t.Parallel()

	assert.Empty(t, buildPhotoURL(5, ""))
	assert.Equal(t, "https://example.invalid/x.jpg", buildPhotoURL(5, "https://example.invalid/x.jpg"),
		"non-upload URLs pass through untouched")
	assert.Equal(t, "/api/students/5/photo/abc.jpg", buildPhotoURL(5, "/uploads/student-photos/abc.jpg"),
		"stored upload paths are rewritten to the authenticated proxy")
}
