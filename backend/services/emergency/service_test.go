package emergency

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	activeRepo "github.com/moto-nrw/project-phoenix/database/repositories/active"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	"github.com/moto-nrw/project-phoenix/internal/strutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/users"

	activeService "github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

type stubAttendanceRepo struct {
	ids []int64
	err error
}

func (r stubAttendanceRepo) ListOpenStudentIDsForDate(_ context.Context, _ timezone.Date) ([]int64, error) {
	return r.ids, r.err
}

type stubStudentRepo struct {
	students map[int64]*users.Student
	err      error
}

func (r stubStudentRepo) FindByIDs(_ context.Context, _ []int64) (map[int64]*users.Student, error) {
	return r.students, r.err
}

type stubPersonRepo struct {
	persons map[int64]*users.Person
	err     error
}

func (r stubPersonRepo) FindByIDs(_ context.Context, _ []int64) (map[int64]*users.Person, error) {
	return r.persons, r.err
}

type stubActivePresence struct {
	mode     string
	statuses map[int64]*activeService.AttendanceStatus
	err      error
}

func (s stubActivePresence) GetPresenceMode(_ context.Context) string {
	return s.mode
}

func (s stubActivePresence) GetStudentsAttendanceStatuses(_ context.Context, _ []int64) (map[int64]*activeService.AttendanceStatus, error) {
	return s.statuses, s.err
}

type stubSettings struct {
	enabled bool
	err     error
}

func (s stubSettings) ResolveBool(_ context.Context, _ string) (bool, error) {
	return s.enabled, s.err
}

// kreativraumRoomID is the room the mocked visit row points at.
const kreativraumRoomID int64 = 7007

// stubRoomDirectory stands in for the Facilities owner the visit repository
// resolves room names through (#2665).
type stubRoomDirectory struct{}

func (stubRoomDirectory) ListRoomsByID(_ context.Context, ids []int64) ([]activeRepo.DirectoryRoom, error) {
	rooms := make([]activeRepo.DirectoryRoom, 0, len(ids))
	for _, id := range ids {
		if id == kreativraumRoomID {
			rooms = append(rooms, activeRepo.DirectoryRoom{ID: id, Name: "Kreativraum"})
		}
	}
	return rooms, nil
}

func newVisitRepo(db *bun.DB) *activeRepo.VisitRepository {
	repo := activeRepo.NewVisitRepository(db).(*activeRepo.VisitRepository)
	repo.BindRoomDirectory(stubRoomDirectory{})
	return repo
}

func newMockBunDB(t *testing.T) (*bun.DB, sqlmock.Sqlmock, func()) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	db := bun.NewDB(sqlDB, pgdialect.New())
	return db, mock, func() {
		mock.ExpectClose()
		require.NoError(t, db.Close())
	}
}

