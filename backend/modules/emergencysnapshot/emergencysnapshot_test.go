package emergencysnapshot

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const kreativraumRoomID int64 = 7007

type fakePresence struct {
	present    []int64
	presentErr error
	mode       PresenceMode
	modeErr    error
	rooms      map[int64]int64
	roomsErr   error
	statuses   map[int64]string
	statusErr  error
	roomsAsked []int64
	statusDate Date
}

func (f *fakePresence) PresentStudentIDs(_ context.Context, _ Date) ([]int64, error) {
	return f.present, f.presentErr
}

func (f *fakePresence) Mode(_ context.Context) (PresenceMode, error) {
	if f.modeErr != nil {
		return "", f.modeErr
	}
	if f.mode == "" {
		return PresenceModeDetailed, nil
	}
	return f.mode, nil
}

func (f *fakePresence) CurrentRoomIDs(_ context.Context, ids []int64) (map[int64]int64, error) {
	f.roomsAsked = ids
	return f.rooms, f.roomsErr
}

func (f *fakePresence) SchoolStatuses(_ context.Context, _ []int64, date Date) (map[int64]string, error) {
	f.statusDate = date
	return f.statuses, f.statusErr
}

type fakeRooms struct {
	names map[int64]string
	err   error
	asked []int64
}

func (f *fakeRooms) Names(_ context.Context, ids []int64) (map[int64]string, error) {
	f.asked = ids
	if f.err != nil {
		return nil, f.err
	}
	out := make(map[int64]string, len(ids))
	for _, id := range ids {
		if name, ok := f.names[id]; ok {
			out[id] = name
		}
	}
	return out, nil
}

type fakeStudents struct {
	students map[int64]Student
	err      error
}

func (f fakeStudents) ByIDs(_ context.Context, _ []int64) (map[int64]Student, error) {
	return f.students, f.err
}

type fakePersons struct {
	persons map[int64]Person
	err     error
	asked   []int64
}

func (f *fakePersons) ByIDs(_ context.Context, ids []int64) (map[int64]Person, error) {
	f.asked = ids
	return f.persons, f.err
}

type fakeContacts struct {
	rows []Contact
	err  error
}

func (f fakeContacts) EmergencyContacts(_ context.Context, _ []int64) ([]Contact, error) {
	return f.rows, f.err
}

type fakeSettings struct {
	enabled bool
	err     error
}

func (f fakeSettings) HealthInfoEnabled(_ context.Context) (bool, error) {
	return f.enabled, f.err
}

type fakeCalendar struct{}

func (fakeCalendar) DayOf(at time.Time) Date { return Date(at.Format("2006-01-02")) }

type fakeRenderer struct {
	doc  Document
	base string
	err  error
}

func (f *fakeRenderer) RenderPDF(doc Document, base string) (File, error) {
	f.doc = doc
	f.base = base
	if f.err != nil {
		return File{}, f.err
	}
	return File{Filename: base + ".pdf", ContentType: "application/pdf", Data: []byte("%PDF")}, nil
}

// collateASCIIFold stands in for the German collation: case-insensitive with
// umlauts folded to their base letter, which is what the ordering tests need.
func collateASCIIFold(a, b string) int {
	fold := strings.NewReplacer("ä", "a", "ö", "o", "ü", "u", "Ä", "a", "Ö", "o", "Ü", "u", "ß", "ss")
	return strings.Compare(strings.ToLower(fold.Replace(a)), strings.ToLower(fold.Replace(b)))
}

func fullDeps() Dependencies {
	return Dependencies{
		Presence: &fakePresence{},
		Rooms:    &fakeRooms{},
		Students: fakeStudents{},
		Persons:  &fakePersons{},
		Contacts: fakeContacts{},
		Settings: fakeSettings{},
		Calendar: fakeCalendar{},
		Renderer: &fakeRenderer{},
		Collate:  collateASCIIFold,
	}
}

func newProjection(t *testing.T, deps Dependencies) *Projection {
	t.Helper()
	projection, err := New(deps)
	require.NoError(t, err)
	return projection
}

var generatedAt = time.Date(2026, 5, 27, 12, 0, 0, 0, time.UTC)

