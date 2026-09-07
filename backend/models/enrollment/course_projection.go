package enrollment

// CourseOfferingReference names an offering and its optional legacy course
// link for the timetable projection.
type CourseOfferingReference struct {
	OfferingID      int64
	ActivityGroupID *int64
}

// CourseGroup is the course-specific read model consumed by the enrollment
// workflow. The timetable projection owns how these facts are read.
type CourseGroup struct {
	ID                  int64
	Active              bool
	ParticipantLimit    *int
	ScheduledWeekdays   []int
	SourceGradeLevels   []int
	SourceSchoolClasses []string
}
