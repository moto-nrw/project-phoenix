package staffmessaging

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	configModels "github.com/moto-nrw/project-phoenix/models/config"
)

// ErrRetentionUnresolved means the tenant's retention window could not be
// determined. The sweep skips that tenant rather than deleting on a guess.
var ErrRetentionUnresolved = errors.New("staffmessaging: retention window could not be resolved")

// CleanupResult reports one tenant's retention sweep.
type CleanupResult struct {
	MessagesDeleted int
	ThreadsDeleted  int
	RetentionDays   int
}

// CleanupService is the retention contract the scheduler wires in.
type CleanupService interface {
	// CleanupExpiredMessages deletes messages past the tenant's retention window
	// and then removes conversations left without any message.
	CleanupExpiredMessages(ctx context.Context) (CleanupResult, error)
}

// CleanupExpiredMessages implements CleanupService.
//
// Staff messages are employee personal data, so the window is enforced whether
// or not the feature is currently switched on: a school that turned the chat
// OFF must still have its old messages aged out, not frozen in place forever.
// That is why this deliberately does NOT call requireEnabled.
func (s *Service) CleanupExpiredMessages(ctx context.Context) (CleanupResult, error) {
	days, err := s.retentionDays(ctx)
	if err != nil {
		// Deliberately NO fallback window. Deleting on a guessed retention is
		// the one outcome worse than not deleting: too short destroys messages
		// the school was entitled to keep, too long keeps employee data past
		// its window and hides the misconfiguration behind a green job. Skip
		// this tenant, surface the error, try again tomorrow.
		return CleanupResult{}, err
	}
	result := CleanupResult{RetentionDays: days}

	cutoff := time.Now().AddDate(0, 0, -days)

	messages, err := s.MessageRepo.DeleteOlderThan(ctx, cutoff)
	if err != nil {
		return result, err
	}
	result.MessagesDeleted = int(messages)

	// Only after the messages are gone: a conversation whose entire history
	// aged out is noise in the inbox, and leaving it there would show a
	// counterpart with no content and no way to tell why.
	//
	// Same cutoff as the messages, which doubles as the grace period for a
	// thread that was opened but never written in - see DeleteEmpty.
	threads, err := s.ThreadRepo.DeleteEmpty(ctx, cutoff)
	if err != nil {
		return result, err
	}
	result.ThreadsDeleted = int(threads)

	return result, nil
}

// retentionDays resolves the tenant's window. It never substitutes a default:
// an unreadable or nonsensical setting is a configuration failure the caller
// has to see, not a number to guess.
func (s *Service) retentionDays(ctx context.Context) (int, error) {
	if s.Settings == nil {
		return 0, ErrRetentionUnresolved
	}
	days, err := s.Settings.ResolveInt(ctx, configModels.KeyGDPRStaffMessageRetentionDays)
	if err != nil {
		s.Logger.Error("staffmessaging: resolve retention days failed, skipping cleanup",
			slog.String("key", configModels.KeyGDPRStaffMessageRetentionDays),
			slog.String("error", err.Error()),
		)
		return 0, fmt.Errorf("%w: %w", ErrRetentionUnresolved, err)
	}
	if days <= 0 {
		s.Logger.Error("staffmessaging: retention days must be positive, skipping cleanup",
			slog.String("key", configModels.KeyGDPRStaffMessageRetentionDays),
			slog.Int("days", days),
		)
		return 0, ErrRetentionUnresolved
	}
	return days, nil
}
