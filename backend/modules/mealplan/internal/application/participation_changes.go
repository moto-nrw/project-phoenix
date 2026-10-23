package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// requireChangeOpen checks the cutoff of a parent's change of the day:
// target true signs the child up, false takes it off, nil returns the day
// to the fixed days. Signing up needs the registration cutoff, taking off
// the cancellation cutoff; a change that keeps the day as it is needs the
// later of both. Closed days have no meal.
func (s *Service) requireChangeOpen(ctx context.Context, rules domain.Rules, studentID int64, date domain.Date, target *bool) (domain.OperationStats, error) {
	if rules.Closed(date) {
		return domain.OperationStats{}, mealplan.ErrInvalidParticipation
	}
	data, stats, err := s.store.FindParticipation(ctx, studentID, date, date)
	if err != nil {
		return stats, err
	}
	overrides := make(map[domain.Date]bool, len(data.Overrides)+1)
	for _, override := range data.Overrides {
		overrides[override.Date] = override.Participating
	}
	sickDays := effectiveSickDays(data.SickDays, rules)
	current, _ := dayParticipation(data.Schedules, overrides, sickDays, date, rules)
	if target == nil {
		delete(overrides, date)
	} else {
		overrides[date] = *target
	}
	next, _ := dayParticipation(data.Schedules, overrides, sickDays, date, rules)
	now := s.now()
	open := rules.CanCancel(now, date)
	if next && !current {
		open = rules.Registration.Changeable(now, date)
	}
	if !open {
		return stats, mealplan.ErrParticipationCutoff
	}
	return stats, nil
}

// ChangeParticipationAsStaff applies a change the team makes for a family.
// It ignores the cutoffs: the team decides for itself whether the kitchen
// can still take it. The day must be today or later, not closed, and the
// child one of the day's candidates.
func (s *Service) ChangeParticipationAsStaff(ctx context.Context, command mealplan.StaffParticipationChange) error {
	date, err := participationDate(command.Date)
	if command.StudentID <= 0 || command.AccountID <= 0 || err != nil {
		return mealplan.ErrInvalidParticipation
	}
	return s.run(ctx, "change_participation_as_staff", func(txCtx context.Context) (domain.OperationStats, error) {
		rules, _, cutoffErr := s.requireRegistration(txCtx, date, date)
		if cutoffErr != nil {
			return domain.OperationStats{}, cutoffErr
		}
		if rules.Closed(date) || date.Before(calendar.DateFromTime(s.now())) {
			return domain.OperationStats{}, mealplan.ErrInvalidParticipation
		}
		candidates, stats, err := s.directory.FindDailyCandidates(txCtx, date)
		if err != nil {
			return stats, err
		}
		if !hasCandidate(candidates, command.StudentID) {
			return stats, mealplan.ErrInvalidParticipation
		}
		var writeStats domain.OperationStats
		if command.Participating == nil {
			writeStats, err = s.store.DeleteParticipationOverride(txCtx, command.StudentID, date)
		} else {
			writeStats, err = s.store.UpsertParticipationOverride(txCtx, command.StudentID, command.AccountID, date, *command.Participating)
		}
		return addStats(stats, writeStats), err
	})
}

func hasCandidate(candidates []mealplan.ParticipantCandidate, studentID int64) bool {
	for _, candidate := range candidates {
		if candidate.StudentID == studentID {
			return true
		}
	}
	return false
}
