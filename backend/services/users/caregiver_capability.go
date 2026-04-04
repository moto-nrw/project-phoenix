package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	authSvc "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// CaregiverCapabilityServiceDependencies contains the repositories and services
// required to manage caregiver capability on existing accounts.
type CaregiverCapabilityServiceDependencies struct {
	AccountRepo            authModels.AccountRepository
	AccountTenantRepo      authModels.AccountTenantRepository
	RoleRepo               authModels.RoleRepository
	PersonRepo             userModels.PersonRepository
	StaffRepo              userModels.StaffRepository
	TeacherRepo            userModels.TeacherRepository
	GroupTeacherRepo       educationModels.GroupTeacherRepository
	GroupSubstitutionRepo  educationModels.GroupSubstitutionRepository
	ActivitySupervisorRepo activitiesModels.SupervisorPlannedRepository
	AuthService            authSvc.AuthService
	DB                     *bun.DB
}

type caregiverCapabilityService struct {
	accountRepo            authModels.AccountRepository
	accountTenantRepo      authModels.AccountTenantRepository
	roleRepo               authModels.RoleRepository
	personRepo             userModels.PersonRepository
	staffRepo              userModels.StaffRepository
	teacherRepo            userModels.TeacherRepository
	groupTeacherRepo       educationModels.GroupTeacherRepository
	groupSubstitutionRepo  educationModels.GroupSubstitutionRepository
	activitySupervisorRepo activitiesModels.SupervisorPlannedRepository
	authService            authSvc.AuthService
	txHandler              *modelBase.TxHandler
	db                     *bun.DB
}

// NewCaregiverCapabilityService creates a tenant-scoped caregiver capability service.
func NewCaregiverCapabilityService(
	deps CaregiverCapabilityServiceDependencies,
) CaregiverCapabilityService {
	return &caregiverCapabilityService{
		accountRepo:            deps.AccountRepo,
		accountTenantRepo:      deps.AccountTenantRepo,
		roleRepo:               deps.RoleRepo,
		personRepo:             deps.PersonRepo,
		staffRepo:              deps.StaffRepo,
		teacherRepo:            deps.TeacherRepo,
		groupTeacherRepo:       deps.GroupTeacherRepo,
		groupSubstitutionRepo:  deps.GroupSubstitutionRepo,
		activitySupervisorRepo: deps.ActivitySupervisorRepo,
		authService:            deps.AuthService,
		txHandler:              modelBase.NewTxHandler(deps.DB),
		db:                     deps.DB,
	}
}

func (s *caregiverCapabilityService) GetCaregiverCapability(
	ctx context.Context,
	accountID int64,
) (*userModels.CaregiverCapabilityState, error) {
	return s.loadCapabilityState(ctx, accountID)
}