func TestBuildSnapshotDocumentLoadsCurrentRows(t *testing.T) {
	t.Parallel()

	db, mock, cleanup := newMockBunDB(t)
	defer cleanup()

	mock.ExpectQuery(`(?s)SELECT .*active\.visits`).
		WillReturnRows(sqlmock.NewRows([]string{"student_id", "room_id"}).
			AddRow(int64(101), kreativraumRoomID))
	mock.ExpectQuery(`(?s)SELECT .*users\.students_guardians`).
		WillReturnRows(sqlmock.NewRows([]string{"student_id", "first_name", "last_name", "phone_number"}).
			AddRow(int64(101), "Lea", "Albrecht", "02551 111").
			AddRow(int64(101), "Noah", "Albrecht", "02551 222").
			AddRow(int64(101), "Lea", "Albrecht", "02551 333"))

	legacyName := "Familie Schmitt"
	legacyPhone := "02551 444"
	svc := NewService(Dependencies{
		AttendanceRepo: stubAttendanceRepo{ids: []int64{101, 202}},
		StudentRepo: stubStudentRepo{students: map[int64]*users.Student{
			101: {PersonID: 301, SchoolClass: "Klasse 3b"},
			202: {PersonID: 302, SchoolClass: "Klasse 2a", GuardianName: &legacyName, GuardianPhone: &legacyPhone},
		}},
		PersonRepo: stubPersonRepo{persons: map[int64]*users.Person{
			301: {FirstName: "Mila", LastName: "Albrecht"},
			302: {FirstName: "Max", LastName: "Schmitt"},
		}},
		ListExport:          listexport.NewService(),
		VisitRepo:           newVisitRepo(db),
		StudentGuardianRepo: usersRepo.NewStudentGuardianRepository(db),
	})

	doc, err := svc.BuildSnapshotDocument(context.Background(), time.Date(2026, 5, 27, 12, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())

	require.Len(t, doc.Rows, 2)
	assert.Equal(t, "Notfallliste", doc.Title)
	assert.Equal(t, "2 anwesende Kinder", doc.Subtitle)
	assert.Equal(t, "Mila Albrecht", doc.Rows[0].Values[listexport.ColumnName])
	assert.Equal(t, "Kreativraum", doc.Rows[0].Values[listexport.ColumnCurrentLocation])
	assert.Equal(t, "02551 111; 02551 222; 02551 333", doc.Rows[0].Values[listexport.ColumnContactPhone])
	assert.Equal(t, "Lea Albrecht; Noah Albrecht", doc.Rows[0].Values[listexport.ColumnContactName])
	assert.Equal(t, "Max Schmitt", doc.Rows[1].Values[listexport.ColumnName])
	assert.Equal(t, "Unterwegs", doc.Rows[1].Values[listexport.ColumnCurrentLocation])
	assert.Equal(t, "02551 444", doc.Rows[1].Values[listexport.ColumnContactPhone])
	assert.Equal(t, "Familie Schmitt", doc.Rows[1].Values[listexport.ColumnContactName])
}

func TestBuildSnapshotDocumentUsesBinaryLocations(t *testing.T) {
	t.Parallel()

	db, mock, cleanup := newMockBunDB(t)
	defer cleanup()

	mock.ExpectQuery(`(?s)SELECT .*users\.students_guardians`).
		WillReturnRows(sqlmock.NewRows([]string{"student_id", "first_name", "last_name", "phone_number"}))

	svc := NewService(Dependencies{
		AttendanceRepo: stubAttendanceRepo{ids: []int64{101, 202}},
		StudentRepo: stubStudentRepo{students: map[int64]*users.Student{
			101: {PersonID: 301, SchoolClass: "Klasse 3b"},
			202: {PersonID: 302, SchoolClass: "Klasse 2a"},
		}},
		PersonRepo: stubPersonRepo{persons: map[int64]*users.Person{
			301: {FirstName: "Mila", LastName: "Albrecht"},
			302: {FirstName: "Max", LastName: "Schmitt"},
		}},
		ActiveService: stubActivePresence{
			mode: "binary",
			statuses: map[int64]*activeService.AttendanceStatus{
				101: {Status: "checked_in"},
				202: {Status: "on_yard"},
			},
		},
		ListExport:          listexport.NewService(),
		VisitRepo:           newVisitRepo(db),
		StudentGuardianRepo: usersRepo.NewStudentGuardianRepository(db),
	})

	doc, err := svc.BuildSnapshotDocument(context.Background(), time.Date(2026, 5, 27, 12, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())

	require.Len(t, doc.Rows, 2)
	assert.Equal(t, "Anwesend", doc.Rows[0].Values[listexport.ColumnCurrentLocation])
	assert.Equal(t, "Schulhof", doc.Rows[1].Values[listexport.ColumnCurrentLocation])
}

func TestBuildSnapshotDocumentWithNoStudents(t *testing.T) {
	t.Parallel()

	db, _, cleanup := newMockBunDB(t)
	defer cleanup()

	svc := NewService(Dependencies{
		AttendanceRepo:      stubAttendanceRepo{ids: []int64{}},
		StudentRepo:         stubStudentRepo{},
		PersonRepo:          stubPersonRepo{},
		ListExport:          listexport.NewService(),
		VisitRepo:           newVisitRepo(db),
		StudentGuardianRepo: usersRepo.NewStudentGuardianRepository(db),
	})

	doc, err := svc.BuildSnapshotDocument(context.Background(), time.Time{})
	require.NoError(t, err)
	assert.Equal(t, "0 anwesende Kinder", doc.Subtitle)
	assert.Empty(t, doc.Rows)
}

func TestRenderSnapshot(t *testing.T) {
	t.Parallel()

	db, _, cleanup := newMockBunDB(t)
	defer cleanup()

	svc := NewService(Dependencies{
		AttendanceRepo:      stubAttendanceRepo{ids: []int64{}},
		StudentRepo:         stubStudentRepo{},
		PersonRepo:          stubPersonRepo{},
		ListExport:          listexport.NewService(),
		VisitRepo:           newVisitRepo(db),
		StudentGuardianRepo: usersRepo.NewStudentGuardianRepository(db),
	})

	file, err := svc.RenderSnapshot(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "application/pdf", file.ContentType)
	assert.Equal(t, "notfallliste.pdf", file.Filename)
	assert.NotEmpty(t, file.Data)
}

func TestBuildSnapshotDocumentRejectsMissingDependencies(t *testing.T) {
	t.Parallel()

	svc := NewService(Dependencies{})

	_, err := svc.BuildSnapshotDocument(context.Background(), time.Time{})
	require.Error(t, err)
}

func TestBuildDocumentRows(t *testing.T) {
	t.Parallel()

	rows := buildDocumentRows([]snapshotRow{
		{
			Name:         "Mila Albrecht",
			SchoolClass:  "Klasse 3b",
			Location:     "Kreativraum",
			ContactPhone: "02551 123",
			ContactName:  "Lea Albrecht",
		},
	}, false)

	assert.Len(t, rows, 1)
	assert.Equal(t, "Mila Albrecht", rows[0].Values[listexport.ColumnName])
	assert.Equal(t, "Klasse 3b", rows[0].Values[listexport.ColumnSchoolClass])
	assert.Equal(t, "Kreativraum", rows[0].Values[listexport.ColumnCurrentLocation])
	assert.Equal(t, "02551 123", rows[0].Values[listexport.ColumnContactPhone])
	assert.Equal(t, "Lea Albrecht", rows[0].Values[listexport.ColumnContactName])
}

func TestJoinUnique(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "Lea Albrecht; Noah Albrecht", strutil.JoinUnique("Lea Albrecht", "Noah Albrecht", "lea albrecht"))
	assert.Equal(t, "02551 111; 02551 222", strutil.JoinUnique("02551 111; 02551 222", "02551 111"))
	assert.Empty(t, strutil.JoinUnique("", " "))
}

func TestBinaryLocationLabel(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "Anwesend", binaryLocationLabel(&activeService.AttendanceStatus{Status: "checked_in"}))
	assert.Equal(t, "Schulhof", binaryLocationLabel(&activeService.AttendanceStatus{Status: "on_yard"}))
	assert.Equal(t, "Abwesend", binaryLocationLabel(&activeService.AttendanceStatus{Status: "checked_out"}))
	assert.Equal(t, "Abwesend", binaryLocationLabel(nil))
}

