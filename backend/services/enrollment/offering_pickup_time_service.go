package enrollment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/moto-nrw/project-phoenix/internal/sliceutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// OfferingPickupTimeService owns the write-side effects of booking-derived
// pickup times. The pickup baseline itself is projected at read time by the
// schedule service; this contract only removes manual overrides and refreshes
// the materialized consumers that still depend on a pickup baseline.
type OfferingPickupTimeService interface {
	ReconcileOfferingPickupForStudents(ctx context.Context, studentIDs []int64) error
	ResetStudentPickupDayToOffering(ctx context.Context, studentID int64, date timezone.Date) error
}

// ErrPickupResetNoOffering protects a manual pickup row when no booking-derived
// pickup time would replace it on the requested date.
var ErrPickupResetNoOffering = errors.New("für diesen Tag gibt es keine Angebots-Gehzeit")

// OfferingPickupBaselineReader is the date-aware read boundary needed by the
// reset write. The schedule projector implements it without coupling this
// service package to the schedule service package.
type OfferingPickupBaselineReader interface {
	OfferingPickupForDate(ctx context.Context, studentID int64, date timezone.Date) (*scheduleModels.StudentPickupSchedule, error)
}

// LockOfferingDerivedWrites establishes the project-wide gate order before a
// caller locks student rows and then changes booking- or offering-derived
// state. Transaction-scoped locks are safe to acquire again downstream.
func (s *decisionService) LockOfferingDerivedWrites(ctx context.Context) error {
	if s.StudentRepo != nil {
		if err := s.StudentRepo.LockStudentClassWritesShared(ctx); err != nil {
			return fmt.Errorf("lock class writes for offering-derived change: %w", err)
		}
	}
	return s.lockTemplateRecurrence(ctx)
}

// ReconcileOfferingPickupForStudents refreshes the remaining derived effects.
// It deliberately writes no schedule.student_pickup_schedules rows: regular
// offering pickup times are a date-aware projection of booking validity.
func (s *decisionService) ReconcileOfferingPickupForStudents(
	ctx context.Context,
	studentIDs []int64,
) error {
	studentIDs = sortedPositiveIDs(studentIDs)
	if len(studentIDs) == 0 {
		return nil
	}
	if err := s.resyncPickupAutoExcusals(ctx, studentIDs); err != nil {
		return err
	}
	s.deferOfferingPickupBroadcasts(ctx, studentIDs)
	return nil
}

// ReconcileOfferingPickupForOffering refreshes every current or future child
// affected by an offering edit. CareOfferingService calls it after update.
func (s *decisionService) ReconcileOfferingPickupForOffering(ctx context.Context, offeringID int64) error {
	studentIDs, err := s.offeringPickupAffectedStudents(ctx, offeringID)
	if err != nil {
		return err
	}
	return s.ReconcileOfferingPickupForStudents(ctx, studentIDs)
}

// ResetStudentPickupDayToOffering removes the stored row for one weekday. A
// staff row is the manual override; an old care_offering row is legacy
// materialization. The row is removed only when the read-time projection has
// an offering time for the requested date.
func (s *decisionService) ResetStudentPickupDayToOffering(
	ctx context.Context,
	studentID int64,
	date timezone.Date,
) error {
	if !s.hasOfferingPickupDependencies() {
		return fmt.Errorf("offering pickup reset: repositories not configured")
	}
	weekday := int(date.Weekday())
	if weekday < scheduleModels.WeekdayMonday || weekday > scheduleModels.WeekdayFriday {
		return fmt.Errorf("pickup reset date must be Monday through Friday")
	}
	if err := s.lockTemplateRecurrence(ctx); err != nil {
		return err
	}
	if err := s.lockPickupStudents(ctx, []int64{studentID}); err != nil {
		return err
	}
	offering, err := s.PickupBaselines.OfferingPickupForDate(ctx, studentID, date)
	if err != nil {
		return fmt.Errorf("load offering pickup for reset: %w", err)
	}
	if offering == nil {
		return ErrPickupResetNoOffering
	}
	rows, err := s.PickupScheduleRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		return fmt.Errorf("load pickup schedules: %w", err)
	}
	for _, row := range rows {
		if row != nil && row.Weekday == weekday {
			if err := s.PickupScheduleRepo.Delete(ctx, row.ID); err != nil {
				return fmt.Errorf("delete pickup schedule override: %w", err)
			}
			break
		}
	}
	if err := s.resyncPickupAutoExcusals(ctx, []int64{studentID}); err != nil {
		return err
	}
	return nil
}

