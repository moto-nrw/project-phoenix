package students

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/api/common"
	ogsGroupLiveService "github.com/moto-nrw/project-phoenix/services/ogsgrouplive"
)

// Compatibility aliases keep the endpoint's documented response names local
// to the API package while the projection and its GDPR contract live in the
// service layer.
type OGSLiveGroupResponse = ogsGroupLiveService.Group
type OGSLiveStudentResponse = ogsGroupLiveService.Student
type OGSLiveRoomStatus = ogsGroupLiveService.RoomStatus
type OGSLiveTransferResponse = ogsGroupLiveService.Transfer
type OGSLiveTrackingIndicators = ogsGroupLiveService.TrackingIndicators
type OGSGroupLiveResponse = ogsGroupLiveService.Projection

// getOGSGroupLive handles GET /students/ogs-group-live?group_id={id}. The
// service owns group authorization, projection shaping, and the all-or-nothing
// sub-load contract; this boundary only parses input and maps domain errors.
func (rs *Resource) getOGSGroupLive(w http.ResponseWriter, r *http.Request) {
	requestedGroupID, ok := parseOptionalGroupID(w, r)
	if !ok {
		return
	}
	if rs.OGSGroupLiveService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("OGS group live service is not configured")))
		return
	}

	response, err := rs.OGSGroupLiveService.Get(r.Context(), requestedGroupID)
	if errors.Is(err, ogsGroupLiveService.ErrForbiddenGroup) {
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
