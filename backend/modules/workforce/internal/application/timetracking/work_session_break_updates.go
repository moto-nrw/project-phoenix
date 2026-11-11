package timetracking

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// applyBreakUpdates stages every interval first. Invalid batches must not write
// even when a caller supplies no surrounding transaction. Successful writes join
// the existing tenant transaction and staff balance lock.
func (s *workSessionService) applyBreakUpdates(ctx context.Context, uc *sessionUpdateContext, updates SessionUpdateRequest) error {
	records, err := s.breakRepo.ListWorkSessionBreaks(ctx, workforce.WorkSessionBreakFilter{SessionID: uc.sessionID})
	if err != nil {
		return fmt.Errorf("failed to load session breaks: %w", err)
	}
	breaks := workSessionBreaksToRecords(records)
	changed, err := stageBreakDurationUpdates(breaks, updates.Breaks)
	if err != nil {
		return err
	}
	if err := validateSessionBreakIntervals(uc.session, breaks); err != nil {
		return err
	}
	minutes, err := correctedBreakMinutes(uc.session.BreakMinutes, breaks, updates.BreakMinutes)
	if err != nil {
		return err
	}
	// Reconciliation can change recorded time even for an otherwise empty edit.
	// Preserve the existing justification gate before any pause write.
	if minutes != uc.session.BreakMinutes {
		resulting := updates
		resulting.BreakMinutes = &minutes
		if err := s.ensureSelfEditJustified(ctx, uc.session, resulting); err != nil {
			return err
		}
	}
	for _, edit := range changed {
		_, err := s.breakRepo.SetWorkSessionBreakDuration(ctx, edit.brk.ID, edit.brk.DurationMinutes, *edit.brk.EndedAt)
		if err != nil {
			return fmt.Errorf("failed to update break %d: %w", edit.brk.ID, err)
		}
		oldVal, newVal := strconv.Itoa(edit.oldMinutes), strconv.Itoa(edit.brk.DurationMinutes)
		uc.addAuditEdit("break_duration", &oldVal, &newVal)
	}
	// Individual duration edits already carry the audit evidence.
	if minutes != uc.session.BreakMinutes && len(changed) == 0 {
		oldVal, newVal := strconv.Itoa(uc.session.BreakMinutes), strconv.Itoa(minutes)
		uc.addAuditEdit("break_minutes", &oldVal, &newVal)
	}
	uc.session.BreakMinutes = minutes
	return nil
}

type stagedBreakDuration struct {
	brk        *WorkSessionBreak
	oldMinutes int
}

func stageBreakDurationUpdates(breaks []*WorkSessionBreak, updates []BreakDurationUpdate) ([]stagedBreakDuration, error) {
	byID := make(map[int64]*WorkSessionBreak, len(breaks))
	for _, brk := range breaks {
		byID[brk.ID] = brk
	}
	changed := make([]stagedBreakDuration, 0, len(updates))
	seen := make(map[int64]bool, len(updates))
	for _, update := range updates {
		brk, ok := byID[update.ID]
		if !ok {
			return nil, &workforce.TimeTrackingError{Kind: workforce.ErrBreakNotOwned, Cause: fmt.Errorf("break %d does not belong to this session", update.ID)}
		}
		if seen[update.ID] {
			return nil, invalidSessionBreaks("duplicate break duration edit")
		}
		seen[update.ID] = true
		if brk.EndedAt == nil {
			return nil, workforce.ErrActiveBreakDurationEdit
		}
		if update.DurationMinutes < 0 {
			return nil, workforce.ErrBreakDurationNegative
		}
		if update.DurationMinutes > int(time.Duration(1<<63-1)/time.Minute) {
			return nil, invalidSessionBreaks("break duration overflows an instant")
		}
		if brk.DurationMinutes == update.DurationMinutes {
			continue
		}
		changed = append(changed, stagedBreakDuration{brk: brk, oldMinutes: brk.DurationMinutes})
		end := brk.StartedAt.Add(time.Duration(update.DurationMinutes) * time.Minute)
		brk.EndedAt, brk.DurationMinutes = &end, update.DurationMinutes
	}
	return changed, nil
}

func validateSessionBreakIntervals(session *WorkSession, breaks []*WorkSessionBreak) error {
	sort.Slice(breaks, func(i, j int) bool { return breaks[i].StartedAt.Before(breaks[j].StartedAt) })
	var previous *WorkSessionBreak
	for _, brk := range breaks {
		if err := validateSessionBreakInterval(session, brk); err != nil {
			return err
		}
		// Zero-minute corrections leave an empty interval, even inside another pause.
		if brk.EndedAt != nil && brk.EndedAt.Equal(brk.StartedAt) {
			continue
		}
		if previous != nil && (previous.EndedAt == nil || previous.EndedAt.After(brk.StartedAt)) {
			return invalidSessionBreaks("break intervals overlap")
		}
		previous = brk
	}
	return nil
}

func validateSessionBreakInterval(session *WorkSession, brk *WorkSessionBreak) error {
	if brk.StartedAt.Before(session.CheckInTime) {
		return invalidSessionBreaks("break starts before check-in")
	}
	if brk.EndedAt == nil {
		if session.CheckOutTime != nil {
			return invalidSessionBreaks("end the active break before correcting checkout")
		}
		return nil
	}
	if brk.EndedAt.Before(brk.StartedAt) {
		return invalidSessionBreaks("break ends before it starts")
	}
	if session.CheckOutTime != nil && brk.EndedAt.After(*session.CheckOutTime) {
		return invalidSessionBreaks("break ends after checkout")
	}
	return nil
}

func correctedBreakMinutes(cached int, breaks []*WorkSessionBreak, scalar *int) (int, error) {
	minutes := cached
	if len(breaks) > 0 {
		minutes = 0
		for _, brk := range breaks {
			if brk.EndedAt != nil {
				minutes += brk.DurationMinutes
			}
		}
		if scalar != nil && *scalar != minutes {
			return 0, invalidSessionBreaks("break_minutes must equal the sum of recorded breaks")
		}
	} else if scalar != nil {
		minutes = *scalar
	}
	if minutes < 0 {
		return 0, workforce.ErrBreakMinutesNegative
	}
	return minutes, nil
}

func invalidSessionBreaks(detail string) error {
	return &workforce.TimeTrackingError{Kind: workforce.ErrTimeTrackingInvalid, Cause: fmt.Errorf("invalid session breaks: %s", detail)}
}
