package api

// The root binds the emergency snapshot projection's consumer-owned ports
// (#2704) to the owners' reads and only translates their records: Student
// Presence for open attendance, visits and the school status, People
// Directory for the child record with its health note, the person names and
// the guardian contacts, Facilities for the room names and Document Rendering
// for the PDF. The projection names none of these owners (#4077); the
// translation is cross-owner glue and lives here.

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	"github.com/moto-nrw/project-phoenix/modules/emergencysnapshot"
	emergencysnapshotCompose "github.com/moto-nrw/project-phoenix/modules/emergencysnapshot/compose"
	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	users "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// emergencySnapshotPresence is the slice of the Student Presence facade the
// projection reads: open attendance, the current visit per student, and the
// school status of the binary presence mode.
type emergencySnapshotPresence interface {
	ListOpenAttendanceStudentIDs(ctx context.Context, date string) ([]int64, error)
	ListVisitLocations(ctx context.Context, filter studentpresence.VisitLocationFilter) ([]studentpresence.VisitLocation, error)
	ListSchoolStatuses(ctx context.Context, ids []int64, date string) ([]studentpresence.SchoolStatus, error)
}

// emergencySnapshotPresenceMode resolves the school's presence concept; the
// retained active service validates the setting value.
type emergencySnapshotPresenceMode interface {
	GetPresenceMode(ctx context.Context) (string, error)
}

// emergencySnapshotStudents is the People Directory student read the
// projection needs: the record carries the health note no public facade
// exposes yet.
type emergencySnapshotStudents interface {
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*users.Student, error)
}

// emergencySnapshotPersons is the People Directory read of display
// identities.
type emergencySnapshotPersons interface {
	ListPersonsByID(ctx context.Context, ids []int64) ([]peopledirectory.Person, error)
}

// emergencySnapshotContacts is the People Directory guardian-contact query.
type emergencySnapshotContacts interface {
	ListEmergencyContactRows(ctx context.Context, studentIDs []int64) ([]users.GuardianEmergencyContactRow, error)
}

// emergencySnapshotRooms is the Facilities read of room names.
type emergencySnapshotRooms interface {
	ListRoomsByID(ctx context.Context, ids []int64) ([]facilities.Room, error)
}

// emergencySnapshotSources are the owner reads the root binds to the
// Notfallliste. Now and Logger are optional; every other source is required.
type emergencySnapshotSources struct {
	Presence     emergencySnapshotPresence
	PresenceMode emergencySnapshotPresenceMode
	Students     emergencySnapshotStudents
	Persons      emergencySnapshotPersons
	Contacts     emergencySnapshotContacts
	Rooms        emergencySnapshotRooms
	Renderer     lists.Renderer
	Now          func() time.Time
	Logger       *slog.Logger
}

// errEmergencySnapshotSources reports a missing source. Missing wiring is a
// configuration error and must fail composition, not the first export.
var errEmergencySnapshotSources = errors.New("emergency snapshot adapters are not fully configured")

// newEmergencySnapshot binds the owner reads to the projection's ports.
func newEmergencySnapshot(sources emergencySnapshotSources) (emergencysnapshot.Query, error) {
	if sources.Presence == nil || sources.PresenceMode == nil || sources.Students == nil ||
		sources.Persons == nil || sources.Contacts == nil || sources.Rooms == nil ||
		sources.Renderer == nil {
		return nil, errEmergencySnapshotSources
	}
	projection, err := emergencysnapshotCompose.New(emergencysnapshot.Dependencies{
		Presence: emergencySnapshotPresenceReads{facade: sources.Presence, mode: sources.PresenceMode},
		Rooms:    emergencySnapshotRoomNames{facade: sources.Rooms},
		Students: emergencySnapshotStudentRecords{repo: sources.Students},
		Persons:  emergencySnapshotPersonNames{facade: sources.Persons},
		Contacts: emergencySnapshotGuardianContacts{repo: sources.Contacts},
		Calendar: emergencySnapshotCalendar{},
		Renderer: emergencySnapshotRenderer{renderer: sources.Renderer},
		Now:      sources.Now,
		Logger:   sources.Logger,
	})
	if err != nil {
		return nil, err
	}
	return projection, nil
}

type emergencySnapshotPresenceReads struct {
	facade emergencySnapshotPresence
	mode   emergencySnapshotPresenceMode
}

func (p emergencySnapshotPresenceReads) PresentStudentIDs(ctx context.Context, date emergencysnapshot.Date) ([]int64, error) {
	return p.facade.ListOpenAttendanceStudentIDs(ctx, string(date))
}

func (p emergencySnapshotPresenceReads) Mode(ctx context.Context) (emergencysnapshot.PresenceMode, error) {
	mode, err := p.mode.GetPresenceMode(ctx)
	if err != nil {
		return "", err
	}
	return emergencysnapshot.PresenceMode(mode), nil
}

// CurrentRoomIDs selects the latest open visit per student inside a running
// session; the owner query applies the selection.
func (p emergencySnapshotPresenceReads) CurrentRoomIDs(ctx context.Context, studentIDs []int64) (map[int64]int64, error) {
	visits, err := p.facade.ListVisitLocations(ctx, studentpresence.CurrentRoomVisitsFilter(studentIDs))
	if err != nil {
		return nil, err
	}
	rooms := make(map[int64]int64, len(visits))
	for _, visit := range visits {
		if visit.Group != nil {
			rooms[visit.Visit.StudentID] = visit.Group.RoomID
		}
	}
	return rooms, nil
}

