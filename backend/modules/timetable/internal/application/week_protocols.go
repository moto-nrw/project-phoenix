package application

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The weekly protocols of #4769 for the staff and the leadership: every read
// and write checks the caller's reach, and only a draft takes notes or is
// closed.

func (s *Service) ListWeekProtocols(ctx context.Context, reach timetable.WeekProtocolReach, filter timetable.WeekProtocolFilter) (result []timetable.WeekProtocol, err error) {
	err = s.runWrite(ctx, "list_week_protocols", false, func(txCtx context.Context, _ *domain.OperationStats) error {
		protocols, listErr := s.store.ListWeekProtocols(txCtx, filter)
		if listErr != nil {
			return listErr
		}
		result = make([]timetable.WeekProtocol, 0, len(protocols))
		for _, protocol := range protocols {
			if reach.Sees(protocol) {
				result = append(result, protocol)
			}
		}
		return nil
	})
	return result, err
}

func (s *Service) OpenWeekProtocol(ctx context.Context, reach timetable.WeekProtocolReach, id int64) (result timetable.WeekProtocol, err error) {
	if id <= 0 || (!reach.Leadership && !reach.Staff) {
		// No protocol has such an id, and a caller without reach sees none:
		// both answer without a transaction.
		return timetable.WeekProtocol{}, timetable.ErrWeekProtocolNotFound
	}
	err = s.runWrite(ctx, "open_week_protocol", false, func(txCtx context.Context, _ *domain.OperationStats) error {
		protocol, found, findErr := s.store.FindWeekProtocol(txCtx, id, false)
		if findErr != nil {
			return findErr
		}
		if !found || !reach.Sees(protocol) {
			return timetable.ErrWeekProtocolNotFound
		}
		result = protocol
		return nil
	})
	return result, err
}

func (s *Service) SaveWeekProtocolNotes(ctx context.Context, reach timetable.WeekProtocolReach, id int64, notes timetable.WeekProtocolNotes) (result timetable.WeekProtocol, err error) {
	err = s.runWrite(ctx, "save_week_protocol_notes", true, func(txCtx context.Context, _ *domain.OperationStats) error {
		protocol, findErr := s.visibleWeekProtocolDraft(txCtx, reach, id)
		if findErr != nil {
			return findErr
		}
		cleaned, validateErr := cleanWeekProtocolNotes(protocol, notes)
		if validateErr != nil {
			return validateErr
		}
		if saveErr := s.store.SaveWeekProtocolNotes(txCtx, id, cleaned); saveErr != nil {
			return saveErr
		}
		result, err = s.visibleWeekProtocol(txCtx, reach, id, false)
		return err
	})
	return result, err
}

func (s *Service) CloseWeekProtocol(ctx context.Context, reach timetable.WeekProtocolReach, id int64, actor timetable.WeekProtocolActor) (result timetable.WeekProtocol, err error) {
	if actor.AccountID <= 0 {
		return timetable.WeekProtocol{}, fmt.Errorf("%w: closing needs the acting account", timetable.ErrWeekProtocolInvalid)
	}
	err = s.runWrite(ctx, "close_week_protocol", true, func(txCtx context.Context, _ *domain.OperationStats) error {
		if _, findErr := s.visibleWeekProtocolDraft(txCtx, reach, id); findErr != nil {
			return findErr
		}
		if closeErr := s.store.CloseWeekProtocol(txCtx, id, actor, time.Now()); closeErr != nil {
			return closeErr
		}
		result, err = s.visibleWeekProtocol(txCtx, reach, id, false)
		return err
	})
	return result, err
}

// DeleteExpiredWeekProtocols deletes the protocols whose week ended before
// the first Berlin calendar day the window of retentionDays keeps.
func (s *Service) DeleteExpiredWeekProtocols(ctx context.Context, retentionDays int) (deleted int, err error) {
	if retentionDays <= 0 {
		return 0, fmt.Errorf("%w, got %d", timetable.ErrWeekProtocolRetentionWindow, retentionDays)
	}
	today, err := calendar.ParseDate(s.today())
	if err != nil {
		return 0, fmt.Errorf("week protocol retention: today: %w", err)
	}
	err = s.runWrite(ctx, "delete_expired_week_protocols", false, func(txCtx context.Context, stats *domain.OperationStats) error {
		deleted, err = s.store.DeleteWeekProtocolsEndedBefore(txCtx, today.AddDays(-retentionDays))
		// The observation reports the deleted protocols; their days go with
		// them through the foreign key.
		stats.Queries++
		stats.Rows += int64(deleted)
		return err
	})
	return deleted, err
}

// visibleWeekProtocol loads a protocol the caller may see; one outside the
// reach is answered as not found, so its existence stays hidden.
func (s *Service) visibleWeekProtocol(ctx context.Context, reach timetable.WeekProtocolReach, id int64, lock bool) (timetable.WeekProtocol, error) {
	protocol, found, err := s.store.FindWeekProtocol(ctx, id, lock)
	if err != nil {
		return timetable.WeekProtocol{}, err
	}
	if !found || !reach.Sees(protocol) {
		return timetable.WeekProtocol{}, timetable.ErrWeekProtocolNotFound
	}
	return protocol, nil
}

func (s *Service) visibleWeekProtocolDraft(ctx context.Context, reach timetable.WeekProtocolReach, id int64) (timetable.WeekProtocol, error) {
	protocol, err := s.visibleWeekProtocol(ctx, reach, id, true)
	if err != nil {
		return timetable.WeekProtocol{}, err
	}
	if protocol.Status != timetable.WeekProtocolStatusDraft {
		return timetable.WeekProtocol{}, timetable.ErrWeekProtocolNotDraft
	}
	return protocol, nil
}

// cleanWeekProtocolNotes trims the notes and refuses days outside the week
// and texts over the column limits.
func cleanWeekProtocolNotes(protocol timetable.WeekProtocol, notes timetable.WeekProtocolNotes) (timetable.WeekProtocolNotes, error) {
	cleaned := timetable.WeekProtocolNotes{WeekNote: strings.TrimSpace(notes.WeekNote), Days: map[calendar.Date]string{}}
	if utf8.RuneCountInString(cleaned.WeekNote) > timetable.WeekProtocolWeekNoteMaxLength {
		return timetable.WeekProtocolNotes{}, fmt.Errorf("%w: week note longer than %d characters", timetable.ErrWeekProtocolInvalid, timetable.WeekProtocolWeekNoteMaxLength)
	}
	for day, note := range notes.Days {
		if day.Before(protocol.WeekStart) || day.After(protocol.WeekEnd) {
			return timetable.WeekProtocolNotes{}, fmt.Errorf("%w: %s is not in the week of %s", timetable.ErrWeekProtocolInvalid, day, protocol.WeekStart)
		}
		note = strings.TrimSpace(note)
		if utf8.RuneCountInString(note) > timetable.WeekProtocolDayNoteMaxLength {
			return timetable.WeekProtocolNotes{}, fmt.Errorf("%w: note of %s longer than %d characters", timetable.ErrWeekProtocolInvalid, day, timetable.WeekProtocolDayNoteMaxLength)
		}
		cleaned.Days[day] = note
	}
	return cleaned, nil
}
