package planexport

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

// areaWithoutShiftType labels shifts that carry no Schichtart, so they still
// have a row in the Einsatzbereich view instead of quietly disappearing.
const areaWithoutShiftType = "Dienst ohne Schichtart"

// ExportDienstplan renders the staff week: who works when, where, on what.
//
// The data is the very same StaffScheduleOverview projection the Dienstplan
// screen renders from, so the printout cannot drift from the screen it was
// printed from.
func (s *service) ExportDienstplan(ctx context.Context, params Params) (listexport.File, error) {
	if err := params.validate(TemplatesForDienstplan); err != nil {
		return listexport.File{}, err
	}
	if s.deps.Overview == nil || s.deps.Renderer == nil {
		return listexport.File{}, errors.New("plan export service is not fully wired")
	}

	weeks, err := expandWeeks(params.From, params.To)
	if err != nil {
		return listexport.File{}, err
	}
	from, to := weeks[0].days[0], weeks[len(weeks)-1].last()

	overview, err := s.deps.Overview.GetOverview(ctx, from, to)
	if err != nil {
		return listexport.File{}, fmt.Errorf("load staff schedule overview: %w", err)
	}

	data := newDienstplanData(overview, s.shiftTypes(ctx), s.nonWorkingDays(ctx, from, to), params.Variant)
	// The range is loaded Monday–Sunday; the sheet keeps the weekend only
	// where the roster actually uses it.
	weeks = narrowWeeks(weeks, data.plannedDays())

	title := "Dienstplan"
	rowLabel := "Mitarbeitende"
	legend := "Fett: Dienstzeit · darunter die Aufgaben aus dem Betreuungsplan"
	build := data.rowsByPerson
	if params.Template == TemplateByArea {
		rowLabel = "Einsatzbereich"
		legend = "Fett: Zeitfenster · darunter die eingeteilten Personen"
		build = data.rowsByArea
	}

	doc := s.document(title, rowLabel, legend, params, weeks, build, data.emptyWeekRows)

	s.getLogger().Info("dienstplan export rendered",
		"from", from.String(),
		"to", to.String(),
		"template", string(params.Template),
		"variant", string(params.Variant),
		"format", string(params.Format),
		"week_count", len(weeks),
	)
	return s.deps.Renderer.Render(doc, params.Format, filename("Dienstplan", weeks))
}

// shiftTypes maps Schichtart ids to their name and colour. An unwired or
// failing reader costs the label and the colour bar on the sheet, never the
// shift itself.
func (s *service) shiftTypes(ctx context.Context) map[int64]shiftTypeInfo {
	out := map[int64]shiftTypeInfo{}
	if s.deps.ShiftTypes == nil {
		return out
	}
	types, err := s.deps.ShiftTypes.ListAll(ctx)
	if err != nil {
		s.getLogger().Warn("plan export: shift type lookup failed", "error", err.Error())
		return out
	}
	for _, shiftType := range types {
		if shiftType != nil {
			out[shiftType.ID] = shiftTypeInfo{name: shiftType.Name, color: shiftType.Color}
		}
	}
	return out
}

// shiftTypeInfo is the slice of a Schichtart the sheet prints.
type shiftTypeInfo struct {
	name  string
	color string
}

// dienstplanData is the overview indexed the way the two templates read it.
type dienstplanData struct {
	staff       []*usersModel.Staff
	staffByID   map[int64]*usersModel.Staff
	shifts      map[staffDay][]*scheduleModel.StaffShift
	assignments map[staffDay][]scheduleSvc.StaffScheduleAssignment
	// coversByOrigin lists the replacement shifts attached to a cancelled
	// shift, so the cancelled row can name who steps in (#1841).
	coversByOrigin map[int64][]*scheduleModel.StaffShift
	shiftTypes     map[int64]shiftTypeInfo
	closedDays     map[timezone.Date]string
	variant        Variant
}

