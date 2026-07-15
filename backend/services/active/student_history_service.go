package active

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
)

// StudentHistoryService exposes the reads (and the GDPR access-log write)
// behind the student attendance-history endpoint (issue #584: handlers must
// not hold repositories). Results and errors are returned VERBATIM; the
// handler keeps its assembly, scope checks, and audit-or-refuse decision.
type StudentHistoryService interface {
	// GetAttendanceByStudentAndDateRange returns a student's attendance rows
	// between two dates (inclusive).
	GetAttendanceByStudentAndDateRange(ctx context.Context, studentID int64, startDate, endDate timezone.Date) ([]*activeModels.Attendance, error)

	// GetVisitsByStudentAndTimeRange returns a student's visits (active or
	// ended) overlapping the time range.
	GetVisitsByStudentAndTimeRange(ctx context.Context, studentID int64, start, end time.Time) ([]*activeModels.Visit, error)

	GetSlotAttendanceByStudentAndDateRange(ctx context.Context, studentID int64, startDate, endDate timezone.Date) ([]*scheduleModels.ScheduledInstanceRow, error)

	// RecordDataAccess writes a GDPR data-access log entry.
	RecordDataAccess(ctx context.Context, entry *auditModels.DataAccessLog) error
}

type studentHistoryService struct {
	attendanceRepo activeModels.AttendanceRepository
	visitRepo      activeModels.VisitRepository
	accessLogRepo  auditModels.DataAccessLogRepository
	slotRepo       scheduleModels.InstanceStudentRepository
}

// NewStudentHistoryService creates a StudentHistoryService backed by the
// attendance, visit, and data-access-log repositories.
func NewStudentHistoryService(attendanceRepo activeModels.AttendanceRepository, visitRepo activeModels.VisitRepository, accessLogRepo auditModels.DataAccessLogRepository, slotRepos ...scheduleModels.InstanceStudentRepository) StudentHistoryService {
	service := &studentHistoryService{
		attendanceRepo: attendanceRepo,
		visitRepo:      visitRepo,
		accessLogRepo:  accessLogRepo,
	}
	if len(slotRepos) > 0 {
		service.slotRepo = slotRepos[0]
	}
	return service
}

func (s *studentHistoryService) GetSlotAttendanceByStudentAndDateRange(ctx context.Context, studentID int64, startDate, endDate timezone.Date) ([]*scheduleModels.ScheduledInstanceRow, error) {
	if s.slotRepo == nil {
		return []*scheduleModels.ScheduledInstanceRow{}, nil
	}
	return s.slotRepo.FindInstancesWithAttendanceByStudentAndDateRange(ctx, studentID, startDate, endDate)
}

func (s *studentHistoryService) GetAttendanceByStudentAndDateRange(ctx context.Context, studentID int64, startDate, endDate timezone.Date) ([]*activeModels.Attendance, error) {
	return s.attendanceRepo.FindByStudentAndDateRange(ctx, studentID, startDate, endDate)
}

func (s *studentHistoryService) GetVisitsByStudentAndTimeRange(ctx context.Context, studentID int64, start, end time.Time) ([]*activeModels.Visit, error) {
	return s.visitRepo.FindByStudentAndTimeRange(ctx, studentID, start, end)
}

func (s *studentHistoryService) RecordDataAccess(ctx context.Context, entry *auditModels.DataAccessLog) error {
	return s.accessLogRepo.Create(ctx, entry)
}
