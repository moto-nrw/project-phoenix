package legacy

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/facilities"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type fakePresence struct {
	open     []int64
	openDate string
	visits   []studentpresence.VisitLocation
	filter   studentpresence.VisitLocationFilter
	statuses []studentpresence.SchoolStatus
	err      error
}

func (f *fakePresence) ListOpenAttendanceStudentIDs(_ context.Context, date string) ([]int64, error) {
	f.openDate = date
	return f.open, f.err
}

func (f *fakePresence) ListVisitLocations(_ context.Context, filter studentpresence.VisitLocationFilter) ([]studentpresence.VisitLocation, error) {
	f.filter = filter
	return f.visits, f.err
}

func (f *fakePresence) ListSchoolStatuses(_ context.Context, _ []int64, _ string) ([]studentpresence.SchoolStatus, error) {
	return f.statuses, f.err
}

type fakeMode struct {
	mode string
	err  error
}

func (f fakeMode) GetPresenceMode(_ context.Context) (string, error) { return f.mode, f.err }

type fakeStudents struct {
	rows map[int64]*usersModels.Student
	err  error
}

func (f fakeStudents) FindByIDs(_ context.Context, _ []int64) (map[int64]*usersModels.Student, error) {
	return f.rows, f.err
}

type fakePersons struct {
	rows []peopledirectory.Person
	err  error
}

func (f fakePersons) ListPersonsByID(_ context.Context, _ []int64) ([]peopledirectory.Person, error) {
	return f.rows, f.err
}

type fakeContacts struct {
	rows []usersModels.GuardianEmergencyContactRow
	err  error
}

func (f fakeContacts) ListEmergencyContactRows(_ context.Context, _ []int64) ([]usersModels.GuardianEmergencyContactRow, error) {
	return f.rows, f.err
}

type fakeRooms struct {
	rows []facilities.Room
	err  error
	ids  []int64
}

func (f *fakeRooms) ListRoomsByID(_ context.Context, ids []int64) ([]facilities.Room, error) {
	f.ids = ids
	return f.rows, f.err
}

type fakeSettings struct {
	enabled bool
	key     string
	err     error
}

func (f *fakeSettings) ResolveBool(_ context.Context, key string) (bool, error) {
	f.key = key
	return f.enabled, f.err
}

func fullSources() Sources {
	return Sources{
		Presence:     &fakePresence{},
		PresenceMode: fakeMode{mode: "detailed"},
		Students:     fakeStudents{},
		Persons:      fakePersons{},
		Contacts:     fakeContacts{},
		Rooms:        &fakeRooms{},
		Settings:     &fakeSettings{},
		Renderer:     listexport.NewService(),
	}
}

func TestNewRejectsMissingSources(t *testing.T) {
	t.Parallel()
	_, err := New(Sources{})
	require.ErrorIs(t, err, ErrIncompleteSources)

	sources := fullSources()
	sources.Renderer = nil
	_, err = New(sources)
	require.ErrorIs(t, err, ErrIncompleteSources)

	query, err := New(fullSources())
	require.NoError(t, err)
	require.NotNil(t, query)
}

func nullString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