type staffDay struct {
	staffID int64
	date    timezone.Date
}

func newDienstplanData(
	overview *scheduleSvc.StaffScheduleOverview,
	shiftTypes map[int64]shiftTypeInfo,
	closedDays map[timezone.Date]string,
	variant Variant,
) *dienstplanData {
	data := &dienstplanData{
		staffByID:      map[int64]*usersModel.Staff{},
		shifts:         map[staffDay][]*scheduleModel.StaffShift{},
		assignments:    map[staffDay][]scheduleSvc.StaffScheduleAssignment{},
		coversByOrigin: map[int64][]*scheduleModel.StaffShift{},
		shiftTypes:     shiftTypes,
		closedDays:     closedDays,
		variant:        variant,
	}
	if overview == nil {
		return data
	}

	data.staff = overview.Staff
	for _, member := range overview.Staff {
		if member != nil {
			data.staffByID[member.ID] = member
		}
	}
	for _, shift := range overview.Shifts {
		if shift == nil {
			continue
		}
		key := staffDay{staffID: shift.StaffID, date: shift.Date}
		data.shifts[key] = append(data.shifts[key], shift)
		if shift.OriginShiftID != nil {
			data.coversByOrigin[*shift.OriginShiftID] = append(data.coversByOrigin[*shift.OriginShiftID], shift)
		}
	}
	for _, assignment := range overview.Assignments {
		key := staffDay{staffID: assignment.StaffID, date: assignment.Date}
		data.assignments[key] = append(data.assignments[key], assignment)
	}
	for key := range data.shifts {
		sortShifts(data.shifts[key])
	}
	for key := range data.assignments {
		sortAssignments(data.assignments[key])
	}
	return data
}

// plannedDays are the calendar days the roster puts something on, whichever
// weekday they fall on. It decides whether the sheet carries a Saturday and
// Sunday column: a weekend shift exists whenever somebody planned one, and a
// plan that omits it is wrong in the way that sends nobody to work.
func (d *dienstplanData) plannedDays() map[timezone.Date]bool {
	days := make(map[timezone.Date]bool, len(d.shifts)+len(d.assignments))
	for key := range d.shifts {
		days[key.date] = true
	}
	for key := range d.assignments {
		days[key.date] = true
	}
	return days
}

func sortShifts(shifts []*scheduleModel.StaffShift) {
	sort.SliceStable(shifts, func(i, j int) bool {
		return shifts[i].StartTime.Before(shifts[j].StartTime)
	})
}

func sortAssignments(assignments []scheduleSvc.StaffScheduleAssignment) {
	sort.SliceStable(assignments, func(i, j int) bool {
		if assignments[i].StartTime.Equal(assignments[j].StartTime) {
			return assignments[i].ActivityTitle < assignments[j].ActivityTitle
		}
		return assignments[i].StartTime.Before(assignments[j].StartTime)
	})
}

func (d *dienstplanData) staffName(staffID int64) string {
	member := d.staffByID[staffID]
	if member == nil || member.Person == nil {
		return "Unbekannt"
	}
	return shortName(member.Person.FirstName, member.Person.LastName)
}

// rowsByPerson is the on-screen layout: one row per staff member, their
// shifts and the tasks inside them per day. Staff with nothing at all in the
// week are left out — a wall plan listing twenty empty rows wastes the page
// the reader needs for the rest.
func (d *dienstplanData) rowsByPerson(w week) []listexport.Row {
	rows := make([]listexport.Row, 0, len(d.staff))
	for _, member := range d.staff {
		if member == nil {
			continue
		}
		cells := newDayCells(memberFullName(member), w)
		content := false
		for i, day := range w.days {
			lines := d.personDayLines(member.ID, day)
			if len(lines) > 0 {
				content = true
			}
			cells.days[i] = append(d.closedDayLines(day), lines...)
		}
		if !content {
			continue
		}
		rows = append(rows, cells.toRow())
	}
	return rows
}

