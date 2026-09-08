// Package legacy adapts the retained owner services and the public owner
// facades to the emergency snapshot projection's consumer-owned ports
// (#2704). It exists because the student record with its health note and
// legacy contact columns, the guardian contact rows, the presence-mode
// setting and the Document Rendering renderer still live in legacy packages;
// the adapters translate rows into plain records and delegate every rule to
// its owner, deciding nothing themselves. Delete this package with the last
// legacy source once each owner exposes the fact through its public
// capability.
package legacy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/collation"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/emergencysnapshot"
	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/services/listexport"
)

// PresenceSource is the slice of the Student Presence facade the projection
// reads: open attendance, the current visit per student, and the school
// status of the binary presence mode.
type PresenceSource interface {
	ListOpenAttendanceStudentIDs(ctx context.Context, date string) ([]int64, error)
	ListVisitLocations(ctx context.Context, filter studentpresence.VisitLocationFilter) ([]studentpresence.VisitLocation, error)
	ListSchoolStatuses(ctx context.Context, ids []int64, date string) ([]studentpresence.SchoolStatus, error)
}

// PresenceModeSource resolves the school's presence concept; the retained
// active service validates the setting value.
type PresenceModeSource interface {
	GetPresenceMode(ctx context.Context) (string, error)
}

// StudentSource is the retained student repository read the projection
// needs: the record carries the health note and the legacy contact columns
// no public facade exposes yet.
type StudentSource interface {
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*usersModels.Student, error)
}

// PersonSource is the People Directory read of display identities.
type PersonSource interface {
	ListPersonsByID(ctx context.Context, ids []int64) ([]peopledirectory.Person, error)
}

// ContactSource is the retained guardian-contact projection query of the
// People Directory owner.
type ContactSource interface {
	ListEmergencyContactRows(ctx context.Context, studentIDs []int64) ([]usersModels.GuardianEmergencyContactRow, error)
}

// RoomSource is the Facilities read of room names.
type RoomSource interface {
	ListRoomsByID(ctx context.Context, ids []int64) ([]facilities.Room, error)
}

// SettingsSource is the one tenant setting the projection reads.
type SettingsSource interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
}

// RendererSource is the Document Rendering renderer.
type RendererSource interface {
	Render(doc listexport.Document, format listexport.Format, filenameBase string) (listexport.File, error)
}

// Sources are the retained services and owner facades the projection's ports
// adapt.
type Sources struct {
	Presence     PresenceSource
	PresenceMode PresenceModeSource
	Students     StudentSource
	Persons      PersonSource
	Contacts     ContactSource
	Rooms        RoomSource
	Settings     SettingsSource
	Renderer     RendererSource
	// Now is optional and defaults to time.Now.
	Now func() time.Time
	// Logger is optional.
	Logger *slog.Logger
}

// ErrIncompleteSources reports a missing source. Missing wiring is a
// configuration error and must fail composition, not the first export.
var ErrIncompleteSources = errors.New("emergency snapshot adapters are not fully configured")

// New composes the emergency snapshot projection over the sources.
func New(sources Sources) (emergencysnapshot.Query, error) {
	if sources.Presence == nil || sources.PresenceMode == nil || sources.Students == nil ||
		sources.Persons == nil || sources.Contacts == nil || sources.Rooms == nil ||
		sources.Settings == nil || sources.Renderer == nil {
		return nil, ErrIncompleteSources
	}
	projection, err := emergencysnapshot.New(emergencysnapshot.Dependencies{
		Presence: presence{facade: sources.Presence, mode: sources.PresenceMode},
		Rooms:    rooms{facade: sources.Rooms},
		Students: students{repo: sources.Students},
		Persons:  persons{facade: sources.Persons},
		Contacts: contacts{repo: sources.Contacts},
		Settings: settings{settings: sources.Settings},
		Calendar: calendar{},
		Renderer: renderer{renderer: sources.Renderer},
		Collate:  collation.CompareGerman,
		Now:      sources.Now,
		Logger:   sources.Logger,
	})
	if err != nil {
		return nil, err
	}
	return projection, nil
}

type presence struct {
	facade PresenceSource
	mode   PresenceModeSource
}

func (p presence) PresentStudentIDs(ctx context.Context, date emergencysnapshot.Date) ([]int64, error) {
	return p.facade.ListOpenAttendanceStudentIDs(ctx, string(date))
}

func (p presence) Mode(ctx context.Context) (emergencysnapshot.PresenceMode, error) {
	mode, err := p.mode.GetPresenceMode(ctx)
	if err != nil {
		return "", err
	}
	return emergencysnapshot.PresenceMode(mode), nil
}

