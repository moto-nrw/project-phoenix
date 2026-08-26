package listexport

import (
	"strings"
	"time"
)

type Format string

const (
	FormatPDF  Format = "pdf"
	FormatDOCX Format = "docx"
	FormatXLSX Format = "xlsx"
)

type ColumnID string

const (
	ColumnName              ColumnID = "name"
	ColumnSchoolClass       ColumnID = "school_class"
	ColumnGroup             ColumnID = "group"
	ColumnEnrollmentSummary ColumnID = "enrollment_summary"
	ColumnCareDays          ColumnID = "care_days"
	ColumnWeeklyMonday      ColumnID = "weekly_monday"
	ColumnWeeklyTuesday     ColumnID = "weekly_tuesday"
	ColumnWeeklyWednesday   ColumnID = "weekly_wednesday"
	ColumnWeeklyThursday    ColumnID = "weekly_thursday"
	ColumnWeeklyFriday      ColumnID = "weekly_friday"
	ColumnPlannedArrival    ColumnID = "planned_arrival"
	ColumnPlannedPickup     ColumnID = "planned_pickup"
	ColumnDailyStatus       ColumnID = "daily_status"
	ColumnDeparture         ColumnID = "departure"
	ColumnDailyNotes        ColumnID = "daily_notes"
	ColumnCurrentLocation   ColumnID = "current_location"
	ColumnRoomName          ColumnID = "room_name"
	ColumnRoomStatus        ColumnID = "room_status"
	ColumnRoomBuilding      ColumnID = "room_building"
	ColumnRoomFloor         ColumnID = "room_floor"
	ColumnRoomActivity      ColumnID = "room_activity"
	ColumnRoomSupervision   ColumnID = "room_supervision"
	ColumnRoomChildCount    ColumnID = "room_child_count"
	ColumnChecklist         ColumnID = "checklist"
	ColumnStudentName       ColumnID = "student_name"
	ColumnStudentClass      ColumnID = "student_class"
	ColumnStudentGroup      ColumnID = "student_group"
	ColumnContactName       ColumnID = "contact_name"
	ColumnContactPhone      ColumnID = "contact_phone"
	ColumnGuardianContacts  ColumnID = "guardian_contacts"
	// ColumnHealthInfo carries the child's stored health note (allergies,
	// medication, medical hints). It exists for the Notfallliste (#2609), which
	// schools print as an offline backup: the note has to be readable next to
	// the phone number, not looked up in a portal that just went offline.
	ColumnHealthInfo     ColumnID = "health_info"
	ColumnBirthday       ColumnID = "birthday"
	ColumnAge            ColumnID = "age"
	ColumnSlot           ColumnID = "slot"
	ColumnPresenceStatus ColumnID = "presence_status"

	// Plan-matrix columns (#2079). Deliberately distinct from the
	// ColumnWeekly* family: those carry one short care marker per weekday in
	// a child list and are weighted narrower than the name column, while a
	// plan cell carries a time window plus its task and room and needs the
	// opposite balance. Sharing the IDs would have meant re-weighting the
	// child lists.
	ColumnPlanRowLabel  ColumnID = "plan_row_label"
	ColumnPlanMonday    ColumnID = "plan_monday"
	ColumnPlanTuesday   ColumnID = "plan_tuesday"
	ColumnPlanWednesday ColumnID = "plan_wednesday"
	ColumnPlanThursday  ColumnID = "plan_thursday"
	ColumnPlanFriday    ColumnID = "plan_friday"
	ColumnPlanSaturday  ColumnID = "plan_saturday"
	ColumnPlanSunday    ColumnID = "plan_sunday"
)

// PlanDayColumns are the plan columns in weekday order, so callers index by
// weekday instead of repeating the list. Saturday and Sunday exist because a
// shift may be planned on them (a series accepts ISO weekdays 1–7); a
// document that plans neither simply stops after Friday.
var PlanDayColumns = [7]ColumnID{
	ColumnPlanMonday,
	ColumnPlanTuesday,
	ColumnPlanWednesday,
	ColumnPlanThursday,
	ColumnPlanFriday,
	ColumnPlanSaturday,
	ColumnPlanSunday,
}

type Preset string

const (
	PresetOGSWeekly          Preset = "ogs_weekly"
	PresetOGSCompact         Preset = "ogs_compact"
	PresetClassRoster        Preset = "class_roster"
	PresetDailyPlanning      Preset = "daily_planning"
	PresetAttendanceSnapshot Preset = "attendance_snapshot"
	PresetPickupList         Preset = "pickup_list"
	PresetBlankChecklist     Preset = "blank_checklist"
	PresetBirthdayList       Preset = "birthday_list"
	// PresetStaffBirthdayList is the staff counterpart of PresetBirthdayList
	// (#1542). Separate rather than shared because the child list carries
	// class, group, and age, none of which belong on a colleague list.
	PresetStaffBirthdayList Preset = "staff_birthday_list"
)

type Column struct {
	ID    ColumnID `json:"id"`
	Label string   `json:"label"`
}

// LineStyle is the emphasis of one line inside a cell. A matrix cell stacks
// facts of different rank — a shift window, the tasks inside it, a note about
// a cancellation — and printing all of them in one weight produces a wall of
// grey that nobody can read from two metres away.
type LineStyle uint8

