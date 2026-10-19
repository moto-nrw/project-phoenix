package application

// CourseOfferingReference names an offering and its optional legacy course
// link for the timetable projection.
type CourseOfferingReference struct {
	OfferingID      int64
	ActivityGroupID *int64
}