// syncOfferingPickupAfterApproval refreshes projected-pickup consumers after
// an approved child has been linked to a student. It writes no weekly rows.
func (s *decisionService) syncOfferingPickupAfterApproval(
	ctx context.Context,
	child *enrollmentModels.RequestChild,
) error {
	studentID := int64(0)
	switch {
	case child == nil:
		return nil
	case child.CreatedStudentID != nil && *child.CreatedStudentID > 0:
		studentID = *child.CreatedStudentID
	case child.MatchedStudentID != nil && *child.MatchedStudentID > 0:
		studentID = *child.MatchedStudentID
	default:
		return nil
	}
	return s.ReconcileOfferingPickupForStudents(ctx, []int64{studentID})
}

func (s *decisionService) offeringPickupAffectedStudents(ctx context.Context, offeringID int64) ([]int64, error) {
	if offeringID <= 0 {
		return nil, fmt.Errorf("%w: offering id is required", ErrCareOfferingInvalid)
	}
	children, err := s.RequestChildOfferingRepo.ListApprovedChildrenByCareOfferingIDs(
		ctx,
		[]int64{offeringID},
		timezone.TodayDate(),
	)
	if err != nil {
		return nil, fmt.Errorf("list approved offering children: %w", err)
	}
	studentIDs := make([]int64, 0, len(children))
	for _, child := range children {
		if child != nil {
			studentIDs = append(studentIDs, child.StudentID)
		}
	}
	return sortedPositiveIDs(studentIDs), nil
}

func (s *decisionService) lockPickupStudents(ctx context.Context, studentIDs []int64) error {
	if s.LockPickupStudents == nil || len(studentIDs) == 0 {
		return nil
	}
	if err := s.LockPickupStudents(ctx, studentIDs); err != nil {
		return fmt.Errorf("lock students for pickup reset: %w", err)
	}
	return nil
}

func (s *decisionService) resyncPickupAutoExcusals(ctx context.Context, studentIDs []int64) error {
	if s.ResyncPickupAutoExcusals == nil || len(studentIDs) == 0 {
		return nil
	}
	if err := s.ResyncPickupAutoExcusals(ctx, studentIDs); err != nil {
		return fmt.Errorf("resync pickup auto excusals: %w", err)
	}
	return nil
}

func (s *decisionService) deferOfferingPickupBroadcasts(ctx context.Context, studentIDs []int64) {
	if len(studentIDs) == 0 || (s.Broadcaster == nil && s.PickupGuardianNotifier == nil) {
		return
	}
	tenantID := tenant.FromContext(ctx)
	tenant.RegisterAfterCommit(ctx, func() {
		if tenantID <= 0 {
			return
		}
		if s.Broadcaster != nil {
			source := "offering_pickup_projection"
			event := realtime.NewEvent(realtime.EventPickupScheduleChanged, "", realtime.EventData{Source: &source})
			if err := s.Broadcaster.BroadcastToTenant(tenantID, event); err != nil {
				s.Logger.Warn(
					"offering pickup projection: failed to broadcast schedule change",
					slog.Int64("tenant_id", tenantID),
					slog.String("error", err.Error()),
				)
			}
		}
		if s.PickupGuardianNotifier != nil {
			for _, studentID := range studentIDs {
				s.PickupGuardianNotifier.BroadcastChildUpdateToGuardians(tenantID, studentID)
			}
		}
	})
}

func (s *decisionService) hasOfferingPickupDependencies() bool {
	return s.PickupScheduleRepo != nil && s.PickupBaselines != nil
}

func sortedPositiveIDs(ids []int64) []int64 {
	out := sliceutil.UniquePositive(ids)
	slices.Sort(out)
	return out
}
