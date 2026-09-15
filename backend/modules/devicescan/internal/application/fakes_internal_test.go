package application

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/facilities"
)

// The workflow is exercised over fakes of its ports and of the narrow
// capability slices it reads. Instants are Berlin wall clocks, which is what
// the production clock adapter derives too.

var berlin = mustLoadBerlin()

func mustLoadBerlin() *time.Location {
	location, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		panic(err)
	}
	return location
}

// fixedNow is the admitted instant of every scan in these tests.
var fixedNow = time.Date(2026, time.August, 5, 9, 0, 0, 0, berlin)

const (
	testDeviceID   = int64(91)
	testDeviceName = "kiosk-eingang"
	testStudentID  = int64(500)
	testPersonID   = int64(700)
	testStaffID    = int64(42)
	testRoomID     = int64(42)
	testTag        = "A1B2C3D4"
)

type fakeClock struct{ now time.Time }

func (c fakeClock) Now() time.Time                      { return c.now }
func (c fakeClock) Day(instant time.Time) timezone.Date { return timezone.DateFromTime(instant) }

type fakeUnit struct{ rollbacks int }

func (u *fakeUnit) MarkRollback(context.Context) { u.rollbacks++ }

func (*fakeUnit) RequireTransaction(context.Context) error { return nil }

type fakePrincipals struct {
	device *ports.Device
	staff  *ports.Staff
}

func (p fakePrincipals) Device(context.Context) (*ports.Device, bool) {
	return p.device, p.device != nil
}
func (p fakePrincipals) Staff(context.Context) (*ports.Staff, bool) { return p.staff, p.staff != nil }

func testDevice() *ports.Device {
	name := testDeviceName
	lastSeen := fixedNow.Add(-time.Minute)
	return &ports.Device{ID: testDeviceID, DeviceID: "dev-001", DeviceType: "rfid", Name: &name, Status: "active", LastSeen: &lastSeen, Active: true}
}

type fakeFleet struct {
	online   bool
	devices  map[string]devicefleet.Device
	findErr  error
	seen     []int64
	seenErr  error
	scans    []devicefleet.RecordUnregisteredTagScan
	scansErr error
}

func (f *fakeFleet) IsDeviceOnline(context.Context, devicefleet.Device) bool { return f.online }
func (f *fakeFleet) FindDeviceByDeviceID(_ context.Context, id string) (devicefleet.Device, error) {
	if f.findErr != nil {
		return devicefleet.Device{}, f.findErr
	}
	device, ok := f.devices[id]
	if !ok {
		return devicefleet.Device{}, devicefleet.ErrDeviceNotFound
	}
	return device, nil
}
func (f *fakeFleet) UpdateDeviceLastSeen(_ context.Context, id int64, _ time.Time) error {
	f.seen = append(f.seen, id)
	return f.seenErr
}
func (f *fakeFleet) RecordUnregisteredTagScan(_ context.Context, scan devicefleet.RecordUnregisteredTagScan) (devicefleet.UnregisteredTagScan, error) {
	f.scans = append(f.scans, scan)
	return devicefleet.UnregisteredTagScan{TagUID: scan.TagUID}, f.scansErr
}

type fakePresence struct {
	roomCounts  map[int64]int
	groupCounts map[int64]int
	roomErr     error
	groupErr    error
}

func (p *fakePresence) CountOpenVisitsInRoom(_ context.Context, id int64) (int, error) {
	return p.roomCounts[id], p.roomErr
}
func (p *fakePresence) CountOpenVisitsInGroup(_ context.Context, id int64) (int, error) {
	return p.groupCounts[id], p.groupErr
}

type fakeRooms struct {
	rooms     map[int64]facilities.Room
	byName    map[string]facilities.Room
	toilet    *facilities.Room
	toiletErr error
	findErr   error
	nameErr   error
	created   []facilities.CreateRoom
	createErr error
	nextID    int64
}

