package parent

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/moto-nrw/project-phoenix/api/common"
)

// relatedAccountResponse is the parent-facing projection of a linked guardian.
type relatedAccountResponse struct {
	GuardianProfileID string `json:"guardian_profile_id"`
	FirstName         string `json:"first_name"`
	LastName          string `json:"last_name"`
	Email             string `json:"email,omitempty"`
	RelationshipType  string `json:"relationship_type"`
	// GuardianRole lets the panel hide the grant-access action for
	// school-managed social-worker contacts (#2172).
	GuardianRole string `json:"guardian_role,omitempty"`
	IsPrimary    bool   `json:"is_primary"`
	Status       string `json:"status"`
	// IsSelf marks the requesting parent's own row; the UI hides the remove
	// action for it since self-removal is rejected by the backend.
	IsSelf bool `json:"is_self"`
}

// inviteRelatedAccountRequest is the wire shape for POST
// /parent/me/children/{studentId}/related-accounts.
type inviteRelatedAccountRequest struct {
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	// ConfirmRoleUpgrade confirms upgrading an existing restrictive contact
	// link to full access (#2172).
	ConfirmRoleUpgrade bool `json:"confirm_role_upgrade"`
}

// inviteRelatedAccountResponse echoes the resolve outcome.
type inviteRelatedAccountResponse struct {
	Outcome           string `json:"outcome"`
	GuardianProfileID string `json:"guardian_profile_id"`
	// ExistingRole carries the current guardian role for the
	// existing_contact_restricted outcome.
	ExistingRole string `json:"existing_role,omitempty"`
}

// listRelatedAccounts returns the guardians linked to the parent's child with
// portal-access status. Ownership is verified in the service.
func (rs *Resource) listRelatedAccounts(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := parsePathStudentID(w, r)
	if !ok {
		return
	}

	accounts, err := rs.ParentService.ListRelatedAccounts(r.Context(), accountID, studentID)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}

	out := make([]relatedAccountResponse, 0, len(accounts))
	for _, a := range accounts {
		out = append(out, relatedAccountResponse{
			GuardianProfileID: strconv.FormatInt(a.GuardianProfileID, 10),
			FirstName:         a.FirstName,
			LastName:          a.LastName,
			Email:             a.Email,
			RelationshipType:  a.RelationshipType,
			GuardianRole:      a.GuardianRole,
			IsPrimary:         a.IsPrimary,
			Status:            string(a.Status),
			IsSelf:            a.IsSelf,
		})
	}
	common.Respond(w, r, http.StatusOK, out, "Related accounts")
}

// inviteRelatedAccount invites a further guardian to the parent's child.
func (rs *Resource) inviteRelatedAccount(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := parsePathStudentID(w, r)
	if !ok {
		return
	}

	var body inviteRelatedAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}

	result, err := rs.ParentService.InviteRelatedAccount(r.Context(), accountID, studentID, body.Email, body.FirstName, body.LastName, body.ConfirmRoleUpgrade)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}

	common.Respond(w, r, http.StatusCreated, inviteRelatedAccountResponse{
		Outcome:           result.Outcome,
		GuardianProfileID: strconv.FormatInt(result.GuardianProfileID, 10),
		ExistingRole:      result.ExistingRole,
	}, "Guardian invited")
}

// removeRelatedAccount removes another account's access to the parent's child.
func (rs *Resource) removeRelatedAccount(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := parsePathStudentID(w, r)
	if !ok {
		return
	}
	guardianProfileID, ok := common.ParsePositiveInt64IDWithError(w, r, "guardianProfileId", "invalid guardian profile id")
	if !ok {
		return
	}

	if err := rs.ParentService.RemoveRelatedAccount(r.Context(), accountID, studentID, guardianProfileID); err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, nil, "Access removed")
}
