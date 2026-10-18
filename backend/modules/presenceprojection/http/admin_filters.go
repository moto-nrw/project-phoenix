package presenceprojectionhttp

import (
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// matchesAdministrativeFilters reports whether a single response satisfies all
// active administrative filters. Empty or "all" values are treated as "off".
func matchesAdministrativeFilters(s StudentResponse, bus, photoConsent, pickupStatus string, planningDate calendar.Date) bool {
	if isActiveFilterValue(bus) {
		// s.Bus is derived from bus_days.HasAny() when the response is built, so
		// filtering on it evaluates bus_days semantics — the single source of
		// truth (#1582). The bus=yes/no query wording stays for API compatibility.
		if !s.HasFullAccess || s.Bus != (bus == "yes") {
			return false
		}
	}
	if isActiveFilterValue(photoConsent) {
		if !s.HasFullAccess || s.PhotoConsentGiven == nil || *s.PhotoConsentGiven != (photoConsent == "yes") {
			return false
		}
	}
	if isActiveFilterValue(pickupStatus) {
		if !s.HasFullAccess || !dailyDepartureMatchesFilter(dailyDepartureForDate(s, planningDate), pickupStatus) {
			return false
		}
	}
	return true
}

// applyAdministrativeFilters returns the subset of responses matching the
// active administrative filters. When none is active the input is returned
// unchanged so callers can invoke it unconditionally.
func applyAdministrativeFilters(responses []StudentResponse, bus, photoConsent, pickupStatus string, planningDate calendar.Date) []StudentResponse {
	if !isActiveFilterValue(bus) && !isActiveFilterValue(photoConsent) && !isActiveFilterValue(pickupStatus) {
		return responses
	}
	filtered := make([]StudentResponse, 0, len(responses))
	for _, response := range responses {
		if matchesAdministrativeFilters(response, bus, photoConsent, pickupStatus, planningDate) {
			filtered = append(filtered, response)
		}
	}
	return filtered
}
