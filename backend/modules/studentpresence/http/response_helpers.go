package studentpresencehttp

import (
	json "encoding/json"

	studentpresence "github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// newPrivacyConsentResponse converts an owner consent record to a response.
// Details travels as the recorded JSON document and is decoded for the wire.
func newPrivacyConsentResponse(consent studentpresence.PrivacyConsent) (PrivacyConsentResponse, error) {
	var details map[string]interface{}
	if len(consent.Details) > 0 {
		if err := json.Unmarshal(consent.Details, &details); err != nil {
			return PrivacyConsentResponse{}, err
		}
	}
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
		Details:           details,
		CreatedAt:         consent.CreatedAt,
		UpdatedAt:         consent.UpdatedAt,
	}, nil
}