func memberFullName(member *usersModel.Staff) string {
	if member.Person == nil {
		return "Unbekannt"
	}
	return fullName(member.Person.FirstName, member.Person.LastName)
}

// closedDayLines prefixes a cell with the reason the day is closed, so an
// empty Tuesday reads as "Schließtag" rather than as a planning mistake. It
// is the dominant fact of that column, so it leads in strong weight.
func (d *dienstplanData) closedDayLines(day timezone.Date) []listexport.Line {
	if label, ok := d.closedDays[day]; ok {
		return []listexport.Line{strong(label)}
	}
	return nil
}

// personDayLines is one staff member's day: their shift windows first, then
// the tasks planned inside them.
func (d *dienstplanData) personDayLines(staffID int64, day timezone.Date) []listexport.Line {
	key := staffDay{staffID: staffID, date: day}
	lines := make([]listexport.Line, 0, 4)

	for _, shift := range d.shifts[key] {
		// A replacement shift is printed in its own row; naming it again
		// under the origin would list the same cover twice.
		lines = append(lines, d.shiftLines(shift)...)
	}
	for _, assignment := range d.assignments[key] {
		lines = append(lines, d.assignmentLines(assignment)...)
	}
	return lines
}

// shiftLines renders one planned presence window. The window leads in strong
// weight — it is the anchor everything else in the cell sits under.
func (d *dienstplanData) shiftLines(shift *scheduleModel.StaffShift) []listexport.Line {
	info := d.shiftTypes[derefID(shift.ShiftTypeID)]
	head := timeRange(shift.StartTime, shift.EndTime)
	if info.name != "" {
		head += " · " + info.name
	}
	if shift.OriginShiftID != nil {
		head += " · Vertretung"
	}
	// The bar carries the Schichtart colour the screen uses, so the printout
	// and the plan it came from group the same way. It opens on the shift
	// line and runs on over the tasks inside it.
	anchor := accented(head, info.color)

	if !shift.Cancelled {
		lines := []listexport.Line{anchor}
		if d.variant.internal() && strings.TrimSpace(shift.Notes) != "" {
			lines = append(lines, muted("Hinweis: "+strings.TrimSpace(shift.Notes)))
		}
		return lines
	}

	anchor.Text += " · entfällt"
	lines := []listexport.Line{anchor}
	if d.variant.internal() && shift.ChangeReason != nil && strings.TrimSpace(*shift.ChangeReason) != "" {
		// The reason is a health datum often enough ("krank") that it is
		// deliberately absent from the wall variant.
		lines = append(lines, muted("Grund: "+strings.TrimSpace(*shift.ChangeReason)))
	}
	if covers := d.coversByOrigin[shift.ID]; len(covers) > 0 {
		names := make([]string, 0, len(covers))
		for _, cover := range covers {
			names = append(names, d.staffName(cover.StaffID))
		}
		lines = append(lines, muted("Vertretung: "+strings.Join(names, ", ")))
	}
	return lines
}

// assignmentLines renders one timetable block inside a shift: the task, not
// a second presence window. It stays in normal weight, so a glance at the
// cell separates "wann ist die Person da" (strong) from "was macht sie
// darin" (normal) without anyone having to read the times.
func (d *dienstplanData) assignmentLines(assignment scheduleSvc.StaffScheduleAssignment) []listexport.Line {
	if assignment.IsAbsent && !d.variant.internal() {
		// Someone marked absent for a block is not working it; the wall
		// sheet should not send anyone looking for them there.
		return nil
	}

	line := timeRange(assignment.StartTime, assignment.EndTime) + " " + assignment.ActivityTitle
	if room := distinctRoom(assignment.ActivityTitle, assignment.RoomName); room != "" {
		line += " · " + room
	}
	if assignment.IsSubstitute {
		line += " · Vertretung"
	}
	if assignment.IsAbsent {
		line += " · abwesend"
	}
	lines := []listexport.Line{normal(line)}

	if d.variant.internal() {
		for _, gap := range assignment.UncoveredIntervals {
			lines = append(lines, muted("ohne Schicht "+timeRange(gap.StartTime, gap.EndTime)))
		}
	}
	return lines
}

