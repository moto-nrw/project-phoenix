package securityruntime

import (
	"context"
	"errors"
	"testing"
)

type visitQueryStub struct {
	visitStudent, person, callerStudent, staff, teacher int64
	teacherGroups, supervised                           []int64
	studentGroup, currentGroup                          int64
	err                                                 error
	teacherErr                                          error
}

func (q visitQueryStub) VisitStudentID(context.Context, int64) (int64, error) {
	return q.visitStudent, q.err
}
func (q visitQueryStub) PersonIDByAccount(context.Context, int64) (int64, bool) {
	return q.person, q.person > 0
}
func (q visitQueryStub) StudentIDByPerson(context.Context, int64) (int64, bool) {
	return q.callerStudent, q.callerStudent > 0
}
func (q visitQueryStub) StaffIDByPerson(context.Context, int64) (int64, bool) {
	return q.staff, q.staff > 0
}
func (q visitQueryStub) TeacherIDByStaff(context.Context, int64) (int64, bool) {
	return q.teacher, q.teacher > 0
}
func (q visitQueryStub) TeacherGroupIDs(context.Context, int64) ([]int64, error) {
	return q.teacherGroups, q.teacherErr
}
func (q visitQueryStub) StudentGroupID(context.Context, int64) (int64, bool) {
	return q.studentGroup, q.studentGroup > 0
}
func (q visitQueryStub) SupervisedActiveGroupIDs(context.Context, int64) []int64 { return q.supervised }
func (q visitQueryStub) StudentCurrentActiveGroupID(context.Context, int64) (int64, bool) {
	return q.currentGroup, q.currentGroup > 0
}

func TestCanViewVisit(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		all   bool
		query visitQueryStub
		want  bool
	}{
		{"broad permission", true, visitQueryStub{}, true},
		{"own visit", false, visitQueryStub{visitStudent: 11, person: 1, callerStudent: 11}, true},
		{"teacher group", false, visitQueryStub{visitStudent: 11, person: 1, staff: 2, teacher: 3, teacherGroups: []int64{4}, studentGroup: 4}, true},
		{"active supervision", false, visitQueryStub{visitStudent: 11, person: 1, staff: 2, teacher: 3, supervised: []int64{8}, currentGroup: 8}, true},
		{"unrelated", false, visitQueryStub{visitStudent: 11, person: 1}, false},
		{"different student", false, visitQueryStub{visitStudent: 11, person: 1, callerStudent: 12}, false},
		{"unlinked account", false, visitQueryStub{visitStudent: 11}, false},
		{"staff without teacher identity", false, visitQueryStub{visitStudent: 11, person: 1, staff: 2, supervised: []int64{8}, currentGroup: 8}, false},
		{"unrelated teacher group", false, visitQueryStub{visitStudent: 11, person: 1, staff: 2, teacher: 3, teacherGroups: []int64{5}, studentGroup: 4}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CanViewVisit(context.Background(), 5, tt.all, 9, tt.query)
			if err != nil || got != tt.want {
				t.Fatalf("CanViewVisit() = %v, %v", got, err)
			}
		})
	}
	_, err := CanViewVisit(context.Background(), 5, false, 9, visitQueryStub{err: errors.New("db")})
	if err == nil {
		t.Fatal("query errors must propagate")
	}
}

func TestCanViewVisitPreservesPolicyFailuresAndBroadAccess(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	allowed, err := CanViewVisit(ctx, 5, false, 0, nil)
	if err != nil || allowed {
		t.Fatalf("invalid visit must deny without querying: allowed=%v, err=%v", allowed, err)
	}
	allowed, err = CanViewVisit(ctx, 5, false, 9, nil)
	if err != nil || allowed {
		t.Fatalf("missing relationship query must deny: allowed=%v, err=%v", allowed, err)
	}
	allowed, err = CanViewVisit(ctx, 5, true, 0, nil)
	if err != nil || !allowed {
		t.Fatalf("broad grant must not require a relationship lookup: allowed=%v, err=%v", allowed, err)
	}
	failure := errors.New("teacher assignments unavailable")
	allowed, err = CanViewVisit(ctx, 5, false, 9, visitQueryStub{
		visitStudent: 11, person: 1, staff: 2, teacher: 3, teacherErr: failure,
		supervised: []int64{8}, currentGroup: 8,
	})
	if allowed || !errors.Is(err, failure) {
		t.Fatalf("teacher query failure must not fall through to supervision: allowed=%v, err=%v", allowed, err)
	}
}
