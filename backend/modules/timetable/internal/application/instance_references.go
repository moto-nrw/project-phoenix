package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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
// tenant and returns the block type of the linked template ("" without one).
// date is the date the rows will LIVE on (the target of a move): it decides
// whether a graduated child is still refused.
func (s *InstanceLifecycleService) validateInstanceReferences(ctx context.Context, date calendar.Date, refs instanceReferences) (string, error) {
	groupType, err := s.validateActivityGroupReference(ctx, refs.activityGroupID)
	if err != nil {
		return "", err
	}
	// A duty occurrence (#3822) may lack a room and never carries children;
	// every other block needs a room.
	isDuty := groupType == timetable.GroupTypeDuty
	if isDuty && len(domain.UniquePositiveIDs(refs.studentIDs)) > 0 {
		return "", fmt.Errorf("%w: a duty has no children", timetable.ErrInvalidInstanceReference)
	}
	if err := s.validateRoomReference(ctx, refs.roomID, isDuty); err != nil {
		return "", err
	}
	if err := s.validateStaffReferences(ctx, refs.staffIDs, refs.createdByStaffID); err != nil {
		return "", err
	}
	return groupType, s.validateStudentReferences(ctx, date, refs.studentIDs)
}

// invalidReference reports a lookup failure as such, and a missing row
// (nil or not found) as an invalid reference.
func invalidReference(field string, err error) error {
	if err != nil && !domain.IsRecordNotFound(err) {
		return fmt.Errorf("validate %s: %w", field, err)
	}
	return fmt.Errorf("%w: invalid %s", timetable.ErrInvalidInstanceReference, field)
}

func (s *InstanceLifecycleService) validateRoomReference(ctx context.Context, roomID int64, roomOptional bool) error {
	if roomID == 0 && roomOptional {
		return nil
	}
	if roomID <= 0 {
		return fmt.Errorf("%w: invalid room_id", timetable.ErrInvalidInstanceReference)
	}
	if _, ok, err := s.deps.Rooms.RoomName(ctx, roomID); err != nil || !ok {
		return invalidReference("room_id", err)
	}
	return nil
}

// validateActivityGroupReference checks the optional template link and
// returns the linked block type ("" without a link).
func (s *InstanceLifecycleService) validateActivityGroupReference(ctx context.Context, activityGroupID *int64) (string, error) {
	if activityGroupID == nil {
		return "", nil
	}
	if *activityGroupID <= 0 {
		return "", fmt.Errorf("%w: invalid activity_group_id", timetable.ErrInvalidInstanceReference)
	}
	group, err := findLegacyGroup(ctx, s.deps.ActivityGroupRepo, *activityGroupID)
	if err != nil || group == nil {
		return "", invalidReference("activity_group_id", err)
	}
	return group.Type, nil
}

func (s *InstanceLifecycleService) validateStaffReferences(ctx context.Context, staffIDs []int64, createdByStaffID *int64) error {
	if err := s.validatePlannableStaffReferences(ctx, staffIDs); err != nil {
		return err
	}
	return s.validateCreatedByStaffReference(ctx, createdByStaffID)
}

// validatePlannableStaffReferences accepts every staff member of the school,
// external caregivers included (#4573): they are planned wherever staff is
// planned. Whether a new external may be chosen is the pickers' rule (D4),
// so an existing assignment of an accountless external stays editable.
func (s *InstanceLifecycleService) validatePlannableStaffReferences(ctx context.Context, staffIDs []int64) error {
	uniqueStaffIDs := domain.UniquePositiveIDs(staffIDs)
	if len(uniqueStaffIDs) == 0 {
		return nil
	}
	found, err := s.deps.StaffRepo.FindByIDs(ctx, uniqueStaffIDs)
	if err != nil {
		return fmt.Errorf("validate staff_ids: %w", err)
	}
	if len(found) != len(uniqueStaffIDs) {
		return fmt.Errorf("%w: invalid staff_ids", timetable.ErrInvalidInstanceReference)
	}
	for _, staffID := range uniqueStaffIDs {
		if found[staffID] == nil {
			return fmt.Errorf("%w: invalid staff_ids", timetable.ErrInvalidInstanceReference)
		}
	}
	return nil
}

func (s *InstanceLifecycleService) validateCreatedByStaffReference(ctx context.Context, createdByStaffID *int64) error {
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
func (s *InstanceLifecycleService) validateStudentReferences(ctx context.Context, date calendar.Date, studentIDs []int64) error {
	uniqueStudentIDs := domain.UniquePositiveIDs(studentIDs)
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
	if date.Before(calendar.TodayDate()) {
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
