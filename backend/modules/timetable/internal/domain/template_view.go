package domain

type TemplateListRow struct {
	TemplateID                int64   `bun:"template_id"`
	Name                      string  `bun:"name"`
	Type                      string  `bun:"type"`
	CategoryID                int64   `bun:"category_id"`
	CategoryName              string  `bun:"category_name"`
	PlanningTrackID           *int64  `bun:"planning_track_id"`
	PlanningTrackName         string  `bun:"planning_track_name"`
	PlanningTrackColor        string  `bun:"planning_track_color"`
	PlanningTrackOrder        *int64  `bun:"planning_track_sort_order"`
	RoomID                    *int64  `bun:"room_id"`
	RoomName                  *string `bun:"room_name"`
	EducationGroupID          *int64  `bun:"education_group_id"`
	EducationGroupName        *string `bun:"education_group_name"`
	IsOpen                    bool    `bun:"is_open"`
	MaxParticipants           int     `bun:"max_participants"`
	RequiredStaff             *int64  `bun:"required_staff"`
	TemplateCalendarPeriodID  *int64  `bun:"template_calendar_period_id"`
	TargetGroupType           string  `bun:"target_group_type"`
	TargetGradeLevel          *int16  `bun:"target_grade_level"`
	TargetSchoolClass         *string `bun:"target_school_class"`
	SourceCareOfferingIDsJSON string  `bun:"source_care_offering_ids_json"`
	SourceGradeLevelsJSON     string  `bun:"source_grade_levels_json"`
	SourceSchoolClassesJSON   string  `bun:"source_school_classes_json"`
	ListKind                  *string `bun:"list_kind"`
	Notes                     *string `bun:"notes"`
	ShiftTypeID               *int64  `bun:"shift_type_id"`
	ShiftTypeName             string  `bun:"shift_type_name"`
	ShiftTypeColor            string  `bun:"shift_type_color"`
	EnrollmentCount           int     `bun:"enrollment_count"`
	SupervisorCount           int     `bun:"supervisor_count"`
	CapacityEnrollmentCount   int     `bun:"capacity_enrollment_count"`
	CapacitySupervisorCount   int     `bun:"capacity_supervisor_count"`
	CapacityOccurrenceFound   bool    `bun:"-"`
	StudentIDs                []int64 `bun:"student_ids,array"`
	StaffIDs                  []int64 `bun:"staff_ids,array"`
	PrimaryStaffID            *int64  `bun:"primary_staff_id"`
	ScheduleID                int64   `bun:"schedule_id"`
	Weekday                   int     `bun:"weekday"`
	StartTime                 *string `bun:"start_time"`
	EndTime                   *string `bun:"end_time"`
	WeekPattern               int     `bun:"week_pattern"`
	CalendarPeriodID          *int64  `bun:"calendar_period_id"`
	ScheduleValidFrom         *string `bun:"schedule_valid_from"`
	ScheduleValidUntil        *string `bun:"schedule_valid_until"`
}

type TemplateWeekdayRosterRow struct {
	TemplateID int64  `bun:"template_id"`
	Weekday    int    `bun:"weekday"`
	Kind       string `bun:"kind"`
	PersonID   int64  `bun:"person_id"`
	IsPrimary  bool   `bun:"is_primary"`
}

type TemplateCapacityPeriod struct {
	ID              int64
	TenantID        int64
	StartDate       string
	EndDate         string
	WeekCycleLength int
	WeekCycleAnchor string
}

type TemplateCapacityOccurrence struct {
	TemplateID       int64  `bun:"template_id"`
	CalendarPeriodID int64  `bun:"calendar_period_id"`
	OccurrenceDate   string `bun:"occurrence_date"`
	EnrollmentCount  int    `bun:"enrollment_count"`
	SupervisorCount  int    `bun:"supervisor_count"`
}