func (r *fakeRooms) FindRoom(_ context.Context, id int64) (facilities.Room, error) {
	if r.findErr != nil {
		return facilities.Room{}, r.findErr
	}
	room, ok := r.rooms[id]
	if !ok {
		return facilities.Room{}, facilities.ErrRoomNotFound
	}
	return room, nil
}
func (r *fakeRooms) FindRoomByName(_ context.Context, name string) (facilities.Room, error) {
	if r.nameErr != nil {
		return facilities.Room{}, r.nameErr
	}
	room, ok := r.byName[name]
	if !ok {
		return facilities.Room{}, facilities.ErrRoomNotFound
	}
	return room, nil
}
func (r *fakeRooms) FindToiletRoom(context.Context, int64) (facilities.Room, error) {
	if r.toiletErr != nil {
		return facilities.Room{}, r.toiletErr
	}
	if r.toilet == nil {
		return facilities.Room{}, facilities.ErrRoomNotFound
	}
	return *r.toilet, nil
}
func (r *fakeRooms) CreateRoom(_ context.Context, input facilities.CreateRoom) (facilities.Room, error) {
	r.created = append(r.created, input)
	if r.createErr != nil {
		return facilities.Room{}, r.createErr
	}
	r.nextID++
	room := facilities.Room{ID: 900 + r.nextID, Name: input.Name, Capacity: input.Capacity, IsSystem: input.IsSystem, IsOpenRoom: input.IsOpenRoom}
	if r.rooms == nil {
		r.rooms = map[int64]facilities.Room{}
	}
	r.rooms[room.ID] = room
	if r.byName == nil {
		r.byName = map[string]facilities.Room{}
	}
	r.byName[room.Name] = room
	if facilities.IsToiletRoomName(room.Name) {
		r.toilet = &room
	}
	return room, nil
}

type fakePeople struct {
	persons    map[string]*ports.Person
	personErr  error
	students   map[int64]*ports.Student
	studentErr error
	staff      map[int64]*ports.StaffMember
	staffErr   error
}

func (p *fakePeople) NormalizeTag(tag string) string { return tag }
func (p *fakePeople) FindPersonByTag(_ context.Context, tag string) (*ports.Person, error) {
	if p.personErr != nil {
		return nil, p.personErr
	}
	person, ok := p.persons[tag]
	if !ok {
		return nil, ports.ErrPersonNotFound
	}
	return person, nil
}
func (p *fakePeople) FindStudentByPerson(_ context.Context, personID int64) (*ports.Student, error) {
	if p.studentErr != nil {
		return nil, p.studentErr
	}
	return p.students[personID], nil
}
func (p *fakePeople) FindStaffByPerson(_ context.Context, personID int64) (*ports.StaffMember, error) {
	if p.staffErr != nil {
		return nil, p.staffErr
	}
	return p.staff[personID], nil
}

type recordedVisit struct{ StudentID, SessionID int64 }

type fakeVisits struct {
	current    *ports.CurrentVisit
	currentErr error
	// currentAfterRecord is what a duplicate-scan lookup finds after the
	// refused write, when it differs from the current visit.
	ended       []int64
	endErr      error
	recorded    []recordedVisit
	recordErr   error
	nextVisitID int64
}

func (v *fakeVisits) Current(context.Context, int64) (*ports.CurrentVisit, error) {
	return v.current, v.currentErr
}
func (v *fakeVisits) End(_ context.Context, id int64) error {
	v.ended = append(v.ended, id)
	return v.endErr
}
func (v *fakeVisits) Record(_ context.Context, studentID, sessionID int64) (int64, error) {
	v.recorded = append(v.recorded, recordedVisit{StudentID: studentID, SessionID: sessionID})
	if v.recordErr != nil {
		return 0, v.recordErr
	}
	v.nextVisitID++
	return 1000 + v.nextVisitID, nil
}

type fakeSessions struct {
	open           []ports.Session
	listErr        error
	deviceSession  *ports.Session
	deviceErr      error
	current        *ports.Session
	currentErr     error
	started        []ports.NewSession
	startErr       error
	nextSessionID  int64
	deleted        []int64
	ended          []int64
	endErr         error
	touched        []int64
	touchErr       error
	supervisors    []ports.Supervisor
	supervisorsErr error
	replaced       [][]int64
	replaceErr     error
}

