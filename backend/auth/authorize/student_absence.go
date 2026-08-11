package authorize

import (
	"context"
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// StudentAbsenceSettings is the narrow subset of the settings service the
// absence gate needs: resolve the tenant's organisational group mode.
// Declared locally (not imported) for the same reason as StudentReadSettings —
// services/config already imports this package.
type StudentAbsenceSettings interface {
	ResolveString(ctx context.Context, key string) (string, error)
}

// ErrAbsencePermissionRequired is the open-care denial: the tenant runs
// without fixed groups, so supervision cannot decide the write, and the caller
// does not hold users:absence.
var ErrAbsencePermissionRequired = errors.New("the users:absence permission is required to manage this student's absences")

// ErrAbsenceStaffRequired is the open-care denial for a caller who holds the
// permission but has no staff record in this tenant (guest/guardian accounts).
var ErrAbsenceStaffRequired = errors.New("only staff members can manage student absences")

// CanManageStudentAbsence decides whether the caller may write a child's
// absence statuses — krank, entschuldigt, Klassenfahrt — for today or for
// planned days, and decide a guardian's excused/sick request.
//
// It is deliberately NOT CanModifyStudent: that gate answers "may this caller
// edit the child's row", which in a school running
// operations.group_mode = open_care is admin-only for every child, because
// there are no groups and therefore no supervision to derive authority from
// (#2232). Absence is the one write such a school still needs from ordinary
// staff, so it gets its own action-scoped decision:
//
//  1. Admin (admin:* / *:*) → allowed, as everywhere else.
//  2. The caller supervises the child's education group → allowed. This is the
//     unchanged fixed-groups behavior; a tenant on fixed_groups never reaches
//     any branch below.
//  3. group_mode == open_care AND the caller holds users:absence AND is a
//     verified staff member of this tenant → allowed, regardless of whether
//     the child has a group at all.
//  4. Otherwise → denied, carrying the supervisor gate's own message so the
//     403 reason stays the familiar one wherever the mode is not open_care.
//
// The relaxation is scoped to this action on purpose. It grants nothing on the
// child's Stammdaten (address, health info, photo consent, Datenschutz): those
// keep running through CanModifyStudent, which open care does not touch.
//
// Resolution fails CLOSED: a settings error is an operational fault, never a
// tenant choice, so it leaves the caller with the supervisor gate's verdict.
func CanManageStudentAbsence(
	ctx context.Context,
	userPermissions []string,
	student *users.Student,
	userCtx StudentModifyUserContext,
	settings StudentAbsenceSettings,
	logger *slog.Logger,
) (bool, error) {
	supervisorOK, supervisorErr := CanModifyStudent(ctx, userPermissions, student, userCtx, "update")
	if supervisorOK {
		return true, nil
	}
	if student == nil {
		return false, supervisorErr
	}
	if !openCareMode(ctx, settings, logger) {
		return false, supervisorErr
	}
	if !HasPermission(permissions.UsersAbsence, userPermissions) {
		return false, ErrAbsencePermissionRequired
	}
	if !isVerifiedStaff(ctx, userCtx) {
		return false, ErrAbsenceStaffRequired
	}
	return true, nil
}

// AbsenceWritableStudentFilter is the set form of CanManageStudentAbsence,
// precomputing the caller's supervised-group set (and the open-care verdict)
// once so a caller-side loop — the excused-request review queue and its
// sidebar badge — does not re-resolve them per student.
//
// Callers that scope a queue with this MUST gate the corresponding write with
// CanManageStudentAbsence too: the filter decides visibility, the gate decides
// the write, and they have to agree.
func AbsenceWritableStudentFilter(
	ctx context.Context,
	userPermissions []string,
	userCtx StudentModifyUserContext,
	settings StudentAbsenceSettings,
	logger *slog.Logger,
) func(*users.Student) bool {
	supervised := WritableStudentFilter(ctx, userPermissions, userCtx)
	if HasAdminWildcard(userPermissions) {
		return supervised
	}
	if !openCareMode(ctx, settings, logger) ||
		!HasPermission(permissions.UsersAbsence, userPermissions) ||
		!isVerifiedStaff(ctx, userCtx) {
		return supervised
	}
	return func(student *users.Student) bool { return student != nil }
}

// openCareMode reports whether the tenant runs without fixed groups. Mirrors
// api/active.openCareMode: a resolution failure is logged and read as
// fixed_groups, never as the permissive mode.
func openCareMode(ctx context.Context, settings StudentAbsenceSettings, logger *slog.Logger) bool {
	if settings == nil {
		return false
	}
	mode, err := settings.ResolveString(ctx, configModel.KeyGroupMode)
	if err != nil {
		if logger != nil {
			logger.WarnContext(ctx, "group mode resolution failed in absence gate",
				slog.String("key", configModel.KeyGroupMode),
				slog.String("error", err.Error()),
			)
		}
		return false
	}
	return mode == configModel.GroupModeOpenCare
}

// isVerifiedStaff reports whether the caller has a staff record in the current
// tenant. Guests and guardians authenticate against the same tenant portal, so
// the open-care branch checks this rather than trusting the permission alone.
func isVerifiedStaff(ctx context.Context, userCtx StudentModifyUserContext) bool {
	if userCtx == nil {
		return false
	}
	staff, err := userCtx.GetCurrentStaff(ctx)
	return err == nil && staff != nil
}
