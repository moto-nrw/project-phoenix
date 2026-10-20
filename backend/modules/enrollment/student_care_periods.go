package enrollment

type StudentCarePeriod struct {
	RequestChildID    int64
	RequestID         int64
	PhaseID           int64
	PhaseName         string
	ServiceStartDate  Date
	ServiceEndDate    Date
	TargetGradeLevel  *int16
	TargetSchoolClass *string
}
