package students

import (
	"fmt"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
)

type arrivalSettingsResponse struct {
	CareDaysSource string `json:"care_days_source"`
}

func (rs *Resource) getArrivalSettings(w http.ResponseWriter, r *http.Request) {
	if rs.SettingsService == nil {
		renderError(w, r, common.ErrorInternalServer(fmt.Errorf("arrival settings service is not configured")))
		return
	}

	bookingsAuthoritative, err := rs.SettingsService.ResolveBool(
		r.Context(),
		configModel.KeyEnrollmentBookingsAuthoritative,
	)
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(fmt.Errorf("resolve arrival settings: %w", err)))
		return
	}

	careDaysSource := "weekly_plan"
	if bookingsAuthoritative {
		careDaysSource = "bookings"
	}
	common.Respond(w, r, http.StatusOK, arrivalSettingsResponse{
		CareDaysSource: careDaysSource,
	}, "Arrival settings retrieved successfully")
}