func (s *fakeSessions) ListOpenInRoom(context.Context, int64) ([]ports.Session, error) {
	return s.open, s.listErr
}
func (s *fakeSessions) FindDeviceSessionInRoom(context.Context, int64, int64) (*ports.Session, error) {
	return s.deviceSession, s.deviceErr
}
func (s *fakeSessions) Current(context.Context, int64) (*ports.Session, error) {
	return s.current, s.currentErr
}
func (s *fakeSessions) Start(_ context.Context, input ports.NewSession) (ports.Session, error) {
	s.started = append(s.started, input)
	if s.startErr != nil {
		return ports.Session{}, s.startErr
	}
	s.nextSessionID++
	activityID := input.ActivityID
	return ports.Session{ID: 200 + s.nextSessionID, RoomID: input.RoomID, StartTime: fixedNow, TemplateID: &activityID}, nil
}
func (s *fakeSessions) Delete(_ context.Context, id int64) error {
	s.deleted = append(s.deleted, id)
	return nil
}
func (s *fakeSessions) End(_ context.Context, id int64) error {
	s.ended = append(s.ended, id)
	return s.endErr
}
func (s *fakeSessions) Touch(_ context.Context, id int64) error {
	s.touched = append(s.touched, id)
	return s.touchErr
}
func (s *fakeSessions) Supervisors(context.Context, int64) ([]ports.Supervisor, error) {
	return s.supervisors, s.supervisorsErr
}
func (s *fakeSessions) ReplaceSupervisors(_ context.Context, _ int64, staffIDs []int64) error {
	s.replaced = append(s.replaced, staffIDs)
	return s.replaceErr
}

type toggleCall struct {
	StudentID, StaffID, DeviceID int64
	SkipAuthCheck                bool
}

type fakeAttendance struct {
	state        *ports.AttendanceState
	statusErr    error
	toggleAction string
	toggleErr    error
	toggles      []toggleCall
	dailyAction  string
	dailyErr     error
	dailyCalls   []string
}

func (a *fakeAttendance) Status(context.Context, int64) (*ports.AttendanceState, error) {
	return a.state, a.statusErr
}
func (a *fakeAttendance) Toggle(_ context.Context, studentID, staffID, deviceID int64, skipAuthCheck bool) (string, error) {
	a.toggles = append(a.toggles, toggleCall{StudentID: studentID, StaffID: staffID, DeviceID: deviceID, SkipAuthCheck: skipAuthCheck})
	return a.toggleAction, a.toggleErr
}
func (a *fakeAttendance) ConfirmDailyCheckout(_ context.Context, _, _ int64, destination string) (string, error) {
	a.dailyCalls = append(a.dailyCalls, destination)
	return a.dailyAction, a.dailyErr
}

type fakeActivities struct {
	byID       map[int64]*ports.Activity
	findErr    error
	byName     map[string][]ports.Activity
	listErr    error
	listCalls  int
	categories []ports.Category
	created    []ports.NewActivity
	createErr  error
}

func (a *fakeActivities) Find(_ context.Context, id int64) (*ports.Activity, error) {
	return a.byID[id], a.findErr
}
func (a *fakeActivities) ListByName(_ context.Context, name string) ([]ports.Activity, error) {
	a.listCalls++
	return a.byName[name], a.listErr
}
func (a *fakeActivities) ListCategories(context.Context) ([]ports.Category, error) {
	return a.categories, nil
}
func (a *fakeActivities) CreateCategory(_ context.Context, input ports.NewCategory) (ports.Category, error) {
	category := ports.Category{ID: int64(len(a.categories) + 1), Name: input.Name}
	a.categories = append(a.categories, category)
	return category, nil
}
func (a *fakeActivities) CreateActivity(_ context.Context, input ports.NewActivity) (ports.Activity, error) {
	a.created = append(a.created, input)
	if a.createErr != nil {
		return ports.Activity{}, a.createErr
	}
	activity := ports.Activity{ID: 300 + int64(len(a.created)), Name: input.Name, MaxParticipants: input.MaxParticipants, PlannedRoomID: input.PlannedRoomID, IsSystem: input.IsSystem, IsOpen: input.IsOpen}
	if a.byName == nil {
		a.byName = map[string][]ports.Activity{}
	}
	a.byName[input.Name] = append(a.byName[input.Name], activity)
	return activity, nil
}

type fakeGroups struct {
	group *ports.Group
	err   error
}

func (g fakeGroups) Find(context.Context, int64) (*ports.Group, error) { return g.group, g.err }

type fakePickups struct {
	pickup *ports.Pickup
	err    error
}

func (p fakePickups) Effective(context.Context, int64, timezone.Date) (*ports.Pickup, error) {
	return p.pickup, p.err
}

type fakeSettings struct {
	mode             string
	modeErr          error
	feedback         bool
	feedbackErr      error
	roomDetails      bool
	activityDetails  bool
	detailsErr       error
	dailyCheckout    string
	dailyCheckoutErr error
	perStudent       bool
	delta            int
	deltaErr         error
	allRooms         bool
	allRoomsErr      error
}

