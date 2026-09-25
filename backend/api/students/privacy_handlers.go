package students

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// errPrivacyConsentsUnavailable is the bare-Resource outcome: without the
// owner capability the route refuses rather than reaching users.privacy_consents
// through a second path.
var errPrivacyConsentsUnavailable = errors.New("privacy consent capability is not configured")

// defaultDataRetentionDays is the retention window reported for a child with
// no recorded consent: the tenant's override when one exists, else the owner's
// default. The Settings Platform lookup lives here because this resource holds
// the settings service; the fallback rule belongs to Student Presence.
func (rs *Resource) defaultDataRetentionDays(ctx context.Context) int {
	return studentpresence.DataRetentionDaysOrDefault(resolveIntSetting(
		ctx, rs.SettingsService, settingPrivacyConsentRetentionDays, 0, rs.Logger))
}

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

	if rs.PrivacyConsents == nil {
		renderError(w, r, common.ErrorInternalServer(errPrivacyConsentsUnavailable))
		return
	}

	// Get privacy consents
	consents, err := rs.PrivacyConsents.ListPrivacyConsents(r.Context(), student.ID)
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Find the most recent accepted consent
	var consent *studentpresence.PrivacyConsent
	for i, c := range consents {
		if c.Accepted && (consent == nil || c.CreatedAt.After(consent.CreatedAt)) {
			consent = &consents[i]
		}
	}

	// If no consent exists, return a default response
	if consent == nil {
		response := PrivacyConsentResponse{
			StudentID:         student.ID,
			PolicyVersion:     "1.0",
			Accepted:          false,
			RenewalRequired:   true,
			DataRetentionDays: rs.defaultDataRetentionDays(r.Context()),
		}
		common.Respond(w, r, http.StatusOK, response, "No privacy consent found, returning defaults")
		return
	}

	response, err := newPrivacyConsentResponse(*consent)
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, response, "Privacy consent retrieved successfully")
}

// findOrCreateConsent finds existing consent for a policy version or creates a new one
func findOrCreateConsent(consents []studentpresence.PrivacyConsent, studentID int64, policyVersion string) studentpresence.PrivacyConsent {
	var consent *studentpresence.PrivacyConsent
	for i, c := range consents {
		if c.PolicyVersion == policyVersion && (consent == nil || c.CreatedAt.After(consent.CreatedAt)) {
			consent = &consents[i]
		}
	}

	if consent == nil {
		return studentpresence.PrivacyConsent{StudentID: studentID}
	}
	return *consent
}

// applyConsentUpdates updates consent fields from the request. Acceptance
// stamping and expiry derivation are decided by the owner of
// users.privacy_consents (issue #586, Rule 12: the consent lifecycle no longer
// lives on the model; #3349 moved it to Student Presence).
func applyConsentUpdates(consent studentpresence.PrivacyConsent, req *PrivacyConsentRequest, now time.Time) (studentpresence.PrivacyConsent, error) {
	details, err := json.Marshal(req.Details)
	if err != nil {
		return studentpresence.PrivacyConsent{}, err
	}
	consent.PolicyVersion = req.PolicyVersion
	consent.Accepted = req.Accepted
	consent.DurationDays = req.DurationDays
	consent.DataRetentionDays = req.DataRetentionDays
	consent.Details = details

	if req.Accepted && consent.AcceptedAt == nil {
		return studentpresence.AcceptPrivacyConsent(consent, now), nil
	}
	return studentpresence.DerivePrivacyConsentExpiry(consent), nil
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

	if rs.PrivacyConsents == nil {
		renderError(w, r, common.ErrorInternalServer(errPrivacyConsentsUnavailable))
		return
	}

	consents, err := rs.PrivacyConsents.ListPrivacyConsents(r.Context(), student.ID)
	if err != nil && !strings.Contains(err.Error(), "not found") {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}

	consent, err := applyConsentUpdates(findOrCreateConsent(consents, student.ID, req.PolicyVersion), req, rs.Now())
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.DB, tenantID, func(ctx context.Context, _ bun.Tx) error {
		var err error
		if consent.ID == 0 {
			consent, err = rs.PrivacyConsents.RecordPrivacyConsent(ctx, consent)
		} else {
			consent, err = rs.PrivacyConsents.RevisePrivacyConsent(ctx, consent)
		}
		return err
	}); err != nil {
		if errors.Is(err, studentpresence.ErrInvalidPrivacyConsent) {
			renderError(w, r, common.ErrorInvalidRequest(err))
			return
		}
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}

	response, err := newPrivacyConsentResponse(consent)
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, response, "Privacy consent updated successfully")
}
