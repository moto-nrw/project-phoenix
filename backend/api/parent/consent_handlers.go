package parent

import (
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
)

// ChildConsentResponse is the wire projection for one stored consent or
// acknowledgement. Only the voluntary photo consent can be withdrawn.
type ChildConsentResponse struct {
	Key         string  `json:"key"`
	State       string  `json:"state"`
	ChangedAt   *string `json:"changed_at,omitempty"`
	CanWithdraw bool    `json:"can_withdraw"`
}

func toChildConsentResponses(consents []parentService.ChildConsent) []ChildConsentResponse {
	responses := make([]ChildConsentResponse, 0, len(consents))
	for _, consent := range consents {
		var changedAt *string
		if consent.ChangedAt != nil {
			formatted := consent.ChangedAt.Format("2006-01-02T15:04:05Z07:00")
			changedAt = &formatted
		}
		responses = append(responses, ChildConsentResponse{
			Key:         consent.Key,
			State:       consent.State,
			ChangedAt:   changedAt,
			CanWithdraw: consent.CanWithdraw,
		})
	}
	return responses
}

func (rs *Resource) getChildConsents(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := parsePathStudentID(w, r)
	if !ok {
		return
	}

	consents, err := rs.ParentService.GetChildConsents(r.Context(), accountID, studentID)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toChildConsentResponses(consents), "Consents retrieved")
}

func (rs *Resource) withdrawPhotoConsent(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := parsePathStudentID(w, r)
	if !ok {
		return
	}

	consents, err := rs.ParentService.WithdrawPhotoConsent(r.Context(), accountID, studentID)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toChildConsentResponses(consents), "Photo consent withdrawn")
}
