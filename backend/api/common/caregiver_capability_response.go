package common

import (
	"net/http"

	"github.com/go-chi/render"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

type CaregiverCapabilityBlockedResponse struct {
	HTTPStatusCode int                                         `json:"-"`
	Status         string                                      `json:"status"`
	ErrorText      string                                      `json:"error"`
	Blockers       []userModels.CaregiverCapabilityBlockerCode `json:"blockers"`
}

func NewCaregiverCapabilityBlockedResponse(
	httpStatusCode int,
	errorText string,
	blockers []userModels.CaregiverCapabilityBlockerCode,
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
