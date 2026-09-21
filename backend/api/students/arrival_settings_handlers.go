package students

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/moto-nrw/project-phoenix/api/common"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
)

// schoolPeriodResponse is one lesson the school has an end time for (#3372).
type schoolPeriodResponse struct {
	Period  int    `json:"period"`
	EndTime string `json:"end_time"`
}

type arrivalSettingsResponse struct {
	CareDaysSource string `json:"care_days_source"`
	// SchoolPeriods lists the lessons with a maintained end time, in lesson
	// order. Empty when the school maintains none; the forms then offer no
	// lesson choice.
	SchoolPeriods []schoolPeriodResponse `json:"school_periods"`
	// DefaultArrivalTime and DefaultPickupTime are the usual clock times the
	// weekly plan offers for one-click adoption (#3371). Empty offers nothing.
	DefaultArrivalTime string `json:"default_arrival_time"`
	DefaultPickupTime  string `json:"default_pickup_time"`
}

func (rs *Resource) getArrivalSettings(w http.ResponseWriter, r *http.Request) {
	if rs.SettingsService == nil {
		renderError(w, r, common.ErrorInternalServer(fmt.Errorf("arrival settings service is not configured")))
		return
	}

	keys := []string{
		configModel.KeyEnrollmentBookingsAuthoritative,
		configModel.KeyCareDefaultArrivalTime,
		configModel.KeyCareDefaultPickupTime,
	}
	for period := 1; period <= configModel.SchoolPeriodCount; period++ {
		keys = append(keys, configModel.SchoolPeriodEndKey(period))
	}
	ctx := common.PrefetchSettings(r.Context(), rs.SettingsService, keys...)

	bookingsAuthoritative, err := rs.SettingsService.ResolveBool(
		ctx,
		configModel.KeyEnrollmentBookingsAuthoritative,
	)
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(fmt.Errorf("resolve arrival settings: %w", err)))
		return
	}

	periods := make([]schoolPeriodResponse, 0, configModel.SchoolPeriodCount)
	for period := 1; period <= configModel.SchoolPeriodCount; period++ {
		endTime, resolveErr := rs.SettingsService.ResolveString(ctx, configModel.SchoolPeriodEndKey(period))
		if resolveErr != nil {
			renderError(w, r, common.ErrorInternalServer(fmt.Errorf("resolve school period %d: %w", period, resolveErr)))
			return
		}
		if endTime = strings.TrimSpace(endTime); endTime != "" {
			periods = append(periods, schoolPeriodResponse{Period: period, EndTime: endTime})
		}
	}

	presets := make(map[string]string, 2)
	for _, key := range []string{configModel.KeyCareDefaultArrivalTime, configModel.KeyCareDefaultPickupTime} {
		value, resolveErr := rs.SettingsService.ResolveString(ctx, key)
		if resolveErr != nil {
			renderError(w, r, common.ErrorInternalServer(fmt.Errorf("resolve %s: %w", key, resolveErr)))
			return
		}
		presets[key] = strings.TrimSpace(value)
	}

	careDaysSource := "weekly_plan"
	if bookingsAuthoritative {
		careDaysSource = "bookings"
	}
	common.Respond(w, r, http.StatusOK, arrivalSettingsResponse{
		CareDaysSource:     careDaysSource,
		SchoolPeriods:      periods,
		DefaultArrivalTime: presets[configModel.KeyCareDefaultArrivalTime],
		DefaultPickupTime:  presets[configModel.KeyCareDefaultPickupTime],
	}, "Arrival settings retrieved successfully")
}
