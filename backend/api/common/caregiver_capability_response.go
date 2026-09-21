package common

import (
	"net/http"

	"github.com/go-chi/render"
)

// CaregiverCapabilityBlockedResponse names the People Directory blockers
// that keep an account's caregiver capability from being removed. The codes
// are People Directory's wire values; the HTTP adapters pass them through as
// strings so they need not import the owner's rows (#2736).
type CaregiverCapabilityBlockedResponse struct {
	HTTPStatusCode int      `json:"-"`
	Status         string   `json:"status"`
	ErrorText      string   `json:"error"`
	Blockers       []string `json:"blockers"`
}

func NewCaregiverCapabilityBlockedResponse(
	httpStatusCode int,
	errorText string,
	blockers []string,
) *CaregiverCapabilityBlockedResponse {
	status := "error"
	if httpStatusCode == 0 {
		httpStatusCode = http.StatusConflict
	}

	return &CaregiverCapabilityBlockedResponse{
		HTTPStatusCode: httpStatusCode,
		Status:         status,
		ErrorText:      errorText,
		Blockers:       blockers,
	}
}

func (e *CaregiverCapabilityBlockedResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, e.HTTPStatusCode)
	return nil
}
