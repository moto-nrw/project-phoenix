package active

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

type StudentPresence interface {
	UnclaimedGroups(context.Context, string) ([]studentpresence.UnclaimedGroup, error)
	ClaimGroup(context.Context, studentpresence.GroupClaim) (studentpresence.ClaimedSupervision, error)
	FindVisit(context.Context, int64) (*studentpresence.Visit, error)
	DeleteVisit(context.Context, int64) error
	CloseVisits(context.Context, []int64, time.Time) ([]studentpresence.Visit, error)
	RecordVisit(context.Context, studentpresence.Visit) (studentpresence.Visit, error)
	ReviseVisit(context.Context, studentpresence.Visit) (studentpresence.Visit, error)
	ListVisits(context.Context, studentpresence.VisitFilter) ([]studentpresence.Visit, error)
	ListVisitLocations(context.Context, studentpresence.VisitLocationFilter) ([]studentpresence.VisitLocation, error)
	TransferOpenVisits(context.Context, int64, int64) (int64, error)
	TransferRecentDeviceVisits(context.Context, int64, int64) (int64, error)
	ListOpenVisitRooms(context.Context, int64) ([]studentpresence.OpenVisitRoom, error)
	CountOpenVisitsInRoom(context.Context, int64) (int, error)
	CountOpenVisitsInGroup(context.Context, int64) (int, error)
	// EndGroupSession and EndGroupSessions are the Student Presence owner's
	// session end commands (#2697): every path that closes a live group's
	// visits, supervisions, and the group itself writes through them.
	EndGroupSession(context.Context, int64, time.Time) (studentpresence.EndedGroupSession, error)
	EndGroupSessions(context.Context, []int64, time.Time) (studentpresence.EndedGroupSessions, error)
	ListSchoolStatuses(context.Context, []int64, string) ([]studentpresence.SchoolStatus, error)
	ListAttendance(context.Context, studentpresence.AttendanceFilter) ([]studentpresence.Attendance, error)
	EnsureAttendance(context.Context, studentpresence.Attendance) (studentpresence.Attendance, bool, error)
	EnsureAttendanceBatch(context.Context, []studentpresence.Attendance) ([]int64, error)
	CloseAttendance(context.Context, studentpresence.AttendanceCheckout) ([]studentpresence.Attendance, error)
	ReviseAttendance(context.Context, studentpresence.Attendance) (studentpresence.Attendance, error)
	LockStudentAttendance(context.Context, int64) error
	HasAttendance(context.Context, studentpresence.AttendanceFilter) (bool, error)
	ListOpenAttendanceStudentIDs(context.Context, string) ([]int64, error)
}

func validPresenceVisit(visit *studentpresence.Visit) bool {
	return visit != nil && visit.StudentID > 0 && visit.ActiveGroupID > 0 &&
		!visit.EntryTime.IsZero() && (visit.ExitTime == nil || !visit.EntryTime.After(*visit.ExitTime))
}

func presenceVisitSnapshot(row *studentpresence.Visit) *studentpresence.Visit {
	if row == nil {
		return nil
	}
	visit := *row
	return &visit
}

func (s *service) currentPresenceVisits(ctx context.Context, studentIDs []int64, lock bool) (map[int64]*studentpresence.Visit, error) {
	result := make(map[int64]*studentpresence.Visit, len(studentIDs))
	if len(studentIDs) == 0 {
		return result, nil
	}
	visits, err := s.SchoolPresence.ListVisits(ctx, studentpresence.VisitFilter{
		StudentIDs: studentIDs, OpenOnly: true, NewestFirst: true, StudentOrder: true, ForUpdate: lock,
	})
	if err != nil {
		return nil, err
	}
	for _, visit := range visits {
		if _, found := result[visit.StudentID]; !found {
			result[visit.StudentID] = &visit
		}
	}
	return result, nil
}

func (s *service) lockOpenAttendance(ctx context.Context, ids []int64) (map[int64]studentpresence.Attendance, error) {
	if len(ids) == 0 {
		return map[int64]studentpresence.Attendance{}, nil
	}
	day := s.todayDate().String()
	rows, err := s.SchoolPresence.ListAttendance(ctx, studentpresence.AttendanceFilter{
		StudentIDs: ids, FromDate: day, UntilDate: day, OpenOnly: true, ForUpdate: true, StudentOrder: true,
	})
	if err != nil {
		return nil, err
	}
	result := make(map[int64]studentpresence.Attendance, len(rows))
	for _, row := range rows {
		result[row.StudentID] = row
	}
	return result, nil
}
