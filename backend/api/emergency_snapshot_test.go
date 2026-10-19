package api

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	"github.com/moto-nrw/project-phoenix/modules/facilities"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type emergencyFakePresence struct {
	open     []int64
	openDate string
	visits   []studentpresence.VisitLocation
	filter   studentpresence.VisitLocationFilter
	statuses []studentpresence.SchoolStatus
	err      error
}

func (f *emergencyFakePresence) ListOpenAttendanceStudentIDs(_ context.Context, date string) ([]int64, error) {
	f.openDate = date
	return f.open, f.err
}

func (f *emergencyFakePresence) ListVisitLocations(_ context.Context, filter studentpresence.VisitLocationFilter) ([]studentpresence.VisitLocation, error) {
	f.filter = filter
	return f.visits, f.err
}

func (f *emergencyFakePresence) ListSchoolStatuses(_ context.Context, _ []int64, _ string) ([]studentpresence.SchoolStatus, error) {
	return f.statuses, f.err
}

type emergencyFakeMode struct {
	mode string
	err  error
}

func (f emergencyFakeMode) GetPresenceMode(_ context.Context) (string, error) { return f.mode, f.err }

type emergencyFakeStudents struct {
	rows map[int64]*peopledirectory.StudentState
	err  error
}

func (f emergencyFakeStudents) FindByIDs(_ context.Context, _ []int64) (map[int64]*peopledirectory.StudentState, error) {
	return f.rows, f.err
}

type emergencyFakePersons struct {
	rows []peopledirectory.Person
	err  error
}

func (f emergencyFakePersons) ListPersonsByID(_ context.Context, _ []int64) ([]peopledirectory.Person, error) {
	return f.rows, f.err
}

type emergencyFakeContacts struct {
	rows []peopledirectory.GuardianEmergencyContactRow
	err  error
}

func (f emergencyFakeContacts) ListEmergencyContactRows(_ context.Context, _ []int64) ([]peopledirectory.GuardianEmergencyContactRow, error) {
	return f.rows, f.err
}

type emergencyFakeRooms struct {
	rows []facilities.Room
	err  error
	ids  []int64
}

func (f *emergencyFakeRooms) ListRoomsByID(_ context.Context, ids []int64) ([]facilities.Room, error) {
	f.ids = ids
	return f.rows, f.err
}

func emergencyFullSources() emergencySnapshotSources {
	return emergencySnapshotSources{
		Presence:     &emergencyFakePresence{},
		PresenceMode: emergencyFakeMode{mode: "detailed"},
		Students:     emergencyFakeStudents{},
		Persons:      emergencyFakePersons{},
		Contacts:     emergencyFakeContacts{},
		Rooms:        &emergencyFakeRooms{},
		Renderer:     lists.NewRendererService(),
	}
}

func TestEmergencySnapshotNewRejectsMissingSources(t *testing.T) {
	t.Parallel()
	_, err := newEmergencySnapshot(emergencySnapshotSources{})
	require.ErrorIs(t, err, errEmergencySnapshotSources)

	sources := emergencyFullSources()
	sources.Renderer = nil
	_, err = newEmergencySnapshot(sources)
	require.ErrorIs(t, err, errEmergencySnapshotSources)

	query, err := newEmergencySnapshot(emergencyFullSources())
	require.NoError(t, err)
	require.NotNil(t, query)
}