const (
	// LineNormal is the ordinary cell text.
	LineNormal LineStyle = iota
	// LineStrong anchors an entry: the shift window, a block's heading.
	LineStrong
	// LineMuted carries secondary detail (a room, a reason, a coverage gap)
	// that must be readable but must not compete with the anchor.
	LineMuted
)

// Line is one styled line of a cell.
type Line struct {
	Text  string
	Style LineStyle
	// Accent is an optional "#RRGGBB" that paints a colour bar down the left
	// edge of this line and every line under it until the next anchor
	// (LineStrong) line — an anchor ends the group above it whether or not it
	// carries a colour of its own.
	// It carries the same colour coding the screen uses (a Schichtart, a
	// Kategorie), so a printed plan and the plan on screen group the same way.
	// The bar is redundant by design: the emphasis above already separates the
	// ranks, so a greyscale copy loses nothing but the recognition.
	Accent string
}

// accentMark separates an accent colour from the line text; it follows the
// style marker: "\x01#83CD2D\x03 07:30-14:00".
const accentMark = "\x03"

// Style markers. They are encoded IN the cell string rather than beside it so
// that Row.Values stays a plain map[ColumnID]string and the eight existing
// export call sites keep working untouched. The markers are C0 control
// characters, which cannot occur in any name, room, or note, and every
// renderer either interprets them (PDF, DOCX) or strips them (XLSX) — they
// never reach a reader.
const (
	markStrong = "\x01"
	markMuted  = "\x02"
)

// StyledCell builds a cell from styled lines. Callers use this instead of
// hand-assembling the markers, which stay an implementation detail.
func StyledCell(lines []Line) string {
	parts := make([]string, 0, len(lines))
	for _, line := range lines {
		prefix := ""
		switch line.Style {
		case LineStrong:
			prefix = markStrong
		case LineMuted:
			prefix = markMuted
		}
		if line.Accent != "" {
			prefix += line.Accent + accentMark
		}
		parts = append(parts, prefix+line.Text)
	}
	return strings.Join(parts, "\n")
}

// DecodeLine splits a stored line into its style, accent, and text.
func DecodeLine(line string) (Line, string) {
	out := Line{Style: LineNormal}
	switch {
	case strings.HasPrefix(line, markStrong):
		out.Style, line = LineStrong, strings.TrimPrefix(line, markStrong)
	case strings.HasPrefix(line, markMuted):
		out.Style, line = LineMuted, strings.TrimPrefix(line, markMuted)
	}
	if idx := strings.Index(line, accentMark); idx >= 0 {
		out.Accent, line = line[:idx], line[idx+len(accentMark):]
	}
	out.Text = line
	return out, line
}

// StripStyleMarkers removes every marker (and accent) from a cell, for
// renderers that cannot express per-line emphasis.
func StripStyleMarkers(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		_, text := DecodeLine(line)
		lines[i] = text
	}
	return strings.Join(lines, "\n")
}

type Row struct {
	Values     map[ColumnID]string `json:"values"`
	GroupTitle string              `json:"group_title,omitempty"`
}

// ClassGroupTitle labels a per-class section in grouped class-list
// exports. Class names that already carry the word "Klasse" (e.g.
// "Klasse 1a") are used as-is to avoid "Klasse Klasse 1a"; students
// without a class collect under "Ohne Klasse".
func ClassGroupTitle(schoolClass string) string {
	schoolClass = strings.TrimSpace(schoolClass)
	if schoolClass == "" {
		return "Ohne Klasse"
	}
	if strings.HasPrefix(strings.ToLower(schoolClass), "klasse") {
		return schoolClass
	}
	return "Klasse " + schoolClass
}

type Document struct {
	Title       string
	Subtitle    string
	GeneratedAt time.Time
	Filters     []string
	Columns     []Column
	Rows        []Row
	// Footer, when set, is stamped at the centre foot of every printed
	// page (e.g. a GDPR confidentiality note). The XLSX counterpart of
	// RecordDocument.Footer on the PDF path.
	Footer string
}

type File struct {
	Data        []byte
	ContentType string
	Filename    string
}

// Field is one label/value pair inside a record or sub-record block.
type Field struct {
	Label string
	Value string
}

// SubRecord is a nested block within a Record (e.g. one child under an
// enrollment). Rendered indented under the parent record.
type SubRecord struct {
	Title  string
	Fields []Field
}

// Record is one top-level block (e.g. one enrollment submission):
// a heading, its own fields, and any nested sub-records.
type Record struct {
	Title  string
	Fields []Field
	Subs   []SubRecord
}

type RecordGroup struct {
	Title   string
	Records []Record
}

// RecordDocument is the input to RenderRecords — the block-layout
// counterpart of Document. Footer prints small at the bottom of every
// page (e.g. a GDPR confidentiality note).
type RecordDocument struct {
	Title       string
	Subtitle    string
	GeneratedAt time.Time
	Footer      string
	// Filters lists the applied filter labels (e.g. "Status: Angenommen"),
	// printed in the page header below the generated-at line — the
	// block-layout counterpart of Document.Filters.
	Filters []string
	Records []Record
	Groups  []RecordGroup
}
