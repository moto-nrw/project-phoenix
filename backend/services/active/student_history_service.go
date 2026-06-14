package active

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
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

	// RecordDataAccess writes a GDPR data-access log entry.
	RecordDataAccess(ctx context.Context, entry *auditModels.DataAccessLog) error
}

type studentHistoryService struct {
	attendanceRepo activeModels.AttendanceRepository
	visitRepo      activeModels.VisitRepository
	accessLogRepo  auditModels.DataAccessLogRepository
}

// NewStudentHistoryService creates a StudentHistoryService backed by the
// attendance, visit, and data-access-log repositories.
func NewStudentHistoryService(attendanceRepo activeModels.AttendanceRepository, visitRepo activeModels.VisitRepository, accessLogRepo auditModels.DataAccessLogRepository) StudentHistoryService {
	return &studentHistoryService{
		attendanceRepo: attendanceRepo,
		visitRepo:      visitRepo,
		accessLogRepo:  accessLogRepo,
	}
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
