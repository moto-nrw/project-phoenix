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

// ErrAbsenceReadRequired is the denial for a caller holding users:absence
// without users:read. users:absence is a write scope layered on top of the
// child data a caller may already see — it is not a read permission and never
// unlocks one, so on its own it grants nothing (see CanManageStudentAbsence).
var ErrAbsenceReadRequired = errors.New("the users:read permission is required alongside users:absence")

// absenceOnlyAuthority reports whether the caller's claim on an absence action
// rests on users:absence alone: they do not hold users:update, the permission
// that gated every one of these writes before #2232 (admins satisfy it through
// their wildcard, HasPermission resolves that).
//
// It is what decides where the users:read prerequisite applies. A users:update
// holder keeps the pre-#2232 behavior untouched — that permission's own read
// requirements are not this change's business. A caller admitted purely by the
// new permission gets it on EVERY path, supervision included.
func absenceOnlyAuthority(userPermissions []string) bool {
	return !HasPermission(permissions.UsersUpdate, userPermissions) &&
		HasPermission(permissions.UsersAbsence, userPermissions)
}

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
//  0. A caller whose only claim is users:absence must also hold users:read —
//     checked BEFORE everything else, supervision included (see below).
//  1. Admin (admin:* / *:*) → allowed, as everywhere else.
//  2. The caller supervises the child's education group → allowed. This is the
//     unchanged fixed-groups behavior; a tenant on fixed_groups never reaches
//     any branch below.
//  3. group_mode == open_care AND the caller holds users:absence AND users:read
//     AND is a verified staff member of this tenant → allowed, regardless of
//     whether the child has a group at all.
//  4. Otherwise → denied, carrying the supervisor gate's own message so the
//     403 reason stays the familiar one wherever the mode is not open_care.
//
// The relaxation is scoped to this action on purpose. It grants nothing on the
// child's Stammdaten (address, health info, photo consent, Datenschutz): those
// keep running through CanModifyStudent, which open care does not touch.
//
// users:absence is a WRITE scope layered on top of the children a caller may
// already see; it is not a read permission and deliberately unlocks none. The
// absence flow needs the child's list entry and detail page to be usable at
// all, and those stay gated on users:read — widening them through a write
// permission would silently extend who may read student data. Requiring the
// pair here is what keeps the two ends consistent: users:absence alone opens
// nothing anywhere, instead of writing absences for a child its holder cannot
// open. The Betreuer role this permission ships to already holds users:read.
//
// That prerequisite is checked FIRST, ahead of the supervisor shortcut, and not
// only inside the open-care branch. The route gate admits users:absence on its
// own, so a supervising holder of that permission alone would otherwise pass
// through step 2 and write absences, decide guardian requests and see queue
// entries in a school on fixed groups — inheriting from supervision exactly
// what the pair is meant to deny them.
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
	if absenceOnlyAuthority(userPermissions) && !HasPermission(permissions.UsersRead, userPermissions) {
		return false, ErrAbsenceReadRequired
	}
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
	if !HasPermission(permissions.UsersRead, userPermissions) {
		return false, ErrAbsenceReadRequired
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
// the write, and they have to agree. That includes the read prerequisite: a
// caller admitted on users:absence alone, without users:read, sees no child
// here either — otherwise the queue would list entries the gate then refuses.
func AbsenceWritableStudentFilter(
	ctx context.Context,
	userPermissions []string,
	userCtx StudentModifyUserContext,
	settings StudentAbsenceSettings,
	logger *slog.Logger,
) func(*users.Student) bool {
	if HasAdminWildcard(userPermissions) {
		return WritableStudentFilter(ctx, userPermissions, userCtx)
	}
	if absenceOnlyAuthority(userPermissions) && !HasPermission(permissions.UsersRead, userPermissions) {
		return func(*users.Student) bool { return false }
	}
	supervised := WritableStudentFilter(ctx, userPermissions, userCtx)
	if !openCareMode(ctx, settings, logger) ||
		!HasPermission(permissions.UsersAbsence, userPermissions) ||
		!HasPermission(permissions.UsersRead, userPermissions) ||
		!isVerifiedStaff(ctx, userCtx) {
		return supervised
	}
	return func(student *users.Student) bool { return student != nil }
}

// CanReviewExcusedAbsenceRequests reports whether the caller may open and
// decide the parent excused-absence queue: users:update (the queue's original
// gate, shared with the Stammdaten queues next to it) or the users:absence +
// users:read pair (the same write under open care, #2232 — users:absence never
// counts on its own, exactly as in CanManageStudentAbsence).
//
// It exists so the read surfaces that merely ANNOUNCE a pending request — the
// day-planning badge carrying the parent's note in the student list/detail and
// in the OGS group live projection — cannot drift away from who may act on it.
// A badge shown to someone the queue then refuses is a note leaked to a reader
// with no say over it; a badge withheld from a decider hides work they own.
//
// Per-child scope is decided separately (AbsenceWritableStudentFilter /
// CanManageStudentAbsence); this is only the coarse permission question.
func CanReviewExcusedAbsenceRequests(userPermissions []string) bool {
	if HasPermission(permissions.UsersUpdate, userPermissions) {
		return true
	}
	return HasPermission(permissions.UsersAbsence, userPermissions) &&
		HasPermission(permissions.UsersRead, userPermissions)
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
