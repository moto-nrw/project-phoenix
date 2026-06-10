package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	authSvc "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

const opOffboardStaff = "offboard staff"

// StaffOffboardingServiceDependencies contains the repositories and services
// required to fully offboard a staff member within a tenant.
type StaffOffboardingServiceDependencies struct {
	PersonRepo             userModels.PersonRepository
	StaffRepo              userModels.StaffRepository
	TeacherRepo            userModels.TeacherRepository
	GroupSupervisorRepo    activeModels.GroupSupervisorRepository
	GroupTeacherRepo       educationModels.GroupTeacherRepository
	GroupSubstitutionRepo  educationModels.GroupSubstitutionRepository
	ActivitySupervisorRepo activitiesModels.SupervisorPlannedRepository
	InstanceStaffRepo      scheduleModels.InstanceStaffRepository
	AccountTenantRepo      authModels.AccountTenantRepository
	RoleRepo               authModels.RoleRepository
	DataDeletionRepo       auditModels.DataDeletionRepository
	AuthService            authSvc.AuthService
	DB                     *bun.DB
	Logger                 *slog.Logger
}

type staffOffboardingService struct {
	personRepo             userModels.PersonRepository
	staffRepo              userModels.StaffRepository
	teacherRepo            userModels.TeacherRepository
	groupSupervisorRepo    activeModels.GroupSupervisorRepository
	groupTeacherRepo       educationModels.GroupTeacherRepository
	groupSubstitutionRepo  educationModels.GroupSubstitutionRepository
	activitySupervisorRepo activitiesModels.SupervisorPlannedRepository
	instanceStaffRepo      scheduleModels.InstanceStaffRepository
	accountTenantRepo      authModels.AccountTenantRepository
	roleRepo               authModels.RoleRepository
	dataDeletionRepo       auditModels.DataDeletionRepository
	authService            authSvc.AuthService
	txHandler              *modelBase.TxHandler
	logger                 *slog.Logger
}

// NewStaffOffboardingService creates a tenant-scoped staff offboarding service.
func NewStaffOffboardingService(deps StaffOffboardingServiceDependencies) StaffOffboardingService {
	return &staffOffboardingService{
		personRepo:             deps.PersonRepo,
		staffRepo:              deps.StaffRepo,
		teacherRepo:            deps.TeacherRepo,
		groupSupervisorRepo:    deps.GroupSupervisorRepo,
		groupTeacherRepo:       deps.GroupTeacherRepo,
		groupSubstitutionRepo:  deps.GroupSubstitutionRepo,
		activitySupervisorRepo: deps.ActivitySupervisorRepo,
		instanceStaffRepo:      deps.InstanceStaffRepo,
		accountTenantRepo:      deps.AccountTenantRepo,
		roleRepo:               deps.RoleRepo,
		dataDeletionRepo:       deps.DataDeletionRepo,
		authService:            deps.AuthService,
		txHandler:              modelBase.NewTxHandler(deps.DB),
		logger:                 deps.Logger,
	}
}

func (s *staffOffboardingService) getLogger() *slog.Logger {
	if s.logger != nil {
		return s.logger
	}
	return slog.Default()
}

// OffboardStaff removes a staff member from daily operations and revokes their
// access for the current tenant:
//
//   - soft-deletes the staff (and teacher) rows so historical references
//     (attendance, work sessions, timetable) keep resolving names
//   - removes planned assignments the old hard-delete CASCADE used to clean up
//   - unlinks the person from RFID card and account (the person row stays as
//     the name-bearer for history)
//   - removes the account's tenant-scoped roles, revokes tokens, and
//     deactivates the account-tenant mapping so the email can be re-invited
//   - deactivates the auth account entirely when no other active tenant
//     mapping remains
//
// Deleting a non-existent staff member is a no-op (idempotent delete).
func (s *staffOffboardingService) OffboardStaff(ctx context.Context, staffID int64, deletedBy string) error {
	return s.txHandler.RunInTx(ctx, func(txCtx context.Context, _ bun.Tx) error {
		return s.offboardStaffInTx(txCtx, staffID, deletedBy)
	})
}

func (s *staffOffboardingService) offboardStaffInTx(ctx context.Context, staffID int64, deletedBy string) error {
	staff, err := s.staffRepo.FindByID(ctx, staffID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil // idempotent: already gone (or soft-deleted)
		}
		return &UsersError{Op: opOffboardStaff, Err: err}
	}

	// Active supervisions block offboarding. With soft delete there is no FK
	// safety net anymore, so a failing pre-check is a hard error.
	supervisors, err := s.groupSupervisorRepo.FindActiveByStaffID(ctx, staffID)
	if err != nil {
		return &UsersError{Op: opOffboardStaff, Err: fmt.Errorf("active supervision check: %w", err)}
	}
	if len(supervisors) > 0 {
		return &UsersError{Op: opOffboardStaff, Err: ErrStaffInUse}
	}

	cleanupCounts, err := s.cleanupAssignments(ctx, staffID)
	if err != nil {
		return err
	}

	if err := s.staffRepo.Delete(ctx, staffID); err != nil {
		return &UsersError{Op: opOffboardStaff, Err: err}
	}

	if err := s.offboardPersonAndAccount(ctx, staff.PersonID); err != nil {
		return err
	}

	return s.recordAudit(ctx, staffID, deletedBy, cleanupCounts)
}

