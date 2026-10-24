package care

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
)

// Pickup permission requests (#4780). A parent who may manage the pickup
// (parent_portal.pickup.manage) asks the OGS to let a contact person collect
// the child on every day or only on some weekdays, and signs the request.
// The OGS confirms it under /anfragen; Care Plan then writes the permission.
// The same eligibility as the direct pickup flag applies: only contacts
// without their own portal account, never a social worker, never a full
// guardian of another family.

// PickupPermissionInput is one pickup permission request. Weekdays empty or
// all five means every day.
type PickupPermissionInput struct {
	Weekdays  []int
	Reason    string
	Signature *PickupSignatureInput
}

// SubmitPickupPermissionRequest records a signed pickup permission request
// for one contact person of the child.
func (s *Service) SubmitPickupPermissionRequest(ctx context.Context, accountID, studentID, guardianProfileID int64, input PickupPermissionInput) (*carerequests.Request, error) {
	weekdays, err := carerequests.NormalizeWeekdays(input.Weekdays)
	if err != nil {
		return nil, ErrInvalidCareRequestPayload
	}
	if input.Signature == nil {
		return nil, ErrPickupSignatureRequired
	}
	child, err := s.ResolvePermittedChild(ctx, accountID, studentID, GuardianPermissionPickupManage)
	if err != nil {
		return nil, err
	}
	if err := child.RequireCareRunning(); err != nil {
		return nil, err
	}
	if err := s.requireGuardianManagementEnabled(ctx, child.TenantID); err != nil {
		return nil, err
	}
	if s.CareRequests == nil || s.Guardians == nil {
		return nil, errors.New("parent: pickup permission requests are not configured")
	}
	var result *carerequests.Request
	txErr := s.Runtime.WithinTenant(ctx, child.TenantID, func(txCtx context.Context) error {
		if err := s.RequireCareRunningForUpdate(txCtx, studentID); err != nil {
			return err
		}
		created, err := s.createPickupPermissionRequestInTx(txCtx, child, accountID, studentID, guardianProfileID, weekdays, input)
		result = created
		return err
	})
	if txErr != nil {
		return nil, MapCareRequestError(txErr, "submit pickup permission request")
	}
	s.Logger.Info("parent requested pickup permission",
		slog.Int64("account_id", accountID),
		slog.Int64("student_id", studentID),
		slog.Int64("guardian_profile_id", guardianProfileID),
		slog.Int64("tenant_id", child.TenantID),
		slog.Int64("request_id", result.ID),
	)
	return result, nil
}

// createPickupPermissionRequestInTx locks the contact, checks it may receive
// a pickup permission from this parent, stores the signature and the request.
// The caller holds the child's lock.
func (s *Service) createPickupPermissionRequestInTx(txCtx context.Context, child *Child, accountID, studentID, guardianProfileID int64, weekdays []int, input PickupPermissionInput) (*carerequests.Request, error) {
	if err := s.GuardianProfileRepo.LockByIDForUpdate(txCtx, guardianProfileID); err != nil {
		if errors.Is(err, ErrGuardianProfileNotFound) {
			return nil, ErrGuardianNotLinked
		}
		return nil, err
	}
	link, err := s.findChildGuardianLinkForUpdate(txCtx, studentID, guardianProfileID)
	if err != nil {
		return nil, err
	}
	profile, err := s.GuardianProfileRepo.FindByID(txCtx, guardianProfileID)
	if err != nil {
		if errors.Is(err, ErrGuardianProfileNotFound) {
			return nil, ErrGuardianNotLinked
		}
		return nil, err
	}
	if err := s.refusePickupPermissionTarget(profile, link, accountID); err != nil {
		return nil, err
	}
	if link.CanPickup && slices.Equal(normalizedWeekdays(link.PickupWeekdays), weekdays) {
		return nil, ErrPickupPermissionUnchanged
	}
	name := strings.TrimSpace(profile.FirstName + " " + profile.LastName)
	statement := PickupPermissionStatement(child.StudentName, name, weekdays)
	signed, err := s.recordPickupSignature(txCtx, studentID, accountID, statement, input.Signature)
	if err != nil {
		return nil, err
	}
	return s.CareRequests.CreatePickupPermission(txCtx, carerequests.PickupPermissionCreateInput{
		StudentID: studentID, GuardianAccountID: accountID,
		GuardianProfileID: guardianProfileID, RelationshipID: link.ID, PersonName: name,
		Weekdays: weekdays, Reason: input.Reason, PreviousLabel: previousPickupPermissionLabel(link),
		Signature: *signed,
	})
}

// refusePickupPermissionTarget applies the eligibility of the direct pickup
// flag (#1667): no account holder (also not the caller), no social worker,
// no full guardian.
func (s *Service) refusePickupPermissionTarget(profile *GuardianProfile, link *GuardianRelationship, accountID int64) error {
	if err := refusePickupFlagEdit(profile, link); err != nil {
		return err
	}
	isSelf := profile.AccountID != nil && *profile.AccountID == accountID
	if !isSelf && s.GuardianPolicy.IsFullRole(link.GuardianRole) {
		return ErrGuardianRoleManaged
	}
	return nil
}

// normalizedWeekdays reads a stored weekday limit the way a request states
// it: nil for every day.
func normalizedWeekdays(days []int) []int {
	normalized, err := carerequests.NormalizeWeekdays(days)
	if err != nil {
		return days
	}
	return normalized
}

// previousPickupPermissionLabel names the person's pickup permission before
// the request, in the review's words.
func previousPickupPermissionLabel(link *GuardianRelationship) string {
	if !link.CanPickup {
		return "Nicht abholberechtigt"
	}
	return carerequests.WeekdayListLabel(normalizedWeekdays(link.PickupWeekdays))
}

// ListPickupPermissionRequests lists the child's pickup permission requests
// of the last two months the caller may see: their own, and those shared with
// them.
func (s *Service) ListPickupPermissionRequests(ctx context.Context, accountID, studentID int64) ([]carerequests.Request, error) {
	child, err := s.ResolveOwnedChild(ctx, accountID, studentID)
	if err != nil {
		return nil, err
	}
	if s.CareRequests == nil {
		return nil, errors.New("parent: pickup permission requests are not configured")
	}
	var rows []carerequests.Request
	err = s.Runtime.WithinTenant(ctx, child.TenantID, func(txCtx context.Context) error {
		var listErr error
		rows, listErr = s.CareRequests.ListPickupPermissionRequests(txCtx, studentID, s.now().AddDate(0, -2, 0))
		if listErr != nil {
			return listErr
		}
		visibility, visibilityErr := s.RequestSharing.LoadRequestShareVisibility(txCtx, studentID)
		if visibilityErr != nil {
			return visibilityErr
		}
		visible := rows[:0]
		for _, row := range rows {
			if visibility.Allows(RequestSharePickupChange, row.ID, accountID, row.SubmittedBy) {
				visible = append(visible, row)
			}
		}
		rows = visible
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("parent: list pickup permission requests: %w", err)
	}
	return rows, nil
}
