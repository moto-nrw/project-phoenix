package staffmessaging

import (
	"context"
	"log/slog"
	"time"

	configModels "github.com/moto-nrw/project-phoenix/models/config"
)

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
	days := s.retentionDays(ctx)
	result := CleanupResult{RetentionDays: days}
	if days <= 0 {
		return result, nil
	}

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

// retentionDays resolves the tenant's window, falling back to the registry
// default when the setting cannot be read.
func (s *Service) retentionDays(ctx context.Context) int {
	const fallbackDays = 365
	if s.Settings == nil {
		return fallbackDays
	}
	days, err := s.Settings.ResolveInt(ctx, configModels.KeyGDPRStaffMessageRetentionDays)
	if err != nil {
		s.Logger.Warn("staffmessaging: resolve retention days failed, using default",
			slog.String("key", configModels.KeyGDPRStaffMessageRetentionDays),
			slog.Int("fallback_days", fallbackDays),
			slog.String("error", err.Error()),
		)
		return fallbackDays
	}
	if days <= 0 {
		return fallbackDays
	}
	return days
}
