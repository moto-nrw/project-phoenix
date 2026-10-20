package application

// CalendarFeeds serves the staff and parent calendar feed tokens.
type CalendarFeeds struct {
	*StaffCalendarFeeds
	*ParentCalendarFeeds
}

// NewCalendarFeeds serves the calendar feeds over staff and parent.
func NewCalendarFeeds(staff *StaffCalendarFeeds, parent *ParentCalendarFeeds) CalendarFeeds {
	return CalendarFeeds{StaffCalendarFeeds: staff, ParentCalendarFeeds: parent}
}
