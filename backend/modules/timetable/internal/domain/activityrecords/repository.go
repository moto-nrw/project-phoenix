package activityrecords

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// CategoryRepository defines operations for managing activity categories

// GroupRepository defines operations for managing activity groups

// GroupTargetRepository manages dynamic target cohorts for timetable templates.

// TemplateStartTime is a (activity_group_id, weekday) → timeframe.start_time
// lookup row returned by ScheduleRepository.FindTemplateStartTimesByGroupIDs.
// Used by the WP-B13 exception-conflict endpoint to resolve the "original"
// start_time for modified exceptions.
//
// Multiple rows may share the same (ActivityGroupID, Weekday) when a template
// has several schedules on the same weekday (e.g. morning + afternoon slots).
// The caller is responsible for disambiguating or flagging ambiguity. Rows
// are returned sorted by (ActivityGroupID ASC, Weekday ASC, StartTime ASC),
// so the earliest slot comes first when multiple exist for a given key.
type TemplateStartTime struct {
	ActivityGroupID int64     `bun:"activity_group_id"`
	Weekday         int       `bun:"weekday"`
	StartTime       time.Time `bun:"start_time"`
}

// ScheduleRepository defines operations for managing activity schedules

// SupervisorPlannedRepository defines operations for managing activity supervisors

type PlannedSupervisionBlocker struct {
	ID           int64
	ActivityID   int64
	ActivityName string
	IsPrimary    bool
}

// StudentEnrollmentRepository defines operations for managing student enrollments

// StudentEnrollmentDate keeps legacy compatibility adapters from importing
// the lower-layer calendar-date package.

// TemplateListRow is one row of the template list read model produced by
// GroupRepository.ListTemplateRows: template fields joined with one schedule
// row and the aggregated people counts. Issue #584: moved verbatim from
// api/timetable.
type TemplateListRow struct {
	TemplateID         int64      `bun:"template_id"`
	Name               string     `bun:"name"`
	Type               string     `bun:"type"`
	CategoryID         int64      `bun:"category_id"`
	CategoryName       string     `bun:"category_name"`
	PlanningTrackID    NullInt64  `bun:"planning_track_id"`
	PlanningTrackName  string     `bun:"planning_track_name"`
	PlanningTrackColor string     `bun:"planning_track_color"`
	PlanningTrackOrder NullInt64  `bun:"planning_track_sort_order"`
	RoomID             NullInt64  `bun:"room_id"`
	RoomName           NullString `bun:"room_name"`
	EducationGroupID   NullInt64  `bun:"education_group_id"`
	EducationGroupName NullString `bun:"education_group_name"`
	IsOpen             bool       `bun:"is_open"`
	MaxParticipants    int        `bun:"max_participants"`
	// RequiredStaff is the template's manual Personalbedarf override (#1839);
	// NULL = derive from the Betreuungsschlüssel.
	RequiredStaff NullInt64 `bun:"required_staff"`
	// TemplateCalendarPeriodID is the template's OWN period pin (activityContract.Group.
	// CalendarPeriodID), distinct from CalendarPeriodID below which is the
	// per-schedule-row pin. See materialization_service.go's
	// schedulePinnedPeriodID for the precedence between the two.
	TemplateCalendarPeriodID NullInt64  `bun:"template_calendar_period_id"`
	TargetGroupType          string     `bun:"target_group_type"`
	TargetGradeLevel         NullInt16  `bun:"target_grade_level"`
	TargetSchoolClass        NullString `bun:"target_school_class"`
	// SourceCareOfferingIDsJSON carries the jsonb source-offering id array as
	// its text form ('' = NULL); parse with ParseSourceCareOfferingIDs.
	SourceCareOfferingIDsJSON string `bun:"source_care_offering_ids_json"`
	// SourceGradeLevelsJSON carries the jsonb grade filter as its text form
	// ('' = NULL); parse with ParseSourceGradeLevels.
	SourceGradeLevelsJSON string `bun:"source_grade_levels_json"`
	// SourceSchoolClassesJSON carries the jsonb class filter as its text form
	// ('' = NULL); parse with ParseSourceSchoolClasses (#2482).
	SourceSchoolClassesJSON string `bun:"source_school_classes_json"`
	// ListKind classifies the template for printable daily lists (#1565).
	ListKind NullString `bun:"list_kind"`
	// Notes is the template's durable Wochennotiz (#1837 follow-up); NULL = none.
	Notes NullString `bun:"notes"`
	// IncludeClosingDays is the series' closing-day opt-in (#3594).
	IncludeClosingDays bool `bun:"include_closing_days"`
	// SeriesLastDay is the inclusive last day of the series (#3594); nil = none.
	SeriesLastDay NullString `bun:"series_last_day"`
	// ShiftTypeName/ShiftTypeColor come from the category's optional
	// Kategorie↔Schichtart mapping (#1836/#1837 follow-up); empty when unmapped.
	ShiftTypeID     NullInt64 `bun:"shift_type_id"`
	ShiftTypeName   string    `bun:"shift_type_name"`
	ShiftTypeColor  string    `bun:"shift_type_color"`
	EnrollmentCount int       `bun:"enrollment_count"`
	SupervisorCount int       `bun:"supervisor_count"`
	// CapacityEnrollmentCount and CapacitySupervisorCount are the roster on
	// the actual recurrence date selected as worst for the template. They
	// intentionally differ from the period-tolerant display roster below.
	CapacityEnrollmentCount int `bun:"capacity_enrollment_count"`
	CapacitySupervisorCount int `bun:"capacity_supervisor_count"`
	// CapacityOccurrenceFound reports whether a real occurrence was selected
	// for the capacity counts above. When false (every date cancelled,
	// AB-week-filtered, or unmaterializable) the manual required_staff
	// override must not create demand — otherwise the list shows a false 0/N
	// understaffing indicator for a block that never occurs (#1839).
	CapacityOccurrenceFound bool       `bun:"-"`
	StudentIDs              []int64    `bun:"student_ids,array"`
	StaffIDs                []int64    `bun:"staff_ids,array"`
	PrimaryStaffID          NullInt64  `bun:"primary_staff_id"`
	ScheduleID              int64      `bun:"schedule_id"`
	Weekday                 int        `bun:"weekday"`
	StartTime               NullString `bun:"start_time"`
	EndTime                 NullString `bun:"end_time"`
	WeekPattern             int        `bun:"week_pattern"`
	CalendarPeriodID        NullInt64  `bun:"calendar_period_id"`
	ScheduleValidFrom       NullString `bun:"schedule_valid_from"`
	ScheduleValidUntil      NullString `bun:"schedule_valid_until"`
}