// twoChildren is the fixture of the retained service tests: one child in a
// room with three guardian rows (one duplicate name), one child without a
// visit whose only contact is the legacy free-text guardian.
func twoChildren() Dependencies {
	deps := fullDeps()
	deps.Presence = &fakePresence{present: []int64{101, 202}, rooms: map[int64]int64{101: kreativraumRoomID}}
	deps.Rooms = &fakeRooms{names: map[int64]string{kreativraumRoomID: "Kreativraum"}}
	deps.Students = fakeStudents{students: map[int64]Student{
		101: {ID: 101, PersonID: 301, SchoolClass: "Klasse 3b"},
		202: {ID: 202, PersonID: 302, SchoolClass: "Klasse 2a", GuardianName: "Familie Schmitt", GuardianPhone: "02551 444"},
	}}
	deps.Persons = &fakePersons{persons: map[int64]Person{
		301: {ID: 301, FirstName: "Mila", LastName: "Albrecht"},
		302: {ID: 302, FirstName: "Max", LastName: "Schmitt"},
	}}
	deps.Contacts = fakeContacts{rows: []Contact{
		{StudentID: 101, FirstName: "Lea", LastName: "Albrecht", Phone: "02551 111"},
		{StudentID: 101, FirstName: "Noah", LastName: "Albrecht", Phone: "02551 222"},
		{StudentID: 101, FirstName: "Lea", LastName: "Albrecht", Phone: "02551 333"},
	}}
	return deps
}

func TestNewRejectsMissingPorts(t *testing.T) {
	t.Parallel()
	_, err := New(Dependencies{})
	require.ErrorIs(t, err, ErrIncompleteDependencies)

	deps := fullDeps()
	deps.Collate = nil
	_, err = New(deps)
	require.ErrorIs(t, err, ErrIncompleteDependencies)

	_, err = New(fullDeps())
	require.NoError(t, err)
}

func TestSnapshotProjectsCurrentRows(t *testing.T) {
	t.Parallel()
	deps := twoChildren()
	projection := newProjection(t, deps)

	snapshot, err := projection.Snapshot(context.Background(), generatedAt)
	require.NoError(t, err)

	assert.Equal(t, generatedAt, snapshot.GeneratedAt)
	assert.Equal(t, Date("2026-05-27"), snapshot.Date)
	assert.False(t, snapshot.IncludeHealthInfo)
	require.Equal(t, []Row{
		{StudentID: 101, Name: "Mila Albrecht", SchoolClass: "Klasse 3b", Location: "Kreativraum",
			ContactName: "Lea Albrecht; Noah Albrecht", ContactPhone: "02551 111; 02551 222; 02551 333"},
		{StudentID: 202, Name: "Max Schmitt", SchoolClass: "Klasse 2a", Location: "Unterwegs",
			ContactName: "Familie Schmitt", ContactPhone: "02551 444"},
	}, snapshot.Rows)
	assert.Equal(t, []int64{301, 302}, deps.Persons.(*fakePersons).asked, "person ids are requested in stable order")
	assert.Equal(t, []int64{kreativraumRoomID}, deps.Rooms.(*fakeRooms).asked, "only the rooms in use are resolved")
}

// The document is the retained service's Notfallliste: same title, subtitle,
// column ids, labels and cell values.
func TestDocumentMatchesRetainedLayout(t *testing.T) {
	t.Parallel()
	projection := newProjection(t, twoChildren())

	doc, err := projection.Document(context.Background(), generatedAt)
	require.NoError(t, err)

	assert.Equal(t, Document{
		Title:       "Notfallliste",
		Subtitle:    "2 anwesende Kinder",
		GeneratedAt: generatedAt,
		Columns: []Column{
			{ID: "name", Label: "Name"},
			{ID: "school_class", Label: "Klasse"},
			{ID: "current_location", Label: "Ort / Raum"},
			{ID: "contact_phone", Label: "Telefonnummer"},
			{ID: "contact_name", Label: "Kontakt"},
		},
		Rows: []map[ColumnID]string{
			{"name": "Mila Albrecht", "school_class": "Klasse 3b", "current_location": "Kreativraum", "contact_phone": "02551 111; 02551 222; 02551 333", "contact_name": "Lea Albrecht; Noah Albrecht"},
			{"name": "Max Schmitt", "school_class": "Klasse 2a", "current_location": "Unterwegs", "contact_phone": "02551 444", "contact_name": "Familie Schmitt"},
		},
	}, doc)
}

