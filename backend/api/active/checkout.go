package active

import (
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
)

// checkoutStudent handles immediate checkout of a student.
// This handler orchestrates the checkout workflow through focused helper functions.
func (rs *Resource) checkoutStudent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. Validate JWT token
	userClaims := jwt.ClaimsFromCtx(ctx)
	if userClaims.ID == 0 {
		common.RespondWithError(w, r, http.StatusUnauthorized, "Invalid token")
		return
	}

	// 2. Parse student ID from URL
	studentID, err := parseStudentIDFromRequest(r)
	if err != nil {
		common.RespondWithError(w, r, http.StatusBadRequest, "Invalid student ID")
		return
	}

	// 3. Get checkout context (visit + attendance status)
	checkoutCtx, err := rs.getCheckoutContext(ctx, studentID)
	if err != nil {
		rs.handleCheckoutContextError(w, r, err)
		return
	}

	// 4. Authorize the checkout operation
	staff, err := rs.authorizeStudentCheckout(ctx, userClaims, checkoutCtx)
	if err != nil {
		rs.handleAuthorizationError(w, r, err)
		return
	}

	// 5. Execute the checkout
	result, err := rs.executeStudentCheckout(ctx, staff, checkoutCtx)
	if err != nil {
		common.RespondWithError(w, r, http.StatusInternalServerError, "Failed to checkout student from daily attendance")
		return
	}

	// 6. Send success response
	common.RespondWithJSON(w, r, http.StatusOK, buildCheckoutResponse(studentID, result))
}

// handleCheckoutContextError maps context errors to appropriate HTTP responses
func (rs *Resource) handleCheckoutContextError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, ErrNotCheckedIn) {
		common.RespondWithError(w, r, http.StatusNotFound, "Student is not currently checked in")
		return
	}
	common.RespondWithError(w, r, http.StatusInternalServerError, "Failed to get attendance status")
}

// handleAuthorizationError maps authorization errors to appropriate HTTP responses
func (rs *Resource) handleAuthorizationError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, ErrNotAuthorized) {
		common.RespondWithError(w, r, http.StatusForbidden,
			"You are not authorized to checkout this student. You must be supervising their current room or be their group teacher.")
		return
	}
	common.RespondWithError(w, r, http.StatusInternalServerError, "Failed to get staff information")
}
