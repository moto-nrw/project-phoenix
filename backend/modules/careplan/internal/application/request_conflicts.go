package application

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type RequestConflicts struct {
	Records   ports.RequestConflictRecords
	Plans     ports.RequestConflictPlans
	Decisions carerequests.Decisions
	Today     func() calendar.Date
}

func (s *RequestConflicts) ConflictCandidate(ctx context.Context, id int64) (*carerequests.ConflictCandidate, error) {
	row, err := s.pending(ctx, id, false)
	if err != nil {
		return nil, err
	}
	return &carerequests.ConflictCandidate{StudentID: row.StudentID, UpdatedAt: row.UpdatedAt}, nil
}

func (s *RequestConflicts) LockConflictRequest(ctx context.Context, id int64) error {
	_, err := s.pending(ctx, id, true)
	return err
}

func (s *RequestConflicts) pending(ctx context.Context, id int64, lock bool) (carerequests.Request, error) {
	row, err := s.Records.Find(ctx, id, lock)
	if err != nil {
		return carerequests.Request{}, err
	}
	if row.Status != "pending" {
		return carerequests.Request{}, careplan.ErrCareScheduleRequestNotPending
	}
	return row, nil
}

func (s *RequestConflicts) DecideConflictRequest(ctx context.Context, input carerequests.DecideInput) error {
	// The coordinator already locked and version-checked the group.
	input.RequireImpactToken = false
	_, err := s.Decisions.Decide(ctx, input)
	return err
}

func (s *RequestConflicts) WriteStaffValue(ctx context.Context, write carerequests.StaffValueWrite) error {
	pickup, err := staffPickupTime(write.PickupTime)
	if err != nil {
		return err
	}
	if len(write.RequestIDs) == 0 {
		return carerequests.ErrStaffValueUnsupported
	}
	request, err := s.Records.Find(ctx, write.RequestIDs[0], false)
	if err != nil {
		return fmt.Errorf("schedule: load conflict group request: %w", err)
	}
	if request.RequestKind == "pickup_change" {
		return s.writePickupException(ctx, request, write.Reason, pickup)
	}
	weekday, err := conflictKeyWeekday(write.ConflictKey)
	if err != nil {
		return err
	}
	staffID, err := s.Plans.ActingStaffID(ctx)
	if err != nil {
		return err
	}
	return s.Plans.UpsertStudentPickupSchedule(ctx, &careplan.PickupSchedule{
		StudentID: write.StudentID, Weekday: weekday, PickupTime: calendar.NormalizeWallClock(pickup),
		Notes: &write.Reason, CreatedBy: staffID,
	})
}

func (s *RequestConflicts) writePickupException(ctx context.Context, request carerequests.Request, reason string, pickup time.Time) error {
	date, _, _, err := carerequests.ParsePickup(request.Payload)
	if err != nil {
		return err
	}
	if date.Before(s.Today()) {
		return carerequests.ErrPickupChangeExpired
	}
	if err := s.Plans.LockStudentAndExceptionDay(ctx, request.StudentID, date.String()); err != nil {
		return fmt.Errorf("schedule: lock staff pickup day: %w", err)
	}
	staffID, err := s.Plans.ActingStaffID(ctx)
	if err != nil {
		return err
	}
	exceptionID, err := s.Plans.SaveApprovedException(ctx, request.TenantID, request.StudentID, date, pickup, reason, staffID)
	if err != nil {
		return err
	}
	if err := s.Plans.Sync(ctx, exceptionID); err != nil {
		return fmt.Errorf("schedule: sync staff pickup exception: %w", err)
	}
	return nil
}

func conflictKeyWeekday(key string) (int, error) {
	parts := strings.Split(key, ":")
	if len(parts) != 3 || parts[0] != "care" {
		return 0, carerequests.ErrStaffValueUnsupported
	}
	weekday, err := strconv.Atoi(parts[1])
	if err != nil || weekday < 1 || weekday > 7 {
		return 0, carerequests.ErrStaffValueUnsupported
	}
	return weekday, nil
}

func staffPickupTime(raw string) (time.Time, error) {
	parsed, err := time.Parse("15:04", strings.TrimSpace(raw))
	if err != nil {
		return time.Time{}, carerequests.ErrInvalidPayload
	}
	return calendar.NormalizeWallClock(parsed), nil
}
