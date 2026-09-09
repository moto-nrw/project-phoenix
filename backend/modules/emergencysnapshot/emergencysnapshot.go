// Package emergencysnapshot is the public capability of the emergency
// snapshot read projection (#2704): the "Notfallliste", every child currently
// checked in with location, reachable adults and, when the school prints it,
// the health note. It reads every foreign fact through consumer-owned ports
// with plain records, owns the row order and the document shape, persists
// nothing and never writes.
package emergencysnapshot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"strings"
	"time"
)

// ErrIncompleteDependencies reports a missing port. Missing wiring is a
// configuration error and must fail composition, not the first export.
var ErrIncompleteDependencies = errors.New("emergency snapshot projection is not fully configured")

// Labels of the projected columns and cells. They are the printed wording of
// the Notfallliste and part of the projection's output contract.
const (
	Title        = "Notfallliste"
	FilenameBase = "notfallliste"

	// LocationUnknown prints for a child with an open attendance but no
	// open visit in a running session (detailed presence mode).
	LocationUnknown = "Unterwegs"
	// Binary presence mode prints the school status instead of a room.
	LocationPresent = "Anwesend"
	LocationYard    = "Schulhof"
	LocationAbsent  = "Abwesend"

	// HealthInfoMissing is what a child WITHOUT a stored health note prints
	// as. An empty cell reads as "no allergies" to whoever grabs the sheet,
	// which is the one reading that could get a child hurt, so the absence
	// of data says so in words (#2609).
	HealthInfoMissing = "Nicht hinterlegt"
)

// ColumnID names a document column. The ids match the Document Rendering
// column catalog so the renderer keeps its widths and styles.
type ColumnID string

const (
	ColumnName            ColumnID = "name"
	ColumnSchoolClass     ColumnID = "school_class"
	ColumnCurrentLocation ColumnID = "current_location"
	ColumnContactPhone    ColumnID = "contact_phone"
	ColumnContactName     ColumnID = "contact_name"
	ColumnHealthInfo      ColumnID = "health_info"
)

// Column is one printed column with its German heading.
type Column struct {
	ID    ColumnID
	Label string
}

// Row is one present child.
type Row struct {
	StudentID   int64
	Name        string
	SchoolClass string
	Location    string
	// ContactName and ContactPhone join the linked guardians and the legacy
	// free-text contact of the student row, duplicates removed.
	ContactName  string
	ContactPhone string
	// HealthInfo is the stored note, whitespace trimmed; empty when none.
	HealthInfo string
}

// Snapshot is the projected state of one instant: the present children in
// output order (location, then German dictionary order of the name).
type Snapshot struct {
	GeneratedAt time.Time
	Date        Date
	// IncludeHealthInfo reports whether the school prints the health column.
	IncludeHealthInfo bool
	Rows              []Row
}

// Document is the printable form of a Snapshot.
type Document struct {
	Title       string
	Subtitle    string
	GeneratedAt time.Time
	Columns     []Column
	Rows        []map[ColumnID]string
}

// File is a rendered document.
type File struct {
	Filename    string
	ContentType string
	Data        []byte
}

// Query is the one public capability of the projection.
type Query interface {
	// Snapshot projects the present children at the instant.
	Snapshot(ctx context.Context, at time.Time) (Snapshot, error)
	// Document projects the present children at the instant as the printable
	// Notfallliste.
	Document(ctx context.Context, at time.Time) (Document, error)
	// Export renders the Notfallliste of now as a PDF.
	Export(ctx context.Context) (File, error)
}

// Dependencies are the consumer-owned ports of the projection.
type Dependencies struct {
	Presence Presence
	Rooms    Rooms
	Students Students
	Persons  Persons
	Contacts Contacts
	Settings Settings
	Calendar Calendar
	Renderer Renderer
	// Collate orders two names in German dictionary order (DIN 5007-1).
	Collate func(a, b string) int
	// Now supplies the export instant; optional, defaults to time.Now.
	Now func() time.Time
	// Logger is optional; a nil logger falls back to slog.Default.
	Logger *slog.Logger
}

// Projection implements Query over the ports.
type Projection struct {
	deps Dependencies
}

