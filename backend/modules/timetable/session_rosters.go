package timetable

import "context"

// SessionRosterQuery answers which running sessions list a child in the day
// roster of their block (#2762): Student Presence knows which block a session
// runs, Timetable who is planned on it.
type SessionRosterQuery interface {
	SessionsRosteringStudent(ctx context.Context, studentID int64, sessionIDs []int64) ([]int64, error)
}

// SessionsRosteringStudent returns the sessions among sessionIDs that run a
// block whose day roster lists the child. No session lists nobody.
func (m *Module) SessionsRosteringStudent(ctx context.Context, studentID int64, sessionIDs []int64) ([]int64, error) {
	if len(sessionIDs) == 0 {
		return nil, nil
	}
	return m.engine.SessionsRosteringStudent(ctx, studentID, sessionIDs)
}
