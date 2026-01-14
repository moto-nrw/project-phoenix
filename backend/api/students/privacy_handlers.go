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

// PrivacyConsentResponse represents a privacy consent response
type PrivacyConsentResponse struct {
	ID                int64                  `json:"id"`
	StudentID         int64                  `json:"student_id"`
	PolicyVersion     string                 `json:"policy_version"`
	Accepted          bool                   `json:"accepted"`
	AcceptedAt        *time.Time             `json:"accepted_at,omitempty"`
	ExpiresAt         *time.Time             `json:"expires_at,omitempty"`
	DurationDays      *int                   `json:"duration_days,omitempty"`
	RenewalRequired   bool                   `json:"renewal_required"`
	DataRetentionDays int                    `json:"data_retention_days"`
	Details           map[string]interface{} `json:"details,omitempty"`
	CreatedAt         time.Time              `json:"created_at"`
	UpdatedAt         time.Time              `json:"updated_at"`
}

// PrivacyConsentRequest represents a privacy consent update request
type PrivacyConsentRequest struct {
	PolicyVersion     string                 `json:"policy_version"`
	Accepted          bool                   `json:"accepted"`
	DurationDays      *int                   `json:"duration_days,omitempty"`
	DataRetentionDays int                    `json:"data_retention_days"`
	Details           map[string]interface{} `json:"details,omitempty"`
}

// Bind validates the privacy consent request
func (req *PrivacyConsentRequest) Bind(_ *http.Request) error {
	if req.PolicyVersion == "" {
		return errors.New("policy version is required")
	}
	if req.DataRetentionDays < 1 || req.DataRetentionDays > 31 {
		return errors.New("data retention days must be between 1 and 31")
	}
	return nil
}

// newPrivacyConsentResponse converts a privacy consent model to a response
func newPrivacyConsentResponse(consent *users.PrivacyConsent) PrivacyConsentResponse {
	return PrivacyConsentResponse{
		ID:                consent.ID,
		StudentID:         consent.StudentID,
		PolicyVersion:     consent.PolicyVersion,
		Accepted:          consent.Accepted,
		AcceptedAt:        consent.AcceptedAt,
		ExpiresAt:         consent.ExpiresAt,
		DurationDays:      consent.DurationDays,
		RenewalRequired:   consent.RenewalRequired,
		DataRetentionDays: consent.DataRetentionDays,
		Details:           consent.Details,
		CreatedAt:         consent.CreatedAt,
		UpdatedAt:         consent.UpdatedAt,
	}
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
	consents, err := rs.StudentService.GetPrivacyConsent(r.Context(), student.ID)
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

	consents, err := rs.StudentService.GetPrivacyConsent(r.Context(), student.ID)
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
		err = rs.StudentService.CreatePrivacyConsent(r.Context(), consent)
	} else {
		err = rs.StudentService.UpdatePrivacyConsent(r.Context(), consent)
	}

	if err != nil {
		renderError(w, r, ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, newPrivacyConsentResponse(consent), "Privacy consent updated successfully")
}
