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

// recordAnnouncementRecipients keeps who this publication reached (#6988).
// It runs in the publish transaction before the push and the e-mails, so
// statistics, the staff recipient list and every later reminder count the
// people the announcement went to, not whoever the targets reach on the day
// someone looks. Until the mark is set, AudienceRecipients still resolves
// the audience of this moment, which is what gets recorded.
func (s *service) recordAnnouncementRecipients(ctx context.Context, a *StaffParentAnnouncement) error {
	accountIDs, _, err := s.resolveAnnouncementPushAudience(ctx, a)
	if err != nil {
		return fmt.Errorf("announcement: resolve recipients to record: %w", err)
	}
	if err := s.repo.RecordRecipients(ctx, a.ID, accountIDs); err != nil {
		return fmt.Errorf("announcement: record recipients: %w", err)
	}
	return nil
}
