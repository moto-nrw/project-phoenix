package application

import (
	"time"

	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/domain"
)

// dayState is everything that decides one child's meal on one day.
type dayState struct {
	closed    bool
	holiday   bool
	scheduled bool
	override  *bool
	absences  []domain.SickDay
}

// dayResult is the resolved meal of one child on one day. Regular is what
// the weekly pattern gives (outside holidays), Registered what the day's
// exception makes of it, Participating what remains after absences.
type dayResult struct {
	participating bool
	registered    bool
	regular       bool
	source        domain.ParticipationSource
}

// resolveDay applies the rules in order: a closed day ends everything, the
// weekly pattern applies outside school holidays, the day's exception
// replaces it, and an absence at cutoff cancels the meal (#3873). An
// exception still registers a child in the holidays, for holiday care.
func resolveDay(state dayState, cutoff time.Time) dayResult {
	if state.closed {
		return dayResult{source: domain.ParticipationClosed}
	}
	result := dayResult{source: domain.ParticipationNone}
	if state.scheduled && !state.holiday {
		result.regular, result.registered, result.source = true, true, domain.ParticipationRegular
	}
	if state.override != nil {
		result.registered, result.source = *state.override, domain.ParticipationOverride
	}
	result.participating = result.registered
	if absent, sick := absenceAt(state.absences, cutoff); absent {
		result.participating, result.source = false, domain.ParticipationAbsent
		if sick {
			result.source = domain.ParticipationSick
		}
	}
	return result
}

// absenceAt reports whether any absence status holds at cutoff, and whether
// sick is one of them. The latest change of each status up to cutoff decides
// that status, so clearing one status never hides another one of the day.
func absenceAt(changes []domain.SickDay, cutoff time.Time) (absent, sick bool) {
	latest := make(map[string]domain.SickDay, len(changes))
	for _, change := range changes {
		if change.ChangedAt.After(cutoff) {
			continue
		}
		current, exists := latest[change.Status]
		if !exists || change.ChangedAt.After(current.ChangedAt) || (change.ChangedAt.Equal(current.ChangedAt) && change.ID > current.ID) {
			latest[change.Status] = change
		}
	}
	for status, change := range latest {
		if sickAtCutoff(change.ReportedAt, change.ClearedAt, cutoff) {
			absent = true
			sick = sick || status == domain.AbsenceSick
		}
	}
	return absent, sick
}

func sickAtCutoff(reportedAt time.Time, clearedAt *time.Time, cutoff time.Time) bool {
	return !reportedAt.After(cutoff) && (clearedAt == nil || clearedAt.After(cutoff))
}