// rowsByArea is the layout the schools keep in Excel: one row per
// Einsatzbereich, the names in the cells.
//
// A row is a place someone is deployed, and there are two kinds. Shifts fill
// a row named after their Schichtart (Randstunde, Ganztag) — the outer
// presence; timetable blocks fill a row named after the block (Mensa,
// Lernzeit) — a task inside that presence. Someone doing both appears in
// both rows, with different times, exactly as the hand-kept sheets do. Making
// a block suppress its own Schichtart row was tried and read worse: the
// Ganztag row then went blank on precisely the days someone also ran an
// Angebot, which is the opposite of what a deployment sheet is for. Nothing
// is double counted here either — no minutes are summed on this sheet.
func (d *dienstplanData) rowsByArea(w week) []listexport.Row {
	areas := map[areaKey]*areaRow{}

	for _, member := range d.staff {
		if member == nil {
			continue
		}
		name := d.staffName(member.ID)
		for i, day := range w.days {
			key := staffDay{staffID: member.ID, date: day}
			assignments := d.assignments[key]

			for _, assignment := range assignments {
				if assignment.IsAbsent && !d.variant.internal() {
					continue
				}
				label := name
				if assignment.IsSubstitute {
					label += " (Vertretung)"
				}
				if assignment.IsAbsent {
					label += " (abwesend)"
				}
				// Block rows deliberately carry no colour bar here: on the
				// staff plan the bar means "this is a Dienst", which is the
				// distinction the sheet exists to make. The care plan colours
				// its blocks by Kategorie, where that is the useful axis.
				area := areaFor(areas, areaOffering, derefID(assignment.ActivityGroupID),
					assignment.ActivityTitle, assignment.StartTime)
				area.add(i, assignment.StartTime, assignment.EndTime,
					distinctRoom(assignment.ActivityTitle, assignment.RoomName), label, "")
			}

			for _, shift := range d.shifts[key] {
				info := d.shiftTypes[derefID(shift.ShiftTypeID)]
				title := info.name
				if title == "" {
					title = areaWithoutShiftType
				}
				label := name
				if shift.OriginShiftID != nil {
					label += " (Vertretung)"
				}
				if shift.Cancelled {
					label += " (entfällt)"
				}
				detail := ""
				if d.variant.internal() && shift.Cancelled && shift.ChangeReason != nil && strings.TrimSpace(*shift.ChangeReason) != "" {
					detail = "Grund: " + strings.TrimSpace(*shift.ChangeReason)
				}
				// Schichtarten need no id in the key: their name is unique per
				// tenant (uniq_shift_types_tenant_name), so two rows of the
				// same name are the same Schichtart.
				area := areaFor(areas, areaShift, 0, title, shift.StartTime)
				area.color = info.color
				area.add(i, shift.StartTime, shift.EndTime, "", label, detail)
			}
		}
	}

	ordered := make([]*areaRow, 0, len(areas))
	for _, row := range areas {
		ordered = append(ordered, row)
	}
	// Earliest start first, so the sheet reads down the day the way the
	// hand-kept Excel does (Randstunde at the top, Angebote at the bottom).
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].earliest != ordered[j].earliest {
			return ordered[i].earliest < ordered[j].earliest
		}
		if ordered[i].title != ordered[j].title {
			return ordered[i].title < ordered[j].title
		}
		if ordered[i].kind != ordered[j].kind {
			return ordered[i].kind < ordered[j].kind
		}
		// Two Angebote of the same name at the same time still need a fixed
		// order, or the sheet swaps their rows between two exports.
		return ordered[i].id < ordered[j].id
	})

	rows := make([]listexport.Row, 0, len(ordered))
	for _, row := range ordered {
		cells := newDayCells(row.title, w)
		for i, day := range w.days {
			cells.days[i] = append(d.closedDayLines(day), row.lines(i)...)
		}
		rows = append(rows, cells.toRow())
	}
	return rows
}

