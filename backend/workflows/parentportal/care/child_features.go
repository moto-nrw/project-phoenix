package care

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
)

// ChildFeatures resolves the parent-portal feature toggles for the child's
// tenant after verifying ownership.
func (s *Service) ChildFeatures(ctx context.Context, accountID, studentID int64) (ChildFeatureFlags, error) {
	child, err := s.ResolveOwnedChild(ctx, accountID, studentID)
	if err != nil {
		return ChildFeatureFlags{}, err
	}
	settings, err := s.resolveChildFeatureSettings(ctx, child.TenantID)
	if err != nil {
		return ChildFeatureFlags{}, err
	}

	// The child has left the OGS (#2487). Every WRITE capability goes off in
	// this one place rather than in each screen: the portal builds its buttons
	// from these flags, so a family sees a read-only profile instead of
	// affordances that would all end in the same 403. CareEnded travels
	// alongside so the portal can say why, and the read flags (meal plan, news)
	// stay untouched — what happened stays readable.
	if child.CareEnded {
		return ChildFeatureFlags{
			CareEnded:               true,
			SickRequiresApproval:    settings.sickApproval,
			ExcusedRequiresApproval: settings.excusedApproval,
			MealPlanEnabled:         settings.mealPlan,
			MealRegistrationEnabled: false,
			NewsEnabled:             settings.news,
		}, nil
	}

	return settings.activeChildFlags(child, s.hasOpenChangeRequest(ctx, child.TenantID, accountID, studentID)), nil
}

// activeChildFlags combines the tenant settings with the guardian's
// permissions for a child who is still in care.
func (f childFeatureSettings) activeChildFlags(child *Child, hasOpenChangeRequest bool) ChildFeatureFlags {
	canEditMasterData := f.masterEdit && child.HasPermission(authorize.GuardianPermissionMasterDataEdit)
	canManagePickup := child.HasPermission(authorize.GuardianPermissionPickupManage)
	canManageGuardianContacts := child.HasPermission(authorize.GuardianPermissionGuardianEdit)
	return ChildFeatureFlags{
		HasOpenChangeRequest:         hasOpenChangeRequest,
		SickNoteEnabled:              f.sick && f.sickReports && child.HasPermission(authorize.GuardianPermissionSickNoteSubmit),
		ExcusedNoteEnabled:           f.sick && f.excusedReports && child.HasPermission(authorize.GuardianPermissionSickNoteSubmit),
		SickRequiresApproval:         f.sickApproval,
		ExcusedRequiresApproval:      f.excusedApproval,
		NotesEnabled:                 f.notes && child.HasPermission(authorize.GuardianPermissionNotesWrite),
		RequestSubmitEnabled:         f.notes && child.HasPermission(authorize.GuardianPermissionRequestSubmit),
		PickupChangeEnabled:          f.pickupChange && canManagePickup,
		PickupChangeCutoffTime:       f.pickupCutoff.Clock,
		PickupChangeTodayClosed:      f.pickupCutoff.Closed(timezone.DateFromTime(f.pickupCutoff.Now)),
		PickupManageAllowed:          f.guardianManagement && canManagePickup,
		GuardianContactManageAllowed: f.guardianManagement && canManageGuardianContacts,
		RelatedAccountsInviteEnabled: f.inviteMode != configModels.ParentInviteModeDisabled,
		RelatedAccountsRemoveEnabled: f.canRemove && f.inviteMode != configModels.ParentInviteModeDisabled,
		MasterDataEditEnabled:        canEditMasterData,
		MasterDataContactEditEnabled: canEditMasterData && f.guardianManagement,
		MasterDataRequestEnabled:     f.masterRequest && child.HasPermission(authorize.GuardianPermissionMasterDataRequest),
		MealPlanEnabled:              f.mealPlan,
		MealRegistrationEnabled:      f.mealRegistration && child.HasPermission(authorize.GuardianPermissionMealParticipationManage),
		NewsEnabled:                  f.news,
		ReasonRequired:               usersSvc.ReasonRequiredFor(f.reasonPolicy, false),
	}
}

// hasOpenChangeRequest reports whether the child has a pending change request
// (care schedule OR master data) awaiting an OGS decision, so the parent
// overview can badge the Stammdaten entry. The lookups hit tenant-scoped/RLS
// tables, so they must run inside a tenant transaction — ChildFeatures is only
// parent-authenticated and carries no tenant context otherwise. Best-effort: a
// query error logs and yields false so a transient failure never shows a
// phantom badge.
func (s *Service) hasOpenChangeRequest(ctx context.Context, tenantID, accountID, studentID int64) bool {
	open := false
	err := InTenant(ctx, tenantID, func(txCtx context.Context) error {
		visibility, err := s.RequestSharing.LoadRequestShareVisibility(txCtx, studentID)
		if err != nil {
			return err
		}
		if s.hasOpenCareRequest(txCtx, visibility, accountID, studentID) {
			open = true
			return nil
		}
		open = s.hasOpenMasterDataRequest(txCtx, visibility, accountID, studentID)
		return nil
	})
	if err != nil {
		s.Logger.Warn("parent: open change-request check failed",
			slog.Int64("student_id", studentID),
			slog.String("error", err.Error()),
		)
		return false
	}
	return open
}

// hasOpenCareRequest reports a pending care-schedule request the account may
// see. A lookup error logs and yields false.
func (s *Service) hasOpenCareRequest(txCtx context.Context, visibility RequestShareVisibility, accountID, studentID int64) bool {
	if s.CareRequests == nil {
		return false
	}
	req, _, err := s.CareRequests.GetPendingForStudent(txCtx, studentID)
	if err != nil {
		s.Logger.Warn("parent: pending care-request check failed",
			slog.Int64("student_id", studentID),
			slog.String("error", err.Error()),
		)
		return false
	}
	return req != nil && visibility.Allows(RequestShareCareSchedule, req.ID, accountID, req.SubmittedBy)
}

// hasOpenMasterDataRequest reports a pending master-data request the account
// may see. A lookup error logs and yields false.
func (s *Service) hasOpenMasterDataRequest(txCtx context.Context, visibility RequestShareVisibility, accountID, studentID int64) bool {
	if s.ChangeRequestRepo == nil {
		return false
	}
	pending, err := s.ChangeRequestRepo.ListByStudent(txCtx, studentID, []string{usersModels.DataChangeStatusPending}, 0)
	if err != nil {
		s.Logger.Warn("parent: pending master-data check failed",
			slog.Int64("student_id", studentID),
			slog.String("error", err.Error()),
		)
		return false
	}
	for _, req := range pending {
		if req != nil && visibility.Allows(RequestShareMasterData, req.ID, accountID, req.SubmittedBy) {
			return true
		}
	}
	return false
}
