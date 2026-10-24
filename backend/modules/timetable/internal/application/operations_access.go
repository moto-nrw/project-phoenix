package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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

// requireCanView lets the operational overview read the rosters it lists
// without granting action rights; everybody else, and every other block,
// falls to requireCanOperate's people (#3874).
func (s *operations) requireCanView(ctx context.Context, accountID int64, isAdmin bool, instanceID int64) (int64, error) {
	staffID, hasStaff, err := s.resolveStaffID(ctx, accountID)
	if err != nil {
		return 0, err
	}
	overview, err := s.operationalOverview(ctx, isAdmin, hasStaff)
	if err != nil {
		return 0, err
	}
	if overview {
		inst, err := s.loadInstance(ctx, instanceID)
		if err != nil {
			return 0, err
		}
		if s.overviewReaches(inst) {
			return staffID, nil
		}
	}
	if s.hasAdministrativeActionAccess(ctx, isAdmin) {
		return staffID, nil
	}
	if !hasStaff {
		return 0, timetable.ErrTimetableOperationForbidden
	}
	return s.requireFixedGroupOperationAccess(ctx, staffID, instanceID)
}

// overviewReaches names the blocks the overview reads: running sessions and
// today's finished or expired blocks, which PlannedNow(scope=past) lists
// (#2335). Rosters of other days stay closed: ids are guessable and a roster
// carries pickup times.
func (s *operations) overviewReaches(inst *scheduleModels.ActivityInstance) bool {
	if inst.Status == scheduleModels.InstanceStatusActive && inst.ActiveGroupID != nil {
		return true
	}
	return s.overviewPastReaches(inst)
}

// overviewPastReaches is the historical part of the operational overview.
// Duties belong only in the day plan and must not be reached through a past
// roster URL.
func (s *operations) overviewPastReaches(inst *scheduleModels.ActivityInstance) bool {
	if inst.TemplateType == timetable.GroupTypeDuty {
		return false
	}
	now := s.now().In(calendar.Berlin)
	return calendar.Date(inst.Date) == calendar.DateFromTime(now) && plannedPastToday(inst, now)
}

// requireScopedAction gates attendance, starts and ends, which each have a
// school scope setting (#3180, #3622). all_staff adds every verified staff
// member to requireCanOperate's people for that one action; starting adds
// no supervisor.
func (s *operations) requireScopedAction(ctx context.Context, accountID int64, isAdmin bool, instanceID int64, action ports.ScopedAction) (int64, error) {
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
// ends reach running sessions, a start today's planned blocks that have not
// expired into the read-only past overview.
func (s *operations) scopeAdmits(action ports.ScopedAction, inst *scheduleModels.ActivityInstance) bool {
	if action == ports.ScopedBlockStart {
		now := s.now().In(calendar.Berlin)
		return inst.Status == scheduleModels.InstanceStatusPlanned &&
			calendar.Date(inst.Date) == calendar.DateFromTime(now) &&
			!plannedPastToday(inst, now)
	}
	return inst.Status == scheduleModels.InstanceStatusActive && inst.ActiveGroupID != nil
}

// schoolWideScope asks authorize.SchoolWideActionScope for the action's scope
// setting: its all_staff value counts only with the all_staff overview, so
// nobody acts on a block they cannot see. Portals keep their assignment
// boundary and admins their own rights.
func (s *operations) schoolWideScope(ctx context.Context, accountID int64, isAdmin bool, action ports.ScopedAction) (bool, error) {
	if s.isAssignmentBoundPortal(ctx) || s.hasAdministrativeActionAccess(ctx, isAdmin) || !s.isOGSActorToken(ctx, accountID) {
		return false, nil
	}
	key, err := s.deps.Settings.ActionScopeKey(action)
	if err != nil {
		return false, err
	}
	return s.caller.SchoolWideActionScope(ctx, s.deps.Settings, key)
}

// isOGSActorToken accepts a tenant or organisation session of the caller's
// own tenant with schedules:read.
func (s *operations) isOGSActorToken(ctx context.Context, accountID int64) bool {
	return s.caller.OGSActorToken(ctx, accountID)
}

// requireCanReportAbsence covers reporting a child sick or excused on the
// block. The block's operators may, subject to the direct-absence scope; so
// may a verified OGS actor under the all-staff absence scope.
func (s *operations) requireCanReportAbsence(ctx context.Context, accountID int64, isAdmin bool, instanceID int64) error {
	_, operationErr := s.requireCanOperate(ctx, accountID, isAdmin, instanceID)
	if operationErr == nil {
		return s.requireDirectAbsenceScope(ctx, isAdmin)
	}
	if !errors.Is(operationErr, timetable.ErrTimetableOperationForbidden) || s.isAssignmentBoundPortal(ctx) {
		return operationErr
	}
	if err := s.requireDirectAbsenceScope(ctx, isAdmin); err != nil {
		return err
	}
	_, hasStaff, err := s.resolveStaffID(ctx, accountID)
	if err != nil {
		return err
	}
	if !hasStaff || !s.isOGSActorToken(ctx, accountID) {
		return timetable.ErrTimetableOperationForbidden
	}
	overview, err := s.operationalOverview(ctx, isAdmin, hasStaff)
	if err != nil {
		return err
	}
	if overview {
		inst, err := s.loadInstance(ctx, instanceID)
		if err != nil {
			return err
		}
		if s.overviewPastReaches(inst) {
			return timetable.ErrTimetableOperationForbidden
		}
	}
	return nil
}

// requireDirectAbsenceScope keeps the school-portal and admin contracts;
// everybody else needs the all-staff absence scope.
func (s *operations) requireDirectAbsenceScope(ctx context.Context, isAdmin bool) error {
	if s.isAssignmentBoundPortal(ctx) || s.hasAdministrativeActionAccess(ctx, isAdmin) {
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
	if s.isAssignmentBoundPortal(ctx) {
		return false
	}
	return isAdmin || s.caller.AdministrativeAccess(ctx)
}

func (s *operations) requireFixedGroupOperationAccess(ctx context.Context, staffID, instanceID int64) (int64, error) {
	inst, err := s.loadInstance(ctx, instanceID)
	if err != nil {
		return 0, err
	}
	staffRows, err := staffOfInstance(ctx, s.deps.InstanceStaff, instanceID)
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
	if s.isAssignmentBoundPortal(ctx) {
		return calendar.Date(inst.Date) == s.today() && staffAssigned(staffRows, staffID), nil
	}
	if staffAssigned(staffRows, staffID) {
		return true, nil
	}
	if inst.ActiveGroupID == nil {
		return false, nil
	}
	return supervises()
}

func (s *operations) isAssignmentBoundPortal(ctx context.Context) bool {
	return s.caller.AssignmentBoundPortal(ctx)
}

// operationalOverview reports whether the caller may see every running
// block of the school (#2380) and today's finished ones. It reuses the resolved staff profile but asks
// the same setting as every other surface, so a block PlannedNow lists can
// never 403 on the detail call. Fails closed on a settings fault.
func (s *operations) operationalOverview(ctx context.Context, isAdmin, hasStaff bool) (bool, error) {
	allowed, err := s.caller.OperationalOverview(
		ctx,
		s.deps.Settings,
		s.isAssignmentBoundPortal(ctx),
		s.hasAdministrativeActionAccess(ctx, isAdmin),
		hasStaff,
	)
	if err != nil {
		return false, fmt.Errorf("%w: resolve operational overview: %w", timetable.ErrLifecycleSettings, err)
	}
	return allowed, nil
}
