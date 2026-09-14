package active

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/common"
)

// ======== Unclaimed Groups Management (Deviceless Claiming) ========

// listUnclaimedGroups returns all active groups that have no supervisors
// This is used for deviceless rooms like Schulhof where teachers claim via frontend
func (rs *Resource) listUnclaimedGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := rs.Operations.UnclaimedSessions(r.Context())
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	responses := make([]unclaimedSessionResponse, 0, len(groups))
	for _, group := range groups {
		responses = append(responses, newUnclaimedSessionResponse(group))
	}

	common.Respond(w, r, http.StatusOK, responses, "Unclaimed groups retrieved successfully")
}

// claimGroup allows authenticated staff to claim supervision of an active group
func (rs *Resource) claimGroup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get group ID from URL
	groupIDStr := chi.URLParam(r, "id")
	groupID, err := strconv.ParseInt(groupIDStr, 10, 64)
	if err != nil {
		common.RespondWithError(w, r, http.StatusBadRequest, "Invalid group ID")
		return
	}

	// Read the authenticated account from the validated security principal.
	principal, principalErr := common.CurrentPrincipal(ctx)
	if principalErr != nil {
		common.RespondWithError(w, r, http.StatusUnauthorized, "Invalid token")
		return
	}

	// Get person from account ID
	person, err := rs.PersonService.FindByAccountID(ctx, principal.AccountID())
	if err != nil || person == nil {
		common.RespondWithError(w, r, http.StatusUnauthorized, "Account not found")
		return
	}

	// Get staff record from person
	staff, err := rs.PersonService.GetStaffByPersonID(ctx, person.ID)
	if err != nil || staff == nil {
		common.RespondWithError(w, r, http.StatusUnauthorized, "Staff authentication required")
		return
	}

	// Claim the group (default role: "supervisor")
	supervisor, err := rs.Operations.ClaimSupervision(ctx, groupID, staff.ID, "supervisor")
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, newClaimedSupervisionResponse(supervisor), "Successfully claimed supervision")
}
