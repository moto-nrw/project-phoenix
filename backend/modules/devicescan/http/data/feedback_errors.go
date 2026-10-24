package data

import (
	"errors"
	"net/http"

	feedbackModule "github.com/moto-nrw/project-phoenix/modules/feedback"
)

// feedbackErrorStatus preserves the ordered Feedback error classification.
// Rendering the shared envelope belongs to the injected HTTP runtime.
func feedbackErrorStatus(err error) int {
	var invalid *feedbackModule.InvalidEntryDataError
	switch {
	case errors.As(err, &invalid):
		return http.StatusBadRequest
	case errors.Is(err, feedbackModule.ErrEntryNotFound):
		return http.StatusNotFound
	case errors.Is(err, feedbackModule.ErrInvalidEntryData):
		return http.StatusBadRequest
	case errors.Is(err, feedbackModule.ErrStudentNotFound):
		return http.StatusNotFound
	case errors.Is(err, feedbackModule.ErrInvalidDateRange):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}