// New composes the projection. Now and Logger are optional; every port is
// required.
func New(deps Dependencies) (*Projection, error) {
	if deps.Presence == nil || deps.Rooms == nil || deps.Students == nil || deps.Persons == nil ||
		deps.Contacts == nil || deps.Settings == nil || deps.Calendar == nil || deps.Renderer == nil ||
		deps.Collate == nil {
		return nil, ErrIncompleteDependencies
	}
	if deps.Now == nil {
		deps.Now = time.Now
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	return &Projection{deps: deps}, nil
}

func (p *Projection) Export(ctx context.Context) (File, error) {
	doc, err := p.Document(ctx, p.deps.Now())
	if err != nil {
		return File{}, err
	}
	return p.deps.Renderer.RenderPDF(doc, FilenameBase)
}

func (p *Projection) Document(ctx context.Context, at time.Time) (Document, error) {
	snapshot, err := p.Snapshot(ctx, at)
	if err != nil {
		return Document{}, err
	}
	return BuildDocument(snapshot), nil
}

func (p *Projection) Snapshot(ctx context.Context, at time.Time) (Snapshot, error) {
	date := p.deps.Calendar.DayOf(at)
	studentIDs, err := p.deps.Presence.PresentStudentIDs(ctx, date)
	if err != nil {
		return Snapshot{}, err
	}

	rows, err := p.loadRows(ctx, date, studentIDs)
	if err != nil {
		return Snapshot{}, err
	}

	return Snapshot{
		GeneratedAt:       at,
		Date:              date,
		IncludeHealthInfo: p.healthInfoEnabled(ctx),
		Rows:              rows,
	}, nil
}

// healthInfoEnabled reports whether the school prints health notes on the
// Notfallliste. It is NOT a read gate: the note is already visible to every
// account with users:read in the child's record, and this export requires
// the same permission; the switch only decides what lands on the paper.
//
// It errs towards leaving the column OFF when the setting cannot be read: a
// school that switched it off did so for a data-protection reason, and a
// column that appears because a lookup failed would break that silently.
// The rest of the list (names, location, phone numbers) is unaffected, so
// the sheet still does its job.
func (p *Projection) healthInfoEnabled(ctx context.Context) bool {
	enabled, err := p.deps.Settings.HealthInfoEnabled(ctx)
	if err != nil {
		p.deps.Logger.WarnContext(ctx, "emergency list: health info setting could not be resolved, printing list without health column",
			slog.String("error", err.Error()),
		)
		return false
	}
	return enabled
}

func (p *Projection) loadRows(ctx context.Context, date Date, studentIDs []int64) ([]Row, error) {
	if len(studentIDs) == 0 {
		return []Row{}, nil
	}

	students, err := p.deps.Students.ByIDs(ctx, studentIDs)
	if err != nil {
		return nil, err
	}

	personIDs := make([]int64, 0, len(students))
	for _, student := range students {
		personIDs = append(personIDs, student.PersonID)
	}
	slices.Sort(personIDs)

	persons, err := p.deps.Persons.ByIDs(ctx, personIDs)
	if err != nil {
		return nil, err
	}

	locations, err := p.loadLocations(ctx, date, studentIDs)
	if err != nil {
		return nil, err
	}

	contacts, err := p.loadContacts(ctx, studentIDs)
	if err != nil {
		return nil, err
	}

	rows := buildRows(studentIDs, students, persons, locations, contacts)
	p.sortRows(rows)
	return rows, nil
}

func buildRows(
	studentIDs []int64,
	students map[int64]Student,
	persons map[int64]Person,
	locations map[int64]string,
	contacts map[int64]contact,
) []Row {
	rows := make([]Row, 0, len(studentIDs))
	for _, id := range studentIDs {
		student, ok := students[id]
		if !ok {
			continue
		}
		person, ok := persons[student.PersonID]
		if !ok {
			continue
		}
		row := Row{
			StudentID:   id,
			Name:        person.FirstName + " " + person.LastName,
			SchoolClass: student.SchoolClass,
			Location:    locations[id],
			HealthInfo:  strings.TrimSpace(student.HealthInfo),
		}
		if row.Location == "" {
			row.Location = LocationUnknown
		}
		linked := contacts[id]
		row.ContactName = joinUnique(linked.name, student.GuardianName)
		row.ContactPhone = joinUnique(linked.phone, student.GuardianPhone, student.GuardianContact)
		rows = append(rows, row)
	}
	return rows
}

// sortRows orders by location (plain byte order, so the room blocks stay
// stable) and then by name in German dictionary order.
func (p *Projection) sortRows(rows []Row) {
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Location != rows[j].Location {
			return rows[i].Location < rows[j].Location
		}
		return p.deps.Collate(rows[i].Name, rows[j].Name) < 0
	})
}

func (p *Projection) loadLocations(ctx context.Context, date Date, studentIDs []int64) (map[int64]string, error) {
	mode, err := p.deps.Presence.Mode(ctx)
	if err != nil {
		return nil, err
	}
	if mode == PresenceModeBinary {
		return p.loadBinaryLocations(ctx, date, studentIDs)
	}
	return p.loadRoomLocations(ctx, studentIDs)
}

