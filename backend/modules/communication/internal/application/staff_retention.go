package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ErrRetentionWindow refuses a retention window that is not positive:
// nothing is deleted on a wrong period, and the run is caught up once the
// setting is fixed.
var ErrRetentionWindow = errors.New("parent announcement retention window must be positive")

// DeleteExpiredParentAnnouncements deletes the announcements that ended
// before the Berlin midnight of the first calendar day a window of
// retentionDays keeps (#4045).
func (s *service) DeleteExpiredParentAnnouncements(ctx context.Context, retentionDays int) (int, error) {
	if retentionDays <= 0 {
		return 0, fmt.Errorf("%w, got %d", ErrRetentionWindow, retentionDays)
	}
	return s.DeleteExpired(ctx, calendar.TodayDate().AddDays(-retentionDays).BerlinMidnight())
}

// DeleteExpired deletes the announcements of the school in context that
// ended before cutoff (#4045): a published one at its expiry or, without
// one, at its publication; a draft at its last change. Cancellation notices
// of the system go the same way. An announcement whose declaration has
// submissions stays: the submissions prove the consent and go with their
// child. Each announcement is deleted under its row lock, like a manual
// delete, and re-checked there.
func (s *service) DeleteExpired(ctx context.Context, cutoff time.Time) (int, error) {
	announcements, err := s.repo.ListForTenant(ctx, true)
	if err != nil {
		return 0, fmt.Errorf("announcement: list for retention: %w", err)
	}
	deleted := 0
	for _, listed := range announcements {
		if !announcementEndedBefore(listed, cutoff) {
			continue
		}
		a, err := s.repo.FindByIDForUpdate(ctx, listed.ID)
		if err != nil {
			return deleted, fmt.Errorf("announcement: load for retention: %w", err)
		}
		if a == nil || !announcementEndedBefore(a, cutoff) {
			continue
		}
		submissions, err := s.repo.CountDeclarationSubmissions(ctx, a.GetTenantID(), a.ID)
		if err != nil {
			return deleted, fmt.Errorf("announcement: count declaration submissions: %w", err)
		}
		if submissions > 0 {
			continue
		}
		if err := s.deleteLocked(ctx, a); err != nil {
			return deleted, err
		}
		deleted++
	}
	return deleted, nil
}

// announcementEndedBefore reports whether the announcement ended before
// cutoff: at its expiry or publication once published, at its last change as
// a draft.
func announcementEndedBefore(a *StaffParentAnnouncement, cutoff time.Time) bool {
	ended := a.UpdatedAt
	switch {
	case a.PublishedAt == nil:
	case a.ExpiresAt != nil:
		ended = *a.ExpiresAt
	default:
		ended = *a.PublishedAt
	}
	return ended.Before(cutoff)
}
