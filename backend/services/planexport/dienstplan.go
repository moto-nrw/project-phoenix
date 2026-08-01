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
	from, to := weeks[0].days[0], weeks[len(weeks)-1].days[4]

	overview, err := s.deps.Overview.GetOverview(ctx, from, to)
	if err != nil {
		return listexport.File{}, fmt.Errorf("load staff schedule overview: %w", err)
	}

	data := newDienstplanData(overview, s.shiftTypeNames(ctx), s.nonWorkingDays(ctx, from, to), params.Variant)

	title := "Dienstplan"
	rowLabel := "Mitarbeitende"
	build := data.rowsByPerson
	if params.Template == TemplateByArea {
		rowLabel = "Einsatzbereich"
		build = data.rowsByArea
	}

	doc := s.document(title, rowLabel, params, weeks, build)

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

// shiftTypeNames maps Schichtart ids to their names. An unwired or failing
// reader costs the type label on the sheet, never the shift itself.
func (s *service) shiftTypeNames(ctx context.Context) map[int64]string {
	names := map[int64]string{}
	if s.deps.ShiftTypes == nil {
		return names
	}
	types, err := s.deps.ShiftTypes.ListAll(ctx)
	if err != nil {
		s.getLogger().Warn("plan export: shift type lookup failed", "error", err.Error())
		return names
	}
	for _, shiftType := range types {
		if shiftType != nil {
			names[shiftType.ID] = shiftType.Name
		}
	}
	return names
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
	shiftTypeNames map[int64]string
	closedDays     map[timezone.Date]string
	variant        Variant
}

type staffDay struct {
	staffID int64
	date    timezone.Date
}