func TestSnapshotUsesBinaryLocations(t *testing.T) {
	t.Parallel()
	deps := twoChildren()
	presence := &fakePresence{present: []int64{101, 202}, mode: PresenceModeBinary,
		statuses: map[int64]string{101: StatusCheckedIn, 202: StatusOnYard}}
	deps.Presence = presence
	deps.Rooms = &fakeRooms{err: errors.New("rooms must not be read in binary mode")}
	projection := newProjection(t, deps)

	snapshot, err := projection.Snapshot(context.Background(), generatedAt)
	require.NoError(t, err)
	require.Len(t, snapshot.Rows, 2)
	assert.Equal(t, "Anwesend", snapshot.Rows[0].Location)
	assert.Equal(t, "Schulhof", snapshot.Rows[1].Location)
	assert.Equal(t, Date("2026-05-27"), presence.statusDate)
	assert.Nil(t, presence.roomsAsked, "binary mode never reads visits")
}

func TestBinaryLocationLabel(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "Anwesend", binaryLocationLabel(StatusCheckedIn))
	assert.Equal(t, "Schulhof", binaryLocationLabel(StatusOnYard))
	assert.Equal(t, "Abwesend", binaryLocationLabel("checked_out"))
	assert.Equal(t, "Abwesend", binaryLocationLabel(""))
}

func TestSnapshotWithNoStudentsReadsNothingElse(t *testing.T) {
	t.Parallel()
	deps := fullDeps()
	deps.Presence = &fakePresence{present: []int64{}, modeErr: errors.New("mode must not be read")}
	deps.Students = fakeStudents{err: errors.New("students must not be read")}
	projection := newProjection(t, deps)

	doc, err := projection.Document(context.Background(), time.Time{})
	require.NoError(t, err)
	assert.Equal(t, "0 anwesende Kinder", doc.Subtitle)
	assert.Empty(t, doc.Rows)
}

func TestExportRendersThroughTheRendererPort(t *testing.T) {
	t.Parallel()
	deps := twoChildren()
	renderer := &fakeRenderer{}
	deps.Renderer = renderer
	deps.Now = func() time.Time { return generatedAt }
	projection := newProjection(t, deps)

	file, err := projection.Export(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "notfallliste.pdf", file.Filename)
	assert.Equal(t, "application/pdf", file.ContentType)
	assert.NotEmpty(t, file.Data)
	assert.Equal(t, "notfallliste", renderer.base)
	assert.Equal(t, generatedAt, renderer.doc.GeneratedAt)
	assert.Len(t, renderer.doc.Rows, 2)

	renderer.err = errors.New("renderer down")
	_, err = projection.Export(context.Background())
	require.ErrorIs(t, err, renderer.err)
}

// One owner-query failure surfaces unchanged, whichever port fails, and stops
// the later reads.
func TestSnapshotSurfacesOwnerFailures(t *testing.T) {
	t.Parallel()
	injected := errors.New("owner query failed")
	cases := map[string]func(deps *Dependencies){
		"present":  func(deps *Dependencies) { deps.Presence = &fakePresence{presentErr: injected} },
		"students": func(deps *Dependencies) { deps.Students = fakeStudents{err: injected} },
		"persons":  func(deps *Dependencies) { deps.Persons = &fakePersons{err: injected} },
		"mode":     func(deps *Dependencies) { deps.Presence = &fakePresence{present: []int64{101, 202}, modeErr: injected} },
		"rooms": func(deps *Dependencies) {
			deps.Presence = &fakePresence{present: []int64{101, 202}, roomsErr: injected}
		},
		"room names": func(deps *Dependencies) { deps.Rooms = &fakeRooms{err: injected} },
		"statuses": func(deps *Dependencies) {
			deps.Presence = &fakePresence{present: []int64{101, 202}, mode: PresenceModeBinary, statusErr: injected}
		},
		"contacts": func(deps *Dependencies) { deps.Contacts = fakeContacts{err: injected} },
	}
	for name, breakPort := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			deps := twoChildren()
			renderer := &fakeRenderer{}
			deps.Renderer = renderer
			breakPort(&deps)
			projection := newProjection(t, deps)

			_, err := projection.Export(context.Background())
			require.ErrorIs(t, err, injected)
			assert.Empty(t, renderer.base, "a failed read never reaches the renderer")
		})
	}
}

