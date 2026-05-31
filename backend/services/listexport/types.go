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
	Values map[ColumnID]string `json:"values"`
}

type Document struct {
	Title       string
	Subtitle    string
	GeneratedAt time.Time
	Filters     []string
	Columns     []Column
	Rows        []Row
}

type File struct {
	Data        []byte
	ContentType string
	Filename    string
}

type Service interface {
	Render(doc Document, format Format, filenameBase string) (File, error)
}