// ParseSourceCareOfferingIDs decodes the SourceCareOfferingIDsJSON text form
// of the jsonb source-offering id array. Empty text (NULL column) yields nil.
func (r TemplateListRow) ParseSourceCareOfferingIDs() ([]int64, error) {
	if r.SourceCareOfferingIDsJSON == "" {
		return nil, nil
	}
	var ids []int64
	if err := json.Unmarshal([]byte(r.SourceCareOfferingIDsJSON), &ids); err != nil {
		return nil, fmt.Errorf("parse source_care_offering_ids: %w", err)
	}
	if len(ids) == 0 {
		return nil, nil
	}
	return ids, nil
}

// ParseSourceGradeLevels decodes the SourceGradeLevelsJSON text form of the
// jsonb grade filter. Empty text (NULL column) yields nil.
func (r TemplateListRow) ParseSourceGradeLevels() ([]int, error) {
	if r.SourceGradeLevelsJSON == "" {
		return nil, nil
	}
	var levels []int
	if err := json.Unmarshal([]byte(r.SourceGradeLevelsJSON), &levels); err != nil {
		return nil, fmt.Errorf("parse source_grade_levels: %w", err)
	}
	if len(levels) == 0 {
		return nil, nil
	}
	return levels, nil
}

// ParseSourceSchoolClasses decodes the SourceSchoolClassesJSON text form of
// the jsonb class filter (#2482). Empty text (NULL column) yields nil.
func (r TemplateListRow) ParseSourceSchoolClasses() ([]string, error) {
	if r.SourceSchoolClassesJSON == "" {
		return nil, nil
	}
	var classes []string
	if err := json.Unmarshal([]byte(r.SourceSchoolClassesJSON), &classes); err != nil {
		return nil, fmt.Errorf("parse source_school_classes: %w", err)
	}
	if len(classes) == 0 {
		return nil, nil
	}
	return classes, nil
}

type TemplateCapacityOccurrence struct {
	TemplateID       int64         `bun:"template_id"`
	CalendarPeriodID int64         `bun:"calendar_period_id"`
	OccurrenceDate   calendar.Date `bun:"occurrence_date"`
	EnrollmentCount  int           `bun:"enrollment_count"`
	SupervisorCount  int           `bun:"supervisor_count"`
}
