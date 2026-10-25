package grouplivehttp

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/grouplive"
)

// getOGSGroupLive handles GET /students/ogs-group-live?group_id={id}. The
// projection owns group authorization, response shaping, and the
// all-or-nothing sub-load contract; this boundary only parses input and maps
// domain errors.
func (rs *Resource) getOGSGroupLive(w http.ResponseWriter, r *http.Request) {
	requestedGroupID, ok := parseOptionalGroupID(w, r)
	if !ok {
		return
	}
	if rs.OGSGroupLiveService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("OGS group live service is not configured")))
		return
	}

	response, err := rs.OGSGroupLiveService.LiveGroup(r.Context(), requestedGroupID)
	if errors.Is(err, grouplive.ErrForbiddenGroup) {
		common.RenderError(w, r, common.ErrorForbidden(errors.New("you do not supervise this group")))
		return
	}
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("failed to build OGS group live projection", err))
		return
	}

	message := "OGS group live data retrieved successfully"
	if response.GroupID == nil {
		message = "No supervised groups"
	}
	common.Respond(w, r, http.StatusOK, response, message)
}

func parseOptionalGroupID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get("group_id"))
	if raw == "" {
		return 0, true
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid group_id")))
		return 0, false
	}
	return id, true
}

// Resource serves the Group Live projection at the host's students mount.
type Resource struct{ OGSGroupLiveService grouplive.Query }

func NewResource(query grouplive.Query) *Resource { return &Resource{OGSGroupLiveService: query} }

// Register mounts the live view; it also opens for students:read, the
// view-only child access of the role guest (ADR 0091).
func (rs *Resource) Register(r chi.Router, withTx common.Middleware) {
	r.With(common.RequiresAnyPermission(permissions.UsersRead, permissions.StudentsRead), withTx).Get("/ogs-group-live", rs.getOGSGroupLive)
	r.With(withTx).Get("/ogs-group-navigation", common.Fetch(func(ctx context.Context) ([]grouplive.Group, error) {
		if rs.OGSGroupLiveService == nil {
			return nil, errors.New("OGS group live service is not configured")
		}
		return rs.OGSGroupLiveService.ListGroups(ctx)
	}, common.ErrorInternalServer, "OGS group navigation retrieved successfully"))
}
