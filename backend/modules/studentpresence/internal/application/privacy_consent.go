package application

import (
	"context"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

const (
	minPrivacyConsentRetentionDays = 1
	maxPrivacyConsentRetentionDays = 31
)

func (s *Service) ListPrivacyConsents(ctx context.Context, studentID int64) (rows []studentpresence.PrivacyConsent, err error) {
	err = s.runWrite(ctx, "list_privacy_consents", func(ctx context.Context) (ports.Stats, error) {
		if studentID <= 0 {
			return ports.Stats{}, studentpresence.ErrInvalidPrivacyConsent
		}
		var stats ports.Stats
		rows, stats, err = s.store.ListPrivacyConsents(ctx, studentID)
		return stats, err
	})
	return listResult(rows, err)
}

// RecordPrivacyConsent appends one consent row inside the caller's tenant
// transaction. The retention window and the acceptance/expiry pairing are
// validated here, the same rules the retained model enforces. The store
// assigns the identity and timestamps of the new row.
func (s *Service) RecordPrivacyConsent(ctx context.Context, value studentpresence.PrivacyConsent) (studentpresence.PrivacyConsent, error) {
	row := studentpresence.PrivacyConsent{
		StudentID: value.StudentID, PolicyVersion: value.PolicyVersion, Accepted: value.Accepted,
		AcceptedAt: value.AcceptedAt, ExpiresAt: value.ExpiresAt, DurationDays: value.DurationDays,
		RenewalRequired: value.RenewalRequired, DataRetentionDays: value.DataRetentionDays, Details: value.Details,
	}
	err := s.run("record_privacy_consent", func() (ports.Stats, error) {
		if !validPrivacyConsent(&row) {
			return ports.Stats{}, studentpresence.ErrInvalidPrivacyConsent
		}
		if err := s.tx.Require(ctx); err != nil {
			return ports.Stats{}, err
		}
		return s.store.RecordPrivacyConsent(ctx, &row)
	})
	if err != nil {
		return studentpresence.PrivacyConsent{}, err
	}
	return row, nil
}

func (s *Service) ListAcceptedRetentionSettings(ctx context.Context) (rows []studentpresence.StudentRetentionSetting, err error) {
	err = s.runWrite(ctx, "list_accepted_retention_settings", func(ctx context.Context) (ports.Stats, error) {
		var stats ports.Stats
		rows, stats, err = s.store.ListAcceptedRetentionSettings(ctx)
		return stats, err
	})
	return listResult(rows, err)
}

func validPrivacyConsent(row *studentpresence.PrivacyConsent) bool {
	return row != nil && row.StudentID > 0 && strings.TrimSpace(row.PolicyVersion) != "" &&
		row.DataRetentionDays >= minPrivacyConsentRetentionDays && row.DataRetentionDays <= maxPrivacyConsentRetentionDays &&
		(row.ExpiresAt == nil || row.AcceptedAt == nil || !row.ExpiresAt.Before(*row.AcceptedAt))
}

// RevisePrivacyConsent revises one consent row inside the caller's tenant
// transaction; the store keeps the row's tenant and timestamps.
func (s *Service) RevisePrivacyConsent(ctx context.Context, value studentpresence.PrivacyConsent) (studentpresence.PrivacyConsent, error) {
	row := studentpresence.PrivacyConsent{
		ID: value.ID, StudentID: value.StudentID, PolicyVersion: value.PolicyVersion, Accepted: value.Accepted,
		AcceptedAt: value.AcceptedAt, ExpiresAt: value.ExpiresAt, DurationDays: value.DurationDays,
		RenewalRequired: value.RenewalRequired, DataRetentionDays: value.DataRetentionDays, Details: value.Details,
	}
	err := s.run("revise_privacy_consent", func() (ports.Stats, error) {
		if !validPrivacyConsent(&row) || row.ID <= 0 {
			return ports.Stats{}, studentpresence.ErrInvalidPrivacyConsent
		}
		if err := s.tx.Require(ctx); err != nil {
			return ports.Stats{}, err
		}
		return s.store.RevisePrivacyConsent(ctx, &row)
	})
	if err != nil {
		return studentpresence.PrivacyConsent{}, err
	}
	return row, nil
}
