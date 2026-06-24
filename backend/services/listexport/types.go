package listexport

import "time"

type Format string

const (
	FormatPDF  Format = "pdf"
	FormatDOCX Format = "docx"
	FormatXLSX Format = "xlsx"
)

type ColumnID string

const (
	ColumnName            ColumnID = "name"
	ColumnSchoolClass     ColumnID = "school_class"
	ColumnGroup           ColumnID = "group"
	ColumnCareDays        ColumnID = "care_days"
	ColumnWeeklyMonday    ColumnID = "weekly_monday"
	ColumnWeeklyTuesday   ColumnID = "weekly_tuesday"
	ColumnWeeklyWednesday ColumnID = "weekly_wednesday"
	ColumnWeeklyThursday  ColumnID = "weekly_thursday"
	ColumnWeeklyFriday    ColumnID = "weekly_friday"
	ColumnPlannedArrival  ColumnID = "planned_arrival"
	ColumnPlannedPickup   ColumnID = "planned_pickup"
	ColumnDailyStatus     ColumnID = "daily_status"
	ColumnDeparture       ColumnID = "departure"
	ColumnDailyNotes      ColumnID = "daily_notes"
	ColumnCurrentLocation ColumnID = "current_location"
	ColumnRoomName        ColumnID = "room_name"
	ColumnRoomStatus      ColumnID = "room_status"
	ColumnRoomBuilding    ColumnID = "room_building"
	ColumnRoomFloor       ColumnID = "room_floor"
	ColumnRoomActivity    ColumnID = "room_activity"
	ColumnRoomSupervision ColumnID = "room_supervision"
	ColumnRoomChildCount  ColumnID = "room_child_count"
	ColumnChecklist       ColumnID = "checklist"
	ColumnStudentName     ColumnID = "student_name"
	ColumnStudentClass    ColumnID = "student_class"
	ColumnStudentGroup    ColumnID = "student_group"
	ColumnContactName     ColumnID = "contact_name"
	ColumnContactPhone    ColumnID = "contact_phone"
)

type Preset string

const (
	PresetOGSWeekly          Preset = "ogs_weekly"
	PresetOGSCompact         Preset = "ogs_compact"
	PresetDailyPlanning      Preset = "daily_planning"
	PresetAttendanceSnapshot Preset = "attendance_snapshot"
	PresetPickupList         Preset = "pickup_list"
	PresetBlankChecklist     Preset = "blank_checklist"
)

type Column struct {
	ID    ColumnID `json:"id"`
	Label string   `json:"label"`
}

type Row struct {
	Values     map[ColumnID]string `json:"values"`
	GroupTitle string              `json:"group_title,omitempty"`
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

type Service interface {
	Render(doc Document, format Format, filenameBase string) (File, error)
	// RenderRecords renders one block per record as a print-friendly
	// PDF (A4 portrait). Use this instead of the flat-table Render when
	// each record carries hierarchical/variable data (e.g. one
	// enrollment with N children, offerings, consents and per-phase
	// custom fields) that a fixed-column table can't lay out legibly.
	RenderRecords(doc RecordDocument, filenameBase string) (File, error)
	RenderRecordsDOCX(doc RecordDocument, filenameBase string) (File, error)
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
