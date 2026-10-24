package timetable

import "context"

// SessionRosterQuery answers which running sessions list a child in the day
// roster of their block (#2762): Student Presence knows which block a session
// runs, Timetable who is planned on it.
type SessionRosterQuery interface {
	SessionsRosteringStudent(ctx context.Context, studentID int64, sessionIDs []int64) ([]int64, error)
}
