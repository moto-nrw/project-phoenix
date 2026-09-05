package timetable

type TemplateListRow struct {
	TemplateID                int64
	Name                      string
	Type                      string
	CategoryID                int64
	CategoryName              string
	PlanningTrackID           *int64
	PlanningTrackName         string
	PlanningTrackColor        string
	PlanningTrackOrder        *int64
	RoomID                    *int64
	RoomName                  *string
	EducationGroupID          *int64
	EducationGroupName        *string
	IsOpen                    bool
	MaxParticipants           int
	RequiredStaff             *int64
	TemplateCalendarPeriodID  *int64
	TargetGroupType           string
	TargetGradeLevel          *int16
	TargetSchoolClass         *string
	SourceCareOfferingIDsJSON string
	SourceGradeLevelsJSON     string
	SourceSchoolClassesJSON   string
	ListKind                  *string
	Notes                     *string
	ShiftTypeID               *int64
	ShiftTypeName             string
	ShiftTypeColor            string
	EnrollmentCount           int
	SupervisorCount           int
	CapacityEnrollmentCount   int
	CapacitySupervisorCount   int
	CapacityOccurrenceFound   bool
	StudentIDs                []int64
	StaffIDs                  []int64
	PrimaryStaffID            *int64
	ScheduleID                int64
	Weekday                   int
	StartTime                 *string
	EndTime                   *string
	WeekPattern               int
	CalendarPeriodID          *int64
	ScheduleValidFrom         *string
	ScheduleValidUntil        *string
}

type TemplateWeekdayRosterRow struct {
	TemplateID int64
	Weekday    int
	Kind       string
	PersonID   int64
	IsPrimary  bool
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
	TemplateID       int64
	CalendarPeriodID int64
	OccurrenceDate   string
	EnrollmentCount  int
	SupervisorCount  int
}
