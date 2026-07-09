package students

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/models/users"
	userService "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// getStudentPrivacyConsent handles getting a student's privacy consent
func (rs *Resource) getStudentPrivacyConsent(w http.ResponseWriter, r *http.Request) {
	// Parse ID and get student
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return
	}

	// Check if user has permission to view this student's data
	hasFullAccess := rs.checkStudentReadAccess(r, student)
	if !hasFullAccess {
		renderError(w, r, common.ErrorForbidden(errors.New("insufficient permissions to access this student's data")))
		return
	}

	// Get privacy consents
	consents, err := rs.StudentService.ListPrivacyConsents(r.Context(), student.ID)
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
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
		consentSvc := userService.NewPrivacyConsentService(rs.SettingsService, rs.Logger)
		response := PrivacyConsentResponse{
			StudentID:         student.ID,
			PolicyVersion:     "1.0",
			Accepted:          false,
			RenewalRequired:   true,
			DataRetentionDays: consentSvc.DefaultDataRetentionDays(r.Context()),
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

// applyConsentUpdates updates consent fields from the request. Acceptance
// stamping and expiry derivation are delegated to the privacy-consent service
// (issue #586, Rule 12: the consent lifecycle no longer lives on the model).
func (rs *Resource) applyConsentUpdates(consent *users.PrivacyConsent, req *PrivacyConsentRequest) {
	consent.PolicyVersion = req.PolicyVersion
	consent.Accepted = req.Accepted
	consent.DurationDays = req.DurationDays
	consent.DataRetentionDays = req.DataRetentionDays
	consent.Details = req.Details

	consentSvc := userService.NewPrivacyConsentService(rs.SettingsService, rs.Logger)
	if req.Accepted && consent.AcceptedAt == nil {
		consentSvc.Accept(consent, rs.Now())
	} else {
		consentSvc.DeriveExpiry(consent)
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
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	if !rs.checkStudentFullAccess(r, student) {
		renderError(w, r, common.ErrorForbidden(errors.New("insufficient permissions to update this student's data")))
		return
	}

	consents, err := rs.StudentService.ListPrivacyConsents(r.Context(), student.ID)
	if err != nil && !strings.Contains(err.Error(), "not found") {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}

	consent := findOrCreateConsent(consents, student.ID, req.PolicyVersion)
	rs.applyConsentUpdates(consent, req)

	if err := consent.Validate(); err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.DB, tenantID, func(ctx context.Context, _ bun.Tx) error {
		if consent.ID == 0 {
			return rs.StudentService.CreatePrivacyConsent(ctx, consent)
		}
		return rs.StudentService.UpdatePrivacyConsent(ctx, consent)
	}); err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, newPrivacyConsentResponse(consent), "Privacy consent updated successfully")
}
