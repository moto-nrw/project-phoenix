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
	ColumnBirthday          ColumnID = "birthday"
	ColumnAge               ColumnID = "age"
	ColumnSlot              ColumnID = "slot"
	ColumnPresenceStatus    ColumnID = "presence_status"

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
)

// PlanDayColumns are the Monday–Friday plan columns in weekday order, so
// callers index by weekday instead of repeating the list.
var PlanDayColumns = [5]ColumnID{
	ColumnPlanMonday,
	ColumnPlanTuesday,
	ColumnPlanWednesday,
	ColumnPlanThursday,
	ColumnPlanFriday,
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
}

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
		switch line.Style {
		case LineStrong:
			parts = append(parts, markStrong+line.Text)
		case LineMuted:
			parts = append(parts, markMuted+line.Text)
		default:
			parts = append(parts, line.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// DecodeLine splits a stored line into its style and its text.
func DecodeLine(line string) (LineStyle, string) {
	switch {
	case strings.HasPrefix(line, markStrong):
		return LineStrong, strings.TrimPrefix(line, markStrong)
	case strings.HasPrefix(line, markMuted):
		return LineMuted, strings.TrimPrefix(line, markMuted)
	default:
		return LineNormal, line
	}
}

// StripStyleMarkers removes every marker from a cell, for renderers that
// cannot express per-line emphasis.
func StripStyleMarkers(s string) string {
	return strings.NewReplacer(markStrong, "", markMuted, "").Replace(s)
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