// The retained service's fixture (#2609 tests): one child in the Kreativraum
// with three guardian rows, one child whose only contact is the legacy
// free-text guardian. The rendered document is the one the service rendered.
func TestExportRendersTheRetainedNotfallliste(t *testing.T) {
	t.Parallel()
	const kreativraum int64 = 7007
	presence := &fakePresence{
		open: []int64{101, 202},
		visits: []studentpresence.VisitLocation{{
			Visit: studentpresence.Visit{StudentID: 101},
			Group: &studentpresence.VisitGroup{RoomID: kreativraum},
		}},
	}
	rooms := &fakeRooms{rows: []facilities.Room{{ID: kreativraum, Name: "Kreativraum"}}}
	settings := &fakeSettings{enabled: true}
	sources := fullSources()
	sources.Presence = presence
	sources.Rooms = rooms
	sources.Settings = settings
	sources.Students = fakeStudents{rows: map[int64]*usersModels.Student{
		101: {PersonID: 301, SchoolClass: "Klasse 3b", HealthInfo: new("Nussallergie, Epipen im Gruppenraum")},
		202: {PersonID: 302, SchoolClass: "Klasse 2a", GuardianName: new("Familie Schmitt"), GuardianPhone: new("02551 444")},
	}}
	sources.Persons = fakePersons{rows: []peopledirectory.Person{
		{ID: 301, FirstName: "Mila", LastName: "Albrecht"},
		{ID: 302, FirstName: "Max", LastName: "Schmitt"},
	}}
	sources.Contacts = fakeContacts{rows: []usersModels.GuardianEmergencyContactRow{
		{StudentID: 101, FirstName: nullString("Lea"), LastName: nullString("Albrecht"), PhoneNumber: nullString("02551 111")},
		{StudentID: 101, FirstName: nullString("Noah"), LastName: nullString("Albrecht"), PhoneNumber: nullString("02551 222")},
		{StudentID: 101, FirstName: nullString("Lea"), LastName: nullString("Albrecht"), PhoneNumber: nullString("02551 333")},
	}}
	generatedAt := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	sources.Now = func() time.Time { return generatedAt }
	query, err := New(sources)
	require.NoError(t, err)

	doc, err := query.Document(context.Background(), generatedAt)
	require.NoError(t, err)
	assert.Equal(t, "2026-08-26", presence.openDate)
	assert.Equal(t, studentpresence.VisitLocationFilter{
		VisitFilter:       studentpresence.VisitFilter{StudentIDs: []int64{101, 202}, OpenOnly: true},
		RunningGroupsOnly: true, LatestPerStudent: true,
	}, presence.filter, "the owner query keeps the retained selection")
	assert.Equal(t, []int64{kreativraum}, rooms.ids)
	assert.Equal(t, "operations.emergency_list_health_info", settings.key)

	assert.Equal(t, listexport.Document{
		Title:       "Notfallliste",
		Subtitle:    "2 anwesende Kinder",
		GeneratedAt: generatedAt,
		Columns: []listexport.Column{
			{ID: listexport.ColumnName, Label: "Name"},
			{ID: listexport.ColumnSchoolClass, Label: "Klasse"},
			{ID: listexport.ColumnCurrentLocation, Label: "Ort / Raum"},
			{ID: listexport.ColumnContactPhone, Label: "Telefonnummer"},
			{ID: listexport.ColumnContactName, Label: "Kontakt"},
			{ID: listexport.ColumnHealthInfo, Label: "Gesundheit / Allergien"},
		},
		Rows: []listexport.Row{
			{Values: map[listexport.ColumnID]string{
				listexport.ColumnName:            "Mila Albrecht",
				listexport.ColumnSchoolClass:     "Klasse 3b",
				listexport.ColumnCurrentLocation: "Kreativraum",
				listexport.ColumnContactPhone:    "02551 111; 02551 222; 02551 333",
				listexport.ColumnContactName:     "Lea Albrecht; Noah Albrecht",
				listexport.ColumnHealthInfo:      "Nussallergie, Epipen im Gruppenraum",
			}},
			{Values: map[listexport.ColumnID]string{
				listexport.ColumnName:            "Max Schmitt",
				listexport.ColumnSchoolClass:     "Klasse 2a",
				listexport.ColumnCurrentLocation: "Unterwegs",
				listexport.ColumnContactPhone:    "02551 444",
				listexport.ColumnContactName:     "Familie Schmitt",
				listexport.ColumnHealthInfo:      "Nicht hinterlegt",
			}},
		},
	}, ToListExportDocument(doc))

	file, err := query.Export(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "application/pdf", file.ContentType)
	assert.Equal(t, "notfallliste.pdf", file.Filename)
	assert.NotEmpty(t, file.Data)
}

