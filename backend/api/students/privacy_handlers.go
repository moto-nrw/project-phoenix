package students

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// getStudentPrivacyConsent handles getting a student's privacy consent
func (rs *Resource) getStudentPrivacyConsent(w http.ResponseWriter, r *http.Request) {
	// Parse ID and get student
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return
	}

	// Check if user has permission to view this student's data
	hasFullAccess := rs.checkStudentFullAccess(r, student)
	if !hasFullAccess {
		renderError(w, r, ErrorForbidden(errors.New("insufficient permissions to access this student's data")))
		return
	}

	// Get privacy consents
	consents, err := rs.PrivacyConsentRepo.FindByStudentID(r.Context(), student.ID)
	if err != nil {
		renderError(w, r, ErrorInternalServer(err))
		return
	}

	// Find the most recent accepted consent
	var consent *users.PrivacyConsent
	for _, c := range consents {
		if c.Accepted && (consent == nil || c.CreatedAt.After(consent.CreatedAt)) {
			consent = c
		}
	}

	// If no consent exists, return a default response
	if consent == nil {
		response := PrivacyConsentResponse{
			StudentID:         student.ID,
			PolicyVersion:     "1.0",
			Accepted:          false,
			RenewalRequired:   true,
			DataRetentionDays: 30, // Default 30 days
		}
		common.Respond(w, r, http.StatusOK, response, "No privacy consent found, returning defaults")
		return
	}

	common.Respond(w, r, http.StatusOK, newPrivacyConsentResponse(consent), "Privacy consent retrieved successfully")
}

// findOrCreateConsent finds existing consent for a policy version or creates a new one
func findOrCreateConsent(consents []*users.PrivacyConsent, studentID int64, policyVersion string) *users.PrivacyConsent {
	var consent *users.PrivacyConsent
	for _, c := range consents {
		if c.PolicyVersion == policyVersion && (consent == nil || c.CreatedAt.After(consent.CreatedAt)) {
			consent = c
		}
	}

	if consent == nil {
		return &users.PrivacyConsent{StudentID: studentID}
	}
	return consent
}

// applyConsentUpdates updates consent fields from the request
func applyConsentUpdates(consent *users.PrivacyConsent, req *PrivacyConsentRequest) {
	consent.PolicyVersion = req.PolicyVersion
	consent.Accepted = req.Accepted
	consent.DurationDays = req.DurationDays
	consent.DataRetentionDays = req.DataRetentionDays
	consent.Details = req.Details

	if req.Accepted && consent.AcceptedAt == nil {
		now := time.Now()
		consent.AcceptedAt = &now
	}
}

// updateStudentPrivacyConsent handles updating a student's privacy consent
func (rs *Resource) updateStudentPrivacyConsent(w http.ResponseWriter, r *http.Request) {
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return
	}

	req := &PrivacyConsentRequest{}
	if err := render.Bind(r, req); err != nil {
		renderError(w, r, ErrorInvalidRequest(err))
		return
	}

	if !rs.checkStudentFullAccess(r, student) {
		renderError(w, r, ErrorForbidden(errors.New("insufficient permissions to update this student's data")))
		return
	}

	consents, err := rs.PrivacyConsentRepo.FindByStudentID(r.Context(), student.ID)
	if err != nil && !strings.Contains(err.Error(), "not found") {
		renderError(w, r, ErrorInternalServer(err))
		return
	}

	consent := findOrCreateConsent(consents, student.ID, req.PolicyVersion)
	applyConsentUpdates(consent, req)

	if err := consent.Validate(); err != nil {
		renderError(w, r, ErrorInvalidRequest(err))
		return
	}

	if consent.ID == 0 {
		err = rs.PrivacyConsentRepo.Create(r.Context(), consent)
	} else {
		err = rs.PrivacyConsentRepo.Update(r.Context(), consent)
	}

	if err != nil {
		renderError(w, r, ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, newPrivacyConsentResponse(consent), "Privacy consent updated successfully")
}
