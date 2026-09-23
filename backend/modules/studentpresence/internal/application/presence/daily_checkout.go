package presence

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/internal/timezone"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

type deviceSupervisorUnavailableError struct {
	reason string
}

func (e *deviceSupervisorUnavailableError) Error() string {
	return e.reason
}

// getDeviceSupervisorID retrieves the supervisor staff ID for a device's active group.
// Expected absence states use a typed error so daily checkout may safely fall
// back to device attribution without swallowing repository failures.
func (s *service) getDeviceSupervisorID(ctx context.Context, deviceID int64) (int64, error) {
	// Find active group for device
	activeGroup, err := s.GroupRepo.FindActiveByDeviceID(ctx, deviceID)
	if err != nil {
		// Handle case where no active group exists for this device
		if errors.Is(err, ErrNoActiveSession) {
			return 0, &deviceSupervisorUnavailableError{reason: fmt.Sprintf("no active group assigned to device %d", deviceID)}
		}
		return 0, fmt.Errorf("error finding active group for device %d: %w", deviceID, err)
	}

	if activeGroup == nil {
		return 0, &deviceSupervisorUnavailableError{reason: fmt.Sprintf("no active group assigned to device %d", deviceID)}
	}

	// Get supervisors for the active group
	day := timezone.TodayDate().String()
	supervisors, err := s.SchoolPresence.QueryGroupSupervisions(ctx, studentpresence.GroupSupervisionFilter{GroupIDs: []int64{activeGroup.ID}, ActiveOn: &day})
	if err != nil {
		return 0, fmt.Errorf("failed to get supervisors for group %d: %w", activeGroup.ID, &ActiveError{Op: "FindSupervisorsByActiveGroupID", Err: ErrDatabaseOperation})
	}

	if len(supervisors) == 0 {
		return 0, &deviceSupervisorUnavailableError{reason: fmt.Sprintf("no supervisors assigned to active group %d", activeGroup.ID)}
	}

	// Use first active supervisor
	today := timezone.TodayDate()
	for _, supervisor := range supervisors {
		running, err := supervisionRunsOn(supervisor, today)
		if err != nil {
			return 0, err
		}
		if running {
			return supervisor.StaffID, nil
		}
	}

	return 0, &deviceSupervisorUnavailableError{reason: fmt.Sprintf("no active supervisors found in group %d", activeGroup.ID)}
}

// supervisionRunsOn reports whether the supervision has started by day and
// ends after it. An unreadable date is a database failure.
func supervisionRunsOn(supervisor studentpresence.GroupSupervision, day timezone.Date) (bool, error) {
	start, err := timezone.ParseDate(supervisor.StartDate)
	if err != nil {
		return false, &ActiveError{Op: "FindSupervisorsByActiveGroupID", Err: ErrDatabaseOperation}
	}
	if start.After(day) {
		return false, nil
	}
	if supervisor.EndDate == nil {
		return true, nil
	}
	end, err := timezone.ParseDate(*supervisor.EndDate)
	if err != nil {
		return false, &ActiveError{Op: "FindSupervisorsByActiveGroupID", Err: ErrDatabaseOperation}
	}
	return day.Before(end), nil
}

// ConfirmDailyCheckout processes the deferred daily-checkout confirmation for an
// IoT device. Normally the student's visit was already ended by the checkin
// handler (student is "unterwegs") and this only updates the attendance record
// when the student confirms "nach Hause". If a visit is still open,
// CheckOutStudentFromDevice ends it in the same request transaction (issue #895).
//
// The student must already have an attendance record for today (status
// "checked_in" or "checked_out"); otherwise ErrNoAttendanceRecordForCheckout is
// returned. Attendance is only mutated when destination is "zuhause" and the
// student is still "checked_in"; a concurrent checkout is treated as an
// idempotent no-op.
func (s *service) ConfirmDailyCheckout(ctx context.Context, studentID, deviceID int64, destination string) (*DailyCheckoutResult, error) {
	var result *DailyCheckoutResult
	err := s.runInSessionTx(ctx, func(txCtx context.Context) error {
		var err error
		result, err = s.confirmDailyCheckout(txCtx, studentID, deviceID, destination)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *service) confirmDailyCheckout(ctx context.Context, studentID, deviceID int64, destination string) (*DailyCheckoutResult, error) {
	s.getLogger().InfoContext(ctx, "confirming daily checkout",
		slog.Int64("student_id", studentID),
		slog.String("destination", destination),
	)

	currentStatus, err := s.GetStudentAttendanceStatus(ctx, studentID)
	if err != nil {
		s.getLogger().ErrorContext(ctx, "failed to get attendance status",
			slog.Int64("student_id", studentID),
			slog.String("error", err.Error()),
		)
		return nil, err
	}

	if currentStatus.Status != "checked_in" && currentStatus.Status != "checked_out" {
		s.getLogger().ErrorContext(ctx, "student has no attendance record for today",
			slog.Int64("student_id", studentID),
			slog.String("status", currentStatus.Status),
		)
		return nil, ErrNoAttendanceRecordForCheckout
	}

	if destination == "zuhause" {
		switch currentStatus.Status {
		case "checked_out":
			s.getLogger().DebugContext(ctx, "student already checked out, skipping attendance toggle",
				slog.Int64("student_id", studentID),
			)
		case "checked_in":
			// CheckOutStudentFromDevice broadcasts the student_checkout /
			// dashboard_counts_changed pair itself, after the request
			// transaction commits (#2113) — the explicit BroadcastDailyCheckout
			// that used to sit here fired before the commit and is now a
			// duplicate.
			if _, err := s.CheckOutStudentFromDevice(ctx, studentID, deviceID); err != nil {
				s.getLogger().ErrorContext(ctx, "failed to update attendance for daily checkout",
					slog.Int64("student_id", studentID),
					slog.String("error", err.Error()),
				)
				return nil, err
			}
		}
	}

	action := "checked_out_daily"
	if destination == "unterwegs" {
		action = "checked_out"
	}

	s.getLogger().InfoContext(ctx, "daily checkout confirmed",
		slog.Int64("student_id", studentID),
		slog.String("action", action),
		slog.String("destination", destination),
	)

	return &DailyCheckoutResult{Action: action}, nil
}