func TestRoomLocationsSkipStudentsWithoutRunningSession(t *testing.T) {
	t.Parallel()
	deps := fullDeps()
	presence := &fakePresence{rooms: map[int64]int64{1: 10, 2: 10, 3: 99}}
	rooms := &fakeRooms{names: map[int64]string{10: "Kreativraum"}}
	deps.Presence = presence
	deps.Rooms = rooms
	projection := newProjection(t, deps)

	locations, err := projection.loadRoomLocations(context.Background(), []int64{1, 2, 3, 4})
	require.NoError(t, err)
	assert.Equal(t, map[int64]string{1: "Kreativraum", 2: "Kreativraum"}, locations, "an unknown room leaves the child without a location")
	assert.Equal(t, []int64{10, 99}, rooms.asked, "rooms are asked once each, in student order")
	assert.Equal(t, []int64{1, 2, 3, 4}, presence.roomsAsked)

	locations, err = projection.loadRoomLocations(context.Background(), nil)
	require.NoError(t, err)
	assert.Empty(t, locations)
}

func TestRowsOrderByLocationThenGermanName(t *testing.T) {
	t.Parallel()
	projection := newProjection(t, fullDeps())
	rows := []Row{
		{Name: "Jan Zimmermann", Location: "Raum A"},
		{Name: "emre özdemir", Location: "Raum A"},
		{Name: "Lena Ärmel", Location: "Raum A"},
		{Name: "Anna Müller", Location: "Unterwegs"},
		{Name: "Ben Anders", Location: "Unterwegs"},
	}

	projection.sortRows(rows)

	got := make([]string, 0, len(rows))
	for _, row := range rows {
		got = append(got, row.Location+"/"+row.Name)
	}
	assert.Equal(t, []string{
		"Raum A/emre özdemir",
		"Raum A/Jan Zimmermann",
		"Raum A/Lena Ärmel",
		"Unterwegs/Anna Müller",
		"Unterwegs/Ben Anders",
	}, got)
}

func TestRowsSkipUnknownStudentsAndPersons(t *testing.T) {
	t.Parallel()
	const known, personless, unknown int64 = 101, 202, 303
	rows := buildRows([]int64{known, personless, unknown},
		map[int64]Student{known: {ID: known, PersonID: 301}, personless: {ID: personless, PersonID: 302}},
		map[int64]Person{301: {ID: 301, FirstName: "Ada", LastName: "Lovelace"}},
		nil, nil)
	require.Len(t, rows, 1)
	assert.Equal(t, known, rows[0].StudentID)
	assert.Equal(t, "Ada Lovelace", rows[0].Name)
	assert.Equal(t, "Unterwegs", rows[0].Location)
}

func TestJoinUnique(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "Lea Albrecht; Noah Albrecht", joinUnique("Lea Albrecht", "Noah Albrecht", "lea albrecht"))
	assert.Equal(t, "02551 111; 02551 222", joinUnique("02551 111; 02551 222", "02551 111"))
	assert.Empty(t, joinUnique("", " "))
}

// --- Gesundheitsinfos auf der Notfallliste (#2609) ---

func healthDeps(settings Settings, health map[int64]string) Dependencies {
	deps := twoChildren()
	deps.Students = fakeStudents{students: map[int64]Student{
		101: {ID: 101, PersonID: 301, SchoolClass: "Klasse 3b", HealthInfo: health[101]},
		202: {ID: 202, PersonID: 302, SchoolClass: "Klasse 2a", HealthInfo: health[202]},
	}}
	deps.Settings = settings
	return deps
}

// healthByName indexes the rendered rows by child name: the document is
// sorted by location and then German collation, so an index is the wrong
// handle for "the child with the allergy".
func healthByName(doc Document) map[string]string {
	out := make(map[string]string, len(doc.Rows))
	for _, row := range doc.Rows {
		out[row[ColumnName]] = row[ColumnHealthInfo]
	}
	return out
}

