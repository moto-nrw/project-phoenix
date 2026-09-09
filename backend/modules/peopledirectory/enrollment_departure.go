package peopledirectory

import (
	"strings"
	"unicode/utf8"
)

// The workflow attests same-day companion coverage under the graph locks.
// The owner derives every legacy mirror from the canonical non-exclusive plan.
func normalizeEnrollmentDeparture(input *EnrollmentProfilePatch) error {
	if !input.DepartureSet {
		return nil
	}
	input.DepartureDays = map[string]string{}
	input.BusDays = map[string]bool{}
	input.PickupDays = map[string]bool{}
	input.PickupStatus = "Geht alleine nach Hause"
	allowed := map[string][]string{}
	accompanied, pickup := false, false
	for day, modes := range input.AllowedDepartureModes {
		if day != "mon" && day != "tue" && day != "wed" && day != "thu" && day != "fri" {
			return &InvalidStudentError{Reason: "invalid departure weekday"}
		}
		seen := map[string]bool{}
		for _, mode := range modes {
			if (mode != "alone" && mode != "bus" && mode != "pickup" && mode != "accompanied") || seen[mode] {
				return &InvalidStudentError{Reason: "invalid or duplicate departure mode"}
			}
			seen[mode] = true
		}
		for _, mode := range []string{"alone", "bus", "pickup", "accompanied"} {
			if seen[mode] {
				allowed[day] = append(allowed[day], mode)
			}
		}
		if seen["bus"] {
			input.BusDays[day] = true
		}
		if seen["pickup"] {
			input.PickupDays[day] = true
			pickup = true
		}
		if seen["accompanied"] {
			accompanied = true
			if (input.DepartureCompanionNote == nil || strings.TrimSpace(*input.DepartureCompanionNote) == "") && !input.DepartureCompanionDays[day] {
				return &InvalidStudentError{Reason: "departure companion detail is required"}
			}
		}
		for _, mode := range []string{"pickup", "bus", "accompanied"} {
			if seen[mode] {
				input.DepartureDays[day] = mode
				break
			}
		}
	}
	input.AllowedDepartureModes = allowed
	if accompanied {
		input.PickupStatus = "Geht mit anderem Kind"
	} else {
		input.DepartureCompanionNote = nil
	}
	if pickup {
		input.PickupStatus = "Wird abgeholt"
	}
	if input.DepartureCompanionNote != nil {
		note := strings.TrimSpace(*input.DepartureCompanionNote)
		if utf8.RuneCountInString(note) > 255 {
			return &InvalidStudentError{Reason: "departure companion note exceeds 255 characters"}
		}
		input.DepartureCompanionNote = nil
		if note != "" {
			input.DepartureCompanionNote = &note
		}
	}
	return nil
}
