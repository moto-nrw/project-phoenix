package users

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
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
	StaffShiftRepo         scheduleModels.StaffShiftRepository
	StaffShiftSeriesRepo   scheduleModels.StaffShiftSeriesRepository
	StaffAbsenceRepo       activeModels.StaffAbsenceRepository
	AccountTenantRepo      authModels.AccountTenantRepository
	RoleRepo               authModels.RoleRepository
	AccountPermissionRepo  authModels.AccountPermissionRepository
	DataDeletionRepo       auditModels.DataDeletionRepository
	TimeTrackingDeleteRepo auditModels.TimeTrackingDeletionRepository
	AuthService            authSvc.AuthService
	DB                     *bun.DB
	Logger                 *slog.Logger
}

type staffOffboardingService struct {
	StaffOffboardingServiceDependencies
	txHandler   *modelBase.TxHandler
	broadcaster realtime.Broadcaster
}

// NewStaffOffboardingService creates a tenant-scoped staff offboarding service.
func NewStaffOffboardingService(deps StaffOffboardingServiceDependencies) StaffOffboardingService {
	return &staffOffboardingService{
		StaffOffboardingServiceDependencies: deps,
		txHandler:                           modelBase.NewTxHandler(deps.DB),
	}
}

func (s *staffOffboardingService) getLogger() *slog.Logger {
	return cmp.Or(s.Logger, slog.Default())
}

// SetBroadcaster injects the tenant-wide SSE broadcaster without widening the
// public StaffOffboardingService interface used by API mocks.
func (s *staffOffboardingService) SetBroadcaster(broadcaster realtime.Broadcaster) {
	s.broadcaster = broadcaster
}

// OffboardStaff removes a staff member from daily operations and revokes their
// access for the current tenant:
//
//   - soft-deletes the staff (and teacher) rows so historical references
//     (attendance, work sessions, timetable) keep resolving names
//   - removes planned assignments the old hard-delete CASCADE used to clean up
//   - unlinks the person from RFID card and account (the person row stays as
//     the name-bearer for history)
//   - removes the account's tenant-scoped roles and direct permission grants,
//     revokes tokens, and deactivates the account-tenant mapping so the email
//     can be re-invited
//   - keeps the guardian role and the tenant mapping when the account is also
//     a guardian at this school (parents portal access survives offboarding)
//   - deactivates the auth account entirely when no other active tenant
//     mapping remains
//
// Deleting a non-existent staff member is a no-op (idempotent delete).
func (s *staffOffboardingService) OffboardStaff(ctx context.Context, staffID, deletedByStaffID int64, deletedBy string) error {
	groupAccessChanged := false
	if err := s.txHandler.RunInTx(ctx, func(txCtx context.Context, _ bun.Tx) error {
		return s.offboardStaffInTx(txCtx, staffID, deletedByStaffID, deletedBy, &groupAccessChanged)
	}); err != nil {
		return err
	}
	if groupAccessChanged {
		// Direct service calls commit in RunInTx above, while HTTP calls reuse
		// the surrounding WithTenantTx and register this on its hook holder.
		// Both paths therefore publish only after their owning transaction.
		realtime.QueueGroupAccessChanged(ctx, s.broadcaster, s.getLogger(), "staff_offboarding")
	}
	return nil
}

func (s *staffOffboardingService) offboardStaffInTx(ctx context.Context, staffID, deletedByStaffID int64, deletedBy string, groupAccessChanged *bool) error {
	staff, err := s.StaffRepo.FindByID(ctx, staffID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil // idempotent: already gone (or soft-deleted)
		}
		return &UsersError{Op: opOffboardStaff, Err: err}
	}

	// Active supervisions block offboarding. With soft delete there is no FK
	// safety net anymore, so a failing pre-check is a hard error.
	supervisors, err := s.GroupSupervisorRepo.FindActiveByStaffID(ctx, staffID)
	if err != nil {
		return &UsersError{Op: opOffboardStaff, Err: fmt.Errorf("active supervision check: %w", err)}
	}
	if len(supervisors) > 0 {
		return &UsersError{Op: opOffboardStaff, Err: ErrStaffInUse}
	}

	cleanupCounts, err := s.cleanupAssignments(ctx, staffID, deletedByStaffID)
	if err != nil {
		return err
	}
	groupTeachersDeleted, _ := cleanupCounts["group_teacher"].(int64)
	groupSubstitutionsDeleted, _ := cleanupCounts["group_substitutions"].(int64)
	if groupTeachersDeleted > 0 || groupSubstitutionsDeleted > 0 {
		*groupAccessChanged = true
	}

	// Clear the work-time-model FK before the soft delete: the row keeps its
	// reference otherwise and blocks work-time-model deletion via the RESTRICT
	// FK while the live-staff pre-check reports zero assignments.
	if staff.WorkTimeModelID != nil {
		if err := s.StaffRepo.ClearWorkTimeModel(ctx, staffID); err != nil {
			return &UsersError{Op: opOffboardStaff, Err: fmt.Errorf("clear work time model: %w", err)}
		}
	}

	if err := s.StaffRepo.Delete(ctx, staffID); err != nil {
		return &UsersError{Op: opOffboardStaff, Err: err}
	}

	if err := s.offboardPersonAndAccount(ctx, staff.PersonID, cleanupCounts); err != nil {
		return err
	}

	return s.recordAudit(ctx, staffID, deletedBy, cleanupCounts)
}

