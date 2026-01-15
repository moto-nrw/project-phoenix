package groups

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// TransferGroupRequest represents a request to transfer group access to another user
type TransferGroupRequest struct {
	TargetUserID int64 `json:"target_user_id"`
}

// Bind validates the transfer group request
func (req *TransferGroupRequest) Bind(_ *http.Request) error {
	if req.TargetUserID <= 0 {
		return errors.New("target_user_id is required")
	}
	return nil
}

// validateGroupLeaderAccess ensures current user is a teacher who leads the specified group
func (rs *Resource) validateGroupLeaderAccess(w http.ResponseWriter, r *http.Request, groupID int64) (*users.Staff, *users.Teacher, bool) {
	currentStaff, err := rs.UserContextService.GetCurrentStaff(r.Context())
	if err != nil {
		//nolint:staticcheck // ST1005: German user-facing message
		common.RenderError(w, r, ErrorForbidden(errors.New("Du musst ein Mitarbeiter sein, um Gruppen zu übergeben")))
		return nil, nil, false
	}

	currentTeacher, err := rs.UserContextService.GetCurrentTeacher(r.Context())
	if err != nil {
		//nolint:staticcheck // ST1005: German user-facing message
		common.RenderError(w, r, ErrorForbidden(errors.New("Du musst ein Gruppenleiter sein, um Gruppen zu übergeben")))
		return nil, nil, false
	}

	isGroupLeader, err := rs.isUserGroupLeader(r.Context(), currentTeacher.ID, groupID)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return nil, nil, false
	}

	if !isGroupLeader {
		//nolint:staticcheck // ST1005: German user-facing message
		common.RenderError(w, r, ErrorForbidden(errors.New("Du bist kein Leiter dieser Gruppe. Nur der Original-Gruppenleiter kann Übertragungen vornehmen")))
		return nil, nil, false
	}

	return currentStaff, currentTeacher, true
}

// resolveTargetStaff validates and retrieves the target staff for a group transfer
func (rs *Resource) resolveTargetStaff(w http.ResponseWriter, r *http.Request, targetUserID int64) (*users.Person, *users.Staff, bool) {
	targetPerson, err := rs.UserService.Get(r.Context(), targetUserID)
	if err != nil {
		//nolint:staticcheck // ST1005: German user-facing message
		common.RenderError(w, r, ErrorNotFound(errors.New("Der ausgewählte Betreuer wurde nicht gefunden")))
		return nil, nil, false
	}

	targetStaff, err := rs.UserService.GetStaffByPersonID(r.Context(), targetPerson.ID)
	if err != nil {
		//nolint:staticcheck // ST1005: German user-facing message
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("Der ausgewählte Betreuer ist kein Mitarbeiter")))
		return nil, nil, false
	}

	return targetPerson, targetStaff, true
}

// translateTransferRequestError converts transfer request errors to German messages
func (rs *Resource) translateTransferRequestError(err error) string {
	if err.Error() == "target_user_id is required" {
		return "Bitte wähle einen Betreuer aus"
	}
	return "Ungültige Anfrage"
}

// checkDuplicateTransfer verifies target doesn't already have access to this group
func (rs *Resource) checkDuplicateTransfer(w http.ResponseWriter, r *http.Request, groupID int64, targetStaffID int64, targetPerson *users.Person) bool {
	today := time.Date(time.Now().UTC().Year(), time.Now().UTC().Month(), time.Now().UTC().Day(), 0, 0, 0, 0, time.UTC)
	existingTransfers, err := rs.EducationService.GetActiveGroupSubstitutions(r.Context(), groupID, today)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return false
	}

	for _, transfer := range existingTransfers {
		if transfer.RegularStaffID == nil && transfer.SubstituteStaffID == targetStaffID {
			targetName := targetPerson.FirstName + " " + targetPerson.LastName
			errorMsg := fmt.Sprintf("Du hast diese Gruppe bereits an %s übergeben", targetName)
			common.RenderError(w, r, ErrorInvalidRequest(errors.New(errorMsg)))
			return false
		}
	}
	return true
}

