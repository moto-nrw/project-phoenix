package presence

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

type attendanceStaffNamesFunc func(context.Context, []int64) (map[int64]string, error)

func (f attendanceStaffNamesFunc) StaffNames(ctx context.Context, ids []int64) (map[int64]string, error) {
	return f(ctx, ids)
}

func TestAttendanceActorsPreserveDisplayAndLookupFailure(t *testing.T) {
	t.Parallel()
	checkOut := time.Date(2026, 9, 9, 15, 0, 0, 0, time.UTC)
	var staffOut int64 = 42
	for _, tc := range []struct {
		name        string
		names       map[int64]string
		err         error
		wantIn      *studentpresence.AttendanceActor
		wantOut     *studentpresence.AttendanceActor
		wantInName  string
		wantOutName string
	}{
		{
			name:   "full names",
			names:  map[int64]string{41: "Anna Beispiel", 42: "Ben Muster"},
			wantIn: &studentpresence.AttendanceActor{Kind: studentpresence.AttendanceActorStaff, Name: "Anna Beispiel"}, wantInName: "Anna Beispiel",
			wantOut: &studentpresence.AttendanceActor{Kind: studentpresence.AttendanceActorStaff, Name: "Ben Muster"}, wantOutName: "Ben Muster",
		},
		{
			name:   "formatting unchanged",
			names:  map[int64]string{41: "Anna ", 42: "Ben"},
			wantIn: &studentpresence.AttendanceActor{Kind: studentpresence.AttendanceActorStaff, Name: "Anna "}, wantInName: "Anna ",
			wantOut: &studentpresence.AttendanceActor{Kind: studentpresence.AttendanceActorStaff, Name: "Ben"}, wantOutName: "Ben",
		},
		{
			name:    "missing staff is a former staff member",
			names:   map[int64]string{},
			wantIn:  &studentpresence.AttendanceActor{Kind: studentpresence.AttendanceActorFormerStaff},
			wantOut: &studentpresence.AttendanceActor{Kind: studentpresence.AttendanceActorFormerStaff},
		},
		{name: "failed lookup names nobody", names: map[int64]string{41: "partial"}, err: errors.New("directory unavailable")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			svc := &service{ServiceDependencies: ServiceDependencies{StaffNames: attendanceStaffNamesFunc(func(got context.Context, ids []int64) (map[int64]string, error) {
				assert.Equal(t, ctx, got)
				assert.ElementsMatch(t, []int64{41, 42}, ids)
				return tc.names, tc.err
			})}}
			result := &AttendanceStatus{StudentID: 7}
			svc.applyAttendanceActors(ctx, result, studentpresence.Attendance{CheckedInBy: 41, CheckedOutBy: &staffOut, CheckOutTime: &checkOut})
			assert.Equal(t, tc.wantIn, result.CheckedInActor)
			assert.Equal(t, tc.wantOut, result.CheckedOutActor)
			assert.Equal(t, tc.wantInName, result.CheckedInBy)
			assert.Equal(t, tc.wantOutName, result.CheckedOutBy)
		})
	}
}

func TestAttendanceActorsWithoutStaffSkipTheNameRead(t *testing.T) {
	t.Parallel()
	var device int64 = 9
	checkOut := time.Date(2026, 9, 9, 15, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name    string
		row     studentpresence.Attendance
		wantOut *studentpresence.AttendanceActor
	}{
		{name: "device check-in, still present", row: studentpresence.Attendance{}},
		{
			name:    "device checkout",
			row:     studentpresence.Attendance{CheckOutTime: &checkOut, CheckedOutDeviceID: &device},
			wantOut: &studentpresence.AttendanceActor{Kind: studentpresence.AttendanceActorDevice},
		},
		{
			name:    "automatic checkout",
			row:     studentpresence.Attendance{CheckOutTime: &checkOut},
			wantOut: &studentpresence.AttendanceActor{Kind: studentpresence.AttendanceActorAutomatic},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			svc := &service{ServiceDependencies: ServiceDependencies{StaffNames: attendanceStaffNamesFunc(func(context.Context, []int64) (map[int64]string, error) {
				t.Fatal("no staff ID, no name read")
				return nil, nil
			})}}
			result := &AttendanceStatus{}
			svc.applyAttendanceActors(context.Background(), result, tc.row)
			require.NotNil(t, result.CheckedInActor)
			assert.Equal(t, studentpresence.AttendanceActor{Kind: studentpresence.AttendanceActorDevice}, *result.CheckedInActor)
			assert.Equal(t, tc.wantOut, result.CheckedOutActor)
			assert.Empty(t, result.CheckedInBy)
			assert.Empty(t, result.CheckedOutBy)
		})
	}
}