func TestBinaryModeMapsSchoolStatuses(t *testing.T) {
	t.Parallel()
	sources := fullSources()
	sources.PresenceMode = fakeMode{mode: "binary"}
	sources.Presence = &fakePresence{open: []int64{101, 202}, statuses: []studentpresence.SchoolStatus{
		{StudentID: 101, Status: "checked_in"},
		{StudentID: 202, Status: "on_yard"},
	}}
	sources.Students = fakeStudents{rows: map[int64]*usersModels.Student{
		101: {PersonID: 301, SchoolClass: "Klasse 3b"},
		202: {PersonID: 302, SchoolClass: "Klasse 2a"},
	}}
	sources.Persons = fakePersons{rows: []peopledirectory.Person{
		{ID: 301, FirstName: "Mila", LastName: "Albrecht"},
		{ID: 302, FirstName: "Max", LastName: "Schmitt"},
	}}
	sources.Rooms = &fakeRooms{err: errors.New("rooms must not be read in binary mode")}
	query, err := New(sources)
	require.NoError(t, err)

	snapshot, err := query.Snapshot(context.Background(), time.Date(2026, 5, 27, 12, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.Len(t, snapshot.Rows, 2)
	assert.Equal(t, "Anwesend", snapshot.Rows[0].Location)
	assert.Equal(t, "Schulhof", snapshot.Rows[1].Location)
}

func TestBinaryStatusFailurePreservesLegacyMessage(t *testing.T) {
	t.Parallel()
	port := presence{facade: &fakePresence{err: errors.New("private database details")}}
	_, err := port.SchoolStatuses(context.Background(), []int64{101}, "2026-09-09")
	require.EqualError(t, err, "active: GetStudentsAttendanceStatuses: database operation failed")
}

// One failing owner query surfaces through the projection unchanged.
func TestOwnerFailuresSurface(t *testing.T) {
	t.Parallel()
	injected := errors.New("owner unavailable")
	cases := map[string]func(*Sources){
		"presence mode": func(s *Sources) { s.PresenceMode = fakeMode{err: injected} },
		"students":      func(s *Sources) { s.Students = fakeStudents{err: injected} },
		"persons":       func(s *Sources) { s.Persons = fakePersons{err: injected} },
		"contacts":      func(s *Sources) { s.Contacts = fakeContacts{err: injected} },
		"rooms":         func(s *Sources) { s.Rooms = &fakeRooms{err: injected} },
	}
	for name, breakSource := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			sources := fullSources()
			sources.Presence = &fakePresence{open: []int64{101}, visits: []studentpresence.VisitLocation{{
				Visit: studentpresence.Visit{StudentID: 101}, Group: &studentpresence.VisitGroup{RoomID: 1},
			}}}
			sources.Students = fakeStudents{rows: map[int64]*usersModels.Student{101: {PersonID: 301}}}
			sources.Persons = fakePersons{rows: []peopledirectory.Person{{ID: 301, FirstName: "Mila", LastName: "Albrecht"}}}
			breakSource(&sources)
			query, err := New(sources)
			require.NoError(t, err)
			_, err = query.Export(context.Background())
			require.ErrorIs(t, err, injected)
		})
	}
}

// --- Database-backed adapters ---

func newPresenceModule(t *testing.T, db *bun.DB) *studentpresence.Module {
	t.Helper()
	module, err := presenceCompose.New(presenceCompose.Dependencies{DB: db, Observe: func(presenceCompose.Observation) {}})
	require.NoError(t, err)
	return module
}

func TestPersonIdentityFilteringMatchesRetainedReader(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Filtering", "Parity", "3a")
	people, err := peopleCompose.New(peopleCompose.Dependencies{DB: db, Observe: func(peopleCompose.Observation) {}})
	require.NoError(t, err)
	old := usersRepo.NewPersonRepository(db)
	ids := []int64{student.PersonID}
	beforeOld, err := old.FindByIDs(ctx, ids)
	require.NoError(t, err)
	beforeNew, err := people.ListPersonsByID(ctx, ids)
	require.NoError(t, err)
	require.Len(t, beforeOld, 1)
	require.Len(t, beforeNew, 1)
	require.NoError(t, people.DeletePerson(ctx, student.PersonID))
	afterOld, err := old.FindByIDs(ctx, ids)
	require.NoError(t, err)
	afterNew, err := people.ListPersonsByID(ctx, ids)
	require.NoError(t, err)
	require.Empty(t, afterOld, "Bun applies the retained model's soft-delete filter")
	require.Empty(t, afterNew, "the owner facade preserves that filter")
}