func emergencyOptionalText(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

// The retained service's fixture (#2609 tests): one child in the Kreativraum
// with three guardian rows, one child whose only contact is the legacy
// free-text guardian. The rendered document is the one the service rendered.
func TestExportRendersTheRetainedNotfallliste(t *testing.T) {
	t.Parallel()
	const kreativraum int64 = 7007
	presence := &emergencyFakePresence{
		open: []int64{101, 202},
		visits: []studentpresence.VisitLocation{{
			Visit: studentpresence.Visit{StudentID: 101},
			Group: &studentpresence.VisitGroup{RoomID: kreativraum},
		}},
	}
	rooms := &emergencyFakeRooms{rows: []facilities.Room{{ID: kreativraum, Name: "Kreativraum"}}}
	sources := emergencyFullSources()
	sources.Presence = presence
	sources.Rooms = rooms
	sources.Students = emergencyFakeStudents{rows: map[int64]*peopledirectory.StudentState{
		101: {PersonID: 301, SchoolClass: "Klasse 3b", HealthInfo: new("Nussallergie, Epipen im Gruppenraum")},
		202: {PersonID: 302, SchoolClass: "Klasse 2a"},
	}}
	sources.Persons = emergencyFakePersons{rows: []peopledirectory.Person{
		{ID: 301, FirstName: "Mila", LastName: "Albrecht"},
		{ID: 302, FirstName: "Max", LastName: "Schmitt"},
	}}
	sources.Contacts = emergencyFakeContacts{rows: []peopledirectory.GuardianEmergencyContactRow{
		{StudentID: 202, FirstName: emergencyOptionalText("Familie"), LastName: emergencyOptionalText("Schmitt"), PhoneNumber: emergencyOptionalText("02551 444")},
		{StudentID: 101, FirstName: emergencyOptionalText("Lea"), LastName: emergencyOptionalText("Albrecht"), PhoneNumber: emergencyOptionalText("02551 111")},
		{StudentID: 101, FirstName: emergencyOptionalText("Noah"), LastName: emergencyOptionalText("Albrecht"), PhoneNumber: emergencyOptionalText("02551 222")},
		{StudentID: 101, FirstName: emergencyOptionalText("Lea"), LastName: emergencyOptionalText("Albrecht"), PhoneNumber: emergencyOptionalText("02551 333")},
	}}
	generatedAt := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	sources.Now = func() time.Time { return generatedAt }
	query, err := newEmergencySnapshot(sources)
	require.NoError(t, err)

	doc, err := query.Document(context.Background(), generatedAt)
	require.NoError(t, err)
	assert.Equal(t, "2026-08-26", presence.openDate)
	assert.Equal(t, studentpresence.VisitLocationFilter{
		VisitFilter:       studentpresence.VisitFilter{StudentIDs: []int64{101, 202}, OpenOnly: true},
		RunningGroupsOnly: true, LatestPerStudent: true,
	}, presence.filter, "the owner query keeps the retained selection")
	assert.Equal(t, []int64{kreativraum}, rooms.ids)

	assert.Equal(t, lists.Document{
		Title:       "Notfallliste",
		Subtitle:    "2 anwesende Kinder",
		GeneratedAt: generatedAt,
		Columns: []lists.Column{
			{ID: lists.ColumnName, Label: "Name"},
			{ID: lists.ColumnSchoolClass, Label: "Klasse"},
			{ID: lists.ColumnCurrentLocation, Label: "Ort / Raum"},
			{ID: lists.ColumnID("contact_phone"), Label: "Telefonnummer"},
			{ID: lists.ColumnContactName, Label: "Kontakt"},
			{ID: lists.ColumnHealthInfo, Label: "Gesundheit / Allergien"},
		},
		Rows: []lists.Row{
			{Values: map[lists.ColumnID]string{
				lists.ColumnName:                "Mila Albrecht",
				lists.ColumnSchoolClass:         "Klasse 3b",
				lists.ColumnCurrentLocation:     "Kreativraum",
				lists.ColumnID("contact_phone"): "02551 111; 02551 222; 02551 333",
				lists.ColumnContactName:         "Lea Albrecht; Noah Albrecht",
				lists.ColumnHealthInfo:          "Nussallergie, Epipen im Gruppenraum",
			}},
			{Values: map[lists.ColumnID]string{
				lists.ColumnName:                "Max Schmitt",
				lists.ColumnSchoolClass:         "Klasse 2a",
				lists.ColumnCurrentLocation:     "Unterwegs",
				lists.ColumnID("contact_phone"): "02551 444",
				lists.ColumnContactName:         "Familie Schmitt",
				lists.ColumnHealthInfo:          "Nicht hinterlegt",
			}},
		},
	}, emergencySnapshotListDocument(doc))

	file, err := query.Export(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "application/pdf", file.ContentType)
	assert.Equal(t, "notfallliste.pdf", file.Filename)
	assert.NotEmpty(t, file.Data)
}

func TestEmergencySnapshotBinaryModeMapsSchoolStatuses(t *testing.T) {
	t.Parallel()
	sources := emergencyFullSources()
	sources.PresenceMode = emergencyFakeMode{mode: "binary"}
	sources.Presence = &emergencyFakePresence{open: []int64{101, 202}, statuses: []studentpresence.SchoolStatus{
		{StudentID: 101, Status: "checked_in"},
		{StudentID: 202, Status: "on_yard"},
	}}
	sources.Students = emergencyFakeStudents{rows: map[int64]*peopledirectory.StudentState{
		101: {PersonID: 301, SchoolClass: "Klasse 3b"},
		202: {PersonID: 302, SchoolClass: "Klasse 2a"},
	}}
	sources.Persons = emergencyFakePersons{rows: []peopledirectory.Person{
		{ID: 301, FirstName: "Mila", LastName: "Albrecht"},
		{ID: 302, FirstName: "Max", LastName: "Schmitt"},
	}}
	sources.Rooms = &emergencyFakeRooms{err: errors.New("rooms must not be read in binary mode")}
	query, err := newEmergencySnapshot(sources)
	require.NoError(t, err)

	snapshot, err := query.Snapshot(context.Background(), time.Date(2026, 5, 27, 12, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.Len(t, snapshot.Rows, 2)
	assert.Equal(t, "Anwesend", snapshot.Rows[0].Location)
	assert.Equal(t, "Schulhof", snapshot.Rows[1].Location)
}

func TestEmergencySnapshotBinaryStatusFailurePreservesLegacyMessage(t *testing.T) {
	t.Parallel()
	port := emergencySnapshotPresenceReads{facade: &emergencyFakePresence{err: errors.New("private database details")}}
	_, err := port.SchoolStatuses(context.Background(), []int64{101}, "2026-09-09")
	require.EqualError(t, err, "active: GetStudentsAttendanceStatuses: database operation failed")
}

// One failing owner query surfaces through the projection unchanged.
func TestEmergencySnapshotOwnerFailuresSurface(t *testing.T) {
	t.Parallel()
	injected := errors.New("owner unavailable")
	cases := map[string]func(*emergencySnapshotSources){
		"presence mode": func(s *emergencySnapshotSources) { s.PresenceMode = emergencyFakeMode{err: injected} },
		"students":      func(s *emergencySnapshotSources) { s.Students = emergencyFakeStudents{err: injected} },
		"persons":       func(s *emergencySnapshotSources) { s.Persons = emergencyFakePersons{err: injected} },
		"contacts":      func(s *emergencySnapshotSources) { s.Contacts = emergencyFakeContacts{err: injected} },
		"rooms":         func(s *emergencySnapshotSources) { s.Rooms = &emergencyFakeRooms{err: injected} },
	}
	for name, breakSource := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			sources := emergencyFullSources()
			sources.Presence = &emergencyFakePresence{open: []int64{101}, visits: []studentpresence.VisitLocation{{
				Visit: studentpresence.Visit{StudentID: 101}, Group: &studentpresence.VisitGroup{RoomID: 1},
			}}}
			sources.Students = emergencyFakeStudents{rows: map[int64]*peopledirectory.StudentState{101: {PersonID: 301}}}
			sources.Persons = emergencyFakePersons{rows: []peopledirectory.Person{{ID: 301, FirstName: "Mila", LastName: "Albrecht"}}}
			breakSource(&sources)
			query, err := newEmergencySnapshot(sources)
			require.NoError(t, err)
			_, err = query.Export(context.Background())
			require.ErrorIs(t, err, injected)
		})
	}
}

