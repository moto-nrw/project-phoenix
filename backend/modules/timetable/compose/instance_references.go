package compose

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/sliceutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// instanceReferences are the tenant-scoped foreign ids a planner write
// supplies.
type instanceReferences struct {
	roomID           int64
	activityGroupID  *int64
	staffIDs         []int64
	studentIDs       []int64
	createdByStaffID *int64
}

// validateInstanceReferences checks every supplied id against the current
// tenant. date is the date the rows will LIVE on (the target of a move): it
// decides whether a graduated child is still refused.
func (s *InstanceLifecycleService) validateInstanceReferences(ctx context.Context, date timezone.Date, refs instanceReferences) error {
	if err := s.validateRoomReference(ctx, refs.roomID); err != nil {
		return err
	}
	if err := s.validateActivityGroupReference(ctx, refs.activityGroupID); err != nil {
		return err
	}
	if err := s.validateStaffReferences(ctx, refs.staffIDs, refs.createdByStaffID); err != nil {
		return err
	}
	return s.validateStudentReferences(ctx, date, refs.studentIDs)
}

// invalidReference reports a lookup failure as such, and a missing row
// (nil or not found) as an invalid reference.
func invalidReference(field string, err error) error {
	if err != nil && !modelBase.IsNoRows(err) {
		return fmt.Errorf("validate %s: %w", field, err)
	}
	return fmt.Errorf("%w: invalid %s", timetable.ErrInvalidInstanceReference, field)
}

func (s *InstanceLifecycleService) validateRoomReference(ctx context.Context, roomID int64) error {
	if roomID <= 0 {
		return fmt.Errorf("%w: invalid room_id", timetable.ErrInvalidInstanceReference)
	}
	if _, ok, err := s.deps.Rooms.RoomName(ctx, roomID); err != nil || !ok {
		return invalidReference("room_id", err)
	}
	return nil
}

func (s *InstanceLifecycleService) validateActivityGroupReference(ctx context.Context, activityGroupID *int64) error {
	if activityGroupID == nil {
		return nil
	}
	if *activityGroupID <= 0 {
		return fmt.Errorf("%w: invalid activity_group_id", timetable.ErrInvalidInstanceReference)
	}
	if group, err := s.deps.ActivityGroupRepo.FindByID(ctx, *activityGroupID); err != nil || group == nil {
		return invalidReference("activity_group_id", err)
	}
	return nil
}

func (s *InstanceLifecycleService) validateStaffReferences(ctx context.Context, staffIDs []int64, createdByStaffID *int64) error {
	uniqueStaffIDs := sliceutil.UniquePositive(staffIDs)
	if len(uniqueStaffIDs) > 0 {
		found, err := s.deps.StaffRepo.FindByIDs(ctx, uniqueStaffIDs)
		if err != nil {
			return fmt.Errorf("validate staff_ids: %w", err)
		}
		if len(found) != len(uniqueStaffIDs) {
			return fmt.Errorf("%w: invalid staff_ids", timetable.ErrInvalidInstanceReference)
		}
	}
	if createdByStaffID == nil {
		return nil
	}
	if *createdByStaffID <= 0 {
		return fmt.Errorf("%w: invalid created_by_staff_id", timetable.ErrInvalidInstanceReference)
	}
	if staff, err := s.deps.StaffRepo.FindByID(ctx, *createdByStaffID); err != nil || staff == nil {
		return invalidReference("created_by_staff_id", err)
	}
	return nil
}

// validateStudentReferences refuses unknown children and, for a roster that
// lives today or later, graduates and children whose care ended before the
// block. The directory read is unfiltered, and these manual paths write
// rosters directly: a form opened before a graduation and saved after it
// would otherwise recreate the rows the graduation archived (#405 review).
// The caller holds the grade-transition gate, so the status cannot flip.
// Past rosters are frozen history and stay editable.
func (s *InstanceLifecycleService) validateStudentReferences(ctx context.Context, date timezone.Date, studentIDs []int64) error {
	uniqueStudentIDs := sliceutil.UniquePositive(studentIDs)
	if len(uniqueStudentIDs) == 0 {
		return nil
	}
	found, err := s.deps.StudentRepo.FindByIDs(ctx, uniqueStudentIDs)
	if err != nil {
		return fmt.Errorf("validate student_ids: %w", err)
	}
	if len(found) != len(uniqueStudentIDs) {
		return fmt.Errorf("%w: invalid student_ids", timetable.ErrInvalidInstanceReference)
	}
	if date.Before(timezone.TodayDate()) {
		return nil
	}
	for _, id := range uniqueStudentIDs {
		if found[id].IsAlumnus() {
			return fmt.Errorf("%w: graduated student in student_ids", timetable.ErrInvalidInstanceReference)
		}
		// Checked against the block's date, not today: a child leaving at
		// the end of the month may still be planned onto next week (#2487).
		if found[id].CareEndedOn(date) {
			return fmt.Errorf("%w: student in student_ids has left the OGS", timetable.ErrInvalidInstanceReference)
		}
	}
	return nil
}
