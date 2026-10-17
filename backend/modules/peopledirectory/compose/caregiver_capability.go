package compose

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/userscontract"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// CaregiverCapabilityService manages the operational caregiver capability of an
// existing account inside a tenant.
type CaregiverCapabilityService interface {
	GetCaregiverCapability(ctx context.Context, accountID int64) (*userModels.CaregiverCapabilityState, error)
	EnableCaregiverCapability(ctx context.Context, accountID int64, input userModels.EnableCaregiverCapabilityInput) (*userModels.CaregiverCapabilityState, error)
	DisableCaregiverCapability(ctx context.Context, accountID int64) (*userModels.CaregiverCapabilityState, error)
}

// CaregiverCapabilityServiceDependencies contains the repositories and ports
// required to manage caregiver capability on existing accounts. The bindings
// that keep an account a caregiver live in tables of other owners; each is
// reached through a port the composition root binds to that owner.
type CaregiverCapabilityServiceDependencies struct {
	RequestActorScope      func(context.Context) (accountID int64, scope string)
	Identity               CaregiverIdentity
	Audit                  CaregiverCapabilityAudit
	PersonRepo             peoplePorts.PersonRepository
	StaffRepo              peoplePorts.StaffRepository
	CaregiverBindingLock   peoplePorts.CaregiverBindingLocker
	TeacherRepo            peoplePorts.TeacherRepository
	GroupTeacherRepo       GroupTeacherBlockers
	GroupSubstitutionRepo  SubstitutionBlockers
	GroupSupervisorRepo    SupervisionBlockers
	ActivitySupervisorRepo PlannedSupervisionBlockers
	// RoleAssignments assigns and removes the caregiver role through the
	// Identity & Access role administration (#3314).
	RoleAssignments CaregiverRoleAssignments
}

// CaregiverCapabilityAudit records a caregiver capability change in the
// authentication audit trail (Audit Platform) inside the caller's transaction.
type CaregiverCapabilityAudit interface {
	RecordCaregiverCapabilityChange(ctx context.Context, accountID int64, enabled bool, metadata map[string]any) error
}

// GroupTeacherBlockers lists the group assignments a teacher still holds, as
// caregiver-capability blocker rows.
type GroupTeacherBlockers interface {
	ListGroupTeacherBlockers(ctx context.Context, teacherID, tenantID int64) ([]userModels.BlockerGroup, error)
}

// SubstitutionBlockers lists the current and upcoming group handovers a staff
// member holds, as caregiver-capability blocker rows.
type SubstitutionBlockers interface {
	ListActiveSubstitutionBlockers(ctx context.Context, staffID, tenantID int64) ([]userModels.BlockerSubstitution, error)
}

// SupervisionBlockers lists the Student Presence supervision a staff member
// still holds today, as caregiver-capability blocker rows.
type SupervisionBlockers interface {
	ListActiveSupervisionBlockers(ctx context.Context, staffID int64) ([]userModels.BlockerSupervision, error)
}

// PlannedSupervisionBlockers lists the planned activity supervisions
// (Timetable & Activities) a staff member holds, as caregiver-capability
// blocker rows.
type PlannedSupervisionBlockers interface {
	ListPlannedSupervisionBlockers(ctx context.Context, staffID, tenantID int64) ([]userModels.BlockerActivity, error)
}

// CaregiverRoleAssignments is the consumer-owned port over the Identity &
// Access role administration. Both commands join the caller's transaction
// and revoke the account's sessions at the school.
type CaregiverRoleAssignments interface {
	AssignRoleToAccount(ctx context.Context, accountID, roleID int64) error
	RemoveRoleFromAccount(ctx context.Context, accountID, roleID int64) error
}

type caregiverCapabilityService struct {
	CaregiverCapabilityServiceDependencies
	txHandler *tenant.TransactionRunner
}

// NewCaregiverCapabilityService creates a tenant-scoped caregiver capability service.
func NewCaregiverCapabilityService(
	deps CaregiverCapabilityServiceDependencies,
) CaregiverCapabilityService {
	if deps.RequestActorScope == nil {
		panic("request actor scope is required")
	}
	return &caregiverCapabilityService{
		CaregiverCapabilityServiceDependencies: deps,
		txHandler:                              tenant.NewTransactionRunner(),
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
		return nil, &userscontract.UsersError{
			Op:  "enable caregiver capability",
			Err: &userscontract.ValidationError{Err: fmt.Errorf("account ID is required")},
		}
	}

	input.FirstName = strings.TrimSpace(input.FirstName)
	input.LastName = strings.TrimSpace(input.LastName)
	input.Position = strings.TrimSpace(input.Position)

	var result *userModels.CaregiverCapabilityState
	if err := s.txHandler.RunInTx(ctx, func(txCtx context.Context) error {
		var err error
		result, err = s.enableInTx(txCtx, accountID, input)
		return err
	}); err != nil {
		return nil, err
	}

	return result, nil
}