// cleanupAssignments removes planned/future assignments that the old
// ON DELETE CASCADE used to clean up before staff rows became soft-deleted.
// Returns per-table deletion counts for the audit record.
func (s *staffOffboardingService) cleanupAssignments(ctx context.Context, staffID, deletedByStaffID int64) (map[string]any, error) {
	counts := map[string]any{}

	teacher, err := s.TeacherRepo.FindByStaffID(ctx, staffID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, &UsersError{Op: opOffboardStaff, Err: fmt.Errorf("load teacher record: %w", err)}
	}
	if teacher != nil {
		groupAssignments, err := s.GroupTeacherRepo.DeleteByTeacherID(ctx, teacher.ID)
		if err != nil {
			return nil, &UsersError{Op: opOffboardStaff, Err: err}
		}
		counts["group_teacher"] = groupAssignments

		if err := s.TeacherRepo.Delete(ctx, teacher.ID); err != nil {
			return nil, &UsersError{Op: opOffboardStaff, Err: fmt.Errorf("failed to delete teacher record: %w", err)}
		}
	}

	supervisions, err := s.ActivitySupervisorRepo.DeleteByStaffID(ctx, staffID)
	if err != nil {
		return nil, &UsersError{Op: opOffboardStaff, Err: err}
	}
	counts["activity_supervisors"] = supervisions

	today := timezone.TodayDate()
	substitutions, err := s.GroupSubstitutionRepo.DeleteActiveOrFutureByStaffID(ctx, staffID, today)
	if err != nil {
		return nil, &UsersError{Op: opOffboardStaff, Err: err}
	}
	counts["group_substitutions"] = substitutions

	instanceAssignments, err := s.InstanceStaffRepo.DeleteUpcomingByStaffID(ctx, staffID, today)
	if err != nil {
		return nil, &UsersError{Op: opOffboardStaff, Err: err}
	}
	counts["timetable_instance_staff"] = instanceAssignments

	if s.StaffShiftRepo != nil {
		staffShifts, err := s.StaffShiftRepo.DeleteUpcomingByStaffID(ctx, staffID, today)
		if err != nil {
			return nil, &UsersError{Op: opOffboardStaff, Err: err}
		}
		counts["staff_shifts"] = staffShifts
	}

	// Cap shift series (#1889) so a later admin split cannot materialize new
	// rows for the offboarded staff member; staff rows are soft-deleted, so
	// the FK cascade never fires.
	if s.StaffShiftSeriesRepo != nil {
		cappedSeries, err := s.StaffShiftSeriesRepo.CapAllByStaffID(ctx, staffID, today)
		if err != nil {
			return nil, &UsersError{Op: opOffboardStaff, Err: err}
		}
		counts["staff_shift_series"] = cappedSeries
	}

	if err := s.StaffAbsenceRepo.LockStaffAbsenceWrites(ctx, staffID); err != nil {
		return nil, &UsersError{Op: opOffboardStaff, Err: fmt.Errorf("lock staff absences before offboarding: %w", err)}
	}
	absences, err := s.StaffAbsenceRepo.ListNonHistoricalByStaffID(ctx, staffID, today)
	if err != nil {
		return nil, &UsersError{Op: opOffboardStaff, Err: err}
	}
	if len(absences) > 0 && s.TimeTrackingDeleteRepo == nil {
		return nil, &UsersError{Op: opOffboardStaff, Err: errors.New("time tracking deletion audit repository is not configured")}
	}
	for _, absence := range absences {
		payload, err := json.Marshal(absence)
		if err != nil {
			return nil, &UsersError{Op: opOffboardStaff, Err: fmt.Errorf("marshal absence %d for deletion audit: %w", absence.ID, err)}
		}
		if err := s.TimeTrackingDeleteRepo.Create(ctx, &auditModels.TimeTrackingDeletion{
			StaffID:   absence.StaffID,
			Source:    auditModels.TimeTrackingDeletionSourceAbsence,
			SourceID:  absence.ID,
			DeletedBy: deletedByStaffID,
			Payload:   payload,
			Note:      "Personal-Offboarding",
		}); err != nil {
			return nil, &UsersError{Op: opOffboardStaff, Err: fmt.Errorf("audit absence %d before offboarding: %w", absence.ID, err)}
		}
	}
	deletedAbsences, err := s.StaffAbsenceRepo.DeleteNonHistoricalByStaffID(ctx, staffID, today)
	if err != nil {
		return nil, &UsersError{Op: opOffboardStaff, Err: err}
	}
	if deletedAbsences != int64(len(absences)) {
		return nil, &UsersError{
			Op:  opOffboardStaff,
			Err: fmt.Errorf("deleted %d absences after auditing %d", deletedAbsences, len(absences)),
		}
	}
	counts["staff_absences"] = deletedAbsences

	return counts, nil
}

