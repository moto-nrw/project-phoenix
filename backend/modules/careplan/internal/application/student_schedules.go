package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
)

func (s *Service) lockWeekdayPickupNotes(ctx context.Context, studentID int64, stats *domain.OperationStats) (map[int]careplan.PickupNote, error) {
	stored, q, err := s.store.ListPickupNotes(ctx, careplan.StudentScheduleFilter{
		StudentIDs: []int64{studentID}, LockForUpdate: true,
	})
	stats.Add(q)
	if err != nil {
		return nil, err
	}
	byWeekday := make(map[int]careplan.PickupNote, len(stored))
	for _, note := range stored {
		if note.Weekday != 0 {
			byWeekday[note.Weekday] = note
		}
	}
	return byWeekday, nil
}

func (s *Service) deleteRemovedWeekdayPickupNotes(ctx context.Context, stored map[int]careplan.PickupNote, wanted map[int]string, stats *domain.OperationStats) error {
	for weekday, note := range stored {
		if _, keep := wanted[weekday]; !keep {
			q, err := s.store.DeletePickupNote(ctx, note.ID)
			stats.Add(q)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) saveWeekdayPickupNotes(ctx context.Context, studentID, createdBy int64, stored map[int]careplan.PickupNote, wanted map[int]string, stats *domain.OperationStats) error {
	for weekday := 1; weekday <= 5; weekday++ {
		content, wantedForWeekday := wanted[weekday]
		if !wantedForWeekday {
			continue
		}
		storedNote, exists := stored[weekday]
		if !exists {
			_, q, err := s.store.CreatePickupNote(ctx, careplan.PickupNote{
				StudentID: studentID, Weekday: weekday, Content: content, CreatedBy: createdBy,
			})
			stats.Add(q)
			if err != nil {
				return err
			}
			continue
		}
		if storedNote.Content == content {
			continue
		}
		storedNote.Content = content
		q, err := s.store.UpdatePickupNote(ctx, storedNote)
		stats.Add(q)
		if err != nil {
			return err
		}
	}
	return nil
}
