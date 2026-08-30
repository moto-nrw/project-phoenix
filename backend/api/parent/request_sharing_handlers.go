package parent

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
)

type requestSharingBody struct {
	RecipientGuardianProfileIDs []string `json:"recipient_guardian_profile_ids"`
}

type requestSharingRecipientResponse struct {
	GuardianProfileID string `json:"guardian_profile_id"`
	FirstName         string `json:"first_name"`
	LastName          string `json:"last_name"`
	Selected          bool   `json:"selected"`
}

type requestSharingResponse struct {
	FamilyProtected bool                              `json:"family_protected"`
	Recipients      []requestSharingRecipientResponse `json:"recipients"`
}

func (rs *Resource) getRequestSharingOptions(w http.ResponseWriter, r *http.Request) {
	if rs.RequestSharing == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("request sharing service is not configured")))
		return
	}
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := parsePathStudentID(w, r)
	if !ok {
		return
	}
	state, err := rs.RequestSharing.GetRequestSharingOptions(r.Context(), accountID, studentID)
	if err != nil {
		renderRequestSharingError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toRequestSharingResponse(state), "Request sharing options retrieved")
}

func (rs *Resource) getRequestSharing(w http.ResponseWriter, r *http.Request) {
	accountID, studentID, requestType, requestID, ok := rs.requestSharingParams(w, r)
	if !ok {
		return
	}
	state, err := rs.RequestSharing.GetRequestSharing(r.Context(), accountID, studentID, requestType, requestID)
	if err != nil {
		renderRequestSharingError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toRequestSharingResponse(state), "Request sharing retrieved")
}

func (rs *Resource) setRequestSharing(w http.ResponseWriter, r *http.Request) {
	accountID, studentID, requestType, requestID, ok := rs.requestSharingParams(w, r)
	if !ok {
		return
	}
	var body requestSharingBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, "request_sharing_invalid"))
		return
	}
	recipients, err := parseRecipientGuardianProfileIDs(body.RecipientGuardianProfileIDs)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, "request_sharing_invalid"))
		return
	}
	state, err := rs.RequestSharing.SetRequestSharing(r.Context(), accountID, studentID, requestType, requestID, recipients)
	if err != nil {
		renderRequestSharingError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toRequestSharingResponse(state), "Request sharing updated")
}

func (rs *Resource) requestSharingParams(
	w http.ResponseWriter, r *http.Request,
) (accountID, studentID int64, requestType string, requestID int64, ok bool) {
	if rs.RequestSharing == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("request sharing service is not configured")))
		return 0, 0, "", 0, false
	}
	accountID, ok = rs.parentAccountID(w, r)
	if !ok {
		return 0, 0, "", 0, false
	}
	studentID, ok = parsePathStudentID(w, r)
	if !ok {
		return 0, 0, "", 0, false
	}
	requestType = strings.TrimSpace(chi.URLParam(r, "requestType"))
	requestID, ok = common.ParsePositiveInt64IDWithError(w, r, "requestId", "invalid request id")
	return accountID, studentID, requestType, requestID, ok
}

func parseRecipientGuardianProfileIDs(raw []string) ([]int64, error) {
	ids := make([]int64, 0, len(raw))
	seen := make(map[int64]struct{}, len(raw))
	for _, value := range raw {
		id, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		if err != nil || id <= 0 {
			return nil, parentService.ErrRequestSharingInvalid
		}
		if _, exists := seen[id]; exists {
			return nil, parentService.ErrRequestSharingInvalid
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids, nil
}

func toRequestSharingResponse(state *parentService.RequestSharingState) requestSharingResponse {
	response := requestSharingResponse{FamilyProtected: state.FamilyProtected, Recipients: make([]requestSharingRecipientResponse, 0, len(state.Recipients))}
	for _, recipient := range state.Recipients {
		response.Recipients = append(response.Recipients, requestSharingRecipientResponse{
			GuardianProfileID: strconv.FormatInt(recipient.GuardianProfileID, 10),
			FirstName:         recipient.FirstName, LastName: recipient.LastName, Selected: recipient.Selected,
		})
	}
	return response
}

var requestSharingErrorRenderer = common.RulesRenderer([]common.ErrorRule{
	{Target: parentService.ErrRequestSharingInvalid, Render: func(err error) render.Renderer {
		return common.ErrorInvalidRequestWithCode(err, "request_sharing_invalid")
	}},
	{Target: parentService.ErrRequestSharingForbidden, Render: func(err error) render.Renderer {
		return common.ErrorForbiddenWithCode(err, "family_protection")
	}},
	{Target: parentService.ErrRequestSharingNotFound, Render: func(err error) render.Renderer {
		return common.ErrorNotFoundWithCode(err, "request_not_found")
	}},
	{Target: parentService.ErrChildNotLinked, Render: func(err error) render.Renderer {
		return common.ErrorNotFoundWithCode(err, "request_not_found")
	}},
}, common.ErrorInternalServer)

func renderRequestSharingError(w http.ResponseWriter, r *http.Request, err error) {
	common.RenderError(w, r, requestSharingErrorRenderer(err))
}
