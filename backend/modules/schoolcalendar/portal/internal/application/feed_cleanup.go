package application

import (
	"context"
	"fmt"
	"time"
)

// CleanupExpiredFeedTombstones permanently removes the staff feed tombstones
// after every subscribed client has had the documented 90-day cancellation
// window. Cancelled and deleted appointments go in the nightly retention of
// the Appointments owner, which logs them (#4045).
func (s *service) CleanupExpiredFeedTombstones(ctx context.Context) (int, error) {
	if s.cfg.StaffFeedTombstoneRepo == nil {
		return 0, fmt.Errorf("%w: calendar feed cleanup not configured", ErrInvalidRequest)
	}
	cutoff := time.Now().AddDate(0, 0, -feedTombstoneDays)
	return s.cfg.StaffFeedTombstoneRepo.DeleteBefore(ctx, cutoff)
}