func (p *Projection) loadRoomLocations(ctx context.Context, studentIDs []int64) (map[int64]string, error) {
	locations := make(map[int64]string)
	if len(studentIDs) == 0 {
		return locations, nil
	}
	roomByStudent, err := p.deps.Presence.CurrentRoomIDs(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	roomIDs := make([]int64, 0, len(roomByStudent))
	seen := make(map[int64]bool, len(roomByStudent))
	for _, studentID := range studentIDs {
		roomID, ok := roomByStudent[studentID]
		if ok && !seen[roomID] {
			seen[roomID] = true
			roomIDs = append(roomIDs, roomID)
		}
	}
	if len(roomIDs) == 0 {
		return locations, nil
	}
	names, err := p.deps.Rooms.Names(ctx, roomIDs)
	if err != nil {
		return nil, err
	}
	for studentID, roomID := range roomByStudent {
		if name, ok := names[roomID]; ok {
			locations[studentID] = name
		}
	}
	return locations, nil
}

func (p *Projection) loadBinaryLocations(ctx context.Context, date Date, studentIDs []int64) (map[int64]string, error) {
	statuses, err := p.deps.Presence.SchoolStatuses(ctx, studentIDs, date)
	if err != nil {
		return nil, err
	}
	locations := make(map[int64]string, len(studentIDs))
	for _, id := range studentIDs {
		locations[id] = binaryLocationLabel(statuses[id])
	}
	return locations, nil
}

func binaryLocationLabel(status string) string {
	switch status {
	case StatusCheckedIn:
		return LocationPresent
	case StatusOnYard:
		return LocationYard
	default:
		return LocationAbsent
	}
}

type contact struct {
	name  string
	phone string
}

func (p *Projection) loadContacts(ctx context.Context, studentIDs []int64) (map[int64]contact, error) {
	rows, err := p.deps.Contacts.EmergencyContacts(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	contacts := make(map[int64]contact, len(rows))
	for _, row := range rows {
		current := contacts[row.StudentID]
		current.name = joinUnique(current.name, strings.TrimSpace(row.FirstName+" "+row.LastName))
		current.phone = joinUnique(current.phone, row.Phone)
		contacts[row.StudentID] = current
	}
	return contacts, nil
}

// BuildDocument lays the snapshot out as the printable Notfallliste.
func BuildDocument(snapshot Snapshot) Document {
	columns := []Column{
		{ID: ColumnName, Label: "Name"},
		{ID: ColumnSchoolClass, Label: "Klasse"},
		{ID: ColumnCurrentLocation, Label: "Ort / Raum"},
		{ID: ColumnContactPhone, Label: "Telefonnummer"},
		{ID: ColumnContactName, Label: "Kontakt"},
	}
	if snapshot.IncludeHealthInfo {
		columns = append(columns, Column{ID: ColumnHealthInfo, Label: "Gesundheit / Allergien"})
	}
	rows := make([]map[ColumnID]string, 0, len(snapshot.Rows))
	for _, row := range snapshot.Rows {
		values := map[ColumnID]string{
			ColumnName:            row.Name,
			ColumnSchoolClass:     row.SchoolClass,
			ColumnCurrentLocation: row.Location,
			ColumnContactPhone:    row.ContactPhone,
			ColumnContactName:     row.ContactName,
		}
		if snapshot.IncludeHealthInfo {
			values[ColumnHealthInfo] = HealthInfoCell(row.HealthInfo)
		}
		rows = append(rows, values)
	}
	return Document{
		Title:       Title,
		Subtitle:    fmt.Sprintf("%d anwesende Kinder", len(snapshot.Rows)),
		GeneratedAt: snapshot.GeneratedAt,
		Columns:     columns,
		Rows:        rows,
	}
}

// HealthInfoCell renders one child's health note. Control characters are
// dropped because renderers reserve them as style markers, and a
// whitespace-only note counts as no note: "   " on screen and "" on paper
// mean the same thing to a reader, and both must say HealthInfoMissing
// rather than leave a blank that reads as an all-clear.
func HealthInfoCell(note string) string {
	note = stripControls(note)
	if strings.TrimSpace(note) == "" {
		return HealthInfoMissing
	}
	return note
}

// stripControls removes C0 control characters except tab, newline and
// carriage return, so a stored note can never carry a renderer's reserved
// style marker onto the paper.
func stripControls(text string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 && r != '\n' && r != '\t' && r != '\r' {
			return -1
		}
		return r
	}, text)
}

// joinUnique splits every value on ";", trims the parts and joins the
// non-empty ones back on "; " with case-insensitive duplicates removed (first
// occurrence wins, original casing kept).
func joinUnique(values ...string) string {
	seen := make(map[string]struct{}, len(values))
	parts := make([]string, 0, len(values))
	for _, value := range values {
		for part := range strings.SplitSeq(value, ";") {
			trimmed := strings.TrimSpace(part)
			if trimmed == "" {
				continue
			}
			key := strings.ToLower(trimmed)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			parts = append(parts, trimmed)
		}
	}
	return strings.Join(parts, "; ")
}