func TestSortSnapshotRowsGermanNameOrder(t *testing.T) {
	t.Parallel()

	rows := []snapshotRow{
		{Name: "Jan Zimmermann", Location: "Raum A"},
		{Name: "emre özdemir", Location: "Raum A"},
		{Name: "Lena Ärmel", Location: "Raum A"},
		{Name: "Anna Müller", Location: "Unterwegs"},
		{Name: "Ben Anders", Location: "Unterwegs"},
	}

	sortSnapshotRows(rows)

	got := make([]string, 0, len(rows))
	for _, row := range rows {
		got = append(got, row.Location+"/"+row.Name)
	}
	want := []string{
		"Raum A/emre özdemir",
		"Raum A/Jan Zimmermann",
		"Raum A/Lena Ärmel",
		"Unterwegs/Anna Müller",
		"Unterwegs/Ben Anders",
	}
	assert.Equal(t, want, got)
}

// --- Gesundheitsinfos auf der Notfallliste (#2609) ---

func healthDeps(t *testing.T, db *bun.DB, settings settingsReader, health map[int64]*string) Dependencies {
	t.Helper()
	students := map[int64]*users.Student{
		101: {PersonID: 301, SchoolClass: "Klasse 3b", HealthInfo: health[101]},
		202: {PersonID: 302, SchoolClass: "Klasse 2a", HealthInfo: health[202]},
	}
	return Dependencies{
		AttendanceRepo: stubAttendanceRepo{ids: []int64{101, 202}},
		StudentRepo:    stubStudentRepo{students: students},
		PersonRepo: stubPersonRepo{persons: map[int64]*users.Person{
			301: {FirstName: "Mila", LastName: "Albrecht"},
			302: {FirstName: "Max", LastName: "Schmitt"},
		}},
		ListExport:          listexport.NewService(),
		VisitRepo:           newVisitRepo(db),
		StudentGuardianRepo: usersRepo.NewStudentGuardianRepository(db),
		Settings:            settings,
	}
}

func expectEmptySnapshotQueries(mock sqlmock.Sqlmock) {
	mock.ExpectQuery(`(?s)SELECT .*active\.visits`).
		WillReturnRows(sqlmock.NewRows([]string{"student_id", "room_id"}))
	mock.ExpectQuery(`(?s)SELECT .*users\.students_guardians`).
		WillReturnRows(sqlmock.NewRows([]string{"student_id", "first_name", "last_name", "phone_number"}))
}

// healthByName indexes the rendered rows by child name: the document is
// sorted by location and then German collation, so an index is the wrong
// handle for "the child with the allergy".
func healthByName(t *testing.T, doc listexport.Document) map[string]string {
	t.Helper()
	out := make(map[string]string, len(doc.Rows))
	for _, row := range doc.Rows {
		out[row.Values[listexport.ColumnName]] = row.Values[listexport.ColumnHealthInfo]
	}
	return out
}

