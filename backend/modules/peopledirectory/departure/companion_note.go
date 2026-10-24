package departure

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

const MaxDepartureCompanionNoteLen = 255

var ErrDepartureCompanionNoteRequired = errors.New("departure_companion_note is required when a day allows the accompanied departure mode")

// HasCompanionNote reports whether the free-text "mit wem" note carries the
// detail. A blank note is no answer.
func HasCompanionNote(note *string) bool {
	return note != nil && strings.TrimSpace(*note) != ""
}

// NormalizeCompanionNote validates per-day companion coverage and bounds the
// optional note. A structured link covers only its own weekday.
func NormalizeCompanionNote(days DepartureDays, modes AllowedDepartureModes, note *string, covered map[string]bool) (*string, error) {
	var err error
	note, err = trimCompanionNote(note)
	if err != nil {
		return note, err
	}
	if note == nil {
		for day, mode := range days {
			if mode == DepartureAccompanied && !covered[day] {
				return nil, ErrDepartureCompanionNoteRequired
			}
		}
		if err := requireCompanionModeCoverage(modes, covered); err != nil {
			return nil, err
		}
	}
	return note, nil
}

func trimCompanionNote(note *string) (*string, error) {
	if note != nil {
		trimmed := strings.TrimSpace(*note)
		switch {
		case trimmed == "":
			note = nil
		case utf8.RuneCountInString(trimmed) > MaxDepartureCompanionNoteLen:
			return note, fmt.Errorf("departure_companion_note must be at most %d characters", MaxDepartureCompanionNoteLen)
		default:
			note = &trimmed
		}
	}
	return note, nil
}

func requireCompanionModeCoverage(modes AllowedDepartureModes, covered map[string]bool) error {
	for day, values := range modes {
		for _, mode := range values {
			if mode == DepartureAccompanied && !covered[day] {
				return ErrDepartureCompanionNoteRequired
			}
		}
	}
	return nil
}
