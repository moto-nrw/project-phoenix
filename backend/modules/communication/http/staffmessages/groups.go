package staffmessaging

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/moto-nrw/project-phoenix/api/common"
)

// Group chat handlers (#4776). Authorization lives in the service: membership
// for every route, plus creator or leadership for the managing ones.

type groupRequest struct {
	Name       string   `json:"name"`
	AccountIDs []string `json:"account_ids"`
}

// parseAccountIDs turns the wire's string ids into account ids.
func parseAccountIDs(raw []string) ([]int64, bool) {
	ids := make([]int64, 0, len(raw))
	for _, value := range raw {
		id, err := strconv.ParseInt(value, 10, 64)
		if err != nil || id <= 0 {
			return nil, false
		}
		ids = append(ids, id)
	}
	return ids, true
}

func decodeGroupRequest(w http.ResponseWriter, r *http.Request) (groupRequest, []int64, bool) {
	var body groupRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return body, nil, false
	}
	ids, ok := parseAccountIDs(body.AccountIDs)
	if !ok {
		common.RenderError(w, r, common.ErrorInvalidRequestMessage("invalid account id"))
		return body, nil, false
	}
	return body, ids, true
}

func (rs *Resource) createGroup(w http.ResponseWriter, r *http.Request) {
	body, ids, ok := decodeGroupRequest(w, r)
	if !ok {
		return
	}
	detail, err := rs.Service.CreateStaffMessageGroup(r.Context(), body.Name, ids)
	if err != nil {
		renderStaffMessagingError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, toThreadDetail(detail), "Group created")
}

func (rs *Resource) renameGroup(w http.ResponseWriter, r *http.Request) {
	threadID, ok := common.ParsePositiveInt64IDWithError(w, r, "threadID", "invalid thread ID")
	if !ok {
		return
	}
	body, _, ok := decodeGroupRequest(w, r)
	if !ok {
		return
	}
	detail, err := rs.Service.RenameStaffMessageGroup(r.Context(), threadID, body.Name)
	if err != nil {
		renderStaffMessagingError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toThreadDetail(detail), "Group renamed")
}

func (rs *Resource) addGroupMembers(w http.ResponseWriter, r *http.Request) {
	threadID, ok := common.ParsePositiveInt64IDWithError(w, r, "threadID", "invalid thread ID")
	if !ok {
		return
	}
	_, ids, ok := decodeGroupRequest(w, r)
	if !ok {
		return
	}
	detail, err := rs.Service.AddStaffMessageGroupMembers(r.Context(), threadID, ids)
	if err != nil {
		renderStaffMessagingError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toThreadDetail(detail), "Members added")
}

func (rs *Resource) removeGroupMember(w http.ResponseWriter, r *http.Request) {
	threadID, ok := common.ParsePositiveInt64IDWithError(w, r, "threadID", "invalid thread ID")
	if !ok {
		return
	}
	accountID, ok := common.ParsePositiveInt64IDWithError(w, r, "accountID", "invalid account ID")
	if !ok {
		return
	}
	detail, err := rs.Service.RemoveStaffMessageGroupMember(r.Context(), threadID, accountID)
	if err != nil {
		renderStaffMessagingError(w, r, err)
		return
	}
	if detail == nil {
		// The caller removed themselves: that is leaving, and the group is no
		// longer theirs to show.
		common.Respond(w, r, http.StatusOK, nil, "Group left")
		return
	}
	common.Respond(w, r, http.StatusOK, toThreadDetail(detail), "Member removed")
}

func (rs *Resource) leaveGroup(w http.ResponseWriter, r *http.Request) {
	threadID, ok := common.ParsePositiveInt64IDWithError(w, r, "threadID", "invalid thread ID")
	if !ok {
		return
	}
	// Leaving is removing oneself; the service authorizes membership.
	if _, err := rs.Service.RemoveStaffMessageGroupMember(r.Context(), threadID, currentAccountID(r)); err != nil {
		renderStaffMessagingError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, nil, "Group left")
}
