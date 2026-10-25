package care

import (
	"context"
	"errors"
	"fmt"
)

// ErrPickupPermissionContactGone refuses an approval whose contact no longer
// qualifies for a parent-issued pickup permission.
var ErrPickupPermissionContactGone = errors.New("parent: pickup permission contact is no longer eligible")

// ErrPickupPermissionChanged refuses an approval whose stored permission no
// longer matches the state the parent saw when filing the request.
var ErrPickupPermissionChanged = errors.New("parent: pickup permission changed since request")

// PickupPermissionApproval is the frozen permission state that an approved
// request may grant.
type PickupPermissionApproval struct {
	TenantID          int64
	StudentID         int64
	GuardianProfileID int64
	RelationshipID    int64
	Weekdays          []int
	Expected          *PickupPermissionState
}

// PickupPermissionState is the safety-relevant portion of a pickup
// permission. A nil weekday list means that the permission applies every day.
type PickupPermissionState struct {
	CanPickup bool
	Weekdays  []int
}

// PickupPermissionChange is Care Plan's pickup-permission command in the
// workflow's vocabulary. The root translates it to the target owner's public
// capability.
type PickupPermissionChange struct {
	TenantID       int64
	RelationshipID int64
	Weekdays       []int
	Expected       *PickupPermissionState
}

// PickupPermissionWrite applies the target owner's command in the ambient
// tenant transaction after the workflow has locked and rechecked the contact.
type PickupPermissionWrite interface {
	ChangePickupPermission(context.Context, PickupPermissionChange) (bool, error)
}

// PickupPermissionApprovals coordinates the live People Directory eligibility
// check with the target Care Plan command. Both commands share the request
// review transaction and the profile/relationship locks cover the subsequent
// write.
type PickupPermissionApprovals struct {
	GuardianProfiles      GuardianProfileReads
	StudentGuardians      StudentGuardianReads
	GuardianPolicy        GuardianPolicy
	PickupPermissionWrite PickupPermissionWrite
}

func (s *PickupPermissionApprovals) ApplyPickupPermission(ctx context.Context, input PickupPermissionApproval) error {
	if input.Expected == nil {
		return ErrPickupPermissionChanged
	}
	link, err := s.pickupPermissionTarget(ctx, input.StudentID, input.GuardianProfileID)
	if err != nil {
		return err
	}
	if link.ID != input.RelationshipID {
		return ErrPickupPermissionContactGone
	}
	found, err := s.PickupPermissionWrite.ChangePickupPermission(ctx, PickupPermissionChange{
		TenantID: input.TenantID, RelationshipID: link.ID, Weekdays: input.Weekdays, Expected: input.Expected,
	})
	if err != nil {
		return err
	}
	if !found {
		return ErrPickupPermissionChanged
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
			return nil, ErrPickupPermissionContactGone
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
		return nil, ErrPickupPermissionContactGone
	}
	return link, nil
}

func pickupPermissionContactError(err error) error {
	if err == nil || errors.Is(err, ErrGuardianNotLinked) || errors.Is(err, ErrGuardianProfileNotFound) {
		return ErrPickupPermissionContactGone
	}
	return fmt.Errorf("parent: read pickup permission contact: %w", err)
}