// cleanupAssignments removes planned/future assignments that the old
// ON DELETE CASCADE used to clean up before staff rows became soft-deleted.
// Returns per-table deletion counts for the audit record.
func (s *staffOffboardingService) cleanupAssignments(ctx context.Context, staffID int64) (map[string]any, error) {
	counts := map[string]any{}

	teacher, err := s.teacherRepo.FindByStaffID(ctx, staffID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, &UsersError{Op: opOffboardStaff, Err: fmt.Errorf("load teacher record: %w", err)}
	}
	if teacher != nil {
		groupAssignments, err := s.groupTeacherRepo.DeleteByTeacherID(ctx, teacher.ID)
		if err != nil {
			return nil, &UsersError{Op: opOffboardStaff, Err: err}
		}
		counts["group_teacher"] = groupAssignments

		if err := s.teacherRepo.Delete(ctx, teacher.ID); err != nil {
			return nil, &UsersError{Op: opOffboardStaff, Err: fmt.Errorf("failed to delete teacher record: %w", err)}
		}
	}

	supervisions, err := s.activitySupervisorRepo.DeleteByStaffID(ctx, staffID)
	if err != nil {
		return nil, &UsersError{Op: opOffboardStaff, Err: err}
	}
	counts["activity_supervisors"] = supervisions

	today := timezone.TodayUTC()
	substitutions, err := s.groupSubstitutionRepo.DeleteActiveOrFutureByStaffID(ctx, staffID, today)
	if err != nil {
		return nil, &UsersError{Op: opOffboardStaff, Err: err}
	}
	counts["group_substitutions"] = substitutions

	instanceAssignments, err := s.instanceStaffRepo.DeleteFutureByStaffID(ctx, staffID, today)
	if err != nil {
		return nil, &UsersError{Op: opOffboardStaff, Err: err}
	}
	counts["timetable_instance_staff"] = instanceAssignments

	return counts, nil
}

// offboardPersonAndAccount unlinks the person from RFID card and account and
// revokes the account's access for the current tenant. The person row itself
// is kept so historical records keep resolving the staff member's name.
func (s *staffOffboardingService) offboardPersonAndAccount(ctx context.Context, personID int64) error {
	person, err := s.personRepo.FindByID(ctx, personID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return &UsersError{Op: opOffboardStaff, Err: fmt.Errorf("load person: %w", err)}
	}
	if person == nil {
		return nil
	}

	if person.TagID != nil {
		if err := s.personRepo.UnlinkFromRFIDCard(ctx, person.ID); err != nil {
			return &UsersError{Op: opOffboardStaff, Err: fmt.Errorf("unlink rfid card: %w", err)}
		}
	}

	if person.AccountID == nil {
		return nil
	}
	accountID := *person.AccountID

	// Remove tenant-scoped roles (also revokes this tenant's tokens per role).
	roles, err := s.roleRepo.FindByAccountID(ctx, accountID)
	if err != nil {
		return &UsersError{Op: opOffboardStaff, Err: fmt.Errorf("load account roles: %w", err)}
	}
	for _, role := range roles {
		if err := s.authService.RemoveRoleFromAccount(ctx, int(accountID), int(role.ID)); err != nil {
			return &UsersError{Op: opOffboardStaff, Err: fmt.Errorf("remove role %q: %w", role.Name, err)}
		}
	}

	// Free the partial unique index on persons.account_id so a re-invitation
	// can link a fresh person to the same account.
	if err := s.personRepo.UnlinkFromAccount(ctx, person.ID); err != nil {
		return &UsersError{Op: opOffboardStaff, Err: fmt.Errorf("unlink account: %w", err)}
	}

	tenantID := tenant.FromContext(ctx)
	if err := s.accountTenantRepo.Deactivate(ctx, accountID, tenantID); err != nil {
		return &UsersError{Op: opOffboardStaff, Err: fmt.Errorf("deactivate tenant mapping: %w", err)}
	}

	// Deactivate the account entirely (and revoke all tokens) only when it has
	// no active mapping to any other school.
	remaining, err := s.accountTenantRepo.FindActiveByAccountID(ctx, accountID)
	if err != nil {
		return &UsersError{Op: opOffboardStaff, Err: fmt.Errorf("check remaining tenant mappings: %w", err)}
	}
	if len(remaining) == 0 {
		if err := s.authService.DeactivateAccount(ctx, int(accountID)); err != nil {
			return &UsersError{Op: opOffboardStaff, Err: fmt.Errorf("deactivate account: %w", err)}
		}
	}

	s.getLogger().Info("staff account offboarded",
		"account_id", accountID,
		"tenant_id", tenantID,
		"account_deactivated", len(remaining) == 0,
	)

	return nil
}

func (s *staffOffboardingService) recordAudit(ctx context.Context, staffID int64, deletedBy string, cleanupCounts map[string]any) error {
	if deletedBy == "" {
		deletedBy = "system"
	}

	records := 1 // the staff row itself
	for _, v := range cleanupCounts {
		if n, ok := v.(int64); ok {
			records += int(n)
		}
	}

	deletion := auditModels.NewStaffDataDeletion(staffID, auditModels.DeletionTypeManual, records, deletedBy)
	deletion.DeletionReason = "staff offboarding"
	deletion.SetTenantID(tenant.FromContext(ctx))
	for k, v := range cleanupCounts {
		deletion.Metadata[k] = v
	}

	if err := s.dataDeletionRepo.Create(ctx, deletion); err != nil {
		return &UsersError{Op: opOffboardStaff, Err: fmt.Errorf("record data deletion: %w", err)}
	}
	return nil
}
