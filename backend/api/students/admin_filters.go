package students

import "strings"

// Administrative filters (#1492): bus / photo consent / pickup rule.
//
// These run against the enriched StudentResponse objects (not in SQL) so they
// share the same in-memory pipeline as the day-planning filter. Keeping them
// here means the student list endpoint and the list export apply identical
// semantics from a single source of truth, including the GDPR rule that
// redacted students (HasFullAccess == false) never match an administrative
// filter, so the filtered result can't leak which restricted children are bus
// kids, lack photo consent, etc.

// matchesAdministrativeFilters reports whether a single response satisfies all
// active administrative filters. Empty or "all" values are treated as "off".
func matchesAdministrativeFilters(s StudentResponse, bus, photoConsent, pickupStatus string) bool {
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
		if !s.HasFullAccess || pickupStatusKind(s.PickupStatus) != pickupStatus {
			return false
		}
	}
	return true
}

// applyAdministrativeFilters returns the subset of responses matching the
// active administrative filters. When none is active the input is returned
// unchanged so callers can invoke it unconditionally.
func applyAdministrativeFilters(responses []StudentResponse, bus, photoConsent, pickupStatus string) []StudentResponse {
	if !isActiveFilterValue(bus) && !isActiveFilterValue(photoConsent) && !isActiveFilterValue(pickupStatus) {
		return responses
	}
	filtered := make([]StudentResponse, 0, len(responses))
	for _, response := range responses {
		if matchesAdministrativeFilters(response, bus, photoConsent, pickupStatus) {
			filtered = append(filtered, response)
		}
	}
	return filtered
}

// pickupStatusKind normalizes the free-text pickup rule stored on a student
// into one of the filterable buckets: "self", "pickedUp", "other" or "none".
// Shared by the student list filter, the list export filter and their labels
// so the categorisation can never drift between the two paths.
func pickupStatusKind(status string) string {
	raw := strings.TrimSpace(status)
	if raw == "" {
		return "none"
	}
	normalized := strings.ToLower(raw)
	if strings.Contains(normalized, "alleine") ||
		normalized == "selbst" ||
		normalized == "self" ||
		normalized == "alone" ||
		normalized == "walk_home" ||
		normalized == "geht_alleine" {
		return "self"
	}
	if strings.Contains(normalized, "abgeholt") ||
		normalized == "parent" ||
		normalized == "parents" ||
		normalized == "guardian" ||
		normalized == "pickup" ||
		normalized == "picked_up" ||
		normalized == "wird_abgeholt" {
		return "pickedUp"
	}
	return "other"
}
