package active

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/device"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/active"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	"github.com/uptrace/bun"
)

// Visit operations

func (s *service) GetVisit(ctx context.Context, id int64) (*active.Visit, error) {
	visit, err := s.visitRepo.FindByID(ctx, id)
	if err != nil {
		return nil, &ActiveError{Op: "GetVisit", Err: ErrVisitNotFound}
	}
	return visit, nil
}

func (s *service) CreateVisit(ctx context.Context, visit *active.Visit) error {
	if visit == nil || visit.Validate() != nil {
		return &ActiveError{Op: "CreateVisit", Err: ErrInvalidData}
	}

	deviceID, staffID := s.extractContextIDs(ctx)

	err := s.txHandler.RunInTx(ctx, func(txCtx context.Context, tx bun.Tx) error {
		txService := s.WithTx(tx).(*service)

		// Ensure no existing active visit for this student
		if err := txService.ensureStudentHasNoActiveVisit(txCtx, visit.StudentID); err != nil {
			return err
		}

		// Handle attendance (create new or update on re-entry)
		if err := txService.ensureOrUpdateAttendance(txCtx, visit, staffID, deviceID); err != nil {
			return err
		}

		// Auto-clear sickness when student checks in
		txService.autoClearStudentSickness(txCtx, visit.StudentID)

		// Create the visit record
		if txService.visitRepo.Create(txCtx, visit) != nil {
			return &ActiveError{Op: "CreateVisit", Err: ErrDatabaseOperation}
		}

		return nil
	})

	if err != nil {
		if activeErr, ok := err.(*ActiveError); ok {
			return activeErr
		}
		return &ActiveError{Op: "CreateVisit", Err: ErrDatabaseOperation}
	}

	// Broadcast SSE event (fire-and-forget, outside transaction)
	s.broadcastVisitCreated(ctx, visit)

	return nil
}

// extractContextIDs extracts device and staff IDs from context
func (s *service) extractContextIDs(ctx context.Context) (deviceID, staffID int64) {
	if deviceCtx := device.DeviceFromCtx(ctx); deviceCtx != nil {
		deviceID = deviceCtx.ID
	}
	if staffCtx := device.StaffFromCtx(ctx); staffCtx != nil {
		staffID = staffCtx.ID
	}
	return deviceID, staffID
}

func (s *service) UpdateVisit(ctx context.Context, visit *active.Visit) error {
	if visit == nil || visit.Validate() != nil {
		return &ActiveError{Op: "UpdateVisit", Err: ErrInvalidData}
	}

	if s.visitRepo.Update(ctx, visit) != nil {
		return &ActiveError{Op: "UpdateVisit", Err: ErrDatabaseOperation}
	}

	return nil
}

func (s *service) DeleteVisit(ctx context.Context, id int64) error {
	_, err := s.visitRepo.FindByID(ctx, id)
	if err != nil {
		return &ActiveError{Op: "DeleteVisit", Err: ErrVisitNotFound}
	}

	if s.visitRepo.Delete(ctx, id) != nil {
		return &ActiveError{Op: "DeleteVisit", Err: ErrDatabaseOperation}
	}

	return nil
}

func (s *service) ListVisits(ctx context.Context, options *base.QueryOptions) ([]*active.Visit, error) {
	visits, err := s.visitRepo.List(ctx, options)
	if err != nil {
		return nil, &ActiveError{Op: "ListVisits", Err: ErrDatabaseOperation}
	}
	return visits, nil
}

func (s *service) FindVisitsByStudentID(ctx context.Context, studentID int64) ([]*active.Visit, error) {
	visits, err := s.visitRepo.FindActiveByStudentID(ctx, studentID)
	if err != nil {
		return nil, &ActiveError{Op: "FindVisitsByStudentID", Err: ErrDatabaseOperation}
	}
	return visits, nil
}

func (s *service) FindVisitsByActiveGroupID(ctx context.Context, activeGroupID int64) ([]*active.Visit, error) {
	visits, err := s.visitRepo.FindByActiveGroupID(ctx, activeGroupID)
	if err != nil {
		return nil, &ActiveError{Op: "FindVisitsByActiveGroupID", Err: ErrDatabaseOperation}
	}
	return visits, nil
}

func (s *service) FindVisitsByTimeRange(ctx context.Context, start, end time.Time) ([]*active.Visit, error) {
	if start.After(end) {
		return nil, &ActiveError{Op: "FindVisitsByTimeRange", Err: ErrInvalidTimeRange}
	}

	visits, err := s.visitRepo.FindByTimeRange(ctx, start, end)
	if err != nil {
		return nil, &ActiveError{Op: "FindVisitsByTimeRange", Err: ErrDatabaseOperation}
	}
	return visits, nil
}