func (s *caregiverCapabilityService) enableInTx(
	ctx context.Context,
	accountID int64,
	input userModels.EnableCaregiverCapabilityInput,
) (*userModels.CaregiverCapabilityState, error) {
	beforeState, _, err := s.loadCapabilityStateWithRoleFlags(ctx, accountID)
	if err != nil {
		return nil, err
	}

	account, _, err := s.loadAccountAndTenant(ctx, accountID)
	if err != nil {
		return nil, err
	}

	details := map[string]any{}
	if err := s.ensureCaregiverProfile(ctx, account.ID, input, details); err != nil {
		return nil, err
	}
	if input.Position != "" {
		details["requested_position"] = input.Position
	}

	userRoleID, err := s.systemRoleID(ctx, "enable caregiver capability", "user")
	if err != nil {
		return nil, err
	}
	if err := s.RoleAssignments.AssignRoleToAccount(ctx, accountID, userRoleID); err != nil {
		return nil, err
	}

	result, _, err := s.loadCapabilityStateWithRoleFlags(ctx, accountID)
	if err != nil {
		return nil, err
	}

	return result, s.recordCapabilityAuditEvent(
		ctx,
		accountID,
		true,
		s.buildCapabilityAuditMetadata(ctx, beforeState, result, details),
	)
}

// ensureCaregiverProfile completes the person, staff and teacher rows of the
// account, recording each row it creates in details.
func (s *caregiverCapabilityService) ensureCaregiverProfile(
	ctx context.Context,
	accountID int64,
	input userModels.EnableCaregiverCapabilityInput,
	details map[string]any,
) error {
	person, err := s.ensurePerson(ctx, accountID, input, details)
	if err != nil {
		return err
	}
	staff, err := s.ensureStaff(ctx, person.ID, details)
	if err != nil {
		return err
	}
	return s.ensureTeacher(ctx, staff.ID, input.Position, details)
}

func (s *caregiverCapabilityService) ensurePerson(
	ctx context.Context,
	accountID int64,
	input userModels.EnableCaregiverCapabilityInput,
	details map[string]any,
) (*userModels.Person, error) {
	person, err := s.PersonRepo.FindByAccountID(ctx, accountID)
	if err != nil || person != nil {
		return person, err
	}
	if input.FirstName == "" || input.LastName == "" {
		return nil, &userscontract.UsersError{
			Op: "enable caregiver capability",
			Err: &userscontract.ValidationError{
				Err: fmt.Errorf("first_name and last_name are required when the account has no person profile"),
			},
		}
	}

	person = &userModels.Person{
		FirstName: input.FirstName,
		LastName:  input.LastName,
	}
	person.SetTenantID(tenant.FromContext(ctx))
	if err := s.PersonRepo.Create(ctx, person); err != nil {
		return nil, err
	}
	if err := s.PersonRepo.LinkToAccount(ctx, person.ID, accountID); err != nil {
		return nil, err
	}
	details["person_created"] = true
	return person, nil
}

func (s *caregiverCapabilityService) ensureStaff(
	ctx context.Context,
	personID int64,
	details map[string]any,
) (*userModels.Staff, error) {
	staff, err := s.findStaffByPersonID(ctx, personID)
	if err != nil || staff != nil {
		return staff, err
	}
	staff = &userModels.Staff{PersonID: personID}
	staff.SetTenantID(tenant.FromContext(ctx))
	if err := s.StaffRepo.Create(ctx, staff); err != nil {
		return nil, err
	}
	details["staff_created"] = true
	return staff, nil
}

func (s *caregiverCapabilityService) ensureTeacher(
	ctx context.Context,
	staffID int64,
	position string,
	details map[string]any,
) error {
	teacher, err := s.TeacherRepo.FindByStaffID(ctx, staffID)
	if err != nil || teacher != nil {
		return err
	}
	teacher = &userModels.Teacher{StaffID: staffID}
	teacher.SetTenantID(tenant.FromContext(ctx))
	if position != "" {
		teacher.Role = position
	}
	if err := s.TeacherRepo.Create(ctx, teacher); err != nil {
		return err
	}
	details["teacher_created"] = true
	return nil
}

// systemRoleID resolves a system role, reporting a missing role as a failure
// of op.
func (s *caregiverCapabilityService) systemRoleID(ctx context.Context, op, roleName string) (int64, error) {
	roleID, found, err := s.Identity.FindSystemRoleID(ctx, roleName)
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, &userscontract.UsersError{Op: op, Err: fmt.Errorf("%s role not found", roleName)}
	}
	return roleID, nil
}

