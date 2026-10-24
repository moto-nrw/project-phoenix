package application

import (
	"context"
	"fmt"
	"log/slog"
)

func (s *service) resolveAnnouncementPushAudience(ctx context.Context, a *StaffParentAnnouncement) ([]int64, map[int64]string, error) {
	var accountIDs []int64
	seen := make(map[int64]struct{})
	recipients, err := s.repo.AudienceRecipients(ctx, a.GetTenantID(), a.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve guardian recipients: %w", err)
	}
	accountIDs = make([]int64, 0, len(recipients))
	localeByAccount := make(map[int64]string, len(recipients))
	for _, recipient := range recipients {
		if recipient.AccountID <= 0 {
			continue
		}
		if _, exists := seen[recipient.AccountID]; exists {
			continue
		}
		seen[recipient.AccountID] = struct{}{}
		accountIDs = append(accountIDs, recipient.AccountID)
		localeByAccount[recipient.AccountID] = s.deliveryPolicy.NormalizeLocale(recipient.PortalLocale)
	}
	return accountIDs, localeByAccount, nil
}

func (s *service) filterAnnouncementPushAudience(ctx context.Context, announcementID int64, notificationType string, accountIDs []int64) ([]int64, error) {
	var err error
	candidateCount := len(accountIDs)

	// Consent narrows the audience the announcement itself defined. A nil
	// preference service means the dependency was never wired, which the
	// factory always does — the pre-consent behaviour is kept for that case so
	// a partially constructed service does not silently stop notifying. When a
	// service IS present, its answer is final: no consent, no push.
	if s.preferences != nil {
		accountIDs, err = s.preferences.FilterOptedIn(ctx, notificationType, accountIDs)
		if err != nil {
			return nil, fmt.Errorf("filter opted-in guardians: %w", err)
		}
		if len(accountIDs) < candidateCount {
			s.logger.Info("parent announcement push narrowed by consent",
				slog.Int64("announcement_id", announcementID),
				slog.Int("candidate_count", candidateCount),
				slog.Int("recipient_count", len(accountIDs)),
			)
		}
		if len(accountIDs) == 0 {
			return nil, nil
		}
	}
	return accountIDs, nil
}
