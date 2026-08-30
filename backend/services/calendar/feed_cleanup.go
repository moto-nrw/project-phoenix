package calendar

import (
	"context"
	"fmt"
	"time"
)

// CleanupExpiredFeedTombstones permanently removes lifecycle records after
// every subscribed client has had the documented 90-day cancellation window.
func (s *service) CleanupExpiredFeedTombstones(ctx context.Context) (int, error) {
	if s.cfg.AppointmentRepo == nil || s.cfg.StaffFeedTombstoneRepo == nil {
		return 0, fmt.Errorf("%w: calendar feed cleanup not configured", ErrInvalidRequest)
	}
	cutoff := time.Now().AddDate(0, 0, -feedTombstoneDays)
	appointments, err := s.cfg.AppointmentRepo.DeleteFeedTombstonesBefore(ctx, cutoff)
	if err != nil {
		return 0, err
	}
	staffEvents, err := s.cfg.StaffFeedTombstoneRepo.DeleteBefore(ctx, cutoff)
	if err != nil {
		return 0, err
	}
	return appointments + staffEvents, nil
}
