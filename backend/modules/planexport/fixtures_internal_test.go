package planexport

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/services/listexport"
)

// Pure unit tests against in-memory port records: the IDs below are struct
// field values, never database rows. The renderer is captured so the tests
// assert what lands on the page, which is the thing that can silently go
// wrong (a missing cancellation, a reason leaking onto the wall sheet).

// monday is 2026-07-27, a Monday, so days[0..4] are Mon–Fri.
var (
	monday    = timezone.NewDate(2026, time.July, 27)
	tuesday   = monday.AddDays(1)
	wednesday = monday.AddDays(2)
)

var errBoom = errors.New("boom")

type captureRenderer struct {
	doc      listexport.Document
	format   listexport.Format
	filename string
}

func (c *captureRenderer) Render(doc listexport.Document, format listexport.Format, filenameBase string) (listexport.File, error) {
	c.doc = doc
	c.format = format
	c.filename = filenameBase
	return listexport.File{Data: []byte("rendered"), ContentType: "application/pdf", Filename: filenameBase + ".pdf"}, nil
}

// Dienstplan readers.

type stubOverview struct {
	overview *StaffScheduleOverview
}

func (s stubOverview) StaffScheduleOverview(context.Context, Date, Date) (*StaffScheduleOverview, error) {
	out := *s.overview
	return &out, nil
}

type failingOverview struct{}

func (failingOverview) StaffScheduleOverview(context.Context, Date, Date) (*StaffScheduleOverview, error) {
	return nil, errBoom
}

type stubShiftTypes struct{ types []*ShiftType }

func (s stubShiftTypes) ListShiftTypes(context.Context) ([]*ShiftType, error) {
	return s.types, nil
}

type failingShiftTypes struct{}

func (failingShiftTypes) ListShiftTypes(context.Context) ([]*ShiftType, error) {
	return nil, errBoom
}

type stubClosingDays struct {
	days []*ClosingPeriod
}

func (s stubClosingDays) ClosingDaysInRange(context.Context, Date, Date) ([]*ClosingPeriod, error) {
	return s.days, nil
}

type failingClosingDays struct{}

func (failingClosingDays) ClosingDaysInRange(context.Context, Date, Date) ([]*ClosingPeriod, error) {
	return nil, errBoom
}

type stubHolidays struct {
	days []Holiday
	err  error
}

func (s stubHolidays) HolidaysInRange(context.Context, Date, Date) ([]Holiday, error) {
	return s.days, s.err
}

func closingRange(start, end timezone.Date, reason string) *ClosingPeriod {
	return &ClosingPeriod{StartDate: dayKey(start), EndDate: dayKey(end), Reason: reason}
}

func staffMember(id int64, first, last string) *StaffMember {
	return &StaffMember{ID: id, FirstName: first, LastName: last}
}

func clock(hour, minute int) time.Time {
	return time.Date(2000, time.January, 1, hour, minute, 0, 0, time.UTC)
}

func shift(id, staffID int64, date timezone.Date, from, to time.Time) *Shift {
	return &Shift{ID: id, StaffID: staffID, Date: dayKey(date), StartTime: from, EndTime: to}
}

func withType(s *Shift, typeID int64) *Shift {
	s.ShiftTypeID = &typeID
	return s
}

func shiftType(id int64, name string) *ShiftType {
	return &ShiftType{ID: id, Name: name}
}

func ptr[T any](v T) *T { return &v }

// newDienstplanService builds the service with only the Dienstplan side
// wired; the care-plan readers stay nil, which the Dienstplan path never
// touches.
func newDienstplanService(overview *StaffScheduleOverview, types []*ShiftType) (Service, *captureRenderer) {
	renderer := &captureRenderer{}
	return NewService(Dependencies{
		Overview:   stubOverview{overview: overview},
		ShiftTypes: stubShiftTypes{types: types},
		Renderer:   renderer,
	}, nil), renderer
}

func defaultParams() Params {
	return Params{
		From:     dayKey(monday),
		To:       dayKey(monday.AddDays(4)),
		Template: TemplateByPerson,
		Variant:  VariantNotice,
		Format:   listexport.FormatPDF,
	}
}

// Betreuungsplan readers.

type stubInstances struct {
	instances []*Instance
}

func (s stubInstances) InstancesInRange(_ context.Context, from, to Date) ([]*Instance, error) {
	kept := make([]*Instance, 0, len(s.instances))
	for _, instance := range s.instances {
		// Date strings are ISO, so lexical order is calendar order.
		if instance.Date < from || instance.Date > to {
			continue
		}
		kept = append(kept, instance)
	}
	return kept, nil
}

type failingInstances struct{}

func (failingInstances) InstancesInRange(context.Context, Date, Date) ([]*Instance, error) {
	return nil, errBoom
}

type stubInstanceStaff struct {
	rows []*InstanceStaff
}

func (s stubInstanceStaff) InstanceStaffByInstanceIDs(_ context.Context, ids []int64) ([]*InstanceStaff, error) {
	wanted := make(map[int64]bool, len(ids))
	for _, id := range ids {
		wanted[id] = true
	}
	kept := make([]*InstanceStaff, 0, len(s.rows))
	for _, row := range s.rows {
		if wanted[row.InstanceID] {
			kept = append(kept, row)
		}
	}
	return kept, nil
}

type failingInstanceStaff struct{}

func (failingInstanceStaff) InstanceStaffByInstanceIDs(context.Context, []int64) ([]*InstanceStaff, error) {
	return nil, errBoom
}

type stubRooms struct{ rooms []*Room }

func (s stubRooms) RoomsByIDs(context.Context, []int64) ([]*Room, error) {
	return s.rooms, nil
}

type failingRooms struct{}

