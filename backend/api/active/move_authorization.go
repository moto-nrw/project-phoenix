package active

import (
	"context"
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
)

const bulkMoveForbiddenMessage = "not authorized to move the selected students"

type bulkMoveAuthorizationContext struct {
	supervisedGroups   map[int64]struct{}
	targetSupervised   bool
	attendanceStatuses map[int64]*activeSvc.AttendanceStatus
}

func (rs *Resource) bulkStudentMoveAuthorization(w http.ResponseWriter, r *http.Request) (*activeSvc.StudentMoveAuthorization, bool) {
	if canBypassBulkMoveResourceChecks(r) || rs.openCareMode(r.Context()) {
		return &activeSvc.StudentMoveAuthorization{BypassResourceChecks: true}, true
	}

	staff, err := rs.extractStaffFromRequest(w, r)
	if err != nil {
		return nil, false
	}

	return &activeSvc.StudentMoveAuthorization{StaffID: staff.ID}, true
}

func (rs *Resource) authorizeBulkStudentMove(w http.ResponseWriter, r *http.Request, studentIDs []int64, targetActiveGroupID *int64) bool {
	if canBypassBulkMoveResourceChecks(r) || rs.openCareMode(r.Context()) {
		return true
	}

	staff, err := rs.extractStaffFromRequest(w, r)
	if err != nil {
		return false
	}

	authz, targetAllowed, err := rs.prepareBulkMoveAuthorization(r.Context(), staff.ID, studentIDs, targetActiveGroupID)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return false
	}
	if !targetAllowed {
		common.RenderError(w, r, ErrorForbidden(errors.New(bulkMoveForbiddenMessage)))
		return false
	}

	studentsAllowed, err := rs.allStudentsMovable(r.Context(), staff.ID, uniquePositiveStudentIDs(studentIDs), authz)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return false
	}
	if !studentsAllowed {
		common.RenderError(w, r, ErrorForbidden(errors.New(bulkMoveForbiddenMessage)))
		return false
	}

	return true
}

func (rs *Resource) prepareBulkMoveAuthorization(ctx context.Context, staffID int64, studentIDs []int64, targetActiveGroupID *int64) (bulkMoveAuthorizationContext, bool, error) {
	supervisedGroups, err := rs.supervisedActiveGroupIDs(ctx, staffID)
	if err != nil {
		return bulkMoveAuthorizationContext{}, false, err
	}
	authz := bulkMoveAuthorizationContext{supervisedGroups: supervisedGroups}
	if targetActiveGroupID == nil {
		return authz, true, nil
	}

	canUseTarget, err := rs.staffCanUseMoveTarget(ctx, *targetActiveGroupID, supervisedGroups)
	if err != nil || !canUseTarget {
		return authz, canUseTarget, err
	}
	authz.targetSupervised = true
	authz.attendanceStatuses, err = rs.ActiveService.GetStudentsAttendanceStatuses(ctx, uniquePositiveStudentIDs(studentIDs))
	return authz, err == nil, err
}

func (rs *Resource) allStudentsMovable(ctx context.Context, staffID int64, studentIDs []int64, authz bulkMoveAuthorizationContext) (bool, error) {
	for _, studentID := range studentIDs {
		allowed, err := rs.staffCanMoveStudent(ctx, staffID, studentID, authz.supervisedGroups, authz.targetSupervised, authz.attendanceStatuses)
		if err != nil || !allowed {
			return allowed, err
		}
	}
	return true, nil
}

func canBypassBulkMoveResourceChecks(r *http.Request) bool {
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.IsAdmin {
		return true
	}
	return authorize.HasAdminWildcard(jwt.PermissionsFromCtx(r.Context()))
}

func (rs *Resource) supervisedActiveGroupIDs(ctx context.Context, staffID int64) (map[int64]struct{}, error) {
	supervisions, err := rs.ActiveService.GetStaffActiveSupervisions(ctx, staffID)
	if err != nil {
		return nil, err
	}

	ids := make(map[int64]struct{}, len(supervisions))
	for _, supervision := range supervisions {
		if supervision == nil || supervision.GroupID <= 0 {
			continue
		}
		ids[supervision.GroupID] = struct{}{}
	}
	return ids, nil
}

func (rs *Resource) staffCanUseMoveTarget(ctx context.Context, activeGroupID int64, supervisedGroups map[int64]struct{}) (bool, error) {
	targetGroup, err := rs.ActiveService.GetActiveGroup(ctx, activeGroupID)
	if err != nil {
		if errors.Is(err, activeSvc.ErrActiveGroupNotFound) {
			return false, nil
		}
		return false, err
	}
	if targetGroup == nil || !targetGroup.IsActive() {
		return false, nil
	}
	_, ok := supervisedGroups[activeGroupID]
	return ok, nil
}

func (rs *Resource) staffCanMoveStudent(
	ctx context.Context,
	staffID int64,
	studentID int64,
	supervisedGroups map[int64]struct{},
	targetSupervised bool,
	attendanceStatuses map[int64]*activeSvc.AttendanceStatus,
) (bool, error) {
	hasTeacherAccess, err := rs.ActiveService.CheckTeacherStudentAccess(ctx, staffID, studentID)
	if err != nil {
		return false, err
	}
	if hasTeacherAccess {
		return true, nil
	}

	currentVisit, err := rs.ActiveService.GetStudentCurrentVisit(ctx, studentID)
	if err != nil {
		if errors.Is(err, activeSvc.ErrVisitNotFound) {
			return targetSupervised && studentHasOpenAttendanceStatus(attendanceStatuses[studentID]), nil
		}
		return false, err
	}
	if currentVisit == nil {
		return targetSupervised && studentHasOpenAttendanceStatus(attendanceStatuses[studentID]), nil
	}

	_, supervisesCurrentRoom := supervisedGroups[currentVisit.ActiveGroupID]
	return supervisesCurrentRoom, nil
}

func studentHasOpenAttendanceStatus(status *activeSvc.AttendanceStatus) bool {
	if status == nil {
		return false
	}
	return status.Status == "checked_in" || status.Status == "on_yard"
}

func uniquePositiveStudentIDs(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	unique := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	return unique
}
