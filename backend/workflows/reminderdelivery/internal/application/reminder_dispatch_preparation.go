package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/appointments"
	"github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/ports"
)

func (s *preparation) prepareReminderPushInTx(ctx context.Context, delivery reminderPushDelivery) (*appointments.Appointment, []int64, []int64, error) {
	current, err := s.deps.Appointments.FindReminderCandidateForUpdate(ctx, delivery.appointment.ID)
	if err != nil || current == nil {
		return nil, nil, nil, err
	}
	// An edit between claim and dispatch bumps the revision, which invalidates
	// the claim this delivery holds — nothing can ever match it again, so it is
	// released here. Dropping the reminder with it would lose it for good: the
	// scan boundary has already moved past this occurrence and no later tick
	// offers it again. So the claim is re-taken under the new revision instead,
	// and only the checks below decide whether the reminder is still warranted.
	revised := current.Revision != delivery.appointment.Revision
	if revised {
		if err := s.releaseReminderPushProfiles(ctx, delivery.appointment, delivery.occurrence, delivery.profileIDs); err != nil {
			return nil, nil, nil, err
		}
	}
	due, err := s.reminderOccurrenceDueForDispatch(ctx, current, delivery)
	if err != nil || !due {
		return nil, nil, nil, err
	}
	recipients, optedIn, err := s.reminderDispatchAudience(ctx, current.ID, delivery.accountID)
	if err != nil || !optedIn {
		return nil, nil, nil, err
	}
	profileIDs, err := s.claimReminderDispatchProfiles(ctx, current, delivery, recipients, revised)
	if err != nil || len(profileIDs) == 0 {
		return nil, nil, nil, err
	}
	return current, profileIDs, recipients.StudentIDs([]int64{delivery.accountID}), nil
}

func (s *preparation) reminderOccurrenceDueForDispatch(ctx context.Context, current *appointments.Appointment, delivery reminderPushDelivery) (bool, error) {
	rule, err := s.deps.Appointments.FindRecurrenceRule(ctx, current.ID)
	if err != nil {
		return false, fmt.Errorf("reload reminder recurrence: %w", err)
	}
	if (rule == nil && current.StartDate != delivery.occurrence) ||
		(rule != nil && !appointments.OccurrenceExists(current, rule, delivery.occurrence)) {
		return false, nil
	}
	overrides, err := s.deps.Appointments.FindOccurrenceOverrides(ctx, []int64{current.ID}, []appointments.Date{delivery.occurrence})
	if err != nil {
		return false, fmt.Errorf("reload reminder override: %w", err)
	}
	var override *appointments.AppointmentOccurrenceOverride
	if len(overrides) > 0 {
		override = overrides[0]
	}
	if override != nil && override.Cancelled {
		return false, nil
	}
	// An edit can also have moved the occurrence. A reminder is only worth
	// sending at the moment it was due for, so one that moved away is not sent
	// now — returning nothing releases the claim, which leaves the tick
	// that reaches the occurrence's new moment free to take it.
	startsAt := appointments.OccurrenceStartInstant(appointments.EffectiveOccurrence(current, delivery.occurrence, override))
	// Recovery and no-audience deferral can outlive the original scan window.
	// At the start instant the reminder is no longer useful, even if that
	// occurrence still belongs to the saved window.
	return startsAt.After(s.deps.Now()) && delivery.stillDue(startsAt), nil
}

func (s *preparation) reminderDispatchAudience(ctx context.Context, appointmentID, accountID int64) (ports.GuardianAudience, bool, error) {
	if s.deps.FilterPush == nil {
		return ports.GuardianAudience{}, false, nil
	}
	optedIn, err := s.deps.FilterPush(ctx, []int64{accountID})
	if err != nil {
		return ports.GuardianAudience{}, false, fmt.Errorf("filter opted-in guardians: %w", err)
	}
	if len(optedIn) == 0 {
		return ports.GuardianAudience{}, false, nil
	}
	audiences, err := s.deps.Audiences(ctx, []int64{appointmentID})
	if err != nil {
		return ports.GuardianAudience{}, false, err
	}
	return audiences[appointmentID], true, nil
}

func reminderAccountProfiles(recipients ports.GuardianAudience, accountID int64) map[int64]struct{} {
	reachableSet := make(map[int64]struct{}, len(recipients.GuardianIDs))
	for _, profileID := range recipients.GuardianIDs {
		profile, ok := recipients.Profiles[profileID]
		if ok && profile.AccountID != nil && *profile.AccountID == accountID {
			reachableSet[profileID] = struct{}{}
		}
	}
	return reachableSet
}

func (s *preparation) claimReminderDispatchProfiles(ctx context.Context, current *appointments.Appointment, delivery reminderPushDelivery, recipients ports.GuardianAudience, revised bool) ([]int64, error) {
	reachableSet := reminderAccountProfiles(recipients, delivery.accountID)
	var profileIDs []int64
	for _, profileID := range delivery.profileIDs {
		if _, ok := reachableSet[profileID]; !ok {
			continue
		}
		// The claim released above has to be replaced before the push goes out,
		// or a re-scan of this window could send the reminder a second time.
		if revised {
			claimed, err := s.deps.Appointments.ClaimReminderPushDelivery(ctx, current.ID, current.Revision, delivery.occurrence, profileID)
			if err != nil {
				return nil, fmt.Errorf("claim revised reminder push delivery: %w", err)
			}
			if !claimed {
				continue
			}
		}
		profileIDs = append(profileIDs, profileID)
	}
	return profileIDs, nil
}
