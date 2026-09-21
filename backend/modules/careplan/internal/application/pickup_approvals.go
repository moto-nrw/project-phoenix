package application

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type PickupApprovals struct {
	People      ports.PickupApprovalDirectory
	Presence    ports.PickupApprovalPresence
	Exceptions  ports.PickupApprovalExceptions
	Excusal     ports.PickupApprovalExcusal
	Locker      ports.PickupApprovalLocker
	Today       func() calendar.Date
	Fingerprint func([]byte) string
}

var _ carerequests.PickupApprovals = (*PickupApprovals)(nil)

func (s *PickupApprovals) ApplyPickupApproval(ctx context.Context, input carerequests.PickupApproval) (int64, error) {
	date, pickup, reason, err := carerequests.ParsePickup(input.Payload)
	if err != nil {
		return 0, err
	}
	if date.Before(s.Today()) {
		return 0, carerequests.ErrPickupChangeExpired
	}
	until, err := s.People.EnrolledUntil(ctx, input.StudentID)
	if err != nil {
		return 0, fmt.Errorf("schedule: reload student for pickup request decision: %w", err)
	}
	if until != nil && date.After(*until) {
		return 0, careplan.ErrCareScheduleRequestNotFound
	}
	if err := s.Locker.LockStudentAndExceptionDay(ctx, input.StudentID, date.String()); err != nil {
		return 0, fmt.Errorf("schedule: lock pickup request care day: %w", err)
	}
	if err := s.verifyImpact(ctx, input, date, pickup); err != nil {
		return 0, err
	}
	if err := s.ensureNotCompleted(ctx, input.StudentID, date); err != nil {
		return 0, err
	}
	staffID, err := s.People.ActingStaffID(ctx)
	if err != nil {
		return 0, err
	}
	id, err := s.SaveApprovedException(ctx, input.TenantID, input.StudentID, date, pickup, reason, staffID)
	if err != nil {
		return 0, err
	}
	if err := s.Excusal.Sync(ctx, id); err != nil {
		return 0, fmt.Errorf("schedule: sync approved pickup exception: %w", err)
	}
	return id, nil
}

func (s *PickupApprovals) verifyImpact(ctx context.Context, input carerequests.PickupApproval, date calendar.Date, pickup time.Time) error {
	if input.ExpectedImpactToken == nil {
		if input.RequireImpactToken {
			return carerequests.ErrPickupChangeImpactRequired
		}
		return nil
	}
	blocks, err := s.Excusal.Preview(ctx, input.StudentID, date, pickup)
	if err != nil {
		return fmt.Errorf("schedule: verify pickup request impact: %w", err)
	}
	if s.Fingerprint(carerequests.PickupImpactContent(blocks)) != *input.ExpectedImpactToken {
		return carerequests.ErrPickupChangeImpactChanged
	}
	return nil
}

func (s *PickupApprovals) ensureNotCompleted(ctx context.Context, studentID int64, date calendar.Date) error {
	if date != s.Today() {
		return nil
	}
	if err := s.Presence.LockStudentAttendance(ctx, studentID); err != nil {
		return fmt.Errorf("schedule: lock attendance for pickup request: %w", err)
	}
	open, completed, err := s.Presence.AttendanceCompletion(ctx, studentID, date)
	if err != nil {
		return fmt.Errorf("schedule: load attendance for pickup request: %w", err)
	}
	if completed && !open {
		return carerequests.ErrPickupChangeAlreadyCompleted
	}
	return nil
}

// SaveApprovedException is shared by approvals and staff conflict resolutions.
// Both callers hold the owner care-day lock in the ambient tenant transaction.
func (s *PickupApprovals) SaveApprovedException(ctx context.Context, tenantID, studentID int64, date calendar.Date, pickup time.Time, reason string, staffID int64) (int64, error) {
	existing, err := s.Exceptions.FindForDate(ctx, studentID, date)
	if err != nil {
		return 0, fmt.Errorf("schedule: load pickup exception for request: %w", err)
	}
	if existing != nil {
		if existing.Source == careplan.ExceptionSourceStaff || (existing.ExcusedFrom != nil && !existing.ExcusedAuto) {
			return 0, carerequests.ErrPickupChangeConflict
		}
		clock := calendar.NormalizeWallClock(pickup)
		existing.PickupTime = &clock
		existing.Reason = &reason
		existing.Source = careplan.ExceptionSourceStaff
		existing.CreatedBy = staffID
		existing.CreatedByGuardian = nil
		if existing.ExcusedFrom != nil {
			excused := calendar.NormalizeWallClock(*existing.ExcusedFrom)
			existing.ExcusedFrom = &excused
		}
		if err := s.Exceptions.Update(ctx, *existing); err != nil {
			return 0, fmt.Errorf("schedule: update approved pickup exception: %w", err)
		}
		return existing.ID, nil
	}
	id, err := s.Exceptions.Create(ctx, careplan.PickupException{
		TenantID: tenantID, StudentID: studentID, ExceptionDate: careplan.Date(date),
		PickupTime: &pickup, Reason: &reason, Source: careplan.ExceptionSourceStaff, CreatedBy: staffID,
	})
	if err != nil {
		if s.Exceptions.IsUniqueViolation(err) {
			return 0, carerequests.ErrPickupChangeConflict
		}
		return 0, fmt.Errorf("schedule: create approved pickup exception: %w", err)
	}
	return id, nil
}