// --- Database-backed adapters ---

func emergencyPresenceModule(t *testing.T, db *bun.DB) *studentpresence.Module {
	t.Helper()
	module, err := presenceCompose.New(presenceCompose.Dependencies{DB: db, Observe: func(presenceCompose.Observation) {}})
	require.NoError(t, err)
	return module
}

func TestEmergencySnapshotPersonIdentityFilteringMatchesRetainedReader(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Filtering", "Parity", "3a")
	people, err := peopleCompose.New(peopleCompose.Dependencies{StudentDirectory: testpkg.StudentDirectory{}, DB: db, Observe: func(peopleCompose.Observation) {}})
	require.NoError(t, err)
	old := peopleCompose.NewPersonRepository(db, nil)
	ids := []int64{student.PersonID}
	beforeOld, err := testpkg.TenantQuery1(t, ctx, db, old.FindByIDs, ids)
	require.NoError(t, err)
	beforeNew, err := people.ListPersonsByID(ctx, ids)
	require.NoError(t, err)
	require.Len(t, beforeOld, 1)
	require.Len(t, beforeNew, 1)
	require.NoError(t, people.DeletePerson(ctx, student.PersonID))
	afterOld, err := testpkg.TenantQuery1(t, ctx, db, old.FindByIDs, ids)
	require.NoError(t, err)
	afterNew, err := people.ListPersonsByID(ctx, ids)
	require.NoError(t, err)
	require.Empty(t, afterOld, "Bun applies the retained model's soft-delete filter")
	require.Empty(t, afterNew, "the owner facade preserves that filter")
}

