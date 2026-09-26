package students

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/moto-nrw/project-phoenix/api/common"
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
		settingEnrollmentBookingsAuthoritative,
		settingCareDefaultArrivalTime,
		settingCareDefaultPickupTime,
	}
	for period := 1; period <= schoolPeriodCount; period++ {
		keys = append(keys, schoolPeriodEndSetting(period))
	}
	ctx := common.PrefetchSettings(r.Context(), rs.SettingsService, keys...)

	bookingsAuthoritative, err := rs.SettingsService.ResolveBool(
		ctx,
		settingEnrollmentBookingsAuthoritative,
	)
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(fmt.Errorf("resolve arrival settings: %w", err)))
		return
	}

	periods := make([]schoolPeriodResponse, 0, schoolPeriodCount)
	for period := 1; period <= schoolPeriodCount; period++ {
		endTime, resolveErr := rs.SettingsService.ResolveString(ctx, schoolPeriodEndSetting(period))
		if resolveErr != nil {
			renderError(w, r, common.ErrorInternalServer(fmt.Errorf("resolve school period %d: %w", period, resolveErr)))
			return
		}
		if endTime = strings.TrimSpace(endTime); endTime != "" {
			periods = append(periods, schoolPeriodResponse{Period: period, EndTime: endTime})
		}
	}

	presets := make(map[string]string, 2)
	for _, key := range []string{settingCareDefaultArrivalTime, settingCareDefaultPickupTime} {
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
		DefaultArrivalTime: presets[settingCareDefaultArrivalTime],
		DefaultPickupTime:  presets[settingCareDefaultPickupTime],
	}, "Arrival settings retrieved successfully")
}