func (s *service) EndVisit(ctx context.Context, id int64) error {
	autoSyncAttendance := shouldAutoSyncAttendance(ctx)
	deviceID, staffID := s.extractContextIDsIfAutoSync(ctx, autoSyncAttendance)

	var endedVisit *active.Visit
	err := s.txHandler.RunInTx(ctx, func(txCtx context.Context, tx bun.Tx) error {
		txService := s.WithTx(tx).(*service)

		visit, err := txService.endVisitRecord(txCtx, id)
		if err != nil {
			return err
		}
		endedVisit = visit

		if visit.ExitTime == nil || !autoSyncAttendance {
			return nil
		}

		return txService.syncAttendanceOnVisitEnd(txCtx, visit, deviceID, staffID)
	})

	if err != nil {
		if activeErr, ok := err.(*ActiveError); ok {
			return activeErr
		}
		return &ActiveError{Op: "EndVisit", Err: ErrDatabaseOperation}
	}

	s.broadcastVisitCheckout(ctx, endedVisit)
	return nil
}

// extractContextIDsIfAutoSync extracts device and staff IDs from context when auto-sync is enabled
func (s *service) extractContextIDsIfAutoSync(ctx context.Context, autoSyncAttendance bool) (deviceID, staffID int64) {
	if !autoSyncAttendance {
		return 0, 0
	}
	return s.extractContextIDs(ctx)
}

// endVisitRecord ends the visit record and returns the updated visit
func (s *service) endVisitRecord(ctx context.Context, id int64) (*active.Visit, error) {
	visit, err := s.visitRepo.FindByID(ctx, id)
	if err != nil || visit == nil {
		return nil, &ActiveError{Op: "EndVisit", Err: ErrVisitNotFound}
	}

	if s.visitRepo.EndVisit(ctx, id) != nil {
		return nil, &ActiveError{Op: "EndVisit", Err: ErrDatabaseOperation}
	}

	visit, err = s.visitRepo.FindByID(ctx, id)
	if err != nil || visit == nil {
		return nil, &ActiveError{Op: "EndVisit", Err: ErrVisitNotFound}
	}

	return visit, nil
}

// syncAttendanceOnVisitEnd synchronizes attendance record when a visit ends
func (s *service) syncAttendanceOnVisitEnd(ctx context.Context, visit *active.Visit, deviceID, staffID int64) error {
	// Only auto-check the student out if no other active visits remain
	activeVisits, err := s.visitRepo.FindActiveByStudentID(ctx, visit.StudentID)
	if err != nil {
		return &ActiveError{Op: "EndVisit", Err: ErrDatabaseOperation}
	}
	if len(activeVisits) > 0 {
		return nil
	}

	attendance, err := s.getStudentAttendanceOrIgnoreMissing(ctx, visit.StudentID)
	if err != nil {
		return err
	}
	if attendance == nil || attendance.CheckOutTime != nil {
		return nil
	}

	return s.updateAttendanceCheckout(ctx, attendance, visit, deviceID, staffID)
}

// getStudentAttendanceOrIgnoreMissing retrieves attendance or returns nil if not found
func (s *service) getStudentAttendanceOrIgnoreMissing(ctx context.Context, studentID int64) (*active.Attendance, error) {
	attendance, err := s.attendanceRepo.GetStudentCurrentStatus(ctx, studentID)
	if err == nil {
		return attendance, nil
	}

	// Ignore missing attendance – nothing to sync
	var dbErr *base.DatabaseError
	if errors.As(err, &dbErr) && errors.Is(dbErr.Err, sql.ErrNoRows) {
		return nil, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return nil, &ActiveError{Op: "EndVisit", Err: err}
}

// updateAttendanceCheckout updates attendance with checkout information
func (s *service) updateAttendanceCheckout(ctx context.Context, attendance *active.Attendance, visit *active.Visit, deviceID, staffID int64) error {
	resolvedStaffID := staffID
	if resolvedStaffID == 0 && deviceID > 0 {
		if supervisorID, err := s.getDeviceSupervisorID(ctx, deviceID); err == nil {
			resolvedStaffID = supervisorID
		}
	}

	checkoutTime := *visit.ExitTime
	attendance.CheckOutTime = &checkoutTime
	if resolvedStaffID > 0 {
		attendance.CheckedOutBy = &resolvedStaffID
	}

	if err := s.attendanceRepo.Update(ctx, attendance); err != nil {
		return &ActiveError{Op: "EndVisit", Err: err}
	}
	return nil
}

func (s *service) GetStudentCurrentVisit(ctx context.Context, studentID int64) (*active.Visit, error) {
	visits, err := s.visitRepo.FindActiveByStudentID(ctx, studentID)
	if err != nil {
		return nil, &ActiveError{Op: "GetStudentCurrentVisit", Err: ErrDatabaseOperation}
	}

	if len(visits) == 0 {
		return nil, &ActiveError{Op: "GetStudentCurrentVisit", Err: ErrVisitNotFound}
	}

	// Return the first active visit (there should only be one)
	return visits[0], nil
}

func (s *service) GetStudentsCurrentVisits(ctx context.Context, studentIDs []int64) (map[int64]*active.Visit, error) {
	if len(studentIDs) == 0 {
		return map[int64]*active.Visit{}, nil
	}

	visits, err := s.visitRepo.GetCurrentByStudentIDs(ctx, studentIDs)
	if err != nil {
		return nil, &ActiveError{Op: "GetStudentsCurrentVisits", Err: ErrDatabaseOperation}
	}

	result := make(map[int64]*active.Visit)
	for _, v := range visits {
		result[v.StudentID] = v
	}
	return result, nil
}