func (p emergencySnapshotPresenceReads) SchoolStatuses(ctx context.Context, studentIDs []int64, date emergencysnapshot.Date) (map[int64]string, error) {
	rows, err := p.facade.ListSchoolStatuses(ctx, studentIDs, string(date))
	if err != nil {
		// Student Presence masks the owner details on this path.
		return nil, studentpresence.SchoolStatusReadFailed()
	}
	statuses := make(map[int64]string, len(rows))
	for _, row := range rows {
		statuses[row.StudentID] = row.Status
	}
	return statuses, nil
}

type emergencySnapshotRoomNames struct {
	facade emergencySnapshotRooms
}

func (r emergencySnapshotRoomNames) Names(ctx context.Context, roomIDs []int64) (map[int64]string, error) {
	rows, err := r.facade.ListRoomsByID(ctx, roomIDs)
	if err != nil {
		return nil, err
	}
	names := make(map[int64]string, len(rows))
	for _, room := range rows {
		names[room.ID] = room.Name
	}
	return names, nil
}

type emergencySnapshotStudentRecords struct {
	repo emergencySnapshotStudents
}

func (s emergencySnapshotStudentRecords) ByIDs(ctx context.Context, studentIDs []int64) (map[int64]emergencysnapshot.Student, error) {
	rows, err := s.repo.FindByIDs(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]emergencysnapshot.Student, len(rows))
	for id, row := range rows {
		if row == nil {
			continue
		}
		result[id] = emergencysnapshot.Student{
			ID:          id,
			PersonID:    row.PersonID,
			SchoolClass: row.SchoolClass,
			HealthInfo:  stringValue(row.HealthInfo),
		}
	}
	return result, nil
}

type emergencySnapshotPersonNames struct {
	facade emergencySnapshotPersons
}

func (p emergencySnapshotPersonNames) ByIDs(ctx context.Context, personIDs []int64) (map[int64]emergencysnapshot.Person, error) {
	if len(personIDs) == 0 {
		return map[int64]emergencysnapshot.Person{}, nil
	}
	rows, err := p.facade.ListPersonsByID(ctx, personIDs)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]emergencysnapshot.Person, len(rows))
	for _, row := range rows {
		result[row.ID] = emergencysnapshot.Person{ID: row.ID, FirstName: row.FirstName, LastName: row.LastName}
	}
	return result, nil
}

type emergencySnapshotGuardianContacts struct {
	repo emergencySnapshotContacts
}

func (c emergencySnapshotGuardianContacts) EmergencyContacts(ctx context.Context, studentIDs []int64) ([]emergencysnapshot.Contact, error) {
	rows, err := c.repo.ListEmergencyContactRows(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	result := make([]emergencysnapshot.Contact, 0, len(rows))
	for _, row := range rows {
		result = append(result, emergencysnapshot.Contact{
			StudentID: row.StudentID,
			FirstName: stringValue(row.FirstName),
			LastName:  stringValue(row.LastName),
			Phone:     stringValue(row.PhoneNumber),
		})
	}
	return result, nil
}

type emergencySnapshotCalendar struct{}

func (emergencySnapshotCalendar) DayOf(at time.Time) emergencysnapshot.Date {
	return emergencysnapshot.Date(calendar.DateFromTime(at).String())
}

type emergencySnapshotRenderer struct {
	renderer lists.Renderer
}

// RenderPDF maps the projected document onto the Document Rendering shape
// field by field; the column ids are shared, so widths and styles apply.
func (r emergencySnapshotRenderer) RenderPDF(doc emergencysnapshot.Document, filenameBase string) (emergencysnapshot.File, error) {
	file, err := r.renderer.Render(emergencySnapshotListDocument(doc), lists.FormatPDF, filenameBase)
	if err != nil {
		return emergencysnapshot.File{}, err
	}
	return emergencysnapshot.File{Filename: file.Filename, ContentType: file.ContentType, Data: file.Data}, nil
}

// emergencySnapshotListDocument converts the projected document into the
// Document Rendering document.
func emergencySnapshotListDocument(doc emergencysnapshot.Document) lists.Document {
	columns := make([]lists.Column, 0, len(doc.Columns))
	for _, column := range doc.Columns {
		columns = append(columns, lists.Column{ID: lists.ColumnID(column.ID), Label: column.Label})
	}
	rows := make([]lists.Row, 0, len(doc.Rows))
	for _, row := range doc.Rows {
		values := make(map[lists.ColumnID]string, len(row))
		for id, value := range row {
			// Every cell is user text; the renderer's style markers must not
			// reach it from a stored value.
			values[lists.ColumnID(id)] = lists.SanitizeUserText(value)
		}
		rows = append(rows, lists.Row{Values: values})
	}
	return lists.Document{
		Title:       doc.Title,
		Subtitle:    doc.Subtitle,
		GeneratedAt: doc.GeneratedAt,
		Columns:     columns,
		Rows:        rows,
	}
}