// transferGroup handles POST /api/groups/{id}/transfer
// Allows a group leader to grant temporary access to another user until end of day
func (rs *Resource) transferGroup(w http.ResponseWriter, r *http.Request) {
	groupID, ok := common.ParseInt64IDWithError(w, r, "id", "Ungültige Gruppen-ID")
	if !ok {
		return
	}

	req := &TransferGroupRequest{}
	if err := render.Bind(r, req); err != nil {
		errMsg := rs.translateTransferRequestError(err)
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsg)))
		return
	}

	currentStaff, _, ok := rs.validateGroupLeaderAccess(w, r, groupID)
	if !ok {
		return
	}

	targetPerson, targetStaff, ok := rs.resolveTargetStaff(w, r, req.TargetUserID)
	if !ok {
		return
	}

	if targetStaff.ID == currentStaff.ID {
		//nolint:staticcheck // ST1005: German user-facing message
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("Du kannst die Gruppe nicht an dich selbst übergeben")))
		return
	}

	if !rs.checkDuplicateTransfer(w, r, groupID, targetStaff.ID, targetPerson) {
		return
	}

	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	endOfDay := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, time.UTC)

	// Create substitution (without regular_staff_id = additional access, not replacement)
	substitution := &education.GroupSubstitution{
		GroupID:           groupID,
		RegularStaffID:    nil, // NULL = additional access, not replacement
		SubstituteStaffID: targetStaff.ID,
		StartDate:         today,
		EndDate:           endOfDay,
		Reason:            "Gruppenübergabe",
	}

	// Create the transfer via service
	// Note: Service allows overlapping substitutions (multiple groups per staff)
	if err := rs.EducationService.CreateSubstitution(r.Context(), substitution); err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusCreated, map[string]any{
		"substitution_id": substitution.ID,
		"group_id":        groupID,
		"target_staff_id": targetStaff.ID,
		"valid_until":     endOfDay.Format(time.RFC3339),
	}, "Group access transferred successfully")
}

// cancelSpecificTransfer handles DELETE /api/groups/{id}/transfer/{substitutionId}
// Allows a group leader to cancel a specific transfer by substitution ID
func (rs *Resource) cancelSpecificTransfer(w http.ResponseWriter, r *http.Request) {
	groupID, ok := common.ParseInt64IDWithError(w, r, "id", "Ungültige Gruppen-ID")
	if !ok {
		return
	}

	substitutionID, ok := common.ParseInt64IDWithError(w, r, "substitutionId", "Ungültige Substitutions-ID")
	if !ok {
		return
	}

	// Get current user's teacher record
	currentTeacher, err := rs.UserContextService.GetCurrentTeacher(r.Context())
	if err != nil {
		//nolint:staticcheck // ST1005: German user-facing message
		common.RenderError(w, r, ErrorForbidden(errors.New("Du musst ein Gruppenleiter sein, um Übertragungen zurückzunehmen")))
		return
	}

	// Verify that current user is a leader of this group
	isGroupLeader, err := rs.isUserGroupLeader(r.Context(), currentTeacher.ID, groupID)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	if !isGroupLeader {
		//nolint:staticcheck // ST1005: German user-facing message
		common.RenderError(w, r, ErrorForbidden(errors.New("Du bist kein Leiter dieser Gruppe. Nur der Original-Gruppenleiter kann Übertragungen zurücknehmen")))
		return
	}

	// Verify that the substitution exists and belongs to this group
	substitution, err := rs.EducationService.GetSubstitution(r.Context(), substitutionID)
	if err != nil {
		//nolint:staticcheck // ST1005: German user-facing message
		common.RenderError(w, r, ErrorNotFound(errors.New("Übertragung nicht gefunden")))
		return
	}

	// Verify it's a transfer (not admin substitution) and belongs to this group
	if substitution.RegularStaffID != nil {
		//nolint:staticcheck // ST1005: German user-facing message
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("Dies ist eine Admin-Vertretung und kann nicht hier gelöscht werden")))
		return
	}

	if substitution.GroupID != groupID {
		//nolint:staticcheck // ST1005: German user-facing message
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("Diese Übertragung gehört nicht zu dieser Gruppe")))
		return
	}

	// Delete the specific transfer
	if err := rs.EducationService.DeleteSubstitution(r.Context(), substitutionID); err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Transfer cancelled successfully")
}
