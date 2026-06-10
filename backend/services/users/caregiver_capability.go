package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
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
	AuthEventRepo          auditModels.AuthEventRepository
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
	authEventRepo          auditModels.AuthEventRepository
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

type caregiverRoleFlags struct {
	hasAdminRole         bool
	hasUserRole          bool
	hasLegacyTeacherRole bool
	hasOtherUsableRole   bool
}

var caregiverCapabilityBindingTables = []string{
	"education.group_teacher",
	"education.group_substitution",
	"active.group_supervisors",
	"activities.supervisors",
}

const caregiverCapabilityAuditIP = "0.0.0.0"

// NewCaregiverCapabilityService creates a tenant-scoped caregiver capability service.
func NewCaregiverCapabilityService(
	deps CaregiverCapabilityServiceDependencies,
) CaregiverCapabilityService {
	return &caregiverCapabilityService{
		accountRepo:            deps.AccountRepo,
		accountTenantRepo:      deps.AccountTenantRepo,
		authEventRepo:          deps.AuthEventRepo,
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
	state, _, err := s.loadCapabilityStateWithRoleFlags(ctx, accountID)
	return state, err
}

func (s *caregiverCapabilityService) EnableCaregiverCapability(
	ctx context.Context,
	accountID int64,
	input userModels.EnableCaregiverCapabilityInput,
) (*userModels.CaregiverCapabilityState, error) {
	if accountID <= 0 {
		return nil, &UsersError{
			Op:  "enable caregiver capability",
			Err: &ValidationError{Err: fmt.Errorf("account ID is required")},
		}
	}

	input.FirstName = strings.TrimSpace(input.FirstName)
	input.LastName = strings.TrimSpace(input.LastName)
	input.Position = strings.TrimSpace(input.Position)

	var result *userModels.CaregiverCapabilityState
	if err := s.txHandler.RunInTx(ctx, func(txCtx context.Context, _ bun.Tx) error {
		beforeState, _, err := s.loadCapabilityStateWithRoleFlags(txCtx, accountID)
		if err != nil {
			return err
		}

		account, _, err := s.loadAccountAndTenant(txCtx, accountID)
		if err != nil {
			return err
		}

		details := map[string]any{}
		person, err := s.personRepo.FindByAccountID(txCtx, accountID)
		if err != nil {
			return err
		}
		if person == nil {
			if input.FirstName == "" || input.LastName == "" {
				return &UsersError{
					Op: "enable caregiver capability",
					Err: &ValidationError{
						Err: fmt.Errorf("first_name and last_name are required when the account has no person profile"),
					},
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
			details["person_created"] = true
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
			details["staff_created"] = true
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
			details["teacher_created"] = true
		}
		if input.Position != "" {
			details["requested_position"] = input.Position
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

		result, _, err = s.loadCapabilityStateWithRoleFlags(txCtx, accountID)
		if err != nil {
			return err
		}

		return s.recordCapabilityAuditEvent(
			txCtx,
			accountID,
			auditModels.EventTypeCaregiverCapabilityEnabled,
			s.buildCapabilityAuditMetadata(txCtx, beforeState, result, details),
		)
	}); err != nil {
		return nil, err
	}

	return result, nil
}

func (s *caregiverCapabilityService) DisableCaregiverCapability(
	ctx context.Context,
	accountID int64,
) (*userModels.CaregiverCapabilityState, error) {
	if accountID <= 0 {
		return nil, &UsersError{
			Op:  "disable caregiver capability",
			Err: &ValidationError{Err: fmt.Errorf("account ID is required")},
		}
	}

	var result *userModels.CaregiverCapabilityState
	// This transaction LOCK TABLEs the four caregiver-binding tables (incl.
	// active.group_supervisors). Concurrent room/supervision writes take row
	// locks on the same tables in a different order, so a transient deadlock is
	// possible — retry the whole transaction on 40P01/40001.
	if err := s.txHandler.RunInTxWithRetry(ctx, func(txCtx context.Context, tx bun.Tx) error {
		if err := s.lockCaregiverCapabilityBindings(txCtx, tx); err != nil {
			return err
		}

		state, roleFlags, err := s.loadCapabilityStateWithRoleFlags(txCtx, accountID)
		if err != nil {
			return err
		}

		if len(state.DisableBlockers) > 0 {
			return &CaregiverCapabilityBlockedError{Reasons: state.DisableBlockers}
		}

		if !roleFlags.hasUserRole && !roleFlags.hasLegacyTeacherRole {
			result = state
			return nil
		}

		roleNamesToRemove := make([]string, 0, 2)
		if roleFlags.hasUserRole {
			roleNamesToRemove = append(roleNamesToRemove, "user")
		}
		if roleFlags.hasLegacyTeacherRole {
			roleNamesToRemove = append(roleNamesToRemove, "teacher")
		}

		for _, roleName := range roleNamesToRemove {
			role, err := s.resolveSystemRoleByName(txCtx, roleName)
			if err != nil {
				return err
			}
			if role == nil {
				return &UsersError{
					Op:  "disable caregiver capability",
					Err: fmt.Errorf("%s role not found", roleName),
				}
			}

			if err := s.authService.RemoveRoleFromAccount(txCtx, int(accountID), int(role.ID)); err != nil {
				return err
			}
		}

		result, _, err = s.loadCapabilityStateWithRoleFlags(txCtx, accountID)
		if err != nil {
			return err
		}

		return s.recordCapabilityAuditEvent(
			txCtx,
			accountID,
			auditModels.EventTypeCaregiverCapabilityDisabled,
			s.buildCapabilityAuditMetadata(txCtx, state, result, nil),
		)
	}); err != nil {
		return nil, err
	}

	return result, nil
}

func (s *caregiverCapabilityService) lockCaregiverCapabilityBindings(
	ctx context.Context,
	tx bun.Tx,
) error {
	for _, tableName := range caregiverCapabilityBindingTables {
		if _, err := tx.ExecContext(
			ctx,
			fmt.Sprintf("LOCK TABLE %s IN SHARE ROW EXCLUSIVE MODE", tableName),
		); err != nil {
			return &UsersError{
				Op:  "disable caregiver capability",
				Err: fmt.Errorf("lock %s: %w", tableName, err),
			}
		}
	}

	return nil
}

func (s *caregiverCapabilityService) recordCapabilityAuditEvent(
	ctx context.Context,
	accountID int64,
	eventType string,
	metadata map[string]any,
) error {
	if s.authEventRepo == nil {
		return &UsersError{
			Op:  "record caregiver capability audit event",
			Err: fmt.Errorf("auth event repository is not configured"),
		}
	}

	event := auditModels.NewAuthEvent(accountID, eventType, true, caregiverCapabilityAuditIP)
	for key, value := range metadata {
		event.SetMetadata(key, value)
	}

	return s.authEventRepo.Create(ctx, event)
}

func (s *caregiverCapabilityService) buildCapabilityAuditMetadata(
	ctx context.Context,
	before *userModels.CaregiverCapabilityState,
	after *userModels.CaregiverCapabilityState,
	details map[string]any,
) map[string]any {
	metadata := map[string]any{
		"before": capabilityAuditSnapshot(before),
		"after":  capabilityAuditSnapshot(after),
	}

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		metadata["tenant_id"] = tenantID
	}

	claims := jwt.ClaimsFromCtx(ctx)
	if claims.ID > 0 {
		metadata["actor_account_id"] = claims.ID
	}
	if claims.Scope != "" {
		metadata["actor_scope"] = claims.Scope
	}

	for key, value := range details {
		metadata[key] = value
	}

	return metadata
}

func capabilityAuditSnapshot(state *userModels.CaregiverCapabilityState) map[string]any {
	if state == nil {
		return map[string]any{}
	}

	snapshot := map[string]any{
		"has_admin_role":        state.HasAdminRole,
		"has_user_role":         state.HasUserRole,
		"has_person":            state.HasPerson,
		"has_staff":             state.HasStaff,
		"has_teacher":           state.HasTeacher,
		"has_caregiver_profile": state.HasCaregiverProfile,
		"is_active_caregiver":   state.IsActiveCaregiver,
	}

	if state.PersonID != nil {
		snapshot["person_id"] = *state.PersonID
	}
	if state.StaffID != nil {
		snapshot["staff_id"] = *state.StaffID
	}
	if state.TeacherID != nil {
		snapshot["teacher_id"] = *state.TeacherID
	}

	return snapshot
}

func (s *caregiverCapabilityService) loadCapabilityStateWithRoleFlags(
	ctx context.Context,
	accountID int64,
) (*userModels.CaregiverCapabilityState, caregiverRoleFlags, error) {
	account, tenantID, err := s.loadAccountAndTenant(ctx, accountID)
	if err != nil {
		return nil, caregiverRoleFlags{}, err
	}

	state := &userModels.CaregiverCapabilityState{
		AccountID: account.ID,
		Email:     account.Email,
	}
	roleFlags := caregiverRoleFlags{}

	roles, err := s.roleRepo.FindByAccountID(ctx, accountID)
	if err != nil {
		return nil, caregiverRoleFlags{}, err
	}
	for _, role := range roles {
		if role == nil {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(role.Name)) {
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
	state.HasAdminRole = roleFlags.hasAdminRole
	state.HasUserRole = roleFlags.hasUserRole

	person, err := s.personRepo.FindByAccountID(ctx, accountID)
	if err != nil {
		return nil, caregiverRoleFlags{}, err
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
			return nil, caregiverRoleFlags{}, err
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
			return nil, caregiverRoleFlags{}, err
		}
	}
	if teacher != nil {
		state.HasTeacher = true
		state.TeacherID = &teacher.ID
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
	hasLegacyTeacherRole bool,
	hasOtherUsableRole bool,
) ([]userModels.CaregiverCapabilityBlockerCode, error) {
	if state == nil {
		return nil, nil
	}

	var blockers []userModels.CaregiverCapabilityBlockerCode

	if (state.HasUserRole || hasLegacyTeacherRole) &&
		!state.HasAdminRole &&
		!hasOtherUsableRole {
		blockers = append(blockers, userModels.CaregiverCapabilityBlockerMissingUsableRole)
	}

	if state.StaffID != nil {
		supervisions, err := s.listActiveGroupSupervisions(ctx, *state.StaffID, tenantID)
		if err != nil {
			return nil, err
		}
		if len(supervisions) > 0 {
			state.ActiveSupervisions = supervisions
			blockers = append(blockers, userModels.CaregiverCapabilityBlockerActiveGroupSupervisions)
		}

		substitutions, err := s.listActiveGroupSubstitutions(ctx, *state.StaffID, tenantID)
		if err != nil {
			return nil, err
		}
		if len(substitutions) > 0 {
			state.ActiveSubstitutions = substitutions
			blockers = append(blockers, userModels.CaregiverCapabilityBlockerActiveGroupSubstitutions)
		}

		activities, err := s.listPlannedActivitySupervisions(ctx, *state.StaffID, tenantID)
		if err != nil {
			return nil, err
		}
		if len(activities) > 0 {
			state.ActivitySupervisions = activities
			blockers = append(blockers, userModels.CaregiverCapabilityBlockerActivitySupervisions)
		}
	}

	if state.TeacherID != nil {
		groups, err := s.listGroupTeacherAssignments(ctx, *state.TeacherID, tenantID)
		if err != nil {
			return nil, err
		}
		if len(groups) > 0 {
			state.GroupAssignments = groups
			blockers = append(blockers, userModels.CaregiverCapabilityBlockerGroupAssignments)
		}
	}

	return blockers, nil
}

func (s *caregiverCapabilityService) listActiveGroupSupervisions(
	ctx context.Context,
	staffID int64,
	tenantID int64,
) ([]userModels.BlockerSupervision, error) {
	db := s.getDB(ctx)

	var results []userModels.BlockerSupervision
	err := db.NewRaw(`
		SELECT gs.id, COALESCE(g.name, 'Unbekannte Gruppe') AS group_name,
		       gs.start_date::text AS start_date
		FROM active.group_supervisors AS gs
		LEFT JOIN education.groups AS g ON g.id = gs.group_id AND g.tenant_id = gs.tenant_id
		WHERE gs.tenant_id = ?
		  AND gs.staff_id = ?
		  AND (gs.end_date IS NULL OR gs.end_date > NOW())
		ORDER BY gs.start_date DESC
	`, tenantID, staffID).Scan(ctx, &results)
	if err != nil {
		return nil, err
	}
	return results, nil
}

func (s *caregiverCapabilityService) listActiveGroupSubstitutions(
	ctx context.Context,
	staffID int64,
	tenantID int64,
) ([]userModels.BlockerSubstitution, error) {
	db := s.getDB(ctx)

	var results []userModels.BlockerSubstitution
	err := db.NewRaw(`
		SELECT gs.id, COALESCE(g.name, 'Unbekannte Gruppe') AS group_name,
		       CASE WHEN gs.substitute_staff_id = ? THEN 'substitute' ELSE 'regular' END AS role,
		       gs.start_date::text AS start_date,
		       gs.end_date::text AS end_date
		FROM education.group_substitution AS gs
		LEFT JOIN education.groups AS g ON g.id = gs.group_id AND g.tenant_id = gs.tenant_id
		WHERE gs.tenant_id = ?
		  AND (gs.substitute_staff_id = ? OR gs.regular_staff_id = ?)
		  AND gs.end_date >= CURRENT_DATE
		ORDER BY gs.start_date DESC
	`, staffID, tenantID, staffID, staffID).Scan(ctx, &results)
	if err != nil {
		return nil, err
	}
	return results, nil
}

func (s *caregiverCapabilityService) listPlannedActivitySupervisions(
	ctx context.Context,
	staffID int64,
	tenantID int64,
) ([]userModels.BlockerActivity, error) {
	db := s.getDB(ctx)

	var results []userModels.BlockerActivity
	err := db.NewRaw(`
		SELECT s.id, s.group_id AS activity_id,
		       COALESCE(ag.name, 'Unbekannte Aktivität') AS activity_name,
		       COALESCE(s.is_primary, false) AS is_primary
		FROM activities.supervisors AS s
		LEFT JOIN activities.groups AS ag ON ag.id = s.group_id AND ag.tenant_id = s.tenant_id
		WHERE s.tenant_id = ?
		  AND s.staff_id = ?
		ORDER BY ag.name ASC
	`, tenantID, staffID).Scan(ctx, &results)
	if err != nil {
		return nil, err
	}
	return results, nil
}

func (s *caregiverCapabilityService) listGroupTeacherAssignments(
	ctx context.Context,
	teacherID int64,
	tenantID int64,
) ([]userModels.BlockerGroup, error) {
	db := s.getDB(ctx)

	var results []userModels.BlockerGroup
	err := db.NewRaw(`
		SELECT gt.id,
		       gt.group_id,
		       COALESCE(g.name, 'Unbekannte Gruppe') AS group_name,
		       gt.teacher_id,
		       COALESCE((
		           SELECT ARRAY_AGG(gt_all.teacher_id ORDER BY gt_all.id)
		           FROM education.group_teacher AS gt_all
		           WHERE gt_all.tenant_id = gt.tenant_id
		             AND gt_all.group_id = gt.group_id
		       ), '{}'::bigint[]) AS teacher_ids
		FROM education.group_teacher AS gt
		LEFT JOIN education.groups AS g ON g.id = gt.group_id AND g.tenant_id = gt.tenant_id
		WHERE gt.tenant_id = ?
		  AND gt.teacher_id = ?
		ORDER BY g.name ASC
	`, tenantID, teacherID).Scan(ctx, &results)
	if err != nil {
		return nil, err
	}
	return results, nil
}

func (s *caregiverCapabilityService) getDB(ctx context.Context) bun.IDB {
	if tx, ok := modelBase.TxFromContext(ctx); ok && tx != nil {
		return tx
	}
	return s.db
}
