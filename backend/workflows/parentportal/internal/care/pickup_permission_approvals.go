package care

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
)

// PickupPermissionApprovals coordinates the live People Directory eligibility
// check with Care Plan's pickup-permission write. Both owner commands join the
// request-review transaction, and the profile/relationship locks cover the
// check through the subsequent write.
type PickupPermissionApprovals struct {
	GuardianProfiles      GuardianProfileReads
	StudentGuardians      StudentGuardianReads
	GuardianPolicy        GuardianPolicy
	PickupPermissionWrite careplan.GuardianPickupPermissions
}

var _ carerequests.PickupPermissionApprovals = (*PickupPermissionApprovals)(nil)

func (s *PickupPermissionApprovals) ApplyPickupPermission(ctx context.Context, input carerequests.PickupPermissionApproval) error {
	link, err := s.pickupPermissionTarget(ctx, input.StudentID, input.GuardianProfileID)
	if err != nil {
		return err
	}
	if link.ID != input.RelationshipID {
		return carerequests.ErrPickupPermissionContactGone
	}
	grant := true
	found, err := s.PickupPermissionWrite.ChangeGuardianPickupPermission(ctx, careplan.GuardianPickupPermissionChange{
		TenantID: input.TenantID, RelationshipID: link.ID,
		CanPickup: &grant, SetWeekdays: true, Weekdays: input.Weekdays,
	})
	if err != nil {
		return err
	}
	if !found {
		return carerequests.ErrPickupPermissionContactGone
	}
	return nil
}

// ValidatePickupPerson verifies a stored one-day collector while holding the
// People Directory rows. The caller writes its dated Care Plan note in the
// same tenant transaction after this method returns.
func (s *PickupPermissionApprovals) ValidatePickupPerson(ctx context.Context, studentID, guardianProfileID int64) error {
	_, err := s.pickupPermissionTarget(ctx, studentID, guardianProfileID)
	return err
}

func (s *PickupPermissionApprovals) pickupPermissionTarget(ctx context.Context, studentID, guardianProfileID int64) (*GuardianRelationship, error) {
	if err := s.GuardianProfiles.LockByIDForUpdate(ctx, guardianProfileID); err != nil {
		if errors.Is(err, ErrGuardianProfileNotFound) {
			return nil, carerequests.ErrPickupPermissionContactGone
		}
		return nil, fmt.Errorf("parent: lock pickup permission contact: %w", err)
	}
	link, err := s.StudentGuardians.FindByStudentAndGuardianForUpdate(ctx, studentID, guardianProfileID)
	if err != nil || link == nil {
		return nil, pickupPermissionContactError(err)
	}
	profile, err := s.GuardianProfiles.FindByID(ctx, guardianProfileID)
	if err != nil || profile == nil {
		return nil, pickupPermissionContactError(err)
	}
	if err := refusePickupPermissionTarget(profile, link, s.GuardianPolicy); err != nil {
		return nil, carerequests.ErrPickupPermissionContactGone
	}
	return link, nil
}

func pickupPermissionContactError(err error) error {
	if err == nil || errors.Is(err, ErrGuardianNotLinked) || errors.Is(err, ErrGuardianProfileNotFound) {
		return carerequests.ErrPickupPermissionContactGone
	}
	return fmt.Errorf("parent: read pickup permission contact: %w", err)
}
