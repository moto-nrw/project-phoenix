package common

import (
	"errors"
	"net/http"
)

// CaregiverCapabilityBlockedResponse names the People Directory blockers
// that keep an account's caregiver capability from being removed. The codes
// are People Directory's wire values; the HTTP adapters pass them through as
// strings so they need not import the owner's rows (#2736). The shared error
// envelope carries the status, detail and code; Blockers is an extension
// member next to it.
type CaregiverCapabilityBlockedResponse struct {
	*ErrResponse
	Blockers []string `json:"blockers"`
}

// NewCaregiverCapabilityBlockedResponse builds the refusal; an empty Code
// takes the class code of the status when it renders.
func NewCaregiverCapabilityBlockedResponse(
	httpStatusCode int,
	errorText string,
	blockers []string,
) *CaregiverCapabilityBlockedResponse {
	if httpStatusCode == 0 {
		httpStatusCode = http.StatusConflict
	}
	return &CaregiverCapabilityBlockedResponse{
		ErrResponse: &ErrResponse{Err: errors.New(errorText), HTTPStatusCode: httpStatusCode, Detail: errorText},
		Blockers:    blockers,
	}
}