func (s fakeSettings) PresenceMode(context.Context) (string, error) {
	if s.modeErr != nil {
		return "", s.modeErr
	}
	if s.mode == "" {
		return presenceModeDetailed, nil
	}
	return s.mode, nil
}
func (s fakeSettings) FeedbackEnabled(context.Context) (bool, error) {
	return s.feedback, s.feedbackErr
}
func (s fakeSettings) CapacityDetailsDisclosed(_ context.Context, kind ports.CapacityKind) (bool, error) {
	if kind == ports.CapacityActivity {
		return s.activityDetails, s.detailsErr
	}
	return s.roomDetails, s.detailsErr
}
func (s fakeSettings) DailyCheckoutTime(context.Context) (string, error) {
	return s.dailyCheckout, s.dailyCheckoutErr
}
func (s fakeSettings) PerStudentCheckoutEnabled(context.Context) (bool, error) {
	return s.perStudent, nil
}
func (s fakeSettings) PerStudentCheckoutDeltaMinutes(context.Context) (int, error) {
	return s.delta, s.deltaErr
}
func (s fakeSettings) DailyCheckoutFromAllRoomsEnabled(context.Context) (bool, error) {
	return s.allRooms, s.allRoomsErr
}

// harness bundles the fakes of one test service.
type harness struct {
	fleet      *fakeFleet
	presence   *fakePresence
	rooms      *fakeRooms
	principals *fakePrincipals
	people     *fakePeople
	visits     *fakeVisits
	sessions   *fakeSessions
	attendance *fakeAttendance
	activities *fakeActivities
	groups     *fakeGroups
	pickups    *fakePickups
	settings   *fakeSettings
	unit       *fakeUnit
	clock      fakeClock
	logger     *slog.Logger
}

// newHarness builds a service around a device, a student card and an
// ordinary room with a running session, ready for a plain check-in.
func newHarness(t *testing.T) *harness {
	t.Helper()
	sessionID := int64(201)
	templateID := int64(301)
	h := &harness{
		fleet:      &fakeFleet{online: true, devices: map[string]devicefleet.Device{"dev-001": {ID: testDeviceID, DeviceID: "dev-001"}}},
		presence:   &fakePresence{roomCounts: map[int64]int{}, groupCounts: map[int64]int{}},
		rooms:      &fakeRooms{rooms: map[int64]facilities.Room{testRoomID: {ID: testRoomID, Name: "Klassenraum 1a"}}, byName: map[string]facilities.Room{}},
		principals: &fakePrincipals{device: testDevice()},
		people: &fakePeople{
			persons:  map[string]*ports.Person{testTag: {ID: testPersonID, FirstName: "Max", LastName: "Muster", HasTag: true}},
			students: map[int64]*ports.Student{testPersonID: {ID: testStudentID, PersonID: testPersonID, SchoolClass: "1a"}},
			staff:    map[int64]*ports.StaffMember{},
		},
		visits: &fakeVisits{},
		sessions: &fakeSessions{open: []ports.Session{{
			ID: sessionID, RoomID: testRoomID, StartTime: fixedNow.Add(-time.Hour), TemplateID: &templateID,
			Activity: &ports.Activity{ID: templateID, Name: "Hausaufgaben", MaxParticipants: 20},
		}}},
		attendance: &fakeAttendance{},
		activities: &fakeActivities{byID: map[int64]*ports.Activity{}, byName: map[string][]ports.Activity{}},
		groups:     &fakeGroups{},
		pickups:    &fakePickups{},
		settings:   &fakeSettings{roomDetails: true},
		unit:       &fakeUnit{},
		clock:      fakeClock{now: fixedNow},
		logger:     slog.New(slog.DiscardHandler),
	}
	return h
}

func (h *harness) service() *Service {
	return NewService(Dependencies{
		Fleet: h.fleet, Presence: h.presence, Rooms: h.rooms, Principals: h.principals, People: h.people,
		Visits: h.visits, Sessions: h.sessions, Attendance: h.attendance, Activities: h.activities,
		Groups: h.groups, Pickups: h.pickups, Settings: h.settings, UnitOfWork: h.unit, Clock: h.clock, Logger: h.logger,
	})
}

var errBoom = errors.New("database unavailable")
