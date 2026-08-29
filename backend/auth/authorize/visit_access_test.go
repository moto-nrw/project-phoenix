package authorize

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
	return q.teacherGroups, q.err
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
