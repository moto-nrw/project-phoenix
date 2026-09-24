package compose

import (
	"context"
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Who may act on a block (#3167, #2527): operators are the staff planned
// for it and, for a running block, its supervisors; admins act on every
// block of the tenant; an assignment-bound portal (the school portal) reaches
// only today's blocks it is planned for.

// requireCanOperate returns the caller's staff id when they may run the
// block's lifecycle and attendance.
func (s *operations) requireCanOperate(ctx context.Context, accountID int64, isAdmin bool, instanceID int64) (int64, error) {
	staffID, hasStaff, err := s.resolveStaffID(ctx, accountID)
	if err != nil {
		return 0, err
	}
	if s.hasAdministrativeActionAccess(ctx, isAdmin) {
		return staffID, nil
	}
	if !hasStaff {
		return 0, timetable.ErrTimetableOperationForbidden
	}
	return s.requireFixedGroupOperationAccess(ctx, staffID, instanceID)
}

// actionAllowed turns a require* check into a flag for read responses; only
// a denial becomes false, lookup failures still fail the request.
func actionAllowed(_ int64, err error) (bool, error) {
	if errors.Is(err, timetable.ErrTimetableOperationForbidden) {
		return false, nil
	}
	return err == nil, err
}

// requireCanView lets the all-staff operational overview read every running
// roster without granting action rights.
func (s *operations) requireCanView(ctx context.Context, accountID int64, isAdmin bool, instanceID int64) (int64, error) {
	staffID, hasStaff, err := s.resolveStaffID(ctx, accountID)
	if err != nil {
		return 0, err
	}
	if s.operationalOverview(ctx, isAdmin, hasStaff) {
		inst, err := s.loadInstance(ctx, instanceID)
		if err != nil {
			return 0, err
		}
		if inst.Status != scheduleModels.InstanceStatusActive || inst.ActiveGroupID == nil {
			return 0, timetable.ErrTimetableOperationForbidden
		}
		return staffID, nil
	}
	if !hasStaff {
		return 0, timetable.ErrTimetableOperationForbidden
	}
	return s.requireFixedGroupOperationAccess(ctx, staffID, instanceID)
}

// requireScopedAction gates attendance, starts and ends, which each have a
// school scope setting (#3180, #3622). all_staff adds every verified staff
// member to requireCanOperate's people for that one action; starting adds
// no supervisor.
func (s *operations) requireScopedAction(ctx context.Context, accountID int64, isAdmin bool, instanceID int64, action ScopedAction) (int64, error) {
	schoolWide, err := s.schoolWideScope(ctx, accountID, isAdmin, action)
	if err != nil {
		return 0, err
	}
	if schoolWide {
		staffID, hasStaff, err := s.resolveStaffID(ctx, accountID)
		if err != nil {
			return 0, err
		}
		inst, err := s.loadInstance(ctx, instanceID)
		if err != nil {
			return 0, err
		}
		if hasStaff && s.scopeAdmits(action, inst) {
			return staffID, nil
		}
	}
	return s.requireCanOperate(ctx, accountID, isAdmin, instanceID)
}

// scopeAdmits names the blocks a school-wide scope reaches: attendance and
// ends reach running sessions, a start today's planned blocks.
func (s *operations) scopeAdmits(action ScopedAction, inst *scheduleModels.ActivityInstance) bool {
	if action == ScopedBlockStart {
		return inst.Status == scheduleModels.InstanceStatusPlanned && timezone.Date(inst.Date) == s.today()
	}
	return inst.Status == scheduleModels.InstanceStatusActive && inst.ActiveGroupID != nil
}

// schoolWideScope asks authorize.SchoolWideActionScope for the action's scope
// setting: its all_staff value counts only with the all_staff overview, so
// nobody acts on a block they cannot see. Portals keep their assignment
// boundary and admins their own rights.
func (s *operations) schoolWideScope(ctx context.Context, accountID int64, isAdmin bool, action ScopedAction) (bool, error) {
	if isAssignmentBoundPortal(ctx) || s.hasAdministrativeActionAccess(ctx, isAdmin) || !isOGSActorToken(ctx, accountID) {
		return false, nil
	}
	key, err := s.deps.Settings.ActionScopeKey(action)
	if err != nil {
		return false, err
	}
	return authorize.SchoolWideActionScope(ctx, s.deps.Settings, key)
}

// isOGSActorToken accepts a tenant or organisation session of the caller's
// own tenant with schedules:read.
func isOGSActorToken(ctx context.Context, accountID int64) bool {
	claims := jwt.ClaimsFromCtx(ctx)
	return (claims.Scope == "" || claims.Scope == "tenant" || claims.Scope == "org") &&
		int64(claims.ID) == accountID && claims.TenantID > 0 && claims.TenantID == tenant.FromContext(ctx) &&
		authorize.HasPermission("schedules:read", jwt.PermissionsFromCtx(ctx))
}

// requireCanReportAbsence covers reporting a child sick or excused on the
// block. The block's operators may, subject to the direct-absence scope; so
// may a verified OGS actor under the all-staff absence scope.
func (s *operations) requireCanReportAbsence(ctx context.Context, accountID int64, isAdmin bool, instanceID int64) error {
	_, operationErr := s.requireCanOperate(ctx, accountID, isAdmin, instanceID)
	if operationErr == nil {
		return s.requireDirectAbsenceScope(ctx, isAdmin)
	}
	if !errors.Is(operationErr, timetable.ErrTimetableOperationForbidden) || isAssignmentBoundPortal(ctx) {
		return operationErr
	}
	if err := s.requireDirectAbsenceScope(ctx, isAdmin); err != nil {
		return err
	}
	_, hasStaff, err := s.resolveStaffID(ctx, accountID)
	if err == nil && (!hasStaff || !isOGSActorToken(ctx, accountID)) {
		return timetable.ErrTimetableOperationForbidden
	}
	return err
}

// requireDirectAbsenceScope keeps the school-portal and admin contracts;
// everybody else needs the all-staff absence scope.
func (s *operations) requireDirectAbsenceScope(ctx context.Context, isAdmin bool) error {
	if isAssignmentBoundPortal(ctx) || s.hasAdministrativeActionAccess(ctx, isAdmin) {
		return nil
	}
	allStaff, err := s.deps.Settings.StudentAbsenceEditAllStaff(ctx)
	if err != nil {
		return err
	}
	if !allStaff {
		return timetable.ErrTimetableOperationForbidden
	}
	return nil
}

func (s *operations) hasAdministrativeActionAccess(ctx context.Context, isAdmin bool) bool {
	if isAssignmentBoundPortal(ctx) {
		return false
	}
	claims := jwt.ClaimsFromCtx(ctx)
	return isAdmin || claims.IsAdmin || authorize.HasAdminWildcard(jwt.PermissionsFromCtx(ctx))
}

func (s *operations) requireFixedGroupOperationAccess(ctx context.Context, staffID, instanceID int64) (int64, error) {
	inst, err := s.loadInstance(ctx, instanceID)
	if err != nil {
		return 0, err
	}
	staffRows, err := s.deps.InstanceStaff.FindByInstanceID(ctx, instanceID)
	if err != nil {
		return 0, err
	}
	allowed, err := s.operatesLoadedBlock(ctx, inst, staffRows, staffID, func() (bool, error) {
		return s.supervises(ctx, *inst.ActiveGroupID, staffID)
	})
	if err != nil {
		return 0, err
	}
	if !allowed {
		return 0, timetable.ErrTimetableOperationForbidden
	}
	return staffID, nil
}

// supervises reports whether the staff member currently supervises the
// live session.
func (s *operations) supervises(ctx context.Context, activeGroupID, staffID int64) (bool, error) {
	supervisors, err := s.deps.Supervisions.FindByActiveGroupID(ctx, activeGroupID, true)
	if err != nil {
		return false, err
	}
	for _, supervisor := range supervisors {
		if supervisor.StaffID == staffID {
			return true, nil
		}
	}
	return false, nil
}

// operatesLoadedBlock is requireCanOperate's rule for a staff member once
// the block and its plan are loaded, shared by the single check and the bulk
// read (SessionBlocks). supervises is asked only when the plan does not
// decide and the block runs in a live session.
//
// An assignment-bound portal reaches TODAY and its concrete assignment and
// nothing else (#2527): ids are guessable, and without the clamp a Lehrkraft
// could pull a roster with pickup and emergency contacts for any block she
// is or was planned into. Starting a block adds its operator as supervisor,
// so the supervisor list would keep access after a withdrawn assignment.
func (s *operations) operatesLoadedBlock(ctx context.Context, inst *scheduleModels.ActivityInstance, staffRows []*scheduleModels.InstanceStaff, staffID int64, supervises func() (bool, error)) (bool, error) {
	if isAssignmentBoundPortal(ctx) {
		return timezone.Date(inst.Date) == s.today() && staffAssigned(staffRows, staffID), nil
	}
	if staffAssigned(staffRows, staffID) {
		return true, nil
	}
	if inst.ActiveGroupID == nil {
		return false, nil
	}
	return supervises()
}

func isAssignmentBoundPortal(ctx context.Context) bool {
	claims := jwt.ClaimsFromCtx(ctx)
	return claims.IsSchoolScope()
}

// operationalOverview reports whether the caller may see every running
// block of the school (#2380). It reuses the resolved staff profile but asks
// the same setting as every other surface, so a block PlannedNow lists can
// never 403 on the detail call. Fails closed on a settings fault.
func (s *operations) operationalOverview(ctx context.Context, isAdmin, hasStaff bool) bool {
	allowed, err := authorize.HasOperationalOverviewForResolvedStaff(
		ctx,
		s.deps.Settings,
		isAssignmentBoundPortal(ctx),
		s.hasAdministrativeActionAccess(ctx, isAdmin),
		hasStaff,
	)
	if err != nil {
		s.logger().WarnContext(ctx, "operational overview scope check failed for timetable operations",
			slog.String("error", err.Error()))
		return false
	}
	return allowed
}