func (s *caregiverCapabilityService) EnableCaregiverCapability(
	ctx context.Context,
	accountID int64,
	input userModels.EnableCaregiverCapabilityInput,
) (*userModels.CaregiverCapabilityState, error) {
	if accountID <= 0 {
		return nil, &UsersError{Op: "enable caregiver capability", Err: fmt.Errorf("account ID is required")}
	}

	input.FirstName = strings.TrimSpace(input.FirstName)
	input.LastName = strings.TrimSpace(input.LastName)
	input.Position = strings.TrimSpace(input.Position)

	if err := s.txHandler.RunInTx(ctx, func(txCtx context.Context, _ bun.Tx) error {
		account, _, err := s.loadAccountAndTenant(txCtx, accountID)
		if err != nil {
			return err
		}

		person, err := s.personRepo.FindByAccountID(txCtx, accountID)
		if err != nil {
			return err
		}
		if person == nil {
			if input.FirstName == "" || input.LastName == "" {
				return &UsersError{
					Op:  "enable caregiver capability",
					Err: fmt.Errorf("first_name and last_name are required when the account has no person profile"),
				}
			}

			person = &userModels.Person{
				FirstName: input.FirstName,
				LastName:  input.LastName,
			}
			person.SetTenantID(tenant.FromContext(txCtx))
			if err := s.personRepo.Create(txCtx, person); err != nil {
				return err
			}
			if err := s.personRepo.LinkToAccount(txCtx, person.ID, account.ID); err != nil {
				return err
			}
		}

		staff, err := s.findStaffByPersonID(txCtx, person.ID)
		if err != nil {
			return err
		}
		if staff == nil {
			staff = &userModels.Staff{PersonID: person.ID}
			staff.SetTenantID(tenant.FromContext(txCtx))
			if err := s.staffRepo.Create(txCtx, staff); err != nil {
				return err
			}
		}

		teacher, err := s.teacherRepo.FindByStaffID(txCtx, staff.ID)
		if err != nil {
			return err
		}
		if teacher == nil {
			teacher = &userModels.Teacher{StaffID: staff.ID}
			teacher.SetTenantID(tenant.FromContext(txCtx))
			if input.Position != "" {
				teacher.Role = input.Position
			}
			if err := s.teacherRepo.Create(txCtx, teacher); err != nil {
				return err
			}
		}

		userRole, err := s.resolveSystemRoleByName(txCtx, "user")
		if err != nil {
			return err
		}
		if userRole == nil {
			return &UsersError{Op: "enable caregiver capability", Err: fmt.Errorf("user role not found")}
		}

		if err := s.authService.AssignRoleToAccount(txCtx, int(accountID), int(userRole.ID)); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return s.loadCapabilityState(ctx, accountID)
}

func (s *caregiverCapabilityService) DisableCaregiverCapability(
	ctx context.Context,
	accountID int64,
) (*userModels.CaregiverCapabilityState, error) {
	if accountID <= 0 {
		return nil, &UsersError{Op: "disable caregiver capability", Err: fmt.Errorf("account ID is required")}
	}

	state, err := s.loadCapabilityState(ctx, accountID)
	if err != nil {
		return nil, err
	}

	if len(state.DisableBlockers) > 0 {
		return nil, &CaregiverCapabilityBlockedError{Reasons: state.DisableBlockers}
	}

	if !state.HasUserRole {
		return state, nil
	}

	if err := s.txHandler.RunInTx(ctx, func(txCtx context.Context, _ bun.Tx) error {
		userRole, err := s.resolveSystemRoleByName(txCtx, "user")
		if err != nil {
			return err
		}
		if userRole == nil {
			return &UsersError{Op: "disable caregiver capability", Err: fmt.Errorf("user role not found")}
		}
		return s.authService.RemoveRoleFromAccount(txCtx, int(accountID), int(userRole.ID))
	}); err != nil {
		return nil, err
	}

	return s.loadCapabilityState(ctx, accountID)
}

func (s *caregiverCapabilityService) loadCapabilityState(
	ctx context.Context,
	accountID int64,
) (*userModels.CaregiverCapabilityState, error) {
	account, tenantID, err := s.loadAccountAndTenant(ctx, accountID)
	if err != nil {
		return nil, err
	}

	state := &userModels.CaregiverCapabilityState{
		AccountID: account.ID,
		Email:     account.Email,
	}

	roles, err := s.roleRepo.FindByAccountID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	for _, role := range roles {
		if role == nil {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(role.Name)) {
		case "admin":
			state.HasAdminRole = true
		case "user":
			state.HasUserRole = true
		}
	}

	person, err := s.personRepo.FindByAccountID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if person != nil {
		state.HasPerson = true
		state.PersonID = &person.ID
		state.FirstName = person.FirstName
		state.LastName = person.LastName
	}

	var staff *userModels.Staff
	if person != nil {
		staff, err = s.findStaffByPersonID(ctx, person.ID)
		if err != nil {
			return nil, err
		}
	}
	if staff != nil {
		state.HasStaff = true
		state.StaffID = &staff.ID
	}

	var teacher *userModels.Teacher
	if staff != nil {
		teacher, err = s.teacherRepo.FindByStaffID(ctx, staff.ID)
		if err != nil {
			return nil, err
		}
	}
	if teacher != nil {
		state.HasTeacher = true
		state.TeacherID = &teacher.ID
	}

	state.HasCaregiverProfile = state.HasPerson && state.HasStaff && state.HasTeacher
	state.IsActiveCaregiver = state.HasUserRole && state.HasCaregiverProfile

	blockers, err := s.listDisableBlockers(ctx, tenantID, state)
	if err != nil {
		return nil, err
	}
	state.DisableBlockers = blockers
	state.DisableBlockersCount = len(blockers)
	state.DisableBlocked = len(blockers) > 0

	return state, nil
}

func (s *caregiverCapabilityService) loadAccountAndTenant(
	ctx context.Context,
	accountID int64,
) (*authModels.Account, int64, error) {
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return nil, 0, &UsersError{Op: "caregiver capability", Err: fmt.Errorf("tenant context is required")}
	}

	account, err := s.accountRepo.FindByID(ctx, accountID)
	if err != nil {
		return nil, 0, err
	}
	if account == nil {
		return nil, 0, authSvc.ErrAccountNotFound
	}

	exists, err := s.accountTenantRepo.ExistsByAccountAndTenant(ctx, accountID, tenantID)
	if err != nil {
		return nil, 0, err
	}
	if !exists {
		return nil, 0, &AccountNotAssignedToTenantError{AccountID: accountID, TenantID: tenantID}
	}

	return account, tenantID, nil
}

func (s *caregiverCapabilityService) findStaffByPersonID(
	ctx context.Context,
	personID int64,
) (*userModels.Staff, error) {
	staff, err := s.staffRepo.FindByPersonID(ctx, personID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return staff, nil
}

func (s *caregiverCapabilityService) resolveSystemRoleByName(
	ctx context.Context,
	name string,
) (*authModels.Role, error) {
	roles, err := s.roleRepo.List(ctx, map[string]interface{}{
		"name":      strings.TrimSpace(strings.ToLower(name)),
		"is_system": true,
	})
	if err != nil {
		return nil, err
	}
	for _, role := range roles {
		if role == nil {
			continue
		}
		if role.TenantID == nil && role.IsSystem && strings.EqualFold(role.Name, name) {
			return role, nil
		}
	}
	return nil, nil
}

func (s *caregiverCapabilityService) listDisableBlockers(
	ctx context.Context,
	tenantID int64,
	state *userModels.CaregiverCapabilityState,
) ([]string, error) {
	if state == nil {
		return nil, nil
	}

	var blockers []string

	if state.HasUserRole && !state.HasAdminRole {
		blockers = append(blockers, "Das Konto hat keine Verwaltungsrolle und würde ohne Betreuerfähigkeit keine nutzbare Systemrolle behalten.")
	}

	if state.StaffID != nil {
		activeSupervisions, err := s.countActiveGroupSupervisions(ctx, *state.StaffID, tenantID)
		if err != nil {
			return nil, err
		}
		if activeSupervisions > 0 {
			blockers = append(blockers, fmt.Sprintf("Es bestehen noch %d aktive Gruppenaufsichten.", activeSupervisions))
		}

		activeSubstitutions, err := s.countActiveGroupSubstitutions(ctx, *state.StaffID, tenantID)
		if err != nil {
			return nil, err
		}
		if activeSubstitutions > 0 {
			blockers = append(blockers, fmt.Sprintf("Es bestehen noch %d aktive Vertretungen oder Gruppenübergaben.", activeSubstitutions))
		}

		plannedActivities, err := s.countPlannedActivitySupervisions(ctx, *state.StaffID, tenantID)
		if err != nil {
			return nil, err
		}
		if plannedActivities > 0 {
			blockers = append(blockers, fmt.Sprintf("Es bestehen noch %d Aktivitätsleitungen.", plannedActivities))
		}
	}

	if state.TeacherID != nil {
		groupAssignments, err := s.countGroupTeacherAssignments(ctx, *state.TeacherID, tenantID)
		if err != nil {
			return nil, err
		}
		if groupAssignments > 0 {
			blockers = append(blockers, fmt.Sprintf("Es bestehen noch %d Stammgruppen-Zuordnungen.", groupAssignments))
		}
	}

	return blockers, nil
}

func (s *caregiverCapabilityService) countActiveGroupSupervisions(
	ctx context.Context,
	staffID int64,
	tenantID int64,
) (int, error) {
	return s.queryScalarCount(ctx, `
		SELECT COUNT(*)
		FROM active.group_supervisors AS gs
		WHERE gs.tenant_id = ?
		  AND gs.staff_id = ?
		  AND gs.start_date <= CURRENT_DATE
		  AND (gs.end_date IS NULL OR gs.end_date >= CURRENT_DATE)
	`, tenantID, staffID)
}

func (s *caregiverCapabilityService) countActiveGroupSubstitutions(
	ctx context.Context,
	staffID int64,
	tenantID int64,
) (int, error) {
	return s.queryScalarCount(ctx, `
		SELECT COUNT(*)
		FROM education.group_substitution AS gs
		WHERE gs.tenant_id = ?
		  AND (gs.substitute_staff_id = ? OR gs.regular_staff_id = ?)
		  AND gs.start_date <= CURRENT_DATE
		  AND gs.end_date >= CURRENT_DATE
	`, tenantID, staffID, staffID)
}

func (s *caregiverCapabilityService) countPlannedActivitySupervisions(
	ctx context.Context,
	staffID int64,
	tenantID int64,
) (int, error) {
	return s.queryScalarCount(ctx, `
		SELECT COUNT(*)
		FROM activities.supervisors AS s
		WHERE s.tenant_id = ?
		  AND s.staff_id = ?
	`, tenantID, staffID)
}

func (s *caregiverCapabilityService) countGroupTeacherAssignments(
	ctx context.Context,
	teacherID int64,
	tenantID int64,
) (int, error) {
	return s.queryScalarCount(ctx, `
		SELECT COUNT(*)
		FROM education.group_teacher AS gt
		WHERE gt.tenant_id = ?
		  AND gt.teacher_id = ?
	`, tenantID, teacherID)
}

func (s *caregiverCapabilityService) queryScalarCount(
	ctx context.Context,
	query string,
	args ...interface{},
) (int, error) {
	var db bun.IDB = s.db
	if tx, ok := modelBase.TxFromContext(ctx); ok && tx != nil {
		db = tx
	}

	var count int
	if err := db.NewRaw(query, args...).Scan(ctx, &count); err != nil {
		return 0, err
	}
	return count, nil
}
