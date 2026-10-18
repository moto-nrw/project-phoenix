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

// resolveDay applies the rules in order: a closed day ends everything, the
// weekly pattern applies outside school holidays, the day's exception
// replaces it, and an absence at cutoff cancels the meal (#3873). An
// exception still registers a child in the holidays, for holiday care.
func resolveDay(state dayState, cutoff time.Time) (bool, domain.ParticipationSource) {
	if state.closed {
		return false, domain.ParticipationClosed
	}
	participating, source := false, domain.ParticipationNone
	if state.scheduled && !state.holiday {
		participating, source = true, domain.ParticipationRegular
	}
	if state.override != nil {
		participating, source = *state.override, domain.ParticipationOverride
	}
	if absent, sick := absenceAt(state.absences, cutoff); absent {
		if sick {
			return false, domain.ParticipationSick
		}
		return false, domain.ParticipationAbsent
	}
	return participating, source
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
