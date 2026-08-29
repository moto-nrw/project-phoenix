package authorize

import (
	"context"
	"errors"
)

const (
	usersUpdate  = "users:update"
	usersAbsence = "users:absence"
	usersRead    = "users:read"
)

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
// holder's read requirements are that permission's own business; a caller
// admitted purely by users:absence gets the prerequisite on every path.
func absenceOnlyAuthority(userPermissions []string) bool {
	return !HasPermission(usersUpdate, userPermissions) &&
		HasPermission(usersAbsence, userPermissions)
}

// CanManageStudentAbsence decides whether the caller may write a child's
// absence statuses — krank, entschuldigt, Klassenfahrt — for today or for
// planned days, and decide a guardian's excused/sick request.
//
// Since #2329 the per-child answer is the same as CanModifyStudent: admin or
// verified staff, for every child in the tenant. What remains distinct is the
// permission axis: the route gates admit users:absence as an alternative to
// users:update, so a custom role can be limited to absence writes without
// Stammdaten access. One prerequisite survives from #2232:
//
// users:absence is a WRITE scope layered on top of the children a caller may
// already see; it is not a read permission and deliberately unlocks none. The
// absence flow needs the child's list entry and detail page to be usable at
// all, and those stay gated on users:read — widening them through a write
// permission would silently extend who may read student data. Requiring the
// pair here is what keeps the two ends consistent: users:absence alone opens
// nothing anywhere. The Betreuer role this permission ships to already holds
// users:read.
func CanManageStudentAbsence(
	ctx context.Context,
	userPermissions []string,
	student authorizationStudent,
	userCtx StudentAccessUserContext,
) (bool, error) {
	if absenceOnlyAuthority(userPermissions) && !HasPermission(usersRead, userPermissions) {
		return false, ErrAbsenceReadRequired
	}
	return CanModifyStudent(ctx, userPermissions, student, userCtx, "update")
}

// AbsenceWritableStudentFilter is the set form of CanManageStudentAbsence,
// resolving the caller's verdict once so a caller-side loop — the
// excused-request review queue and its sidebar badge — does not re-resolve it
// per student.
//
// Callers that scope a queue with this MUST gate the corresponding write with
// CanManageStudentAbsence too: the filter decides visibility, the gate decides
// the write, and they have to agree. That includes the read prerequisite: a
// caller admitted on users:absence alone, without users:read, sees no child
// here either — otherwise the queue would list entries the gate then refuses.
func AbsenceWritableStudentFilter(
	ctx context.Context,
	userPermissions []string,
	userCtx StudentAccessUserContext,
) func(authorizationStudent) bool {
	if !HasAdminWildcard(userPermissions) &&
		absenceOnlyAuthority(userPermissions) &&
		!HasPermission(usersRead, userPermissions) {
		return func(authorizationStudent) bool { return false }
	}
	return WritableStudentFilter(ctx, userPermissions, userCtx)
}

// CanReviewExcusedAbsenceRequests reports whether the caller may open and
// decide the parent excused-absence queue: users:update (the queue's original
// gate, shared with the Stammdaten queues next to it) or the users:absence +
// users:read pair (#2232 — users:absence never counts on its own, exactly as
// in CanManageStudentAbsence).
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
	if HasPermission(usersUpdate, userPermissions) {
		return true
	}
	return HasPermission(usersAbsence, userPermissions) &&
		HasPermission(usersRead, userPermissions)
}