// The presence adapter selects the latest open visit inside a running
// session: an older closed visit and a visit in an ended session never
// supply a room.
func TestEmergencySnapshotCurrentRoomIDsSelectLatestRunningSession(t *testing.T) {
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
	port := emergencySnapshotPresenceReads{facade: emergencyPresenceModule(t, db), mode: emergencyFakeMode{mode: "detailed"}}

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

// TestEmergencySnapshotInputsEnforceRLS proves the emergency snapshot projection
// (#2704) stays inside the tenant: two schools seed the same shape (present
// child in a running session with a linked guardian and phone number), the
// projection's named inputs are invisible across the tenant boundary under
// the least-privilege role, and each school's Notfallliste lists only its
// own child with its own guardian.
func TestEmergencySnapshotInputsEnforceRLS(t *testing.T) {
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
			phone := &peopledirectory.GuardianProfilePhoneSnapshot{GuardianProfileID: guardian.ID, PhoneNumber: "02551 " + side, PhoneType: peopledirectory.PhoneTypeMobile, IsPrimary: true}
			phone.SetTenantID(testpkg.Tenant(t))
			require.NoError(t, db.NewInsert().Model(phone).ModelTableExpr("users.guardian_phone_numbers").Column("tenant_id", "guardian_profile_id", "phone_number", "phone_type", "is_primary", "priority").Returning("*").Scan(ctx))
			var membershipID int64
			require.NoError(t, db.NewSelect().Table("users.student_school_memberships").Column("id").Where("student_profile_id = ? AND deleted_at IS NULL", student.ID).Scan(ctx, &membershipID))
			fixtures = append(fixtures, fixture{
				ctx: ctx, tenantID: testpkg.Tenant(t), studentID: student.ID, roomName: room.Name,
				guardian: "Guardian " + side, phone: "02551 " + side,
				rows: map[string]int64{
					"users.student_profiles":               student.ID,
					"users.student_school_memberships":     membershipID,
					"users.student_care_profiles":          membershipID,
					"users.student_guardian_relationships": link.ID,
					"users.guardian_profiles":              guardian.ID,
					"users.guardian_phone_numbers":         phone.ID,
					"facilities.rooms":                     room.ID,
					"active.attendance":                    attendance.ID,
					"active.visits":                        visit.ID,
				},
			})
		})
	}
	require.Len(t, fixtures, 2)
	for table, ownID := range fixtures[0].rows {
		t.Run(table, func(t *testing.T) {
			key := "id"
			if table == "users.student_care_profiles" {
				key = "membership_id"
			}
			for _, fixture := range fixtures {
				require.NoError(t, testpkg.WithTenantTx(t, fixture.ctx, db, fixture.tenantID, func(txCtx context.Context, tx bun.Tx) error {
					var bypass bool
					require.NoError(t, tx.NewRaw("SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user").Scan(txCtx, &bypass))
					require.False(t, bypass, "the tenant transaction must run under the least-privilege role")
					var ids []int64
					require.NoError(t, tx.NewSelect().Table(table).Column(key).Where("? IN (?, ?)", bun.Ident(key), ownID, fixtures[1].rows[table]).Scan(txCtx, &ids))
					require.Equal(t, []int64{fixture.rows[table]}, ids, "%s leaks across the tenant boundary", table)
					return nil
				}))
			}
		})
	}

	people, err := peopleCompose.New(peopleCompose.Dependencies{StudentDirectory: testpkg.StudentDirectory{}, DB: db, Observe: func(peopleCompose.Observation) {}})
	require.NoError(t, err)
	facilitiesModule, err := facilitiesCompose.NewUnobserved(db)
	require.NoError(t, err)
	query, err := newEmergencySnapshot(emergencySnapshotSources{
		Presence:     emergencyPresenceModule(t, db),
		PresenceMode: emergencyFakeMode{mode: "detailed"},
		Students:     newEmergencyOwnerStudents(t, db),
		Persons:      people,
		Contacts:     peopleCompose.NewGuardianRelationshipRepository(db, peopleCompose.GuardianRelationshipDependencies{Students: testpkg.StudentDirectory{}, Links: testpkg.GuardianLinks{}}),
		Rooms:        facilitiesModule,
		Renderer:     lists.NewRendererService(),
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

// emergencyOwnerStudents satisfies the snapshot's one-method student port from
// People Directory, which owns users.students since #3349. This adapter-test
// may not reach the legacy composition that production wires, so it speaks to
// the owner directly — and therefore still reads real rows under real RLS,
// which is the point of the checks below.
type emergencyOwnerStudents struct{ directory peopledirectory.Capability }

func newEmergencyOwnerStudents(t *testing.T, db *bun.DB) emergencyOwnerStudents {
	t.Helper()
	directory, err := peopleCompose.New(peopleCompose.Dependencies{
		StudentDirectory: testpkg.StudentDirectory{},
		DB:               db, Observe: func(peopleCompose.Observation) {},
	})
	require.NoError(t, err)
	return emergencyOwnerStudents{directory: directory}
}

func (s emergencyOwnerStudents) FindByIDs(
	ctx context.Context,
	ids []int64,
) (map[int64]*peopledirectory.StudentState, error) {
	records, err := s.directory.ListStudentRecordsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	// Only what the snapshot renders; the owner's record carries the rest.
	result := make(map[int64]*peopledirectory.StudentState, len(records))
	for _, record := range records {
		student := &peopledirectory.StudentState{
			PersonID: record.PersonID, SchoolClass: record.SchoolClass,
			HealthInfo: record.HealthInfo,
		}
		student.ID = record.ID
		result[record.ID] = student
	}
	return result, nil
}
