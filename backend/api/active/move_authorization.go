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

func (rs *Resource) bulkStudentMoveAuthorization(w http.ResponseWriter, r *http.Request) (*activeSvc.StudentMoveAuthorization, bool) {
	if canBypassBulkMoveResourceChecks(r) {
		return &activeSvc.StudentMoveAuthorization{BypassResourceChecks: true}, true
	}

	staff, err := rs.extractStaffFromRequest(w, r)
	if err != nil {
		return nil, false
	}

	return &activeSvc.StudentMoveAuthorization{StaffID: staff.ID}, true
}

func (rs *Resource) authorizeBulkStudentMove(w http.ResponseWriter, r *http.Request, studentIDs []int64, targetActiveGroupID *int64) bool {
	if canBypassBulkMoveResourceChecks(r) {
		return true
	}

	staff, err := rs.extractStaffFromRequest(w, r)
	if err != nil {
		return false
	}

	targetAllowed, err := rs.bulkMoveTargetAllowed(r.Context(), staff.ID, targetActiveGroupID)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return false
	}
	if !targetAllowed {
		common.RenderError(w, r, ErrorForbidden(errors.New(bulkMoveForbiddenMessage)))
		return false
	}

	// Any verified staff member may move any student (#2329); only the move
	// TARGET must be an active group the caller currently supervises.
	return true
}

func (rs *Resource) bulkMoveTargetAllowed(ctx context.Context, staffID int64, targetActiveGroupID *int64) (bool, error) {
	if targetActiveGroupID == nil {
		return true, nil
	}
	supervisedGroups, err := rs.supervisedActiveGroupIDs(ctx, staffID)
	if err != nil {
		return false, err
	}
	return rs.staffCanUseMoveTarget(ctx, *targetActiveGroupID, supervisedGroups)
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