// The presence adapter selects the latest open visit inside a running
// session: an older closed visit and a visit in an ended session never
// supply a room.
func TestCurrentRoomIDsSelectLatestRunningSession(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	firstRoom := testpkg.CreateTestRoom(t, db, "VisitRoom")
	secondRoom := testpkg.CreateTestRoom(t, db, "VisitRoomCurrent")
	activity := testpkg.CreateTestActivityGroup(t, db, "VisitActivity")
	firstGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, firstRoom.ID)
	secondGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, secondRoom.ID)
	first := testpkg.CreateTestStudent(t, db, "Visit", "First", "1a")
	second := testpkg.CreateTestStudent(t, db, "Visit", "Second", "1b")
	port := presence{facade: newPresenceModule(t, db), mode: fakeMode{mode: "detailed"}}

	rooms, err := port.CurrentRoomIDs(ctx, nil)
	require.NoError(t, err)
	require.Empty(t, rooms)

	oldExit := time.Now().Add(-90 * time.Minute)
	testpkg.CreateTestVisit(t, db, first.ID, firstGroup.ID, time.Now().Add(-2*time.Hour), &oldExit)
	testpkg.CreateTestVisit(t, db, first.ID, secondGroup.ID, time.Now().Add(-10*time.Minute), nil)
	testpkg.CreateTestVisit(t, db, second.ID, firstGroup.ID, time.Now().Add(-20*time.Minute), nil)
	rooms, err = port.CurrentRoomIDs(ctx, []int64{first.ID, second.ID})
	require.NoError(t, err)
	assert.Equal(t, map[int64]int64{first.ID: secondRoom.ID, second.ID: firstRoom.ID}, rooms)

	_, err = db.NewUpdate().Table("active.groups").Set("end_time = ?", time.Now()).Where("id = ?", secondGroup.ID).Exec(ctx)
	require.NoError(t, err)
	rooms, err = port.CurrentRoomIDs(ctx, []int64{first.ID})
	require.NoError(t, err)
	assert.NotContains(t, rooms, first.ID, "ended sessions cannot supply a current room")
}

