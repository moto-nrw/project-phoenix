package timetable

import (
	"context"
	"testing"
	"time"
)

type sessionRosterEngine struct {
	engine
	calls int
}

func (e *sessionRosterEngine) SessionsRosteringStudent(context.Context, int64, []int64) ([]int64, error) {
	e.calls++
	return nil, nil
}

func (e *sessionRosterEngine) ObserveRejection(string, time.Duration, error) {}

func TestSessionsRosteringStudentListsNobodyWithoutSessions(t *testing.T) {
	t.Parallel()
	engine := &sessionRosterEngine{}
	result, err := NewModule(engine).SessionsRosteringStudent(t.Context(), 0, nil)
	if err != nil || result != nil || engine.calls != 0 {
		t.Fatalf("SessionsRosteringStudent(no sessions) = %v, %v after %d engine calls", result, err, engine.calls)
	}
}
