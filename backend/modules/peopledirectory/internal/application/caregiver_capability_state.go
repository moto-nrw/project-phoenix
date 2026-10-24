package application

import (
	"context"
	"fmt"
	"strings"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain/userscontract"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type caregiverRoleFlags struct {
	hasAdminRole         bool
	hasUserRole          bool
	hasLegacyTeacherRole bool
	hasOtherUsableRole   bool
}

func caregiverRoleFlagsOf(roles []string) caregiverRoleFlags {
	roleFlags := caregiverRoleFlags{}
	for _, role := range roles {
		switch strings.ToLower(strings.TrimSpace(role)) {
		case "admin":
			roleFlags.hasAdminRole = true
			roleFlags.hasOtherUsableRole = true
		case "user":
			roleFlags.hasUserRole = true
		case "teacher":
			roleFlags.hasLegacyTeacherRole = true
		default:
			roleFlags.hasOtherUsableRole = true
		}
	}
	return roleFlags
}

func (s *caregiverCapabilityService) loadCapabilityStateWithRoleFlags(
	ctx context.Context,
	accountID int64,
) (*peopleValues.CaregiverCapabilityState, caregiverRoleFlags, error) {
	account, tenantID, err := s.loadAccountAndTenant(ctx, accountID)
	if err != nil {
		return nil, caregiverRoleFlags{}, err
	}

	state := &peopleValues.CaregiverCapabilityState{
		AccountID: account.ID,
		Email:     account.Email,
	}

	roles, err := s.Identity.ListSchoolAccountRoleNames(ctx, accountID)
	if err != nil {
		return nil, caregiverRoleFlags{}, err
	}
	roleFlags := caregiverRoleFlagsOf(roles)
	state.HasAdminRole = roleFlags.hasAdminRole
	state.HasUserRole = roleFlags.hasUserRole

	if err := s.loadCaregiverProfile(ctx, accountID, state); err != nil {
		return nil, caregiverRoleFlags{}, err
	}

	state.HasCaregiverProfile = state.HasPerson && state.HasStaff && state.HasTeacher
	state.IsActiveCaregiver = state.HasUserRole && state.HasCaregiverProfile

	blockers, err := s.listDisableBlockers(
		ctx,
		tenantID,
		state,
		roleFlags.hasLegacyTeacherRole,
		roleFlags.hasOtherUsableRole,
	)
	if err != nil {
		return nil, caregiverRoleFlags{}, err
	}
	state.DisableBlockers = blockers
	state.DisableBlocked = len(blockers) > 0

	return state, roleFlags, nil
}

// loadCaregiverProfile fills the person, staff and teacher facts of state.
func (s *caregiverCapabilityService) loadCaregiverProfile(
	ctx context.Context,
	accountID int64,
	state *peopleValues.CaregiverCapabilityState,
) error {
	person, err := s.PersonRepo.FindByAccountID(ctx, accountID)
	if err != nil || person == nil {
		return err
	}
	state.HasPerson = true
	state.PersonID = &person.ID
	state.FirstName = person.FirstName
	state.LastName = person.LastName

	staff, err := s.findStaffByPersonID(ctx, person.ID)
	if err != nil || staff == nil {
		return err
	}
	state.HasStaff = true
	state.StaffID = &staff.ID

	teacher, err := s.TeacherRepo.FindByStaffID(ctx, staff.ID)
	if err != nil || teacher == nil {
		return err
	}
	state.HasTeacher = true
	state.TeacherID = &teacher.ID
	return nil
}

func (s *caregiverCapabilityService) loadAccountAndTenant(
	ctx context.Context,
	accountID int64,
) (*peoplePorts.CaregiverAccount, int64, error) {
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return nil, 0, &userscontract.UsersError{Op: "caregiver capability", Err: fmt.Errorf("tenant context is required")}
	}

	account, err := s.Identity.FindCaregiverAccount(ctx, accountID)
	if err != nil {
		return nil, 0, err
	}
	if account == nil {
		return nil, 0, userscontract.ErrAccountNotFound
	}

	exists, err := s.Identity.HasActiveSchoolMembership(ctx, accountID)
	if err != nil {
		return nil, 0, err
	}
	if !exists {
		return nil, 0, &peopleValues.AccountNotAssignedToTenantError{AccountID: accountID, TenantID: tenantID}
	}

	return account, tenantID, nil
}

func (s *caregiverCapabilityService) listDisableBlockers(
	ctx context.Context,
	tenantID int64,
	state *peopleValues.CaregiverCapabilityState,
	hasLegacyTeacherRole bool,
	hasOtherUsableRole bool,
) ([]peopleValues.CaregiverCapabilityBlockerCode, error) {
	if state == nil {
		return nil, nil
	}

	var blockers []peopleValues.CaregiverCapabilityBlockerCode

	if (state.HasUserRole || hasLegacyTeacherRole) &&
		!state.HasAdminRole &&
		!hasOtherUsableRole {
		blockers = append(blockers, peopleValues.CaregiverCapabilityBlockerMissingUsableRole)
	}

	if state.StaffID != nil {
		staffBlockers, err := s.listStaffBlockers(ctx, *state.StaffID, tenantID, state)
		if err != nil {
			return nil, err
		}
		blockers = append(blockers, staffBlockers...)
	}

	if state.TeacherID != nil {
		groups, err := s.GroupTeacherRepo.ListGroupTeacherBlockers(ctx, *state.TeacherID, tenantID)
		if err != nil {
			return nil, err
		}
		if len(groups) > 0 {
			state.GroupAssignments = groups
			blockers = append(blockers, peopleValues.CaregiverCapabilityBlockerGroupAssignments)
		}
	}

	return blockers, nil
}

// listStaffBlockers fills the supervision, handover and planned-activity
// bindings of the staff member into state and returns their blocker codes.
func (s *caregiverCapabilityService) listStaffBlockers(
	ctx context.Context,
	staffID int64,
	tenantID int64,
	state *peopleValues.CaregiverCapabilityState,
) ([]peopleValues.CaregiverCapabilityBlockerCode, error) {
	var blockers []peopleValues.CaregiverCapabilityBlockerCode

	supervisions, err := s.GroupSupervisorRepo.ListActiveSupervisionBlockers(ctx, staffID)
	if err != nil {
		return nil, err
	}
	if len(supervisions) > 0 {
		state.ActiveSupervisions = supervisions
		blockers = append(blockers, peopleValues.CaregiverCapabilityBlockerActiveGroupSupervisions)
	}

	substitutions, err := s.GroupSubstitutionRepo.ListActiveSubstitutionBlockers(ctx, staffID, tenantID)
	if err != nil {
		return nil, err
	}
	if len(substitutions) > 0 {
		state.ActiveSubstitutions = substitutions
		blockers = append(blockers, peopleValues.CaregiverCapabilityBlockerActiveGroupSubstitutions)
	}

	activities, err := s.ActivitySupervisorRepo.ListPlannedSupervisionBlockers(ctx, staffID, tenantID)
	if err != nil {
		return nil, err
	}
	if len(activities) > 0 {
		state.ActivitySupervisions = activities
		blockers = append(blockers, peopleValues.CaregiverCapabilityBlockerActivitySupervisions)
	}

	return blockers, nil
}