// CurrentRoomIDs selects the latest open visit per student inside a running
// session; the owner query applies the selection.
func (p presence) CurrentRoomIDs(ctx context.Context, studentIDs []int64) (map[int64]int64, error) {
	visits, err := p.facade.ListVisitLocations(ctx, studentpresence.VisitLocationFilter{
		VisitFilter:       studentpresence.VisitFilter{StudentIDs: studentIDs, OpenOnly: true},
		RunningGroupsOnly: true,
		LatestPerStudent:  true,
	})
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

func (p presence) SchoolStatuses(ctx context.Context, studentIDs []int64, date emergencysnapshot.Date) (map[int64]string, error) {
	rows, err := p.facade.ListSchoolStatuses(ctx, studentIDs, string(date))
	if err != nil {
		return nil, err
	}
	statuses := make(map[int64]string, len(rows))
	for _, row := range rows {
		statuses[row.StudentID] = row.Status
	}
	return statuses, nil
}

type rooms struct {
	facade RoomSource
}

func (r rooms) Names(ctx context.Context, roomIDs []int64) (map[int64]string, error) {
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

type students struct {
	repo StudentSource
}

func (s students) ByIDs(ctx context.Context, studentIDs []int64) (map[int64]emergencysnapshot.Student, error) {
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
			ID:              id,
			PersonID:        row.PersonID,
			SchoolClass:     row.SchoolClass,
			HealthInfo:      deref(row.HealthInfo),
			GuardianName:    deref(row.GuardianName),
			GuardianContact: deref(row.GuardianContact),
			GuardianPhone:   deref(row.GuardianPhone),
		}
	}
	return result, nil
}

type persons struct {
	facade PersonSource
}

func (p persons) ByIDs(ctx context.Context, personIDs []int64) (map[int64]emergencysnapshot.Person, error) {
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

type contacts struct {
	repo ContactSource
}

func (c contacts) EmergencyContacts(ctx context.Context, studentIDs []int64) ([]emergencysnapshot.Contact, error) {
	rows, err := c.repo.ListEmergencyContactRows(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	result := make([]emergencysnapshot.Contact, 0, len(rows))
	for _, row := range rows {
		result = append(result, emergencysnapshot.Contact{
			StudentID: row.StudentID,
			FirstName: row.FirstName.String,
			LastName:  row.LastName.String,
			Phone:     row.PhoneNumber.String,
		})
	}
	return result, nil
}

type settings struct {
	settings SettingsSource
}

func (s settings) HealthInfoEnabled(ctx context.Context) (bool, error) {
	enabled, err := s.settings.ResolveBool(ctx, configModel.KeyEmergencyListHealthInfo)
	if err != nil {
		return false, fmt.Errorf("resolve %s: %w", configModel.KeyEmergencyListHealthInfo, err)
	}
	return enabled, nil
}

type calendar struct{}

func (calendar) DayOf(at time.Time) emergencysnapshot.Date {
	return emergencysnapshot.Date(timezone.DateFromTime(at).String())
}

type renderer struct {
	renderer RendererSource
}

// RenderPDF maps the projected document onto the Document Rendering shape
// field by field; the column ids are shared, so widths and styles apply.
func (r renderer) RenderPDF(doc emergencysnapshot.Document, filenameBase string) (emergencysnapshot.File, error) {
	file, err := r.renderer.Render(ToListExportDocument(doc), listexport.FormatPDF, filenameBase)
	if err != nil {
		return emergencysnapshot.File{}, err
	}
	return emergencysnapshot.File{Filename: file.Filename, ContentType: file.ContentType, Data: file.Data}, nil
}

// ToListExportDocument converts the projected document into the Document
// Rendering document.
func ToListExportDocument(doc emergencysnapshot.Document) listexport.Document {
	columns := make([]listexport.Column, 0, len(doc.Columns))
	for _, column := range doc.Columns {
		columns = append(columns, listexport.Column{ID: listexport.ColumnID(column.ID), Label: column.Label})
	}
	rows := make([]listexport.Row, 0, len(doc.Rows))
	for _, row := range doc.Rows {
		values := make(map[listexport.ColumnID]string, len(row))
		for id, value := range row {
			// Every cell is user text; the renderer's style markers must not
			// reach it from a stored value.
			values[listexport.ColumnID(id)] = listexport.SanitizeUserText(value)
		}
		rows = append(rows, listexport.Row{Values: values})
	}
	return listexport.Document{
		Title:       doc.Title,
		Subtitle:    doc.Subtitle,
		GeneratedAt: doc.GeneratedAt,
		Columns:     columns,
		Rows:        rows,
	}
}

func deref(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
