package application

import (
	"context"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

const (
	minPrivacyConsentRetentionDays = 1
	maxPrivacyConsentRetentionDays = 31
)

func (s *Service) ListPrivacyConsents(ctx context.Context, studentID int64) (rows []ports.PrivacyConsent, err error) {
	err = s.run("list_privacy_consents", func() (ports.Stats, error) {
		if studentID <= 0 {
			return ports.Stats{}, ports.ErrInvalidPrivacyConsent
		}
		var stats ports.Stats
		rows, stats, err = s.store.ListPrivacyConsents(ctx, studentID)
		return stats, err
	})
	return rows, err
}

// RecordPrivacyConsent appends one consent row inside the caller's tenant
// transaction. The retention window and the acceptance/expiry pairing are
// validated here, the same rules the retained model enforces.
func (s *Service) RecordPrivacyConsent(ctx context.Context, row *ports.PrivacyConsent) error {
	return s.run("record_privacy_consent", func() (ports.Stats, error) {
		if !validPrivacyConsent(row) {
			return ports.Stats{}, ports.ErrInvalidPrivacyConsent
		}
		if err := s.tx.Require(ctx); err != nil {
			return ports.Stats{}, err
		}
		return s.store.RecordPrivacyConsent(ctx, row)
	})
}

func (s *Service) ListAcceptedRetentionSettings(ctx context.Context) (rows []ports.StudentRetentionSetting, err error) {
	err = s.run("list_accepted_retention_settings", func() (ports.Stats, error) {
		var stats ports.Stats
		rows, stats, err = s.store.ListAcceptedRetentionSettings(ctx)
		return stats, err
	})
	return rows, err
}

func validPrivacyConsent(row *ports.PrivacyConsent) bool {
	return row != nil && row.StudentID > 0 && strings.TrimSpace(row.PolicyVersion) != "" &&
		row.DataRetentionDays >= minPrivacyConsentRetentionDays && row.DataRetentionDays <= maxPrivacyConsentRetentionDays &&
		(row.ExpiresAt == nil || row.AcceptedAt == nil || !row.ExpiresAt.Before(*row.AcceptedAt))
}
func (s *Service) RevisePrivacyConsent(ctx context.Context, row *ports.PrivacyConsent) error {
	return s.run("revise_privacy_consent", func() (ports.Stats, error) {
		if !validPrivacyConsent(row) || row.ID <= 0 {
			return ports.Stats{}, ports.ErrInvalidPrivacyConsent
		}
		if err := s.tx.Require(ctx); err != nil {
			return ports.Stats{}, err
		}
		return s.store.RevisePrivacyConsent(ctx, row)
	})
}