func columnIDs(doc Document) []ColumnID {
	ids := make([]ColumnID, 0, len(doc.Columns))
	for _, col := range doc.Columns {
		ids = append(ids, col.ID)
	}
	return ids
}

// With the setting on, every present child carries its stored health note,
// and a child WITHOUT one says so rather than leaving a blank that reads as
// "no allergies".
func TestDocumentIncludesHealthInfoWhenEnabled(t *testing.T) {
	t.Parallel()
	note := "Nussallergie, Epipen im Gruppenraum"
	projection := newProjection(t, healthDeps(fakeSettings{enabled: true}, map[int64]string{101: note}))

	doc, err := projection.Document(context.Background(), generatedAt)
	require.NoError(t, err)

	require.Equal(t, []ColumnID{ColumnName, ColumnSchoolClass, ColumnCurrentLocation, ColumnContactPhone, ColumnContactName, ColumnHealthInfo}, columnIDs(doc))
	assert.Equal(t, "Gesundheit / Allergien", doc.Columns[5].Label)
	require.Len(t, doc.Rows, 2)
	health := healthByName(doc)
	assert.Equal(t, note, health["Mila Albrecht"])
	assert.Equal(t, "Nicht hinterlegt", health["Max Schmitt"])
}

// A whitespace-only note is no note: on paper "   " and "" are the same
// blank, and both must be spelled out.
func TestDocumentTreatsBlankHealthInfoAsMissing(t *testing.T) {
	t.Parallel()
	projection := newProjection(t, healthDeps(fakeSettings{enabled: true}, map[int64]string{101: "   \n\t "}))

	doc, err := projection.Document(context.Background(), time.Time{})
	require.NoError(t, err)
	require.Len(t, doc.Rows, 2)
	assert.Equal(t, "Nicht hinterlegt", healthByName(doc)["Mila Albrecht"])
}

// A school that switched the setting off gets the old five-column list: no
// health column at all, not an empty one.
func TestDocumentOmitsHealthInfoWhenDisabled(t *testing.T) {
	t.Parallel()
	projection := newProjection(t, healthDeps(fakeSettings{enabled: false}, map[int64]string{101: "Asthma, Spray in der Tasche"}))

	doc, err := projection.Document(context.Background(), time.Time{})
	require.NoError(t, err)

	assert.NotContains(t, columnIDs(doc), ColumnHealthInfo)
	require.Len(t, doc.Rows, 2)
	for _, row := range doc.Rows {
		assert.NotContains(t, row, ColumnHealthInfo)
	}
	assert.Contains(t, healthByName(doc), "Mila Albrecht")
}

// An unreadable setting must not print health data a school may have
// switched off: the column stays out and the remaining columns still render.
func TestDocumentOmitsHealthInfoWhenSettingUnreadable(t *testing.T) {
	t.Parallel()
	projection := newProjection(t, healthDeps(fakeSettings{enabled: true, err: assert.AnError}, map[int64]string{101: "Diabetes Typ 1"}))

	doc, err := projection.Document(context.Background(), time.Time{})
	require.NoError(t, err)

	assert.NotContains(t, columnIDs(doc), ColumnHealthInfo)
	require.Len(t, doc.Rows, 2)
	assert.Contains(t, healthByName(doc), "Mila Albrecht")
}

func TestHealthInfoCell(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "Nicht hinterlegt", HealthInfoCell(""))
	assert.Equal(t, "Nicht hinterlegt", HealthInfoCell("  \t\n "))
	assert.Equal(t, "Nussallergie", HealthInfoCell("Nussallergie"))
	assert.Equal(t, "VorderseiteRückseite", HealthInfoCell("Vorderseite\x03Rückseite"))
	assert.Equal(t, "Nussallergie", HealthInfoCell("\x01Nuss\x02allergie"))
	assert.Equal(t, "Nicht hinterlegt", HealthInfoCell(" \x01\x02\x03 \n"))
	assert.Equal(t, "Zeile 1\nZeile 2", HealthInfoCell("Zeile 1\nZeile 2"), "line breaks survive")
}
