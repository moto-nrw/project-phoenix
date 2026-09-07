package active

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// StudentHistoryService exposes the reads (and the GDPR access-log write)
// behind the student attendance-history endpoint (issue #584: handlers must
// not hold repositories). The handler keeps its assembly, scope checks,
// and audit-or-refuse decision; attendance rows come from the owner facade.
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

	GetSlotAttendanceByStudentAndDateRange(ctx context.Context, studentID int64, startDate, endDate timezone.Date) ([]*scheduleModels.ScheduledInstanceRow, error)

	// HasPlannedSlotsInRange reports whether the tenant has any planned
	// (non-walk-in) slot assignment on a non-cancelled instance within the
	// inclusive date range — the tenant-level signal that the care plan is
	// in use.
	HasPlannedSlotsInRange(ctx context.Context, startDate, endDate timezone.Date) (bool, error)

	// RecordDataAccess writes a GDPR data-access log entry.
	RecordDataAccess(ctx context.Context, entry *auditModels.DataAccessLog) error
}

type AttendanceHistoryReader interface {
	ListAttendance(context.Context, studentpresence.AttendanceFilter) ([]studentpresence.Attendance, error)
	ListVisitLocations(context.Context, studentpresence.VisitLocationFilter) ([]studentpresence.VisitLocation, error)
}

type HistoryRoomReader func(context.Context, []int64) (map[int64]string, error)

type VisitHistoryEntry struct {
	EntryTime time.Time
	ExitTime  *time.Time
	RoomID    *int64
	RoomName  string
}

type studentHistoryService struct {
	presence      AttendanceHistoryReader
	rooms         HistoryRoomReader
	accessLogRepo auditModels.DataAccessLogRepository
	slotRepo      scheduleModels.InstanceStudentRepository
}

// NewStudentHistoryService composes owner attendance reads with visit history,
// data-access logging, and planned slot attendance.
// slotRepo may be nil (tests without a timetable), in which case slot
// attendance reads return empty.
func NewStudentHistoryService(presence AttendanceHistoryReader, rooms HistoryRoomReader, accessLogRepo auditModels.DataAccessLogRepository, slotRepo scheduleModels.InstanceStudentRepository) StudentHistoryService {
	return &studentHistoryService{
		presence:      presence,
		rooms:         rooms,
		accessLogRepo: accessLogRepo,
		slotRepo:      slotRepo,
	}
}

func (s *studentHistoryService) GetSlotAttendanceByStudentAndDateRange(ctx context.Context, studentID int64, startDate, endDate timezone.Date) ([]*scheduleModels.ScheduledInstanceRow, error) {
	if s.slotRepo == nil {
		return []*scheduleModels.ScheduledInstanceRow{}, nil
	}
	return s.slotRepo.FindInstancesWithAttendanceByStudentAndDateRange(ctx, studentID, scheduleModels.Date(startDate), scheduleModels.Date(endDate))
}

func (s *studentHistoryService) HasPlannedSlotsInRange(ctx context.Context, startDate, endDate timezone.Date) (bool, error) {
	if s.slotRepo == nil {
		return false, nil
	}
	return s.slotRepo.HasPlannedSlotsInRange(ctx, scheduleModels.Date(startDate), scheduleModels.Date(endDate))
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

func (s *studentHistoryService) RecordDataAccess(ctx context.Context, entry *auditModels.DataAccessLog) error {
	return s.accessLogRepo.Create(ctx, entry)
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
