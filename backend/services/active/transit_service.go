package active

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
)

const (
	TransitSkipNotInTransit = "not_in_transit"
	TransitSkipCreateFailed = "create_failed"
)

// ListStudentsInTransit returns students who are checked in today but do not
// currently have an open room visit.
func (s *service) ListStudentsInTransit(ctx context.Context) ([]int64, error) {
	openAttendanceIDs, err := s.attendanceRepo.ListOpenStudentIDsForDate(ctx, timezone.TodayDate())
	if err != nil {
		return nil, &ActiveError{Op: "ListStudentsInTransit", Err: ErrDatabaseOperation}
	}
	if len(openAttendanceIDs) == 0 {
		return []int64{}, nil
	}

	currentVisits, err := s.visitRepo.GetCurrentByStudentIDs(ctx, openAttendanceIDs)
	if err != nil {
		return nil, &ActiveError{Op: "ListStudentsInTransit", Err: ErrDatabaseOperation}
	}

	ids := make([]int64, 0, len(openAttendanceIDs))
	for _, studentID := range openAttendanceIDs {
		if _, hasVisit := currentVisits[studentID]; hasVisit {
			continue
		}
		ids = append(ids, studentID)
	}

	return ids, nil
}

// AssignTransitStudentsToActiveGroup assigns checked-in students without an
// active room visit to an existing active group/session.
func (s *service) AssignTransitStudentsToActiveGroup(ctx context.Context, studentIDs []int64, activeGroupID int64) (*TransitAssignResult, error) {
	if activeGroupID <= 0 || len(studentIDs) == 0 {
		return nil, &ActiveError{Op: "AssignTransitStudentsToActiveGroup", Err: ErrInvalidData}
	}

	targetGroup, err := s.GetActiveGroup(ctx, activeGroupID)
	if err != nil {
		return nil, err
	}
	if targetGroup == nil || !targetGroup.IsActive() {
		return nil, &ActiveError{Op: "AssignTransitStudentsToActiveGroup", Err: ErrActiveGroupAlreadyEnded}
	}

	uniqueIDs := uniquePositiveInt64s(studentIDs)
	if len(uniqueIDs) == 0 {
		return nil, &ActiveError{Op: "AssignTransitStudentsToActiveGroup", Err: ErrInvalidData}
	}

	openAttendance, err := s.attendanceRepo.GetTodayByStudentIDs(ctx, uniqueIDs)
	if err != nil {
		return nil, &ActiveError{Op: "AssignTransitStudentsToActiveGroup", Err: ErrDatabaseOperation}
	}
	currentVisits, err := s.visitRepo.GetCurrentByStudentIDs(ctx, uniqueIDs)
	if err != nil {
		return nil, &ActiveError{Op: "AssignTransitStudentsToActiveGroup", Err: ErrDatabaseOperation}
	}

	result := &TransitAssignResult{
		Assigned:      []int64{},
		Skipped:       []TransitAssignSkipped{},
		ActiveGroupID: targetGroup.ID,
		RoomID:        targetGroup.RoomID,
	}

	for _, studentID := range uniqueIDs {
		attendance, hasAttendance := openAttendance[studentID]
		_, hasVisit := currentVisits[studentID]
		if !hasAttendance || attendance == nil || attendance.CheckOutTime != nil || hasVisit {
			result.Skipped = append(result.Skipped, TransitAssignSkipped{StudentID: studentID, Reason: TransitSkipNotInTransit})
			continue
		}

		visit := &active.Visit{
			StudentID:     studentID,
			ActiveGroupID: targetGroup.ID,
			EntryTime:     time.Now(),
		}
		if err := s.CreateVisit(ctx, visit); err != nil {
			if errors.Is(err, ErrStudentAlreadyActive) {
				result.Skipped = append(result.Skipped, TransitAssignSkipped{StudentID: studentID, Reason: TransitSkipNotInTransit})
				continue
			}
			result.Skipped = append(result.Skipped, TransitAssignSkipped{StudentID: studentID, Reason: TransitSkipCreateFailed})
			continue
		}

		result.Assigned = append(result.Assigned, studentID)
	}

	if len(result.Assigned) > 0 {
		if err := s.UpdateSessionActivity(ctx, targetGroup.ID); err != nil {
			return nil, err
		}
	}

	return result, nil
}

func uniquePositiveInt64s(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	unique := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	return unique
}
