package timetable

import "context"

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

func (m *Module) CourseInstances(ctx context.Context, from, to, today string) ([]CourseInstanceRow, error) {
	if !validDate(from) || !validDate(to) || !validDate(today) {
		return nil, m.reject("statistics_course_instances", ErrInvalidInstanceStudentQuery)
	}
	return m.engine.CourseInstances(ctx, from, to, today)
}

func (m *Module) CourseParticipation(ctx context.Context, from, to, today string) ([]CourseParticipationRow, error) {
	if !validDate(from) || !validDate(to) || !validDate(today) {
		return nil, m.reject("statistics_course_participation", ErrInvalidInstanceStudentQuery)
	}
	return m.engine.CourseParticipation(ctx, from, to, today)
}