func (s *caregiverCapabilityService) DisableCaregiverCapability(
	ctx context.Context,
	accountID int64,
) (*userModels.CaregiverCapabilityState, error) {
	if accountID <= 0 {
		return nil, &userscontract.UsersError{
			Op:  "disable caregiver capability",
			Err: &userscontract.ValidationError{Err: fmt.Errorf("account ID is required")},
		}
	}

	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	// The transaction LOCK TABLEs the four caregiver-binding tables (incl.
	// active.group_supervisors). Concurrent room/supervision writes take row
	// locks on the same tables in a different order, so a transient deadlock
	// is possible — retry the whole transaction on 40P01/40001. It must own
	// the transaction it retries: an ambient request transaction is already
	// aborted after a deadlock and cannot be replayed. Its after-commit hooks
	// also belong to that request, not to this independently committed
	// operation.
	var result *userModels.CaregiverCapabilityState
	retryCtx := tenant.ContextWithoutAfterCommitHooks(tenant.ContextWithoutTransaction(ctx))
	err = tenant.WithinTenantRetry(retryCtx, tenantID, func(txCtx context.Context) error {
		if err := requireBunTransaction(txCtx); err != nil {
			return err
		}
		var disableErr error
		result, disableErr = s.disableInTx(txCtx, accountID)
		return disableErr
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func requireBunTransaction(ctx context.Context) error {
	raw, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return fmt.Errorf("disable caregiver capability: unit of work did not provide a transaction")
	}
	if _, ok := raw.(bun.Tx); !ok {
		return fmt.Errorf("disable caregiver capability: unsupported transaction type %T", raw)
	}
	return nil
}

func (s *caregiverCapabilityService) disableInTx(
	ctx context.Context,
	accountID int64,
) (*userModels.CaregiverCapabilityState, error) {
	if err := s.lockCapabilityBindings(ctx, accountID); err != nil {
		return nil, err
	}

	state, roleFlags, err := s.loadCapabilityStateWithRoleFlags(ctx, accountID)
	if err != nil {
		return nil, err
	}

	if len(state.DisableBlockers) > 0 {
		return nil, &userModels.CaregiverCapabilityBlockedError{Reasons: state.DisableBlockers}
	}

	if !roleFlags.hasUserRole && !roleFlags.hasLegacyTeacherRole {
		return state, nil
	}

	if err := s.removeCaregiverRoles(ctx, accountID, roleFlags); err != nil {
		return nil, err
	}

	result, _, err := s.loadCapabilityStateWithRoleFlags(ctx, accountID)
	if err != nil {
		return nil, err
	}

	return result, s.recordCapabilityAuditEvent(
		ctx,
		accountID,
		false,
		s.buildCapabilityAuditMetadata(ctx, state, result, nil),
	)
}

// lockCapabilityBindings locks the account's staff row and the four binding
// tables, so the blocker re-check that follows sees every committed binding.
func (s *caregiverCapabilityService) lockCapabilityBindings(ctx context.Context, accountID int64) error {
	initial, _, err := s.loadCapabilityStateWithRoleFlags(ctx, accountID)
	if err != nil {
		return err
	}
	if initial.StaffID != nil {
		if _, err := s.StaffRepo.FindByIDForUpdate(ctx, *initial.StaffID); err != nil {
			return err
		}
	}
	if s.CaregiverBindingLock == nil {
		return &userscontract.UsersError{Op: "disable caregiver capability", Err: errors.New("caregiver binding lock repository is not configured")}
	}
	if err := s.CaregiverBindingLock.LockCaregiverCapabilityBindings(ctx); err != nil {
		return &userscontract.UsersError{
			Op:  "disable caregiver capability",
			Err: fmt.Errorf("lock caregiver capability bindings: %w", err),
		}
	}
	return nil
}

func (s *caregiverCapabilityService) removeCaregiverRoles(
	ctx context.Context,
	accountID int64,
	roleFlags caregiverRoleFlags,
) error {
	roleNamesToRemove := make([]string, 0, 2)
	if roleFlags.hasUserRole {
		roleNamesToRemove = append(roleNamesToRemove, "user")
	}
	if roleFlags.hasLegacyTeacherRole {
		roleNamesToRemove = append(roleNamesToRemove, "teacher")
	}

	for _, roleName := range roleNamesToRemove {
		roleID, err := s.systemRoleID(ctx, "disable caregiver capability", roleName)
		if err != nil {
			return err
		}
		if err := s.RoleAssignments.RemoveRoleFromAccount(ctx, accountID, roleID); err != nil {
			return err
		}
	}
	return nil
}

func (s *caregiverCapabilityService) recordCapabilityAuditEvent(
	ctx context.Context,
	accountID int64,
	enabled bool,
	metadata map[string]any,
) error {
	if s.Audit == nil {
		return &userscontract.UsersError{
			Op:  "record caregiver capability audit event",
			Err: fmt.Errorf("auth event repository is not configured"),
		}
	}
	return s.Audit.RecordCaregiverCapabilityChange(ctx, accountID, enabled, metadata)
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

	actorID, actorScope := s.RequestActorScope(ctx)
	if actorID > 0 {
		metadata["actor_account_id"] = actorID
	}
	if actorScope != "" {
		metadata["actor_scope"] = actorScope
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

func (s *caregiverCapabilityService) findStaffByPersonID(
	ctx context.Context,
	personID int64,
) (*userModels.Staff, error) {
	staff, err := s.StaffRepo.FindByPersonID(ctx, personID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return staff, nil
}
