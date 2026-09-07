package education

import (
	"context"
	"errors"
	"testing"
	"time"

	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/stretchr/testify/require"
)

type graduationPresenceStub struct {
	visits                  []studentpresence.Visit
	attendance              []studentpresence.Attendance
	visitErr, attendanceErr error
	visitFilter             studentpresence.VisitFilter
	attendanceFilter        studentpresence.AttendanceFilter
}

func (s *graduationPresenceStub) ListVisits(_ context.Context, filter studentpresence.VisitFilter) ([]studentpresence.Visit, error) {
	s.visitFilter = filter
	return s.visits, s.visitErr
}

func (s *graduationPresenceStub) ListAttendance(_ context.Context, filter studentpresence.AttendanceFilter) ([]studentpresence.Attendance, error) {
	s.attendanceFilter = filter
	return s.attendance, s.attendanceErr
}

func TestGraduationPresenceGuard(t *testing.T) {
	t.Parallel()
	checkout := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	readFailure := errors.New("presence read failed")
	for _, tc := range []struct {
		name     string
		presence graduationPresenceStub
		wantErr  error
	}{
		{name: "attendance without room visit", presence: graduationPresenceStub{attendance: []studentpresence.Attendance{{StudentID: 7}}}, wantErr: ErrGraduatesCheckedIn},
		{name: "open room visit", presence: graduationPresenceStub{visits: []studentpresence.Visit{{StudentID: 7}}}, wantErr: ErrGraduatesCheckedIn},
		{name: "latest attendance wins", presence: graduationPresenceStub{attendance: []studentpresence.Attendance{{StudentID: 7, CheckOutTime: &checkout}, {StudentID: 7}}}},
		{name: "visit read failure", presence: graduationPresenceStub{visitErr: readFailure}, wantErr: readFailure},
		{name: "attendance read failure", presence: graduationPresenceStub{attendanceErr: readFailure}, wantErr: readFailure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := NewGradeTransitionService(GradeTransitionServiceDependencies{Presence: &tc.presence})
			day := service.today().String()
			err := service.ensureGraduatesNotCheckedIn(context.Background(), []*educationModels.StudentClassInfo{{StudentID: 7}})
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, []int64{7}, tc.presence.visitFilter.StudentIDs)
			require.True(t, tc.presence.visitFilter.OpenOnly)
			if tc.presence.visitErr == nil {
				require.Equal(t, studentpresence.AttendanceFilter{StudentIDs: []int64{7}, FromDate: day, UntilDate: day, NewestFirst: true, StudentOrder: true}, tc.presence.attendanceFilter)
			}
		})
	}
}