func (d *dienstplanData) emptyWeekRows(w week) []listexport.Row {
	return emptyWeekRows(w, d.closedDays)
}

type areaKind string

const (
	areaOffering areaKind = "offering"
	areaShift    areaKind = "shift"
)

// areaKey identifies one row of the deployment sheet. An Angebot is keyed by
// its Aktivitätsgruppe, because two separately configured Angebote may carry
// the same title and merging them would print one row that is neither of them.
// Only a spontaneous block — which has no Angebot to be identified by — falls
// back to its title, so the same spontaneous block through the week still
// reads as one row.
type areaKey struct {
	kind  areaKind
	id    int64
	title string
}

// areaFor returns the row an entry belongs to, creating it on first use. The
// row is labelled and ordered by its earliest occurrence, so a block renamed
// on a single day does not rename the whole row.
func areaFor(areas map[areaKey]*areaRow, kind areaKind, id int64, title string, start time.Time) *areaRow {
	title = strings.TrimSpace(title)
	if title == "" {
		title = areaWithoutShiftType
	}
	key := areaKey{kind: kind, id: id}
	if id == 0 {
		key.title = title
	}
	row, ok := areas[key]
	if !ok {
		row = &areaRow{kind: kind, id: id, title: title, earliest: "99:99"}
		areas[key] = row
	}
	// "HH:MM" sorts correctly as a string, so the row order needs no extra
	// time parsing; "99:99" is the sentinel that loses to every real clock.
	if clock := start.Format(clockLayout); clock < row.earliest {
		row.earliest, row.title = clock, title
	}
	return row
}

// areaRow collects one Einsatzbereich across the week's days. Entries sharing
// a time window and room are grouped under one heading line, which is what
// turns five separate "12:00–13:00 Name" lines into a readable block.
type areaRow struct {
	kind     areaKind
	id       int64
	title    string
	color    string
	earliest string
	days     [fullWeekDays][]areaEntry
}

type areaEntry struct {
	heading string
	names   []string
	details []string
}

func (r *areaRow) add(dayIndex int, start, end time.Time, room, name, detail string) {
	heading := timeRange(start, end)
	if room != "" {
		heading += " · " + room
	}
	for i := range r.days[dayIndex] {
		if r.days[dayIndex][i].heading == heading {
			r.days[dayIndex][i].names = append(r.days[dayIndex][i].names, name)
			if detail != "" {
				r.days[dayIndex][i].details = append(r.days[dayIndex][i].details, detail)
			}
			return
		}
	}
	entry := areaEntry{heading: heading, names: []string{name}}
	if detail != "" {
		entry.details = []string{detail}
	}
	r.days[dayIndex] = append(r.days[dayIndex], entry)
}

func (r *areaRow) lines(dayIndex int) []listexport.Line {
	entries := r.days[dayIndex]
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].heading < entries[j].heading })

	lines := make([]listexport.Line, 0, len(entries)*2)
	for _, entry := range entries {
		sort.Strings(entry.names)
		// The window heads each block in strong weight, the names under it in
		// normal — otherwise a cell with two windows and five names is one
		// undifferentiated column of text.
		lines = append(lines, accented(entry.heading, r.color))
		for _, name := range entry.names {
			lines = append(lines, normal(name))
		}
		sort.Strings(entry.details)
		for _, detail := range entry.details {
			lines = append(lines, muted(detail))
		}
	}
	return lines
}

func derefID(id *int64) int64 {
	if id == nil {
		return 0
	}
	return *id
}