func columnIDs(doc listexport.Document) []listexport.ColumnID {
	ids := make([]listexport.ColumnID, 0, len(doc.Columns))
	for _, col := range doc.Columns {
		ids = append(ids, col.ID)
	}
	return ids
}

// With the setting on, every present child carries its stored health note —
// and a child WITHOUT one says so, rather than leaving a blank that reads as
// "no allergies".
func TestBuildSnapshotDocumentIncludesHealthInfoWhenEnabled(t *testing.T) {
	t.Parallel()

	db, mock, cleanup := newMockBunDB(t)
	defer cleanup()
	expectEmptySnapshotQueries(mock)

	note := "Nussallergie, Epipen im Gruppenraum"
	svc := NewService(healthDeps(t, db, stubSettings{enabled: true}, map[int64]*string{101: &note}))

	doc, err := svc.BuildSnapshotDocument(context.Background(), time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())

	require.Contains(t, columnIDs(doc), listexport.ColumnHealthInfo)
	require.Len(t, doc.Rows, 2)
	health := healthByName(t, doc)
	assert.Equal(t, note, health["Mila Albrecht"])
	assert.Equal(t, "Nicht hinterlegt", health["Max Schmitt"])
}

// A whitespace-only note is no note: on paper "   " and "" are the same blank,
// and both must be spelled out.
func TestBuildSnapshotDocumentTreatsBlankHealthInfoAsMissing(t *testing.T) {
	t.Parallel()

	db, mock, cleanup := newMockBunDB(t)
	defer cleanup()
	expectEmptySnapshotQueries(mock)

	blank := "   \n\t "
	svc := NewService(healthDeps(t, db, stubSettings{enabled: true}, map[int64]*string{101: &blank}))

	doc, err := svc.BuildSnapshotDocument(context.Background(), time.Time{})
	require.NoError(t, err)
	require.Len(t, doc.Rows, 2)
	assert.Equal(t, "Nicht hinterlegt", healthByName(t, doc)["Mila Albrecht"])
}

// A school that switched the setting off gets the old five-column list: no
// health column at all, not an empty one.
func TestBuildSnapshotDocumentOmitsHealthInfoWhenDisabled(t *testing.T) {
	t.Parallel()

	db, mock, cleanup := newMockBunDB(t)
	defer cleanup()
	expectEmptySnapshotQueries(mock)

	note := "Asthma, Spray in der Tasche"
	svc := NewService(healthDeps(t, db, stubSettings{enabled: false}, map[int64]*string{101: &note}))

	doc, err := svc.BuildSnapshotDocument(context.Background(), time.Time{})
	require.NoError(t, err)

	assert.NotContains(t, columnIDs(doc), listexport.ColumnHealthInfo)
	require.Len(t, doc.Rows, 2)
	for _, row := range doc.Rows {
		assert.NotContains(t, row.Values, listexport.ColumnHealthInfo)
	}
	// The rest of the list is untouched.
	assert.Contains(t, healthByName(t, doc), "Mila Albrecht")
}

// An unreadable setting must not print health data a school may have switched
// off — the column stays out and the remaining columns still render.
func TestBuildSnapshotDocumentOmitsHealthInfoWhenSettingUnreadable(t *testing.T) {
	t.Parallel()

	db, mock, cleanup := newMockBunDB(t)
	defer cleanup()
	expectEmptySnapshotQueries(mock)

	note := "Diabetes Typ 1"
	svc := NewService(healthDeps(t, db, stubSettings{enabled: true, err: assert.AnError}, map[int64]*string{101: &note}))

	doc, err := svc.BuildSnapshotDocument(context.Background(), time.Time{})
	require.NoError(t, err)

	assert.NotContains(t, columnIDs(doc), listexport.ColumnHealthInfo)
	require.Len(t, doc.Rows, 2)
	assert.Contains(t, healthByName(t, doc), "Mila Albrecht")
}

func TestHealthInfoCell(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "Nicht hinterlegt", healthInfoCell(""))
	assert.Equal(t, "Nicht hinterlegt", healthInfoCell("  \t\n "))
	assert.Equal(t, "Nussallergie", healthInfoCell("Nussallergie"))
	assert.Equal(t, "VorderseiteRückseite", healthInfoCell("Vorderseite\x03Rückseite"))
	assert.Equal(t, "Nussallergie", healthInfoCell("\x01Nuss\x02allergie"))
	assert.Equal(t, "Nicht hinterlegt", healthInfoCell(" \x01\x02\x03 \n"))
}
