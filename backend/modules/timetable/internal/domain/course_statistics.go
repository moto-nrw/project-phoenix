package domain

type CourseInstanceRow struct {
	CourseID           int64
	Name               string
	CategoryName       string
	MaxParticipants    int
	HeldInstances      int
	CancelledInstances int
}

type CourseParticipationRow struct {
	CourseID    int64
	StudentID   int64
	PresentDays int
	AbsentDays  int
	OpenDays    int
}