// TestSnapshotInputsEnforceRLS proves the emergency snapshot projection
// (#2704) stays inside the tenant: two schools seed the same shape (present
// child in a running session with a linked guardian and phone number), the
// projection's named inputs are invisible across the tenant boundary under
// the least-privilege role, and each school's Notfallliste lists only its
// own child with its own guardian.
func TestSnapshotInputsEnforceRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	type fixture struct {
		ctx       context.Context
		tenantID  int64
		studentID int64
		roomName  string
		guardian  string
		phone     string
		rows      map[string]int64
	}
	var fixtures []fixture
	for _, side := range []string{"own", "foreign"} {
		t.Run(side, func(t *testing.T) {
			testpkg.OwnTenant(t)
			ctx := testpkg.Ctx(t)
			teacher, _ := testpkg.CreateTestTeacherWithAccount(t, db, "SnapRLS", side)
			room := testpkg.CreateTestRoom(t, db, "SnapRLS-"+side)
			activity := testpkg.CreateTestActivityGroup(t, db, "SnapRLS-"+side)
			activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
			student := testpkg.CreateTestStudent(t, db, "SnapRLS", side, "SR1")
			device := testpkg.CreateTestDevice(t, db, "SnapRLS-"+side)
			checkIn := time.Now().Add(-time.Hour)
			attendance := testpkg.CreateTestAttendance(t, db, student.ID, teacher.Staff.ID, device.ID, checkIn, nil)
			visit := testpkg.CreateTestVisit(t, db, student.ID, activeGroup.ID, checkIn, nil)
			guardian := testpkg.CreateTestGuardianProfileNamed(t, db, "Guardian", side, "guardian-"+side)
			link := testpkg.CreateTestStudentGuardianLink(t, db, student.ID, guardian.ID, "parent")
			phone := &usersModels.GuardianPhoneNumber{GuardianProfileID: guardian.ID, PhoneNumber: "02551 " + side, PhoneType: usersModels.PhoneTypeMobile, IsPrimary: true}
			phone.SetTenantID(testpkg.Tenant(t))
			require.NoError(t, db.NewInsert().Model(phone).ModelTableExpr("users.guardian_phone_numbers").Scan(ctx))
			fixtures = append(fixtures, fixture{
				ctx: ctx, tenantID: testpkg.Tenant(t), studentID: student.ID, roomName: room.Name,
				guardian: "Guardian " + side, phone: "02551 " + side,
				rows: map[string]int64{
					"users.students":               student.ID,
					"users.students_guardians":     link.ID,
					"users.guardian_profiles":      guardian.ID,
					"users.guardian_phone_numbers": phone.ID,
					"facilities.rooms":             room.ID,
					"active.attendance":            attendance.ID,
					"active.visits":                visit.ID,
				},
			})
		})
	}
	require.Len(t, fixtures, 2)
	for table, ownID := range fixtures[0].rows {
		t.Run(table, func(t *testing.T) {
			for _, fixture := range fixtures {
				require.NoError(t, testpkg.WithTenantTx(t, fixture.ctx, db, fixture.tenantID, func(txCtx context.Context, tx bun.Tx) error {
					var bypass bool
					require.NoError(t, tx.NewRaw("SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user").Scan(txCtx, &bypass))
					require.False(t, bypass, "the tenant transaction must run under the least-privilege role")
					var ids []int64
					require.NoError(t, tx.NewSelect().Table(table).Column("id").Where("id IN (?, ?)", ownID, fixtures[1].rows[table]).Scan(txCtx, &ids))
					require.Equal(t, []int64{fixture.rows[table]}, ids, "%s leaks across the tenant boundary", table)
					return nil
				}))
			}
		})
	}

	people, err := peopleCompose.New(peopleCompose.Dependencies{DB: db, Observe: func(peopleCompose.Observation) {}})
	require.NoError(t, err)
	facilitiesModule, err := facilitiesCompose.New(facilitiesCompose.Dependencies{
		DB:            db,
		DeletionLock:  func(context.Context) error { return nil },
		DeletionGuard: func(context.Context, int64) error { return nil },
		Observe:       func(facilitiesCompose.Observation) {},
	})
	require.NoError(t, err)
	query, err := New(Sources{
		Presence:     newPresenceModule(t, db),
		PresenceMode: fakeMode{mode: "detailed"},
		Students:     usersRepo.NewStudentRepository(db),
		Persons:      people,
		Contacts:     usersRepo.NewStudentGuardianRepository(db),
		Rooms:        facilitiesModule,
		Settings:     &fakeSettings{enabled: false},
		Renderer:     listexport.NewService(),
	})
	require.NoError(t, err)
	for _, fixture := range fixtures {
		// Runtime evidence (#2704): the projection issues one statement per
		// owner read, flat in the number of children.
		counter := testpkg.CaptureQueriesForContext(t, db)
		require.NoError(t, testpkg.WithTenantTx(t, counter.Context(fixture.ctx), db, fixture.tenantID, func(txCtx context.Context, _ bun.Tx) error {
			snapshot, err := query.Snapshot(txCtx, time.Now())
			require.NoError(t, err)
			testpkg.AssertQueryBudget(t, "modules.emergencysnapshot.snapshot", counter.Queries())
			require.Len(t, snapshot.Rows, 1, "each school lists exactly its own child")
			row := snapshot.Rows[0]
			assert.Equal(t, fixture.studentID, row.StudentID)
			assert.Equal(t, fixture.roomName, row.Location, "the room of the own tenant is visible")
			assert.Equal(t, fixture.guardian, row.ContactName)
			assert.Equal(t, fixture.phone, row.ContactPhone)
			return nil
		}))
	}
}
