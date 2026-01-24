package active

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
)

// ======== Unclaimed Groups Management (Deviceless Claiming) ========

// listUnclaimedGroups returns all active groups that have no supervisors
// This is used for deviceless rooms like Schulhof where teachers claim via frontend
func (rs *Resource) listUnclaimedGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := rs.ActiveService.GetUnclaimedActiveGroups(r.Context())
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, groups, "Unclaimed groups retrieved successfully")
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

	// Get authenticated user from JWT token
	claims := jwt.ClaimsFromCtx(ctx)
	if claims.ID == 0 {
		common.RespondWithError(w, r, http.StatusUnauthorized, "Invalid token")
		return
	}

	// Get person from account ID
	person, err := rs.PersonService.FindByAccountID(ctx, int64(claims.ID))
	if err != nil || person == nil {
		common.RespondWithError(w, r, http.StatusUnauthorized, "Account not found")
		return
	}

	// Get staff record from person
	staff, err := rs.PersonService.StaffRepository().FindByPersonID(ctx, person.ID)
	if err != nil || staff == nil {
		common.RespondWithError(w, r, http.StatusUnauthorized, "Staff authentication required")
		return
	}

	// Claim the group (default role: "supervisor")
	supervisor, err := rs.ActiveService.ClaimActiveGroup(ctx, groupID, staff.ID, "supervisor")
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, supervisor, "Successfully claimed supervision")
}