func (failingRooms) RoomsByIDs(context.Context, []int64) ([]*Room, error) {
	return nil, errBoom
}

type stubStaffDirectory struct{ members []*StaffMember }

func (s stubStaffDirectory) StaffByIDs(context.Context, []int64) (map[int64]*StaffMember, error) {
	out := make(map[int64]*StaffMember, len(s.members))
	for _, member := range s.members {
		out[member.ID] = member
	}
	return out, nil
}

type failingStaffDirectory struct{}

func (failingStaffDirectory) StaffByIDs(context.Context, []int64) (map[int64]*StaffMember, error) {
	return nil, errBoom
}

type stubStudentCounts struct{ counts map[int64]int }

func (s stubStudentCounts) CountNonAbsentByInstanceIDs(context.Context, []int64) (map[int64]int, error) {
	return s.counts, nil
}

type failingStudentCounts struct{}

func (failingStudentCounts) CountNonAbsentByInstanceIDs(context.Context, []int64) (map[int64]int, error) {
	return nil, errBoom
}

type stubActivityGroups struct {
	groups []*ActivityGroup
	err    error
}

func (s stubActivityGroups) ActivityGroupsByIDs(context.Context, []int64) ([]*ActivityGroup, error) {
	return s.groups, s.err
}

type stubPlanningTracks struct {
	tracks []*PlanningTrack
	err    error
}

func (s stubPlanningTracks) ListPlanningTracks(context.Context) ([]*PlanningTrack, error) {
	return s.tracks, s.err
}

func instance(id int64, date timezone.Date, from, to time.Time, title string, roomID int64) *Instance {
	return &Instance{
		ID:        id,
		Date:      dayKey(date),
		Title:     title,
		StartTime: from,
		EndTime:   to,
		RoomID:    roomID,
	}
}

func room(id int64, name string) *Room {
	return &Room{ID: id, Name: name}
}

func instanceStaff(instanceID, staffID int64) *InstanceStaff {
	return &InstanceStaff{InstanceID: instanceID, StaffID: staffID}
}

func activityGroup(id, planningTrackID int64) *ActivityGroup {
	return &ActivityGroup{ID: id, PlanningTrackID: &planningTrackID}
}

func planningTrack(id int64, color string) *PlanningTrack {
	return &PlanningTrack{ID: id, Color: color}
}

func withGroup(i *Instance, groupID int64) *Instance {
	i.ActivityGroupID = &groupID
	return i
}

func newBetreuungsplanService(
	instances []*Instance,
	staffRows []*InstanceStaff,
	counts map[int64]int,
) (Service, *captureRenderer) {
	renderer := &captureRenderer{}
	service := NewService(Dependencies{
		Instances:     stubInstances{instances: instances},
		InstanceStaff: stubInstanceStaff{rows: staffRows},
		Students:      stubStudentCounts{counts: counts},
		Rooms:         stubRooms{rooms: []*Room{room(3, "Speisesaal"), room(4, "Gruppenraum 1")}},
		Staff: stubStaffDirectory{members: []*StaffMember{
			staffMember(7, "Franziska", "Kessener"),
			staffMember(8, "Anna", "Müller"),
		}},
		Renderer: renderer,
	}, nil)
	return service, renderer
}

func careParams() Params {
	params := defaultParams()
	params.Template = TemplateByOffering
	return params
}

// Document inspection helpers.

// cellFor returns the rendered cell of the row with the given label, with
// the style markers stripped — most assertions are about the text.
func cellFor(t *testing.T, doc listexport.Document, label string, column listexport.ColumnID) string {
	t.Helper()
	return listexport.StripStyleMarkers(rawCellFor(t, doc, label, column))
}

// cellLines returns the cell's lines with their styles, for the assertions
// that are about emphasis rather than wording.
func cellLines(t *testing.T, doc listexport.Document, label string, column listexport.ColumnID) []listexport.Line {
	t.Helper()
	raw := rawCellFor(t, doc, label, column)
	lines := make([]listexport.Line, 0, 4)
	for _, encoded := range strings.Split(raw, "\n") {
		decoded, _ := listexport.DecodeLine(encoded)
		lines = append(lines, decoded)
	}
	return lines
}

func rawCellFor(t *testing.T, doc listexport.Document, label string, column listexport.ColumnID) string {
	t.Helper()
	for _, row := range doc.Rows {
		if listexport.StripStyleMarkers(row.Values[listexport.ColumnPlanRowLabel]) == label {
			return row.Values[column]
		}
	}
	t.Fatalf("no row labelled %q in %v", label, rowLabels(doc))
	return ""
}

func rowLabels(doc listexport.Document) []string {
	labels := make([]string, 0, len(doc.Rows))
	for _, row := range doc.Rows {
		if row.GroupTitle != "" {
			labels = append(labels, "#"+row.GroupTitle)
			continue
		}
		labels = append(labels, listexport.StripStyleMarkers(row.Values[listexport.ColumnPlanRowLabel]))
	}
	return labels
}

// cellsForLabel returns every row carrying the label, unlike cellFor, which
// stops at the first — some tests are about how many rows there are.
func cellsForLabel(doc listexport.Document, label string, column listexport.ColumnID) []string {
	cells := make([]string, 0, 2)
	for _, row := range doc.Rows {
		if listexport.StripStyleMarkers(row.Values[listexport.ColumnPlanRowLabel]) == label {
			cells = append(cells, listexport.StripStyleMarkers(row.Values[column]))
		}
	}
	return cells
}

func columnIDs(doc listexport.Document) []listexport.ColumnID {
	ids := make([]listexport.ColumnID, 0, len(doc.Columns))
	for _, column := range doc.Columns {
		ids = append(ids, column.ID)
	}
	return ids
}