// offboardPersonAndAccount unlinks the person from RFID card and account and
// revokes the account's access for the current tenant. The person row itself
// is kept so historical records keep resolving the staff member's name.
// Deletion counts for the audit record are added to counts.
func (s *staffOffboardingService) offboardPersonAndAccount(ctx context.Context, personID int64, counts map[string]any) error {
	person, err := s.PersonRepo.FindByID(ctx, personID)
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
		if err := s.PersonRepo.UnlinkFromRFIDCard(ctx, person.ID); err != nil {
			return &UsersError{Op: opOffboardStaff, Err: fmt.Errorf("unlink rfid card: %w", err)}
		}
	}

	if person.AccountID == nil {
		return nil
	}
	accountID := *person.AccountID

	// Remove tenant-scoped roles (also revokes this tenant's tokens per role).
	// The guardian role is kept: a dual-role teacher/guardian account must stay
	// able to log into the parents portal after losing staff access. Matching
	// by name mirrors the parent login gate (auth_login_parent.go); custom
	// roles that merely map to the guardian base role are removed because they
	// grant no parent access yet would keep the staff portal open.
	roles, err := s.RoleRepo.FindByAccountID(ctx, accountID)
	if err != nil {
		return &UsersError{Op: opOffboardStaff, Err: fmt.Errorf("load account roles: %w", err)}
	}
	hasGuardian := false
	for _, role := range roles {
		if strings.EqualFold(role.Name, authModels.BaseRoleGuardian) {
			hasGuardian = true
			continue
		}
		if err := s.AuthService.RemoveRoleFromAccount(ctx, int(accountID), int(role.ID)); err != nil {
			return &UsersError{Op: opOffboardStaff, Err: fmt.Errorf("remove role %q: %w", role.Name, err)}
		}
	}

	// Clear direct tenant-scoped permission grants. Re-invitation reactivates
	// the same account ID, so leftover grants would silently restore old
	// elevated permissions on a lower-privilege re-invite.
	permissionsDeleted, err := s.AccountPermissionRepo.DeleteByAccountID(ctx, accountID)
	if err != nil {
		return &UsersError{Op: opOffboardStaff, Err: fmt.Errorf("delete direct account permissions: %w", err)}
	}
	counts["account_permissions"] = permissionsDeleted

	// Free the partial unique index on persons.account_id so a re-invitation
	// can link a fresh person to the same account. Guardian portal access does
	// not depend on this link (guardians link via guardian_profiles.account_id).
	if err := s.PersonRepo.UnlinkFromAccount(ctx, person.ID); err != nil {
		return &UsersError{Op: opOffboardStaff, Err: fmt.Errorf("unlink account: %w", err)}
	}

	tenantID := tenant.FromContext(ctx)

	if hasGuardian {
		// Keep the tenant mapping and the account active so the parents portal
		// keeps working. Consequence: a staff re-invite for this email at this
		// school stays blocked (ErrAccountAlreadyHasTenantAccess) while the
		// guardian mapping is active — the same limitation any active guardian
		// account has today.
		s.getLogger().Info("staff account offboarded",
			"account_id", accountID,
			"tenant_id", tenantID,
			"account_deactivated", false,
			"guardian_access_preserved", true,
		)
		return nil
	}

	if err := s.AccountTenantRepo.Deactivate(ctx, accountID, tenantID); err != nil {
		return &UsersError{Op: opOffboardStaff, Err: fmt.Errorf("deactivate tenant mapping: %w", err)}
	}

	// Deactivate the account entirely (and revoke all tokens) only when it has
	// no active mapping to any other school.
	remaining, err := s.AccountTenantRepo.FindActiveByAccountID(ctx, accountID)
	if err != nil {
		return &UsersError{Op: opOffboardStaff, Err: fmt.Errorf("check remaining tenant mappings: %w", err)}
	}
	if len(remaining) == 0 {
		if err := s.AuthService.DeactivateAccount(ctx, int(accountID)); err != nil {
			return &UsersError{Op: opOffboardStaff, Err: fmt.Errorf("deactivate account: %w", err)}
		}
	}

	s.getLogger().Info("staff account offboarded",
		"account_id", accountID,
		"tenant_id", tenantID,
		"account_deactivated", len(remaining) == 0,
		"guardian_access_preserved", false,
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

	if err := s.DataDeletionRepo.Create(ctx, deletion); err != nil {
		return &UsersError{Op: opOffboardStaff, Err: fmt.Errorf("record data deletion: %w", err)}
	}
	return nil
}