func newDienstplanData(
	overview *scheduleSvc.StaffScheduleOverview,
	shiftTypeNames map[int64]string,
	closedDays map[timezone.Date]string,
	variant Variant,
) *dienstplanData {
	data := &dienstplanData{
		staffByID:      map[int64]*usersModel.Staff{},
		shifts:         map[staffDay][]*scheduleModel.StaffShift{},
		assignments:    map[staffDay][]scheduleSvc.StaffScheduleAssignment{},
		coversByOrigin: map[int64][]*scheduleModel.StaffShift{},
		shiftTypeNames: shiftTypeNames,
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
		cells := dayCells{label: memberFullName(member)}
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
// empty Tuesday reads as "Schließtag" rather than as a planning mistake.
func (d *dienstplanData) closedDayLines(day timezone.Date) []string {
	if label, ok := d.closedDays[day]; ok {
		return []string{label}
	}
	return nil
}

// personDayLines is one staff member's day: their shift windows first, then
// the tasks planned inside them.
func (d *dienstplanData) personDayLines(staffID int64, day timezone.Date) []string {
	key := staffDay{staffID: staffID, date: day}
	lines := make([]string, 0, 4)

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

// shiftLines renders one planned presence window.
func (d *dienstplanData) shiftLines(shift *scheduleModel.StaffShift) []string {
	head := timeRange(shift.StartTime, shift.EndTime)
	if name := d.shiftTypeNames[derefID(shift.ShiftTypeID)]; name != "" {
		head += " · " + name
	}
	if shift.OriginShiftID != nil {
		head += " · Vertretung"
	}

	if !shift.Cancelled {
		lines := []string{head}
		if d.variant.internal() && strings.TrimSpace(shift.Notes) != "" {
			lines = append(lines, "Hinweis: "+strings.TrimSpace(shift.Notes))
		}
		return lines
	}

	head += " · entfällt"
	if d.variant.internal() && shift.ChangeReason != nil && strings.TrimSpace(*shift.ChangeReason) != "" {
		// The reason is a health datum often enough ("krank") that it is
		// deliberately absent from the wall variant.
		head += " (" + strings.TrimSpace(*shift.ChangeReason) + ")"
	}
	lines := []string{head}
	if covers := d.coversByOrigin[shift.ID]; len(covers) > 0 {
		names := make([]string, 0, len(covers))
		for _, cover := range covers {
			names = append(names, d.staffName(cover.StaffID))
		}
		lines = append(lines, "Vertretung: "+strings.Join(names, ", "))
	}
	return lines
}

// assignmentLines renders one timetable block inside a shift: the task, not
// a second presence window.
func (d *dienstplanData) assignmentLines(assignment scheduleSvc.StaffScheduleAssignment) []string {
	if assignment.IsAbsent && !d.variant.internal() {
		// Someone marked absent for a block is not working it; the wall
		// sheet should not send anyone looking for them there.
		return nil
	}

	line := timeRange(assignment.StartTime, assignment.EndTime) + " " + assignment.ActivityTitle
	if assignment.RoomName != "" {
		line += " · " + assignment.RoomName
	}
	if assignment.IsSubstitute {
		line += " · Vertretung"
	}
	if assignment.IsAbsent {
		line += " · abwesend"
	}
	lines := []string{line}

	if d.variant.internal() {
		for _, gap := range assignment.UncoveredIntervals {
			lines = append(lines, "ohne Schicht "+timeRange(gap.StartTime, gap.EndTime))
		}
	}
	return lines
}

// rowsByArea is the layout the schools keep in Excel: one row per
// Einsatzbereich, the names in the cells.
//
// A row is a task, not a person. Tasks come from the timetable blocks
// (Küche, Lernzeit, Angebote); a shift that has no block on that day
// contributes to a row named after its Schichtart (Randstunde), which is
// how the same sheets are built by hand today. That is not double counting:
// a shift is the outer presence, a block a task inside it, and each is shown
// exactly once.
func (d *dienstplanData) rowsByArea(w week) []listexport.Row {
	areas := map[string]*areaRow{}

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
				areaFor(areas, assignment.ActivityTitle).add(i,
					assignment.StartTime, assignment.EndTime, assignment.RoomName, label)
			}

			// Shifts only form their own area row on days where they carry no
			// block, so a person is not listed both under "Küche" and under
			// their Schichtart for the same hours.
			if len(assignments) > 0 {
				continue
			}
			for _, shift := range d.shifts[key] {
				title := d.shiftTypeNames[derefID(shift.ShiftTypeID)]
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
				areaFor(areas, title).add(i, shift.StartTime, shift.EndTime, "", label)
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
		if ordered[i].earliest == ordered[j].earliest {
			return ordered[i].title < ordered[j].title
		}
		return ordered[i].earliest < ordered[j].earliest
	})

	rows := make([]listexport.Row, 0, len(ordered))
	for _, row := range ordered {
		cells := dayCells{label: row.title}
		for i, day := range w.days {
			cells.days[i] = append(d.closedDayLines(day), row.lines(i)...)
		}
		rows = append(rows, cells.toRow())
	}
	return rows
}

func areaFor(areas map[string]*areaRow, title string) *areaRow {
	title = strings.TrimSpace(title)
	if title == "" {
		title = areaWithoutShiftType
	}
	row, ok := areas[title]
	if !ok {
		row = &areaRow{title: title, earliest: "99:99"}
		areas[title] = row
	}
	return row
}

// areaRow collects one Einsatzbereich across the five days. Entries sharing
// a time window and room are grouped under one heading line, which is what
// turns five separate "12:00–13:00 Name" lines into a readable block.
type areaRow struct {
	title    string
	earliest string
	days     [5][]areaEntry
}

type areaEntry struct {
	heading string
	names   []string
}

func (r *areaRow) add(dayIndex int, start, end time.Time, room, name string) {
	heading := timeRange(start, end)
	// "HH:MM" sorts correctly as a string, so the row order needs no extra
	// time parsing; "99:99" is the sentinel that loses to every real clock.
	if clock := start.Format(clockLayout); clock < r.earliest {
		r.earliest = clock
	}
	if room != "" {
		heading += " · " + room
	}
	for i := range r.days[dayIndex] {
		if r.days[dayIndex][i].heading == heading {
			r.days[dayIndex][i].names = append(r.days[dayIndex][i].names, name)
			return
		}
	}
	r.days[dayIndex] = append(r.days[dayIndex], areaEntry{heading: heading, names: []string{name}})
}

func (r *areaRow) lines(dayIndex int) []string {
	entries := r.days[dayIndex]
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].heading < entries[j].heading })

	lines := make([]string, 0, len(entries)*2)
	for _, entry := range entries {
		sort.Strings(entry.names)
		lines = append(lines, entry.heading)
		lines = append(lines, entry.names...)
	}
	return lines
}

func derefID(id *int64) int64 {
	if id == nil {
		return 0
	}
	return *id
}
