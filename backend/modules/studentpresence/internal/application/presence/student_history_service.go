package presence

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// StudentHistoryService exposes the attendance and visit reads behind the
// student attendance-history endpoint (issue #584: handlers must not hold
// repositories). The handler keeps its assembly, scope checks, and
// audit-or-refuse decision; attendance rows come from the owner facade. The
// composition root adds the slot reads and the GDPR access-log write.
type StudentHistoryService interface {
	// GetAttendanceByStudentAndDateRange returns a student's attendance rows
	// between two dates (inclusive).
	GetAttendanceByStudentAndDateRange(ctx context.Context, studentID int64, startDate, endDate timezone.Date) ([]*studentpresence.Attendance, error)

	// GetAttendanceForDate returns every attendance row of the tenant for one
	// calendar date (group day log reads the whole day at once).
	GetAttendanceForDate(ctx context.Context, date timezone.Date) ([]*studentpresence.Attendance, error)

	// GetAttendanceForDateByStudentIDs returns attendance rows for the supplied
	// students on one calendar date.
	GetAttendanceForDateByStudentIDs(ctx context.Context, date timezone.Date, studentIDs []int64) ([]*studentpresence.Attendance, error)

	// GetVisitsByStudentAndTimeRange returns a student's visits (active or
	// ended) entered within the inclusive time range.
	GetVisitsByStudentAndTimeRange(ctx context.Context, studentID int64, start, end time.Time) ([]*VisitHistoryEntry, error)
}

type AttendanceHistoryReader interface {
	ListAttendance(context.Context, studentpresence.AttendanceFilter) ([]studentpresence.Attendance, error)
	ListVisitLocations(context.Context, studentpresence.VisitLocationFilter) ([]studentpresence.VisitLocation, error)
}

type HistoryRoomReader func(context.Context, []int64) (map[int64]string, error)

// HistorySlotReader supplies the student's slot history and the tenant-wide
// signal that planned care is in use. Reads retain the caller's tenant context.
type HistorySlotReader interface {
	Slots(context.Context, int64, timezone.Date, timezone.Date) ([]*HistorySlot, error)
	HasPlannedSlots(context.Context, timezone.Date, timezone.Date) (bool, error)
}

type studentHistoryService struct {
	presence AttendanceHistoryReader
	rooms    HistoryRoomReader
}

// NewStudentHistoryService composes owner attendance reads with visit history.
func NewStudentHistoryService(presence AttendanceHistoryReader, rooms HistoryRoomReader) StudentHistoryService {
	return &studentHistoryService{
		presence: presence,
		rooms:    rooms,
	}
}

func (s *studentHistoryService) GetAttendanceByStudentAndDateRange(ctx context.Context, studentID int64, startDate, endDate timezone.Date) ([]*studentpresence.Attendance, error) {
	return s.attendanceRows(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{studentID}, FromDate: startDate.String(), UntilDate: endDate.String(), NewestFirst: true})
}

func (s *studentHistoryService) GetAttendanceForDate(ctx context.Context, date timezone.Date) ([]*studentpresence.Attendance, error) {
	return s.attendanceRows(ctx, studentpresence.AttendanceFilter{FromDate: date.String(), UntilDate: date.String(), StudentOrder: true})
}

func (s *studentHistoryService) GetAttendanceForDateByStudentIDs(ctx context.Context, date timezone.Date, studentIDs []int64) ([]*studentpresence.Attendance, error) {
	if len(studentIDs) == 0 {
		return []*studentpresence.Attendance{}, nil
	}
	return s.attendanceRows(ctx, studentpresence.AttendanceFilter{StudentIDs: studentIDs, FromDate: date.String(), UntilDate: date.String(), StudentOrder: true})
}

func (s *studentHistoryService) GetVisitsByStudentAndTimeRange(ctx context.Context, studentID int64, start, end time.Time) ([]*VisitHistoryEntry, error) {
	locations, err := s.presence.ListVisitLocations(ctx, studentpresence.VisitLocationFilter{VisitFilter: studentpresence.VisitFilter{
		StudentIDs: []int64{studentID}, EnteredFrom: &start, EnteredUntil: &end,
	}})
	if err != nil {
		return nil, err
	}
	roomIDs := make([]int64, 0, len(locations))
	for _, location := range locations {
		if location.Group != nil {
			roomIDs = append(roomIDs, location.Group.RoomID)
		}
	}
	roomNames := make(map[int64]string)
	if len(roomIDs) > 0 {
		roomNames, err = s.rooms(ctx, roomIDs)
		if err != nil {
			return nil, err
		}
	}
	result := make([]*VisitHistoryEntry, 0, len(locations))
	for _, location := range locations {
		visit := &VisitHistoryEntry{EntryTime: location.Visit.EntryTime, ExitTime: location.Visit.ExitTime}
		if location.Group != nil {
			roomID := location.Group.RoomID
			visit.RoomID, visit.RoomName = &roomID, roomNames[roomID]
		}
		result = append(result, visit)
	}
	return result, nil
}

func (s *studentHistoryService) attendanceRows(ctx context.Context, filter studentpresence.AttendanceFilter) ([]*studentpresence.Attendance, error) {
	rows, err := s.presence.ListAttendance(ctx, filter)
	if err != nil {
		return nil, err
	}
	result := make([]*studentpresence.Attendance, 0, len(rows))
	for _, row := range rows {
		result = append(result, &row)
	}
	return result, nil
}
